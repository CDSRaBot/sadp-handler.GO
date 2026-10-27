package network

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

func TestSendInquiryProbeWritesOneCanonicalProbe(t *testing.T) {
	connection := &recordingPacketConn{}
	target := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 37020}

	err := SendInquiryProbe(context.Background(), connection, target)
	if err != nil {
		t.Fatalf("SendInquiryProbe returned error: %v", err)
	}
	if connection.writeCount != 1 {
		t.Fatalf("write count = %d, want 1", connection.writeCount)
	}
	if connection.target.String() != target.String() {
		t.Fatalf("target = %v, want %v", connection.target, target)
	}
	if !bytes.HasPrefix(connection.payload, []byte(`<?xml version="1.0" encoding="utf-8"?>`)) {
		t.Fatalf("probe lacks canonical XML declaration: %q", connection.payload)
	}

	var probe struct {
		XMLName      xml.Name `xml:"Probe"`
		UUID         string   `xml:"Uuid"`
		Types        string   `xml:"Types"`
		ResponseMode string   `xml:"ResponseMode"`
	}
	if err := xml.Unmarshal(connection.payload, &probe); err != nil {
		t.Fatalf("probe is not XML: %v", err)
	}
	if probe.XMLName.Local != "Probe" || probe.UUID == "" || probe.Types != "inquiry" || probe.ResponseMode != "2" {
		t.Fatalf("unexpected probe: %#v", probe)
	}
}

func TestSendInquiryProbeRejectsInvalidAndCanceledArguments(t *testing.T) {
	target := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 37020}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()

	testCases := []struct {
		name string
		ctx  context.Context
		conn net.PacketConn
		addr net.Addr
	}{
		{name: "nil context", conn: &recordingPacketConn{}, addr: target},
		{name: "nil connection", ctx: context.Background(), addr: target},
		{name: "nil target", ctx: context.Background(), conn: &recordingPacketConn{}},
		{name: "canceled context", ctx: canceled, conn: &recordingPacketConn{}, addr: target},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := SendInquiryProbe(testCase.ctx, testCase.conn, testCase.addr)
			assertTransportStage(t, err, TransportValidation)
			if testCase.name == "canceled context" && !errors.Is(err, context.Canceled) {
				t.Fatalf("error = %v, want context.Canceled", err)
			}
		})
	}
}

func TestSendSADPDatagramPreservesPayloadAndCallerOwnership(t *testing.T) {
	connection := &recordingPacketConn{}
	target := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 37020}
	payload := []byte("<Activate/>")

	err := SendSADPDatagram(context.Background(), connection, target, payload, len(payload))
	if err != nil {
		t.Fatalf("SendSADPDatagram returned error: %v", err)
	}
	if connection.writeCount != 1 || !bytes.Equal(connection.payload, payload) {
		t.Fatalf("write = %d, payload = %q", connection.writeCount, connection.payload)
	}
	if connection.closed {
		t.Fatal("SendSADPDatagram must not close caller-owned connection")
	}
}

func TestSendSADPDatagramRejectsInvalidPayloadAndShortWrites(t *testing.T) {
	target := &net.UDPAddr{IP: net.ParseIP("239.255.255.250"), Port: 37020}
	for _, testCase := range []struct {
		name    string
		conn    *recordingPacketConn
		payload []byte
		limit   int
		stage   TransportStage
	}{
		{name: "empty payload", conn: &recordingPacketConn{}, limit: 10, stage: TransportValidation},
		{name: "invalid limit", conn: &recordingPacketConn{}, payload: []byte("a"), limit: 0, stage: TransportValidation},
		{name: "oversized payload", conn: &recordingPacketConn{}, payload: []byte("ab"), limit: 1, stage: TransportValidation},
		{name: "short write", conn: &recordingPacketConn{writeN: 1}, payload: []byte("ab"), limit: 2, stage: TransportShortWrite},
		{name: "write failure", conn: &recordingPacketConn{writeErr: errors.New("write failed")}, payload: []byte("a"), limit: 1, stage: TransportWrite},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			err := SendSADPDatagram(context.Background(), testCase.conn, target, testCase.payload, testCase.limit)
			assertTransportStage(t, err, testCase.stage)
		})
	}
}

