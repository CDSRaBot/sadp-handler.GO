// Package route selects SADP or ISAPI for one device operation.
package route

import (
	"context"
	"errors"
	"net"
	"net/netip"
)

var ErrNotImplemented = errors.New("not implemented")

type Strategy string

const (
	StrategySADPFirst Strategy = "sadp_first"
	StrategyISAPIOnly Strategy = "isapi_only"
)

type Operation string

const (
	OperationDiscovery  Operation = "discovery"
	OperationActivation Operation = "activation"
	OperationNetwork    Operation = "network"
	OperationStatus     Operation = "status"
)

type ErrorKind string

const (
	ErrorInvalidInput    ErrorKind = "invalid_input"
	ErrorNoRoute         ErrorKind = "no_route"
	ErrorSADPUnavailable ErrorKind = "sadp_unavailable"
	ErrorUnsupported     ErrorKind = "unsupported"
	ErrorRejected        ErrorKind = "device_rejected"
	ErrorAuthentication  ErrorKind = "authentication"
	ErrorMalformed       ErrorKind = "malformed_response"
	ErrorTimeout         ErrorKind = "timeout"
	ErrorCanceled        ErrorKind = "canceled"
	ErrorVerification    ErrorKind = "verification"
)

type Error struct {
	Kind ErrorKind
	Err  error
}

func (e *Error) Error() string { return string(e.Kind) }
func (e *Error) Unwrap() error { return e.Err }

type FallbackDecision string

const (
	FallbackNotAllowed FallbackDecision = "not_allowed"
	FallbackEligible   FallbackDecision = "eligible"
)

type Target struct {
	MAC       net.HardwareAddr
	Address   netip.Addr
	Interface string
}

type Result struct {
	Operation Operation
	Protocol  string
	Target    Target
	Fallback  FallbackDecision
	Err       error
}

type Transport interface {
	Execute(context.Context, Operation, Target) (Result, error)
}

type RouteProvider interface {
	CanReach(context.Context, netip.Addr, string) (bool, error)
}

type CapabilityProvider interface {
	Supports(context.Context, Target, Operation) (bool, error)
}

type Dependencies struct {
	SADP       Transport
	ISAPI      Transport
	Routes     RouteProvider
	Capability CapabilityProvider
}

type Selector struct{}

func NewSelector(strategy Strategy, dependencies Dependencies) (*Selector, error) {
	return &Selector{}, ErrNotImplemented
}

func (s *Selector) Execute(context.Context, Operation, Target) (Result, error) {
	return Result{}, ErrNotImplemented
}
