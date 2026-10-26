package adapter

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path"
	"strings"

	"golang.org/x/net/ipv4"
)

type Interface struct {
	Index        int
	Name         string
	HardwareAddr net.HardwareAddr
	IPv4Addrs    []netip.Prefix
	Flags        net.Flags
}

type MulticastOptions struct {
	Group netip.AddrPort
	TTL   int
}

type AdapterErrorKind string

const (
	AdapterInvalidInterface    AdapterErrorKind = "invalid_interface"
	AdapterInvalidMulticast    AdapterErrorKind = "invalid_multicast"
	AdapterBindFailed          AdapterErrorKind = "bind_failed"
	AdapterPermissionDenied    AdapterErrorKind = "permission_denied"
	AdapterUnsupportedPlatform AdapterErrorKind = "unsupported_platform"
)

type AdapterError struct {
	Kind AdapterErrorKind
	Err  error
}

func (e *AdapterError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.Kind)
	}
	return string(e.Kind) + ": " + e.Err.Error()
}

func (e *AdapterError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ListInterfaces() ([]Interface, error) {
	return listInterfacesWith(net.Interfaces, func(iface net.Interface) ([]net.Addr, error) {
		return iface.Addrs()
	})
}

func listInterfacesWith(
	enumerate func() ([]net.Interface, error),
	addresses func(net.Interface) ([]net.Addr, error),
) ([]Interface, error) {
	interfaces, err := enumerate()
	if err != nil {
		return nil, fmt.Errorf("enumerate network interfaces: %w", err)
	}

	result := make([]Interface, 0, len(interfaces))
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := addresses(iface)
		if err != nil {
			return nil, fmt.Errorf("inspect addresses for interface %q: %w", iface.Name, err)
		}

		ipv4 := make([]netip.Prefix, 0, len(addrs))
		for _, addr := range addrs {
			prefix, ok := ipv4Prefix(addr)
			if ok {
				ipv4 = append(ipv4, prefix)
			}
		}
		if len(ipv4) == 0 {
			continue
		}

		hardware := append(net.HardwareAddr(nil), iface.HardwareAddr...)
		result = append(result, Interface{
			Index:        iface.Index,
			Name:         iface.Name,
			HardwareAddr: hardware,
			IPv4Addrs:    ipv4,
			Flags:        iface.Flags,
		})
	}
	return result, nil
}

func OpenMulticast(iface Interface, options MulticastOptions) (net.PacketConn, error) {
	return openMulticastWith(iface, options, openMulticast)
}

func openMulticastWith(
	iface Interface,
	options MulticastOptions,
	open func(Interface, MulticastOptions) (net.PacketConn, error),
) (net.PacketConn, error) {
	if !eligibleInterface(iface) {
		return nil, &AdapterError{Kind: AdapterInvalidInterface, Err: errors.New("interface is not eligible")}
	}
	if !options.Group.IsValid() || !options.Group.Addr().Is4() || !options.Group.Addr().IsMulticast() ||
		options.Group.Port() == 0 || options.TTL < 1 || options.TTL > 255 {
		return nil, &AdapterError{Kind: AdapterInvalidMulticast, Err: errors.New("group must be an IPv4 multicast address with a non-zero port")}
	}

	connection, err := open(iface, options)
	if err != nil {
		kind := AdapterBindFailed
		if errors.Is(err, os.ErrPermission) {
			kind = AdapterPermissionDenied
		}
		return nil, &AdapterError{Kind: kind, Err: fmt.Errorf("open multicast on interface %q: %w", iface.Name, err)}
	}
	if connection == nil {
		return nil, &AdapterError{Kind: AdapterBindFailed, Err: errors.New("multicast opener returned a nil connection")}
	}
	return connection, nil
}

func getLinuxMAC(interfaceName string) (net.HardwareAddr, error) {
	return getLinuxMACFrom(interfaceName, os.ReadFile)
}

func getLinuxMACFrom(
	interfaceName string,
	readFile func(string) ([]byte, error),
) (net.HardwareAddr, error) {
	if interfaceName == "" || interfaceName == "." || interfaceName == ".." ||
		strings.ContainsAny(interfaceName, `/\`) || strings.ContainsRune(interfaceName, 0) {
		return nil, fmt.Errorf("invalid interface name")
	}

	sysfsPath := path.Join("/sys/class/net", interfaceName, "address")
	data, err := readFile(sysfsPath)
	if err != nil {
		return nil, fmt.Errorf("read Linux MAC for interface %q: %w", interfaceName, err)
	}
	hardware, err := net.ParseMAC(strings.TrimSpace(string(data)))
	if err != nil || len(hardware) != 6 {
		if err == nil {
			err = errors.New("address is not EUI-48")
		}
		return nil, fmt.Errorf("parse Linux MAC for interface %q: %w", interfaceName, err)
	}
	return append(net.HardwareAddr(nil), hardware...), nil
}

func ipv4Prefix(addr net.Addr) (netip.Prefix, bool) {
	switch value := addr.(type) {
	case *net.IPNet:
		ip := value.IP.To4()
		if ip == nil || ip.IsUnspecified() {
			return netip.Prefix{}, false
		}
		parsed, ok := netip.AddrFromSlice(ip)
		if !ok {
			return netip.Prefix{}, false
		}
		ones, bits := value.Mask.Size()
		if bits != 32 || ones < 0 {
			return netip.Prefix{}, false
		}
		return netip.PrefixFrom(parsed, ones), true
	case *net.IPAddr:
		ip := value.IP.To4()
		if ip == nil || ip.IsUnspecified() {
			return netip.Prefix{}, false
		}
		parsed, ok := netip.AddrFromSlice(ip)
		if !ok {
			return netip.Prefix{}, false
		}
		return netip.PrefixFrom(parsed, 32), true
	default:
		return netip.Prefix{}, false
	}
}

func eligibleInterface(iface Interface) bool {
	if iface.Index <= 0 || iface.Name == "" || iface.Flags&net.FlagUp == 0 ||
		iface.Flags&net.FlagLoopback != 0 || len(iface.IPv4Addrs) == 0 {
		return false
	}
	for _, prefix := range iface.IPv4Addrs {
		if prefix.IsValid() && prefix.Addr().Is4() && !prefix.Addr().IsUnspecified() {
			return true
		}
	}
	return false
}

func openMulticast(iface Interface, options MulticastOptions) (net.PacketConn, error) {
	address := &net.UDPAddr{
		IP:   net.IP(options.Group.Addr().AsSlice()),
		Port: int(options.Group.Port()),
	}
	connection, err := net.ListenMulticastUDP("udp4", &net.Interface{Index: iface.Index, Name: iface.Name}, address)
	if err != nil {
		return nil, err
	}
	if err := ipv4.NewPacketConn(connection).SetMulticastTTL(options.TTL); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("set multicast TTL: %w", err)
	}
	return connection, nil
}
