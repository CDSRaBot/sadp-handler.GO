package adapter

import (
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestListInterfacesWithFiltersAndCopiesEligibleIPv4Interfaces(t *testing.T) {
	interfaces := []net.Interface{
		{Index: 1, Name: "loopback", Flags: net.FlagUp | net.FlagLoopback},
		{Index: 2, Name: "down", Flags: 0},
		{Index: 3, Name: "ipv6-only", Flags: net.FlagUp},
		{
			Index:        4,
			Name:         "ethernet",
			HardwareAddr: net.HardwareAddr{0, 1, 2, 3, 4, 5},
			Flags:        net.FlagUp,
		},
	}
	addresses := map[int][]net.Addr{
		1: {&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}},
		2: {&net.IPNet{IP: net.ParseIP("192.168.1.2"), Mask: net.CIDRMask(24, 32)}},
		3: {&net.IPNet{IP: net.ParseIP("2001:db8::1"), Mask: net.CIDRMask(64, 128)}},
		4: {
			&net.IPNet{IP: net.ParseIP("192.168.1.10"), Mask: net.CIDRMask(24, 32)},
			&net.IPNet{IP: net.ParseIP("10.0.0.10"), Mask: net.CIDRMask(16, 32)},
		},
	}

	got, err := listInterfacesWith(
		func() ([]net.Interface, error) { return interfaces, nil },
		func(iface net.Interface) ([]net.Addr, error) { return addresses[iface.Index], nil },
	)
	if err != nil {
		t.Fatalf("ListInterfaces returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("eligible interface count = %d, want 1", len(got))
	}
	if got[0].Index != 4 || got[0].Name != "ethernet" {
		t.Fatalf("interface identity = %#v, want ethernet index 4", got[0])
	}
	if len(got[0].IPv4Addrs) != 2 {
		t.Fatalf("IPv4 address count = %d, want 2", len(got[0].IPv4Addrs))
	}
	if got[0].IPv4Addrs[0] != netip.MustParsePrefix("192.168.1.10/24") {
		t.Fatalf("first IPv4 prefix = %s", got[0].IPv4Addrs[0])
	}

	got[0].HardwareAddr[0] = 255
	if interfaces[3].HardwareAddr[0] == 255 {
		t.Fatal("returned hardware address aliases the net.Interface buffer")
	}
}

func TestListInterfacesWithReturnsEmptyResultWithoutError(t *testing.T) {
	got, err := listInterfacesWith(
		func() ([]net.Interface, error) { return []net.Interface{}, nil },
		func(net.Interface) ([]net.Addr, error) { t.Fatal("address lookup must not be called"); return nil, nil },
	)
	if err != nil {
		t.Fatalf("ListInterfaces returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("interface count = %d, want 0", len(got))
	}
}

func TestListInterfacesWithPreservesEnumerationAndAddressErrors(t *testing.T) {
	t.Run("enumeration", func(t *testing.T) {
		want := errors.New("enumeration failed")
		_, err := listInterfacesWith(
			func() ([]net.Interface, error) { return nil, want },
			func(net.Interface) ([]net.Addr, error) { return nil, nil },
		)
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want wrapped %v", err, want)
		}
	})

	t.Run("address inspection", func(t *testing.T) {
		want := errors.New("address lookup failed")
		_, err := listInterfacesWith(
			func() ([]net.Interface, error) {
				return []net.Interface{{Index: 7, Name: "test", Flags: net.FlagUp}}, nil
			},
			func(net.Interface) ([]net.Addr, error) { return nil, want },
		)
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want wrapped %v", err, want)
		}
	})
}