func TestHandleDatagramDispatchesSynchronouslyAndPreservesSender(t *testing.T) {
	sender := &net.UDPAddr{IP: net.ParseIP("192.168.1.20"), Port: 37020}
	payload := []byte("device advertisement")
	called := false

	err := handleDatagram(context.Background(), payload, sender, len(payload), func(_ context.Context, got []byte, gotSender net.Addr) error {
		called = true
		if !bytes.Equal(got, payload) || gotSender.String() != sender.String() {
			t.Fatal("handler did not receive original datagram and sender")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("handleDatagram returned error: %v", err)
	}
	if !called {
		t.Fatal("handler was not called before handleDatagram returned")
	}
}

func TestHandleDatagramRejectsInvalidInputAndWrapsHandlerError(t *testing.T) {
	sender := &net.UDPAddr{IP: net.ParseIP("192.168.1.20"), Port: 37020}
	for _, testCase := range []struct {
		name    string
		ctx     context.Context
		data    []byte
		sender  net.Addr
		limit   int
		handler PacketHandler
		stage   TransportStage
	}{
		{name: "nil context", data: []byte("a"), sender: sender, limit: 1, handler: func(context.Context, []byte, net.Addr) error { return nil }, stage: TransportValidation},
		{name: "nil sender", ctx: context.Background(), data: []byte("a"), limit: 1, handler: func(context.Context, []byte, net.Addr) error { return nil }, stage: TransportValidation},
		{name: "empty packet", ctx: context.Background(), sender: sender, limit: 1, handler: func(context.Context, []byte, net.Addr) error { return nil }, stage: TransportValidation},
		{name: "oversized packet", ctx: context.Background(), data: []byte("ab"), sender: sender, limit: 1, handler: func(context.Context, []byte, net.Addr) error { return nil }, stage: TransportValidation},
		{name: "handler failure", ctx: context.Background(), data: []byte("a"), sender: sender, limit: 1, handler: func(context.Context, []byte, net.Addr) error { return errors.New("handler failed") }, stage: TransportDispatch},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			assertTransportStage(t, handleDatagram(testCase.ctx, testCase.data, testCase.sender, testCase.limit, testCase.handler), testCase.stage)
		})
	}
}

func TestListenClosesTransferredConnectionOnCancellation(t *testing.T) {
	connection := &recordingPacketConn{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Listen(ctx, connection, DefaultMaxSADPDatagramSize, func(context.Context, []byte, net.Addr) error { return nil })
	if err != nil {
		t.Fatalf("Listen returned error for cancellation: %v", err)
	}
	if !connection.closed {
		t.Fatal("Listen must close its transferred connection")
	}
}

func TestListenCancellationUnblocksBlockedReadAndClosesOnce(t *testing.T) {
	connection := newBlockingPacketConn()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- Listen(ctx, connection, DefaultMaxSADPDatagramSize, func(context.Context, []byte, net.Addr) error {
			return nil
		})
	}()

	select {
	case <-connection.readStarted:
	case <-time.After(time.Second):
		t.Fatal("Listen did not enter ReadFrom")
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Listen returned error after cancellation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Listen did not stop after cancellation")
	}
	if !connection.closed {
		t.Fatal("Listen did not close the transferred connection")
	}
	if connection.closeCount != 1 {
		t.Fatalf("close count = %d, want 1", connection.closeCount)
	}
}

func TestListenPreservesCloseErrorAfterCancellation(t *testing.T) {
	closeErr := errors.New("close failed")
	connection := newBlockingPacketConn()
	connection.closeErr = closeErr
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- Listen(ctx, connection, DefaultMaxSADPDatagramSize, func(context.Context, []byte, net.Addr) error {
			return nil
		})
	}()
	select {
	case <-connection.readStarted:
	case <-time.After(time.Second):
		t.Fatal("Listen did not enter ReadFrom")
	}
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, closeErr) {
			t.Fatalf("error = %v, want close error %v", err, closeErr)
		}
	case <-time.After(time.Second):
		t.Fatal("Listen did not stop after cancellation")
	}
	if connection.closeCount != 1 {
		t.Fatalf("close count = %d, want 1", connection.closeCount)
	}
}

