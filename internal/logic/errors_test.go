package logic_test

import (
	"errors"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

func TestErrors(t *testing.T) {
	t.Run("ValidationError formatting", func(t *testing.T) {
		valErrWithField := &logic.ValidationError{
			Field:   "param",
			Message: "invalid value",
		}
		expectedWithField := "validation error on param: invalid value"
		if got := valErrWithField.Error(); got != expectedWithField {
			t.Errorf("expected %q, got %q", expectedWithField, got)
		}

		valErrNoField := &logic.ValidationError{
			Message: "general validation issue",
		}
		expectedNoField := "validation error: general validation issue"
		if got := valErrNoField.Error(); got != expectedNoField {
			t.Errorf("expected %q, got %q", expectedNoField, got)
		}
	})

	t.Run("DeviceOperationError formatting and unwrap", func(t *testing.T) {
		cause := errors.New("underlying connection closed")
		opErr := &logic.DeviceOperationError{
			Kind: logic.OperationTransport,
			Err:  cause,
		}

		if !errors.Is(opErr, cause) {
			t.Errorf("expected opErr to wrap cause")
		}
		if opErr.Unwrap() != cause {
			t.Errorf("expected Unwrap() to return cause")
		}

		expectedWithErr := "device operation failed (transport): underlying connection closed"
		if got := opErr.Error(); got != expectedWithErr {
			t.Errorf("expected %q, got %q", expectedWithErr, got)
		}

		opErrNoErr := &logic.DeviceOperationError{
			Kind: logic.OperationRejected,
		}
		expectedNoErr := "device operation failed (device_rejected)"
		if got := opErrNoErr.Error(); got != expectedNoErr {
			t.Errorf("expected %q, got %q", expectedNoErr, got)
		}
	})
}

