package discovery

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
)

func validInterface() adapter.Interface {
	return adapter.Interface{
		Index: 1, Name: "ethernet", HardwareAddr: net.HardwareAddr{0, 1, 2, 3, 4, 5},
		IPv4Addrs: []netip.Prefix{netip.MustParsePrefix("192.0.2.10/24")},
		Flags:     net.FlagUp,
	}
}

func validOptions() DiscoveryOptions {
	return DiscoveryOptions{
		Multicast: adapter.MulticastOptions{
			Group: netip.MustParseAddrPort("239.255.255.250:37020"),
			TTL:   64,
		},
		ProbeInterval:       time.Second,
		MaxSADPDatagramSize: 1500,
	}
}

func fakeDeps() discoveryDeps {
	return discoveryDeps{
		open: func(adapter.Interface, adapter.MulticastOptions) (net.PacketConn, error) {
			return &fakePacketConn{}, nil
		},
		send:   func(net.PacketConn) error { return nil },
		listen: func(context.Context, net.PacketConn, EventHandler, int) error { return nil },
		parse: func(data []byte, sender net.Addr, _ int) (parser.Device, error) {
			return parser.Device{Sender: sender}, nil
		},
	}
}

func TestDiscoverOnInterfaceRejectsInvalidInputsBeforeOpening(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		iface   adapter.Interface
		options DiscoveryOptions
		handler EventHandler
	}{
		{"nil context", nil, validInterface(), validOptions(), func(context.Context, Event) error { return nil }},
		{"nil handler", context.Background(), validInterface(), validOptions(), nil},
		{"ineligible interface", context.Background(), adapter.Interface{}, validOptions(), func(context.Context, Event) error { return nil }},
		{"missing timing policy", context.Background(), validInterface(), DiscoveryOptions{MaxSADPDatagramSize: 1500}, func(context.Context, Event) error { return nil }},
		{"mixed timing policy", context.Background(), validInterface(), DiscoveryOptions{ProbeInterval: time.Second, CycleTimeout: time.Second, MaxSADPDatagramSize: 1500}, func(context.Context, Event) error { return nil }},
		{"invalid repeat count", context.Background(), validInterface(), DiscoveryOptions{CycleTimeout: time.Second, RepeatCount: 101, MaxSADPDatagramSize: 1500}, func(context.Context, Event) error { return nil }},
		{"invalid datagram size", context.Background(), validInterface(), DiscoveryOptions{ProbeInterval: time.Second, MaxSADPDatagramSize: 65508}, func(context.Context, Event) error { return nil }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			opened := false
			deps := fakeDeps()
			deps.open = func(adapter.Interface, adapter.MulticastOptions) (net.PacketConn, error) {
				opened = true
				return &fakePacketConn{}, nil
			}
			err := discoverWith(tc.ctx, tc.iface, tc.options, tc.handler, deps)
			if err == nil {
				t.Fatal("discoverWith returned nil for invalid input")
			}
			if opened {
				t.Fatal("socket opened before validation completed")
			}
		})
	}
}

func TestDiscoverWithBoundedModeUsesFullCyclesAndContinuesAfterRejectedDatagram(t *testing.T) {
	options := validOptions()
	options.ProbeInterval = 0
	options.CycleTimeout = 10 * time.Millisecond
	options.RepeatCount = 2
	var events []Event
	deps := fakeDeps()
	deps.listen = func(_ context.Context, _ net.PacketConn, handle EventHandler, _ int) error {
		sender := &net.UDPAddr{IP: net.ParseIP("192.0.2.20"), Port: 37020}
		if err := handle(context.Background(), Event{Kind: DatagramRejected, Sender: sender, Err: errors.New("parse failed")}); err != nil {
			return err
		}
		if err := handle(context.Background(), Event{Kind: DeviceFound, Device: &parser.Device{Sender: sender}, Sender: sender}); err != nil {
			return err
		}
		return nil
	}
	handler := func(_ context.Context, event Event) error {
		events = append(events, event)
		return nil
	}
	if err := discoverWith(context.Background(), validInterface(), options, handler, deps); err != nil {
		t.Fatalf("discoverWith returned error: %v", err)
	}
	if len(events) != 2 || events[0].Kind != DatagramRejected || events[1].Kind != DeviceFound {
		t.Fatalf("events = %#v, want rejected then device", events)
	}
}

func TestDiscoverWithBoundedModeSendsEachProbeAfterFullReceiveWindow(t *testing.T) {
	options := validOptions()
	options.ProbeInterval = 0
	options.CycleTimeout = 20 * time.Millisecond
	options.RepeatCount = 2
	sendCount := 0
	deps := fakeDeps()
	deps.send = func(net.PacketConn) error {
		sendCount++
		return nil
	}
	deps.listen = func(ctx context.Context, _ net.PacketConn, _ EventHandler, _ int) error {
		<-ctx.Done()
		return nil
	}

	started := time.Now()
	if err := discoverWith(context.Background(), validInterface(), options, func(context.Context, Event) error {
		return nil
	}, deps); err != nil {
		t.Fatalf("discoverWith returned error: %v", err)
	}
	if sendCount != options.RepeatCount {
		t.Fatalf("send count = %d, want %d", sendCount, options.RepeatCount)
	}
	if elapsed := time.Since(started); elapsed < options.CycleTimeout*2 {
		t.Fatalf("session duration = %s, want at least %s for full receive windows", elapsed, options.CycleTimeout*2)
	}
}

