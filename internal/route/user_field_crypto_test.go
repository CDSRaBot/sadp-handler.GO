package route

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestEncryptISAPIUserFieldMatchesIndependentVectors(t *testing.T) {
	tests := []struct {
		scheme ISAPIUserSecurity
		size   int
		want   string
	}{
		{scheme: ISAPIUserSecurityAES128, size: 16, want: "KuVhQa0V8qBGQXmXGW6TUg=="},
		{scheme: ISAPIUserSecurityAES256, size: 32, want: "2NQuecJaHF5I50makgrxtg=="},
	}
	iv := make([]byte, 16)
	for index := range iv {
		iv[index] = byte(index + 16)
	}
	for _, tt := range tests {
		key := make([]byte, tt.size)
		for index := range key {
			key[index] = byte(index)
		}
		result, err := encryptISAPIUserField(bytes.NewReader(iv), tt.scheme, key, "synthetic")
		if err != nil {
			t.Fatal(err)
		}
		if result.Security != tt.scheme || result.IV != base64.StdEncoding.EncodeToString(iv) || result.Ciphertext != tt.want {
			t.Fatalf("unexpected encrypted field: %#v", result)
		}
	}
}

func TestEncryptISAPIUserFieldValidatesInputs(t *testing.T) {
	key128 := make([]byte, 16)
	tests := []struct {
		name   string
		random io.Reader
		scheme ISAPIUserSecurity
		key    []byte
		value  string
		want   ErrorKind
	}{
		{name: "unknown scheme", random: bytes.NewReader(make([]byte, 16)), scheme: 3, key: key128, want: ErrorUnsupported},
		{name: "wrong key size", random: bytes.NewReader(make([]byte, 16)), scheme: ISAPIUserSecurityAES256, key: key128, want: ErrorInvalidInput},
		{name: "invalid UTF-8", random: bytes.NewReader(make([]byte, 16)), scheme: ISAPIUserSecurityAES128, key: key128, value: string([]byte{0xff}), want: ErrorInvalidInput},
		{name: "oversized", random: bytes.NewReader(make([]byte, 16)), scheme: ISAPIUserSecurityAES128, key: key128, value: strings.Repeat("x", MaxISAPIUserFieldBytes+1), want: ErrorInvalidInput},
		{name: "no entropy", random: bytes.NewReader(nil), scheme: ISAPIUserSecurityAES128, key: key128, value: "secret", want: ErrorTransport},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := encryptISAPIUserField(tt.random, tt.scheme, tt.key, tt.value)
			var routeErr *Error
			if !errors.As(err, &routeErr) || routeErr.Kind != tt.want || result.Ciphertext != "" || result.IV != "" {
				t.Fatalf("result = %#v, error = %v", result, err)
			}
			if tt.value == "secret" && strings.Contains(err.Error(), tt.value) {
				t.Fatal("error disclosed secret field value")
			}
		})
	}
	if _, err := encryptISAPIUserField(bytes.NewReader(make([]byte, 16)), ISAPIUserSecurityAES128, key128, strings.Repeat("x", MaxISAPIUserFieldBytes)); err != nil {
		t.Fatalf("maximum-sized field rejected: %v", err)
	}
}

func TestEncryptISAPIUserFieldUsesFreshIV(t *testing.T) {
	key := make([]byte, 16)
	first, err := EncryptISAPIUserField(ISAPIUserSecurityAES128, key, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncryptISAPIUserField(ISAPIUserSecurityAES128, key, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if first.IV == second.IV || first.Ciphertext == second.Ciphertext {
		t.Fatal("separate encryptions reused an IV")
	}
}
