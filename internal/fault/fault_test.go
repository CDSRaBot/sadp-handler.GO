package fault_test

import (
	"context"
	"errors"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/fault"
	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
	"github.com/CDSRaBot/sadp-handler.GO/internal/network"
	"github.com/CDSRaBot/sadp-handler.GO/internal/route"
)

func TestClassifyPreservesWrappedOperationCategories(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want fault.Code
	}{
		{
			name: "invalid input",
			err:  &logic.DeviceOperationError{Kind: logic.OperationInvalidInput},
			want: fault.InvalidArgument,
		},
		{
			name: "transport",
			err:  &logic.DeviceOperationError{Kind: logic.OperationTransport},
			want: fault.Transport,
		},
		{
			name: "rejected",
			err:  &logic.DeviceOperationError{Kind: logic.OperationRejected},
			want: fault.DeviceRejected,
		},
		{
			name: "malformed",
			err:  &logic.DeviceOperationError{Kind: logic.OperationMalformed},
			want: fault.MalformedResponse,
		},
		{
			name: "correlation",
			err:  &logic.DeviceOperationError{Kind: logic.OperationCorrelation},
			want: fault.ResponseMismatch,
		},
		{
			name: "missing data",
			err:  &logic.DeviceOperationError{Kind: logic.OperationMissingData},
			want: fault.MissingData,
		},
		{
			name: "timeout",
			err:  &logic.DeviceOperationError{Kind: logic.OperationTimeout},
			want: fault.Timeout,
		},
		{
			name: "canceled",
			err:  &logic.DeviceOperationError{Kind: logic.OperationCanceled},
			want: fault.Canceled,
		},
		{
			name: "route unsupported",
			err:  &route.Error{Kind: route.ErrorUnsupported},
			want: fault.Unsupported,
		},
		{
			name: "route authentication",
			err:  &route.Error{Kind: route.ErrorAuthentication},
			want: fault.Authentication,
		},
		{
			name: "route unavailable",
			err:  &route.Error{Kind: route.ErrorSADPUnavailable},
			want: fault.SADPUnavailable,
		},
		{
			name: "route no route",
			err:  &route.Error{Kind: route.ErrorNoRoute},
			want: fault.NoRoute,
		},
		{
			name: "transport timeout",
			err:  &network.TransportError{Stage: network.TransportTimeout},
			want: fault.Timeout,
		},
		{
			name: "unknown",
			err:  errors.New("unknown failure"),
			want: fault.OperationFailed,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wrapped := errors.New("boundary: " + test.err.Error())
			wrapped = &wrappedError{cause: test.err, message: wrapped.Error()}
			if got := fault.Classify(wrapped); got != test.want {
				t.Fatalf("Classify() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestClassifyRecognizesContextThroughWrapping(t *testing.T) {
	if got := fault.Classify(&wrappedError{cause: context.Canceled, message: "operation: canceled"}); got != fault.Canceled {
		t.Fatalf("Classify() = %q, want %q", got, fault.Canceled)
	}
	if got := fault.Classify(&wrappedError{cause: context.DeadlineExceeded, message: "operation: deadline"}); got != fault.Timeout {
		t.Fatalf("Classify() = %q, want %q", got, fault.Timeout)
	}
}

type wrappedError struct {
	cause   error
	message string
}

func (e *wrappedError) Error() string { return e.message }
func (e *wrappedError) Unwrap() error { return e.cause }