func TestDiscoverWithContinuousModeSendsUntilCancellation(t *testing.T) {
	options := validOptions()
	options.ProbeInterval = 10 * time.Millisecond
	sendObserved := make(chan int, 8)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	deps := fakeDeps()
	sendCount := 0
	deps.send = func(net.PacketConn) error {
		sendCount++
		sendObserved <- sendCount
		return nil
	}
	deps.listen = func(ctx context.Context, _ net.PacketConn, _ EventHandler, _ int) error {
		<-ctx.Done()
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- discoverWith(ctx, validInterface(), options, func(context.Context, Event) error {
			return nil
		}, deps)
	}()
	for want := 1; want <= 2; want++ {
		select {
		case got := <-sendObserved:
			if got != want {
				t.Fatalf("send sequence = %d, want %d", got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("did not observe send %d before timeout", want)
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("discoverWith returned error after cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("discoverWith did not stop after cancellation")
	}
}

func TestDiscoverWithScheduledSendFailureCancelsSession(t *testing.T) {
	options := validOptions()
	options.ProbeInterval = 10 * time.Millisecond
	sendFailure := errors.New("scheduled send failed")
	sendObserved := make(chan int, 4)
	deps := fakeDeps()
	sendCount := 0
	deps.send = func(net.PacketConn) error {
		sendCount++
		sendObserved <- sendCount
		if sendCount == 2 {
			return sendFailure
		}
		return nil
	}
	deps.listen = func(ctx context.Context, _ net.PacketConn, _ EventHandler, _ int) error {
		<-ctx.Done()
		return nil
	}

	done := make(chan error, 1)
	go func() {
		done <- discoverWith(context.Background(), validInterface(), options, func(context.Context, Event) error {
			return nil
		}, deps)
	}()
	select {
	case got := <-sendObserved:
		if got != 1 {
			t.Fatalf("initial send sequence = %d, want 1", got)
		}
	case <-time.After(time.Second):
		t.Fatal("did not observe initial send before timeout")
	}
	select {
	case err := <-done:
		if err == nil || !errors.Is(err, sendFailure) {
			t.Fatalf("error = %v, want scheduled send failure %v", err, sendFailure)
		}
	case <-time.After(time.Second):
		t.Fatal("discoverWith did not stop after scheduled send failure")
	}
	if sendCount != 2 {
		t.Fatalf("send count = %d, want 2", sendCount)
	}
}

func TestDiscoverWithPreservesCancellationAndCallbackErrorsWithoutPayloads(t *testing.T) {
	t.Run("callback error", func(t *testing.T) {
		want := errors.New("callback failed")
		deps := fakeDeps()
		deps.listen = func(_ context.Context, _ net.PacketConn, handle EventHandler, _ int) error {
			return handle(context.Background(), Event{Kind: DatagramRejected, Err: errors.New("secret XML payload")})
		}
		err := discoverWith(context.Background(), validInterface(), validOptions(), func(_ context.Context, event Event) error {
			if event.Err == nil || event.Device != nil {
				t.Fatalf("rejected event = %#v", event)
			}
			return want
		}, deps)
		if err == nil || !errors.Is(err, want) {
			t.Fatalf("error = %v, want callback error %v", err, want)
		}
		if strings.Contains(err.Error(), "secret XML payload") {
			t.Fatalf("error leaked datagram payload: %v", err)
		}
	})

	t.Run("cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := discoverWith(ctx, validInterface(), validOptions(), func(context.Context, Event) error {
			t.Fatal("handler called after cancellation")
			return nil
		}, fakeDeps()); err != nil && !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation error = %v, want nil or context.Canceled", err)
		}
	})
}

func TestDiscoverWithClosesOpenedConnectionAfterListenerStops(t *testing.T) {
	connection := &closeTrackingPacketConn{}
	options := validOptions()
	options.ProbeInterval = 0
	options.CycleTimeout = time.Millisecond
	options.RepeatCount = 1
	deps := fakeDeps()
	deps.open = func(adapter.Interface, adapter.MulticastOptions) (net.PacketConn, error) {
		return connection, nil
	}
	deps.listen = func(ctx context.Context, _ net.PacketConn, _ EventHandler, _ int) error {
		<-ctx.Done()
		return nil
	}

	if err := discoverWith(context.Background(), validInterface(), options, func(context.Context, Event) error {
		return nil
	}, deps); err != nil {
		t.Fatalf("discoverWith returned error: %v", err)
	}
	if connection.closeCount != 1 {
		t.Fatalf("close count = %d, want 1", connection.closeCount)
	}
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

type closeTrackingPacketConn struct {
	closeCount int
}

func (*closeTrackingPacketConn) ReadFrom([]byte) (int, net.Addr, error) {
	return 0, nil, errors.New("read not configured")
}
func (*closeTrackingPacketConn) WriteTo(payload []byte, _ net.Addr) (int, error) {
	return len(payload), nil
}
func (c *closeTrackingPacketConn) Close() error {
	c.closeCount++
	return nil
}
func (*closeTrackingPacketConn) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (*closeTrackingPacketConn) SetDeadline(time.Time) error      { return nil }
func (*closeTrackingPacketConn) SetReadDeadline(time.Time) error  { return nil }
func (*closeTrackingPacketConn) SetWriteDeadline(time.Time) error { return nil }
