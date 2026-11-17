package route

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestISAPIRecoveryEncryptionMatchesIndependentVectors(t *testing.T) {
	tests := []struct {
		name         string
		irreversible bool
		iterations   int
		key          string
		ciphertexts  []string
	}{
		{
			name: "reversible", iterations: 1,
			key: "384ef2689791c86bc45085c29c3259b3",
			ciphertexts: []string{
				"835409363339e55e40a9e5bc109d6335f49e5ba2a4785d519fdcac5cb3f0b3f5",
				"506abc66bb9167ffa1957e33e5dfd77e",
			},
		},
		{
			name: "irreversible", irreversible: true, iterations: 100,
			key: "829ff93b3a720d693d94e20f91ab0986",
			ciphertexts: []string{
				"477b00e2dd7d5f9efd0756f00e5d293eb9928314857ee1d4b07301af877ea6fe",
				"7635ff81ed491e6a9e34ad312356ef68",
			},
		},
	}
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = byte(i)
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := ISAPIRecoveryCryptoParams{
				Username: "admin", Password: "TestPass1!", Salt: "0123456789abcdef",
				Irreversible: tt.irreversible, Iterations: tt.iterations,
			}
			key := deriveISAPIRecoveryKey(params)
			if hex.EncodeToString(key[:]) != tt.key {
				t.Fatal("derived key does not match independent vector")
			}
			result, err := encryptISAPIRecoveryFields(bytes.NewReader(iv), params, []string{"A&B<C>", "answer"})
			if err != nil {
				t.Fatal(err)
			}
			if result.IV != "000102030405060708090a0b0c0d0e0f" || len(result.Values) != 2 ||
				result.Values[0] != tt.ciphertexts[0] || result.Values[1] != tt.ciphertexts[1] {
				t.Fatal("encrypted request fields do not match independent vectors")
			}
		})
	}
}

func TestISAPIRecoveryEncryptionRejectsInvalidInputs(t *testing.T) {
	valid := ISAPIRecoveryCryptoParams{Username: "admin", Password: "TestPass1!", Salt: "salt", Irreversible: true, Iterations: 1}
	tests := []struct {
		name   string
		params ISAPIRecoveryCryptoParams
		values []string
	}{
		{"missing credentials", ISAPIRecoveryCryptoParams{Iterations: 1}, []string{"answer"}},
		{"missing salt", ISAPIRecoveryCryptoParams{Username: "admin", Password: "TestPass1!", Irreversible: true, Iterations: 1}, []string{"answer"}},
		{"zero iterations", ISAPIRecoveryCryptoParams{Username: "admin", Password: "TestPass1!"}, []string{"answer"}},
		{"excess iterations", ISAPIRecoveryCryptoParams{Username: "admin", Password: "TestPass1!", Iterations: MaxISAPISecurityIterations + 1}, []string{"answer"}},
		{"no fields", valid, nil},
		{"too many fields", valid, make([]string, MaxISAPIRecoveryFields+1)},
		{"invalid UTF-8", valid, []string{string([]byte{0xff})}},
		{"oversized field", valid, []string{strings.Repeat("x", MaxISAPIUserFieldBytes+1)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := encryptISAPIRecoveryFields(bytes.NewReader(make([]byte, 16)), tt.params, tt.values)
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != ErrorInvalidInput || result.IV != "" || len(result.Values) != 0 {
				t.Fatalf("invalid input result = %#v, error = %v", result, err)
			}
			if strings.Contains(err.Error(), "TestPass1!") || strings.Contains(err.Error(), "answer") {
				t.Fatal("error disclosed input")
			}
		})
	}
	_, err := encryptISAPIRecoveryFields(bytes.NewReader(nil), valid, []string{"answer"})
	var routeErr *Error
	if !errors.As(err, &routeErr) || routeErr.Kind != ErrorTransport {
		t.Fatalf("entropy failure = %v", err)
	}
}

func TestISAPIRecoveryEncryptionBoundariesAndFreshIV(t *testing.T) {
	params := ISAPIRecoveryCryptoParams{Username: "admin", Password: "TestPass1!", Iterations: MaxISAPISecurityIterations}
	values := make([]string, MaxISAPIRecoveryFields)
	values[0] = strings.Repeat("x", MaxISAPIUserFieldBytes)
	first, err := EncryptISAPIRecoveryFields(params, values)
	if err != nil {
		t.Fatalf("maximum-sized input rejected: %v", err)
	}
	second, err := EncryptISAPIRecoveryFields(params, values)
	if err != nil {
		t.Fatal(err)
	}
	if first.IV == second.IV || first.Values[0] == second.Values[0] {
		t.Fatal("separate encryptions reused an IV")
	}
}
