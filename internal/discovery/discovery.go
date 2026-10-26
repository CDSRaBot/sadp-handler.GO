package discovery

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/network"
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

func DiscoverOnInterface(ctx context.Context, iface adapter.Interface, options DiscoveryOptions, handle EventHandler) error {
	return discoverWith(ctx, iface, options, handle, productionDeps(options))
}

func discoverWith(ctx context.Context, iface adapter.Interface, options DiscoveryOptions, handle EventHandler, deps discoveryDeps) (result error) {
	if err := validateInputs(ctx, iface, options, handle, deps); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	conn, err := deps.open(iface, options.Multicast)
	if err != nil {
		return fmt.Errorf("open discovery multicast: %w", err)
	}
	var closeOnce sync.Once
	var closeErr error
	listenerStarted := false
	closeConn := func() error {
		closeOnce.Do(func() {
			closeErr = conn.Close()
		})
		return closeErr
	}
	defer func() {
		err := closeConn()
		if err != nil && !listenerStarted {
			if result == nil {
				result = fmt.Errorf("close discovery multicast: %w", err)
			} else {
				result = errors.Join(result, fmt.Errorf("close discovery multicast: %w", err))
			}
		}
	}()
	if err := deps.send(conn); err != nil {
		return errors.Join(fmt.Errorf("send initial inquiry probe: %w", err), closeConn())
	}

	listenCtx, cancelListen := context.WithCancel(ctx)
	defer cancelListen()
	events := func(eventCtx context.Context, event Event) error {
		if event.ObservedAt.IsZero() {
			event.ObservedAt = time.Now()
		}
		return handle(eventCtx, event)
	}
	listenDone := make(chan error, 1)
	listenerStarted = true
	go func() {
		listenDone <- deps.listen(listenCtx, conn, events, options.MaxSADPDatagramSize)
	}()

	var listenerErr error
	var listenerObserved bool
	if options.ProbeInterval > 0 {
		result = runContinuous(ctx, options, conn, deps.send, listenDone, &listenerErr, &listenerObserved)
	} else {
		result = runBounded(ctx, options, conn, deps.send, listenDone, &listenerErr, &listenerObserved)
	}
	cancelListen()
	if !listenerObserved {
		listenerErr = <-listenDone
	}
	if result != nil {
		return errors.Join(result, normalizeListenerError(listenerErr, ctx))
	}
	if err := normalizeListenerError(listenerErr, ctx); err != nil {
		return err
	}
	return nil
}

func productionDeps(options DiscoveryOptions) discoveryDeps {
	return discoveryDeps{
		open: func(iface adapter.Interface, options adapter.MulticastOptions) (net.PacketConn, error) {
			return adapter.OpenMulticast(iface, options.Group)
		},
		send: func(conn net.PacketConn) error {
			target := &net.UDPAddr{
				IP:   net.IP(options.Multicast.Group.Addr().AsSlice()),
				Port: int(options.Multicast.Group.Port()),
			}
			return network.SendInquiryProbe(context.Background(), conn, target)
		},
		listen: func(ctx context.Context, conn net.PacketConn, handle EventHandler, maxSize int) error {
			return network.Listen(ctx, conn, maxSize, func(handlerCtx context.Context, data []byte, sender net.Addr) error {
				device, err := parser.ParseAdvertisement(data, sender, maxSize)
				event := Event{Sender: sender, ObservedAt: time.Now()}
				if err != nil {
					event.Kind = DatagramRejected
					event.Err = err
				} else {
					event.Kind = DeviceFound
					event.Device = &device
				}
				return handle(handlerCtx, event)
			})
		},
		parse: parser.ParseAdvertisement,
	}
}

func validateInputs(ctx context.Context, iface adapter.Interface, options DiscoveryOptions, handle EventHandler, deps discoveryDeps) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	if handle == nil {
		return errors.New("event handler is nil")
	}
	if deps.open == nil || deps.send == nil || deps.listen == nil || deps.parse == nil {
		return errors.New("discovery dependencies are incomplete")
	}
	if !eligibleInterface(iface) {
		return errors.New("interface is not eligible")
	}
	if !options.Multicast.Group.IsValid() || !options.Multicast.Group.Addr().Is4() ||
		!options.Multicast.Group.Addr().IsMulticast() || options.Multicast.Group.Port() == 0 ||
		options.Multicast.TTL < 1 || options.Multicast.TTL > 255 {
		return errors.New("multicast options are invalid")
	}
	if options.MaxSADPDatagramSize < 1 || options.MaxSADPDatagramSize > 65507 {
		return errors.New("max datagram size must be between 1 and 65507")
	}
	continuous := options.ProbeInterval > 0 && options.CycleTimeout == 0 && options.RepeatCount == 0
	bounded := options.ProbeInterval == 0 && options.CycleTimeout > 0 && options.RepeatCount >= 1 && options.RepeatCount <= 100
	if !continuous && !bounded {
		return errors.New("timing policy is invalid")
	}
	return nil
}

func eligibleInterface(iface adapter.Interface) bool {
	if iface.Index <= 0 || iface.Name == "" || iface.Flags&net.FlagUp == 0 ||
		iface.Flags&net.FlagLoopback != 0 {
		return false
	}
	for _, prefix := range iface.IPv4Addrs {
		if prefix.IsValid() && prefix.Addr().Is4() && !prefix.Addr().IsUnspecified() {
			return true
		}
	}
	return false
}

func runContinuous(ctx context.Context, options DiscoveryOptions, conn net.PacketConn, send func(net.PacketConn) error, listenDone <-chan error, listenerErr *error, listenerObserved *bool) error {
	ticker := time.NewTicker(options.ProbeInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-listenDone:
			*listenerErr = err
			*listenerObserved = true
			if err != nil {
				return fmt.Errorf("listen: %w", err)
			}
			listenDone = nil
		case <-ticker.C:
			if err := send(conn); err != nil {
				return fmt.Errorf("send scheduled inquiry probe: %w", err)
			}
		}
	}
}

func runBounded(ctx context.Context, options DiscoveryOptions, conn net.PacketConn, send func(net.PacketConn) error, listenDone <-chan error, listenerErr *error, listenerObserved *bool) error {
	for cycle := 0; cycle < options.RepeatCount; cycle++ {
		timer := time.NewTimer(options.CycleTimeout)
		listening := listenDone
		for {
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				return nil
			case err := <-listening:
				listening = nil
				*listenerErr = err
				*listenerObserved = true
				if err != nil {
					if !timer.Stop() {
						<-timer.C
					}
					return fmt.Errorf("listen: %w", err)
				}
			case <-timer.C:
				if cycle+1 < options.RepeatCount {
					if err := send(conn); err != nil {
						return fmt.Errorf("send scheduled inquiry probe: %w", err)
					}
				}
				goto nextCycle
			}
		}
	nextCycle:
	}
	return nil
}

func normalizeListenerError(err error, sessionCtx context.Context) error {
	if err == nil || (sessionCtx.Err() != nil && errors.Is(err, context.Canceled)) {
		return nil
	}
	return fmt.Errorf("listen: %w", err)
}
