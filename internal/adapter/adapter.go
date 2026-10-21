package adapter

import (
	"errors"
	"net"
	"net/netip"
)

var errNotImplemented = errors.New("not implemented")

type Interface struct {
	Index        int
	Name         string
	HardwareAddr net.HardwareAddr
	IPv4Addrs    []netip.Prefix
	Flags        net.Flags
}

type AdapterErrorKind string

const (
	AdapterInvalidInterface    AdapterErrorKind = "invalid_interface"
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
	return nil, errNotImplemented
}

func listInterfacesWith(
	enumerate func() ([]net.Interface, error),
	addresses func(net.Interface) ([]net.Addr, error),
) ([]Interface, error) {
	return nil, errNotImplemented
}

func OpenMulticast(iface Interface, group netip.AddrPort) (net.PacketConn, error) {
	return nil, errNotImplemented
}

func openMulticastWith(
	iface Interface,
	group netip.AddrPort,
	open func(Interface, netip.AddrPort) (net.PacketConn, error),
) (net.PacketConn, error) {
	return nil, errNotImplemented
}

func getLinuxMAC(interfaceName string) (net.HardwareAddr, error) {
	return nil, errNotImplemented
}

func getLinuxMACFrom(
	interfaceName string,
	readFile func(string) ([]byte, error),
) (net.HardwareAddr, error) {
	return nil, errNotImplemented
}
