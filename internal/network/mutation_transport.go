package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

const DefaultMutationExchangeTimeout = 2 * time.Second

// SADPMutationTransport performs one bounded, owned SADP datagram exchange.
// The response remains opaque; operation-specific parsing belongs to M3.
type SADPMutationTransport struct {
	MaxDatagramSize int
	DefaultTimeout  time.Duration
	Open            func() (net.PacketConn, error)
}

func (t SADPMutationTransport) Exchange(ctx context.Context, target logic.MutationTarget, payload []byte) (result logic.MutationExchange, resultErr error) {
	if ctx == nil || len(target.MAC) != 6 || target.Sender == nil || len(payload) == 0 {
		return result, &TransportError{Stage: TransportValidation, Err: errors.New("context, target, sender, and payload are required")}
	}
	if err := ctx.Err(); err != nil {
		return result, &TransportError{Stage: TransportCanceled, Err: err}
	}
	limit := t.MaxDatagramSize
	if limit == 0 {
		limit = DefaultMaxSADPDatagramSize
	}
	if !validDatagramSize(limit) {
		return result, &TransportError{Stage: TransportValidation, Err: errors.New("datagram size must be between 1 and 65507")}
	}
	if len(payload) > limit {
		return result, &TransportError{Stage: TransportValidation, Err: errors.New("payload exceeds datagram size")}
	}
	timeout := t.DefaultTimeout
	if timeout == 0 {
		timeout = DefaultMutationExchangeTimeout
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline && timeout <= 0 {
		return result, &TransportError{Stage: TransportValidation, Err: errors.New("exchange timeout must be positive")}
	}

	open := t.Open
	if open == nil {
		open = func() (net.PacketConn, error) {
			return net.ListenUDP("udp4", &net.UDPAddr{})
		}
	}
	conn, err := open()
	if err != nil {
		return result, &TransportError{Stage: TransportWrite, Err: fmt.Errorf("open mutation socket failed: %w", err)}
	}
	if conn == nil {
		return result, &TransportError{Stage: TransportWrite, Err: errors.New("mutation socket is nil")}
	}

	closeOnceDone := make(chan struct{})
	var closeOnce sync.Once
	var closeErr error
	closeConn := func() error {
		closeOnce.Do(func() { closeErr = conn.Close() })
		return closeErr
	}
	defer func() {
		close(closeOnceDone)
		if closeErr := closeConn(); closeErr != nil {
			resultErr = joinTransportError(transportStage(resultErr, TransportClose), resultErr, closeErr)
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
			_ = closeConn()
		case <-closeOnceDone:
		}
	}()

	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(timeout)
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		return result, &TransportError{Stage: TransportWrite, Err: fmt.Errorf("set mutation write deadline failed: %w", err)}
	}
	n, err := conn.WriteTo(payload, target.Sender)
	if err != nil {
		return result, classifyMutationTransportError(ctx, TransportWrite, err)
	}
	if n != len(payload) {
		return result, &TransportError{Stage: TransportShortWrite, Err: errors.New("short write")}
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return result, &TransportError{Stage: TransportRead, Err: fmt.Errorf("set mutation read deadline failed: %w", err)}
	}

	buffer := make([]byte, limit+1)
	n, sender, err := conn.ReadFrom(buffer)
	if err != nil {
		return result, classifyMutationTransportError(ctx, TransportRead, err)
	}
	if n > limit {
		return result, &TransportError{Stage: TransportResponseTooLarge, Err: errors.New("response exceeds datagram size")}
	}
	if n == 0 || sender == nil {
		return result, &TransportError{Stage: TransportRead, Err: errors.New("empty mutation response")}
	}
	return logic.MutationExchange{Sender: sender, Data: append([]byte(nil), buffer[:n]...)}, nil
}

func classifyMutationTransportError(ctx context.Context, stage TransportStage, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return &TransportError{Stage: TransportTimeout, Err: ctxErr}
		}
		return &TransportError{Stage: TransportCanceled, Err: ctxErr}
	}
	if errors.Is(err, net.ErrClosed) {
		return &TransportError{Stage: stage, Err: errors.New("mutation socket closed")}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &TransportError{Stage: TransportTimeout, Err: context.DeadlineExceeded}
	}
	return &TransportError{Stage: stage, Err: errors.New("mutation socket I/O failed")}
}
