package logic

import "fmt"

// DeviceOperationErrorKind identifies the category of device operation failure.
type DeviceOperationErrorKind string

const (
	OperationInvalidInput DeviceOperationErrorKind = "invalid_input"
	OperationTransport    DeviceOperationErrorKind = "transport"
	OperationRejected     DeviceOperationErrorKind = "device_rejected"
	OperationCanceled     DeviceOperationErrorKind = "canceled"
)

// DeviceOperationError represents an error that occurs during device operations.
type DeviceOperationError struct {
	Kind DeviceOperationErrorKind
	Err  error
}

// Error formats the device operation error message.
func (e *DeviceOperationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("device operation failed (%s): %v", e.Kind, e.Err)
	}
	return fmt.Sprintf("device operation failed (%s)", e.Kind)
}

// Unwrap returns the underlying error cause.
func (e *DeviceOperationError) Unwrap() error {
	return e.Err
}

// ValidationError represents an input validation error in the device logic module.
type ValidationError struct {
	Field   string
	Message string
}

// Error returns a descriptive message without revealing sensitive input data.
func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

var (
	// ErrEmptyPassword indicates that the supplied password is empty.
	ErrEmptyPassword = &ValidationError{
		Field:   "password",
		Message: "password cannot be empty",
	}

	// ErrEmptyChallenge indicates that the supplied challenge is empty.
	ErrEmptyChallenge = &ValidationError{
		Field:   "challenge",
		Message: "challenge cannot be empty",
	}
)
