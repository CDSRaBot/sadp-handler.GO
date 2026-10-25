package discovery

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

type EventKind uint8

const (
	DeviceFound EventKind = iota + 1
	DatagramRejected
)

type Event struct {
	Kind       EventKind
	Device     *parser.Device
	Sender     net.Addr
	ObservedAt time.Time
	Err        error
}

type EventHandler func(context.Context, Event) error

type DiscoveryOptions struct {
	Multicast           adapter.MulticastOptions
	ProbeInterval       time.Duration
	CycleTimeout        time.Duration
	RepeatCount         int
	MaxSADPDatagramSize int
}

type discoveryDeps struct {
	open   func(adapter.Interface, adapter.MulticastOptions) (net.PacketConn, error)
	send   func(net.PacketConn) error
	listen func(context.Context, net.PacketConn, EventHandler, int) error
	parse  func([]byte, net.Addr, int) (parser.Device, error)
}

func DiscoverOnInterface(context.Context, adapter.Interface, DiscoveryOptions, EventHandler) error {
	return errors.New("not implemented")
}

func discoverWith(context.Context, adapter.Interface, DiscoveryOptions, EventHandler, discoveryDeps) error {
	return errors.New("not implemented")
}
