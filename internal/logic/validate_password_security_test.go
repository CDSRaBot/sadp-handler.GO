package logic_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

func TestValidatePasswordUsesRuneLengthAndRejectsControls(t *testing.T) {
	testCases := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{name: "eight unicode code points", password: "АбвГд123", wantErr: false},
		{name: "seven unicode code points", password: "АбвГд12", wantErr: true},
		{name: "combining mark counts as a special rune", password: "Abcdef1\u0301", wantErr: false},
		{name: "newline is rejected", password: "Abcdef1\n", wantErr: true},
		{name: "tab is rejected", password: "Abcdef1\t", wantErr: true},
		{name: "space is a permitted special character", password: "Abcdef1 ", wantErr: false},
		{name: "invalid UTF-8 is rejected", password: string([]byte{'A', 'b', 'c', 'd', 'e', 'f', '1', 0xff}), wantErr: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if !utf8.ValidString(testCase.password) && !testCase.wantErr {
				t.Fatal("test fixture must be valid UTF-8 for a positive case")
			}
			err := logic.ValidatePassword(testCase.password)
			if (err != nil) != testCase.wantErr {
				t.Fatalf("ValidatePassword returned %v, want error=%t", err, testCase.wantErr)
			}
		})
	}
}

func TestValidatePasswordErrorsNeverDiscloseSecret(t *testing.T) {
	const password = "Password-policy-secret-canary-123!"

	err := logic.ValidatePassword(password[:7])
	if err == nil {
		t.Fatal("expected short password to fail")
	}
	if strings.Contains(err.Error(), password) || strings.Contains(err.Error(), password[:7]) {
		t.Fatalf("validation error disclosed password: %v", err)
	}
}