func TestGetLinuxMACFromValidatesNameAndEUI48Address(t *testing.T) {
	t.Run("valid address", func(t *testing.T) {
		var path string
		got, err := getLinuxMACFrom("eth0", func(name string) ([]byte, error) {
			path = name
			return []byte("00:11:22:33:44:55\n"), nil
		})
		if err != nil {
			t.Fatalf("getLinuxMACFrom returned error: %v", err)
		}
		if got.String() != "00:11:22:33:44:55" {
			t.Fatalf("MAC = %s", got)
		}
		if path != "/sys/class/net/eth0/address" {
			t.Fatalf("path = %q", path)
		}
	})

	for _, name := range []string{"", ".", "..", "../eth0", `eth0\name`, "eth0\x00"} {
		t.Run("invalid name "+strings.ReplaceAll(name, "\x00", "NUL"), func(t *testing.T) {
			_, err := getLinuxMACFrom(name, func(string) ([]byte, error) {
				t.Fatal("readFile must not be called for an invalid name")
				return nil, nil
			})
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}

	for _, value := range []string{"not-a-mac", "00:11:22:33:44:55:66:77"} {
		t.Run("invalid address "+value, func(t *testing.T) {
			_, err := getLinuxMACFrom("eth0", func(string) ([]byte, error) {
				return []byte(value), nil
			})
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestGetLinuxMACFromWrapsReadFailureWithoutLeakingContents(t *testing.T) {
	want := errors.New("permission denied")
	_, err := getLinuxMACFrom("eth0", func(string) ([]byte, error) { return nil, want })
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want wrapped %v", err, want)
	}
}

func TestOpenMulticastWithValidatesArgumentsAndPreservesConnection(t *testing.T) {
	validInterface := Interface{
		Index:        5,
		Name:         "ethernet",
		HardwareAddr: net.HardwareAddr{0, 1, 2, 3, 4, 5},
		IPv4Addrs:    []netip.Prefix{netip.MustParsePrefix("192.168.1.10/24")},
		Flags:        net.FlagUp,
	}
	validGroup := netip.MustParseAddrPort("239.255.255.250:37020")

	for _, group := range []netip.AddrPort{
		netip.AddrPort{},
		netip.MustParseAddrPort("192.168.1.1:37020"),
		netip.MustParseAddrPort("239.255.255.250:0"),
	} {
		t.Run("invalid group "+group.String(), func(t *testing.T) {
			_, err := openMulticastWith(validInterface, group, func(Interface, netip.AddrPort) (net.PacketConn, error) {
				t.Fatal("open must not be called for invalid input")
				return nil, nil
			})
			var adapterErr *AdapterError
			if !errors.As(err, &adapterErr) || adapterErr.Kind != AdapterInvalidInterface {
				t.Fatalf("error = %T %v, want invalid-interface AdapterError", err, err)
			}
		})
	}

	t.Run("successful configured connection", func(t *testing.T) {
		connection := &fakePacketConn{}
		got, err := openMulticastWith(validInterface, validGroup, func(iface Interface, group netip.AddrPort) (net.PacketConn, error) {
			if iface.Name != validInterface.Name || group != validGroup {
				t.Fatal("interface or group was not preserved")
			}
			return connection, nil
		})
		if err != nil {
			t.Fatalf("OpenMulticast returned error: %v", err)
		}
		if got != connection {
			t.Fatal("OpenMulticast did not return the configured connection")
		}
	})

	t.Run("open failure", func(t *testing.T) {
		want := errors.New("join failed")
		_, err := openMulticastWith(validInterface, validGroup, func(Interface, netip.AddrPort) (net.PacketConn, error) {
			return nil, want
		})
		if !errors.Is(err, want) {
			t.Fatalf("error = %v, want wrapped %v", err, want)
		}
	})
}

type fakePacketConn struct{}

func (*fakePacketConn) ReadFrom([]byte) (int, net.Addr, error) {
	return 0, nil, errors.New("not implemented")
}
func (*fakePacketConn) WriteTo([]byte, net.Addr) (int, error) {
	return 0, errors.New("not implemented")
}
func (*fakePacketConn) Close() error                     { return nil }
func (*fakePacketConn) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (*fakePacketConn) SetDeadline(time.Time) error      { return nil }
func (*fakePacketConn) SetReadDeadline(time.Time) error  { return nil }
func (*fakePacketConn) SetWriteDeadline(time.Time) error { return nil }
