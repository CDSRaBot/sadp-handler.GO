package network

import (
	"context"
	"crypto/rand"
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

const DefaultMaxSADPDatagramSize = 1500

type PacketHandler func(context.Context, []byte, net.Addr) error

type TransportStage string

const (
	TransportValidation       TransportStage = "validation"
	TransportEncode           TransportStage = "encode"
	TransportWrite            TransportStage = "write"
	TransportShortWrite       TransportStage = "short_write"
	TransportRead             TransportStage = "read"
	TransportDispatch         TransportStage = "dispatch"
	TransportTimeout          TransportStage = "timeout"
	TransportCanceled         TransportStage = "canceled"
	TransportResponseTooLarge TransportStage = "response_too_large"
	TransportClose            TransportStage = "close"
)

type TransportError struct {
	Stage TransportStage
	Err   error
}

func (e *TransportError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.Stage)
	}
	return string(e.Stage) + ": " + e.Err.Error()
}

func (e *TransportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func Listen(ctx context.Context, conn net.PacketConn, maxDatagramSize int, handle PacketHandler) (result error) {
	if conn == nil {
		return &TransportError{Stage: TransportValidation, Err: errors.New("connection is nil")}
	}

	var closeOnce sync.Once
	var closeErr error
	closeConnection := func() {
		closeOnce.Do(func() {
			closeErr = conn.Close()
		})
	}
	defer func() {
		closeConnection()
		if closeErr != nil {
			result = joinTransportError(transportStage(result, TransportClose), result, closeErr)
		}
	}()

	if ctx == nil {
		return &TransportError{Stage: TransportValidation, Err: errors.New("context is nil")}
	}
	if handle == nil {
		return &TransportError{Stage: TransportValidation, Err: errors.New("handler is nil")}
	}
	if !validDatagramSize(maxDatagramSize) {
		return &TransportError{Stage: TransportValidation, Err: errors.New("datagram size must be between 1 and 65507")}
	}
	if err := ctx.Err(); err != nil {
		closeConnection()
		return nil
	}

	done := make(chan struct{})
	var watcher sync.WaitGroup
	watcher.Add(1)
	go func() {
		defer watcher.Done()
		select {
		case <-ctx.Done():
			closeConnection()
		case <-done:
		}
	}()
	defer func() {
		close(done)
		watcher.Wait()
	}()

	buffer := make([]byte, maxDatagramSize+1)
	for {
		if err := ctx.Err(); err != nil {
			closeConnection()
			return nil
		}
		n, sender, err := conn.ReadFrom(buffer)
		if err != nil {
			closeConnection()
			if ctx.Err() != nil {
				return nil
			}
			return &TransportError{Stage: TransportRead, Err: err}
		}
		if n < 0 || n > len(buffer) {
			closeConnection()
			return &TransportError{Stage: TransportRead, Err: errors.New("connection returned an invalid datagram length")}
		}
		if n > maxDatagramSize {
			closeConnection()
			return &TransportError{Stage: TransportResponseTooLarge, Err: errors.New("datagram exceeds configured size")}
		}
		if err := handleDatagram(ctx, buffer[:n], sender, maxDatagramSize, handle); err != nil {
			closeConnection()
			return &TransportError{Stage: transportStage(err, TransportDispatch), Err: err}
		}
	}
}

func SendInquiryProbe(ctx context.Context, conn net.PacketConn, target net.Addr) error {
	if err := validateSendArguments(ctx, conn, target); err != nil {
		return err
	}
	uuid, err := newUUID()
	if err != nil {
		return &TransportError{Stage: TransportEncode, Err: fmt.Errorf("generate probe UUID: %w", err)}
	}
	payload, err := xml.Marshal(struct {
		XMLName      xml.Name `xml:"Probe"`
		UUID         string   `xml:"Uuid"`
		Types        string   `xml:"Types"`
		ResponseMode string   `xml:"ResponseMode"`
	}{UUID: uuid, Types: "inquiry", ResponseMode: "2"})
	if err != nil {
		return &TransportError{Stage: TransportEncode, Err: fmt.Errorf("encode inquiry probe: %w", err)}
	}
	payload = append([]byte(`<?xml version="1.0" encoding="utf-8"?>`), payload...)
	return writeDatagram(ctx, conn, target, payload)
}

func SendSADPDatagram(
	ctx context.Context,
	conn net.PacketConn,
	target net.Addr,
	payload []byte,
	maxDatagramSize int,
) error {
	if err := validateSendArguments(ctx, conn, target); err != nil {
		return err
	}
	if len(payload) == 0 {
		return &TransportError{Stage: TransportValidation, Err: errors.New("payload is empty")}
	}
	if !validDatagramSize(maxDatagramSize) {
		return &TransportError{Stage: TransportValidation, Err: errors.New("datagram size must be between 1 and 65507")}
	}
	if len(payload) > maxDatagramSize {
		return &TransportError{Stage: TransportValidation, Err: errors.New("payload exceeds datagram size")}
	}
	return writeDatagram(ctx, conn, target, payload)
}

func handleDatagram(
	ctx context.Context,
	data []byte,
	sender net.Addr,
	maxDatagramSize int,
	handle PacketHandler,
) error {
	if ctx == nil {
		return &TransportError{Stage: TransportValidation, Err: errors.New("context is nil")}
	}
	if sender == nil || handle == nil || len(data) == 0 || !validDatagramSize(maxDatagramSize) || len(data) > maxDatagramSize {
		return &TransportError{Stage: TransportValidation, Err: errors.New("invalid datagram arguments")}
	}
	if err := ctx.Err(); err != nil {
		return &TransportError{Stage: TransportValidation, Err: err}
	}
	if err := handle(ctx, data, sender); err != nil {
		return &TransportError{Stage: TransportDispatch, Err: err}
	}
	return nil
}

func validDatagramSize(size int) bool {
	return size >= 1 && size <= 65507
}

func validateSendArguments(ctx context.Context, conn net.PacketConn, target net.Addr) error {
	if ctx == nil || conn == nil || target == nil {
		return &TransportError{Stage: TransportValidation, Err: errors.New("context, connection, and target are required")}
	}
	if err := ctx.Err(); err != nil {
		return &TransportError{Stage: TransportValidation, Err: err}
	}
	return nil
}

func writeDatagram(ctx context.Context, conn net.PacketConn, target net.Addr, payload []byte) error {
	if err := ctx.Err(); err != nil {
		return &TransportError{Stage: TransportValidation, Err: err}
	}
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetWriteDeadline(deadline); err != nil {
			return &TransportError{Stage: TransportWrite, Err: err}
		}
		defer func() { _ = conn.SetWriteDeadline(time.Time{}) }()
	}
	n, err := conn.WriteTo(payload, target)
	if err != nil {
		return &TransportError{Stage: TransportWrite, Err: err}
	}
	if n != len(payload) {
		return &TransportError{Stage: TransportShortWrite, Err: errors.New("short write")}
	}
	return nil
}

func joinTransportError(stage TransportStage, primary, cleanup error) error {
	if cleanup == nil {
		if primary == nil {
			return nil
		}
		return &TransportError{Stage: stage, Err: primary}
	}
	if primary == nil {
		return &TransportError{Stage: TransportClose, Err: cleanup}
	}
	return &TransportError{Stage: stage, Err: errors.Join(primary, cleanup)}
}

func transportStage(err error, fallback TransportStage) TransportStage {
	var transportErr *TransportError
	if errors.As(err, &transportErr) {
		return transportErr.Stage
	}
	return fallback
}

func newUUID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		value[0:4], value[4:6], value[6:8], value[8:10], value[10:16]), nil
}
