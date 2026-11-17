package route

import (
	"errors"
	"testing"
)

func TestISAPIResponseStatusMustConfirmSuccess(t *testing.T) {
	tests := []struct {
		payload string
		wantErr ErrorKind
	}{
		{payload: `<ResponseStatus><statusCode>1</statusCode></ResponseStatus>`},
		{payload: `<ResponseStatus><statusCode>4</statusCode></ResponseStatus>`, wantErr: ErrorRejected},
		{payload: `<ResponseStatus/>`, wantErr: ErrorMalformed},
		{payload: `<Other><statusCode>1</statusCode></Other>`, wantErr: ErrorMalformed},
	}
	for _, tt := range tests {
		err := validateISAPIResponseStatus([]byte(tt.payload))
		if tt.wantErr == "" {
			if err != nil {
				t.Fatalf("valid response failed: %v", err)
			}
			continue
		}
		var routeErr *Error
		if !errors.As(err, &routeErr) || routeErr.Kind != tt.wantErr {
			t.Fatalf("payload %q: error = %v, want %s", tt.payload, err, tt.wantErr)
		}
	}
}
