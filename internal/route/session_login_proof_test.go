package route

import (
	"errors"
	"strings"
	"testing"
)

func TestISAPISessionLoginProofMatchesIndependentVectors(t *testing.T) {
	tests := []struct {
		irreversible bool
		iterations   int
		want         string
	}{
		{false, 2, "0be023f57209f39ee1aeabc54a923e175b592490b9cdd3e4eab1281f583b904c"},
		{true, 2, "9e7aa46588e1dd6d635891e09f8325dc9a4b608fc411fd12f57974b8bba74b83"},
		{false, 100, "3b64fc6e8212b92a5d2e1e3f483e81a2c83d7f4700a85b5da6bc54261bbbbab0"},
		{true, 100, "adc8922124651155e631a02cbcafef36de72b03478e71d70b3c84c64d6208676"},
	}
	for _, tt := range tests {
		params := ISAPISessionLoginParams{
			Username: "admin", Password: "TestPass1!", Salt: "0123456789abcdef",
			Challenge: "abcdef0123456789", Irreversible: tt.irreversible, Iterations: tt.iterations,
		}
		got, err := BuildISAPISessionLoginProof(params)
		if err != nil || got != tt.want {
			t.Fatalf("proof does not match independent vector, error = %v", err)
		}
	}
}

func TestISAPISessionLoginProofValidatesInputs(t *testing.T) {
	valid := ISAPISessionLoginParams{Username: "admin", Password: "TestPass1!", Salt: "salt", Challenge: "challenge", Irreversible: true, Iterations: 2}
	tests := []ISAPISessionLoginParams{
		{},
		{Username: "admin", Password: "TestPass1!", Challenge: "challenge", Irreversible: true, Iterations: 2},
		{Username: "admin", Password: "TestPass1!", Challenge: "challenge", Iterations: 1},
		{Username: "admin", Password: "TestPass1!", Challenge: "challenge", Iterations: MaxISAPISecurityIterations + 1},
		{Username: "admin", Password: "TestPass1!", Challenge: string([]byte{0xff}), Iterations: 2},
		{Username: "admin", Password: "TestPass1!", Challenge: strings.Repeat("x", MaxISAPIUserFieldBytes+1), Iterations: 2},
	}
	for _, params := range tests {
		got, err := BuildISAPISessionLoginProof(params)
		var routeErr *Error
		if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput || got != "" {
			t.Fatalf("invalid input returned proof or wrong error: %v", err)
		}
		if strings.Contains(err.Error(), "TestPass1!") || strings.Contains(err.Error(), "challenge") {
			t.Fatal("error disclosed input")
		}
	}
	valid.Iterations = MaxISAPISecurityIterations
	if got, err := BuildISAPISessionLoginProof(valid); err != nil || len(got) != 64 {
		t.Fatalf("maximum supported iterations rejected: %v", err)
	}
}
