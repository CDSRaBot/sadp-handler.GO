package network

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

func mutationTransportTarget() logic.MutationTarget {
	return logic.MutationTarget{
		MAC:    net.HardwareAddr{0, 17, 34, 51, 68, 85},
		Sender: &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 37020},
	}
}

func TestSADPMutationTransportExchangesOneBoundedDatagram(t *testing.T) {
	conn := &recordingPacketConn{
		readN:      len("accepted"),
		readSender: &net.UDPAddr{IP: net.ParseIP("192.0.2.10"), Port: 37020},
	}
	copyRead := []byte("accepted")
	conn.readData = copyRead
	transport := SADPMutationTransport{
		MaxDatagramSize: 64,
		Open:            func() (net.PacketConn, error) { return conn, nil },
	}

	exchange, err := transport.Exchange(context.Background(), mutationTransportTarget(), []byte("<Activate/>"))
	if err != nil {
		t.Fatalf("Exchange() error = %v", err)
	}
	if string(exchange.Data) != "accepted" || exchange.Sender.String() != conn.readSender.String() {
		t.Fatalf("unexpected exchange: %#v", exchange)
	}
	if conn.writeCount != 1 || string(conn.payload) != "<Activate/>" {
		t.Fatalf("write count/payload = %d/%q", conn.writeCount, conn.payload)
	}
	if !conn.closed || conn.closeCount != 1 {
		t.Fatalf("connection cleanup = closed:%t count:%d", conn.closed, conn.closeCount)
	}
}

func TestSADPMutationTransportRejectsOversizedPayloadAndResponse(t *testing.T) {
	target := mutationTransportTarget()
	payloadConn := &recordingPacketConn{}
	transport := SADPMutationTransport{
		MaxDatagramSize: 4,
		Open:            func() (net.PacketConn, error) { return payloadConn, nil },
	}
	_, err := transport.Exchange(context.Background(), target, []byte("12345"))
	assertTransportStage(t, err, TransportValidation)
	if payloadConn.writeCount != 0 {
		t.Fatal("oversized payload opened transport")
	}

	responseConn := &recordingPacketConn{
		readN:      5,
		readData:   []byte("12345"),
		readSender: target.Sender,
	}
	transport.Open = func() (net.PacketConn, error) { return responseConn, nil }
	_, err = transport.Exchange(context.Background(), target, []byte("1234"))
	assertTransportStage(t, err, TransportResponseTooLarge)
}

func TestSADPMutationTransportCancellationClosesOwnedConnection(t *testing.T) {
	conn := newBlockingPacketConn()
	ctx, cancel := context.WithCancel(context.Background())
	transport := SADPMutationTransport{
		MaxDatagramSize: 64,
		Open:            func() (net.PacketConn, error) { return conn, nil },
	}
	done := make(chan error, 1)
	go func() {
		_, err := transport.Exchange(ctx, mutationTransportTarget(), []byte("payload"))
		done <- err
	}()
	select {
	case <-conn.readStarted:
	case <-time.After(time.Second):
		t.Fatal("Exchange() did not reach read")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Exchange() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Exchange() did not stop after cancellation")
	}
	if conn.closeCount != 1 {
		t.Fatalf("close count = %d, want 1", conn.closeCount)
	}
}

func TestSADPMutationTransportPreservesCloseFailure(t *testing.T) {
	closeErr := errors.New("close failed")
	conn := &recordingPacketConn{
		readN:      1,
		readData:   []byte("x"),
		readSender: mutationTransportTarget().Sender,
		closeErr:   closeErr,
	}
	transport := SADPMutationTransport{
		MaxDatagramSize: 8,
		Open:            func() (net.PacketConn, error) { return conn, nil },
	}
	_, err := transport.Exchange(context.Background(), mutationTransportTarget(), []byte("x"))
	if !errors.Is(err, closeErr) {
		t.Fatalf("Exchange() error = %v, want close failure", err)
	}
}

func TestClassifyMutationTransportErrorPreservesIOCause(t *testing.T) {
	cause := errors.New("write failed")
	err := classifyMutationTransportError(context.Background(), TransportWrite, cause)

	if !errors.Is(err, cause) {
		t.Fatalf("classified error = %v, want original cause", err)
	}
	if got := err.Error(); got != "write: mutation socket I/O failed: write failed" {
		t.Fatalf("classified error = %q, want contextual message", got)
	}
}
