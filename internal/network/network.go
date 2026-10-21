package network

import (
	"context"
	"errors"
	"net"
)

const DefaultMaxSADPDatagramSize = 1500

type PacketHandler func(context.Context, []byte, net.Addr) error

type TransportStage string

const (
	TransportValidation TransportStage = "validation"
	TransportEncode     TransportStage = "encode"
	TransportWrite      TransportStage = "write"
	TransportShortWrite TransportStage = "short_write"
	TransportRead       TransportStage = "read"
	TransportDispatch   TransportStage = "dispatch"
	TransportClose      TransportStage = "close"
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

func Listen(ctx context.Context, conn net.PacketConn, maxDatagramSize int, handle PacketHandler) error {
	return errors.New("not implemented")
}

func SendInquiryProbe(ctx context.Context, conn net.PacketConn, target net.Addr) error {
	return errors.New("not implemented")
}

func SendSADPDatagram(
	ctx context.Context,
	conn net.PacketConn,
	target net.Addr,
	payload []byte,
	maxDatagramSize int,
) error {
	return errors.New("not implemented")
}

func handleDatagram(
	ctx context.Context,
	data []byte,
	sender net.Addr,
	maxDatagramSize int,
	handle PacketHandler,
) error {
	return errors.New("not implemented")
}