func TestListenWrapsReadAndCloseErrors(t *testing.T) {
	readErr := errors.New("read failed")
	connection := &recordingPacketConn{readErr: readErr}
	err := Listen(context.Background(), connection, DefaultMaxSADPDatagramSize, func(context.Context, []byte, net.Addr) error {
		return nil
	})
	assertTransportStage(t, err, TransportRead)
	if !errors.Is(err, readErr) {
		t.Fatalf("error = %v, want read cause %v", err, readErr)
	}

	closeErr := errors.New("close failed")
	connection = &recordingPacketConn{readErr: readErr, closeErr: closeErr}
	err = Listen(context.Background(), connection, DefaultMaxSADPDatagramSize, func(context.Context, []byte, net.Addr) error {
		return nil
	})
	if !errors.Is(err, readErr) || !errors.Is(err, closeErr) {
		t.Fatalf("error = %v, want read and close causes", err)
	}
}

func TestListenRejectsDatagramsAboveConfiguredLimit(t *testing.T) {
	connection := &recordingPacketConn{
		readN:      DefaultMaxSADPDatagramSize + 1,
		readSender: &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 37020},
	}
	err := Listen(context.Background(), connection, DefaultMaxSADPDatagramSize, func(context.Context, []byte, net.Addr) error {
		t.Fatal("oversized datagram must not reach the handler")
		return nil
	})
	assertTransportStage(t, err, TransportResponseTooLarge)
	if !connection.closed {
		t.Fatal("Listen must close after an oversized datagram")
	}
}

func TestListenClosesConnectionForInvalidArguments(t *testing.T) {
	testCases := []struct {
		name  string
		ctx   context.Context
		conn  *recordingPacketConn
		limit int
	}{
		{name: "nil context", conn: &recordingPacketConn{}, limit: DefaultMaxSADPDatagramSize},
		{name: "invalid limit", ctx: context.Background(), conn: &recordingPacketConn{}, limit: 0},
		{name: "nil handler", ctx: context.Background(), conn: &recordingPacketConn{}, limit: DefaultMaxSADPDatagramSize},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var handler PacketHandler
			if testCase.name != "nil handler" {
				handler = func(context.Context, []byte, net.Addr) error { return nil }
			}
			err := Listen(testCase.ctx, testCase.conn, testCase.limit, handler)
			if err == nil {
				t.Fatal("Listen accepted invalid arguments")
			}
			if !testCase.conn.closed {
				t.Fatal("Listen did not close the transferred connection")
			}
		})
	}
}

func assertTransportStage(t *testing.T, err error, want TransportStage) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	var transportErr *TransportError
	if !errors.As(err, &transportErr) {
		t.Fatalf("error = %T %v, want TransportError", err, err)
	}
	if transportErr.Stage != want {
		t.Fatalf("stage = %q, want %q", transportErr.Stage, want)
	}
}

type recordingPacketConn struct {
	mu            sync.Mutex
	payload       []byte
	target        net.Addr
	writeCount    int
	writeN        int
	writeErr      error
	closed        bool
	closeCount    int
	closeErr      error
	readErr       error
	readN         int
	readData      []byte
	readSender    net.Addr
	readStarted   chan struct{}
	readUnblock   chan struct{}
	readStartOnce sync.Once
	readCloseOnce sync.Once
}

func newBlockingPacketConn() *recordingPacketConn {
	return &recordingPacketConn{
		readStarted: make(chan struct{}),
		readUnblock: make(chan struct{}),
	}
}

func (c *recordingPacketConn) ReadFrom(buffer []byte) (int, net.Addr, error) {
	if c.readStarted != nil {
		c.readStartOnce.Do(func() { close(c.readStarted) })
		<-c.readUnblock
	}
	if c.readErr != nil {
		return 0, nil, c.readErr
	}
	if c.readN != 0 {
		copy(buffer, c.readData)
		return c.readN, c.readSender, nil
	}
	return 0, nil, errors.New("read not configured")
}

func (c *recordingPacketConn) WriteTo(payload []byte, target net.Addr) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeCount++
	c.payload = append([]byte(nil), payload...)
	c.target = target
	if c.writeErr != nil {
		return 0, c.writeErr
	}
	if c.writeN != 0 {
		return c.writeN, nil
	}
	return len(payload), nil
}

func (c *recordingPacketConn) Close() error {
	c.mu.Lock()
	c.closed = true
	c.closeCount++
	c.mu.Unlock()
	if c.readUnblock != nil {
		c.readCloseOnce.Do(func() { close(c.readUnblock) })
	}
	return c.closeErr
}

func (*recordingPacketConn) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (*recordingPacketConn) SetDeadline(time.Time) error      { return nil }
func (*recordingPacketConn) SetReadDeadline(time.Time) error  { return nil }
func (*recordingPacketConn) SetWriteDeadline(time.Time) error { return nil }
