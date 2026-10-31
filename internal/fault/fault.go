// Package fault defines the public failure contract at the application boundary.
package fault

import (
	"context"
	"errors"

	"github.com/CDSRaBot/sadp-handler.GO/internal/adapter"
	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/network"
	"github.com/CDSRaBot/sadp-handler.GO/internal/parser"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

// Code is a stable failure code exposed to application clients.
type Code string

const (
	InvalidArgument   Code = "invalid_argument"
	Unsupported       Code = "unsupported"
	Transport         Code = "transport"
	Timeout           Code = "timeout"
	Canceled          Code = "canceled"
	DeviceRejected    Code = "device_rejected"
	Authentication    Code = "authentication"
	SADPUnavailable   Code = "sadp_unavailable"
	NoRoute           Code = "no_route"
	MalformedResponse Code = "malformed_response"
	ResponseMismatch  Code = "response_mismatch"
	Verification      Code = "verification"
	MissingData       Code = "missing_data"
	OperationFailed   Code = "operation_failed"
)

// Classify maps a wrapped internal error to its public application code.
func Classify(err error) Code {
	if err == nil {
		return OperationFailed
	}
	if errors.Is(err, context.Canceled) {
		return Canceled
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return Timeout
	}

	var operationErr *logic.DeviceOperationError
	if errors.As(err, &operationErr) {
		return classifyOperation(operationErr.Kind)
	}

	var routeErr *route.Error
	if errors.As(err, &routeErr) {
		return classifyRoute(routeErr.Kind)
	}

	var transportErr *network.TransportError
	if errors.As(err, &transportErr) {
		switch transportErr.Stage {
		case network.TransportTimeout:
			return Timeout
		case network.TransportCanceled:
			return Canceled
		default:
			return Transport
		}
	}

	var parseErr *parser.ParseError
	if errors.As(err, &parseErr) {
		switch parseErr.Kind {
		case parser.ParseInvalidField, parser.ParseMissingField:
			return InvalidArgument
		default:
			return MalformedResponse
		}
	}

	var adapterErr *adapter.AdapterError
	if errors.As(err, &adapterErr) {
		switch adapterErr.Kind {
		case adapter.AdapterInvalidInterface, adapter.AdapterInvalidMulticast:
			return InvalidArgument
		default:
			return Transport
		}
	}

	return OperationFailed
}

func classifyOperation(kind logic.DeviceOperationErrorKind) Code {
	switch kind {
	case logic.OperationInvalidInput:
		return InvalidArgument
	case logic.OperationTransport:
		return Transport
	case logic.OperationRejected:
		return DeviceRejected
	case logic.OperationMalformed:
		return MalformedResponse
	case logic.OperationCorrelation:
		return ResponseMismatch
	case logic.OperationTimeout:
		return Timeout
	case logic.OperationMissingData:
		return MissingData
	case logic.OperationCanceled:
		return Canceled
	default:
		return OperationFailed
	}
}

func classifyRoute(kind route.ErrorKind) Code {
	switch kind {
	case route.ErrorInvalidInput:
		return InvalidArgument
	case route.ErrorUnsupported:
		return Unsupported
	case route.ErrorSADPUnavailable:
		return SADPUnavailable
	case route.ErrorRejected:
		return DeviceRejected
	case route.ErrorAuthentication:
		return Authentication
	case route.ErrorMalformed:
		return MalformedResponse
	case route.ErrorTimeout:
		return Timeout
	case route.ErrorCanceled:
		return Canceled
	case route.ErrorVerification:
		return Verification
	case route.ErrorNoRoute:
		return NoRoute
	default:
		return OperationFailed
	}
}
