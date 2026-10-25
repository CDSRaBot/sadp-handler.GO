package logic

import (
	"unicode"
	"unicode/utf8"
)

// ValidatePassword checks whether the given password satisfies the compatibility policy.
// Policy (see docs/spec/procedures/logic/validate_password.md):
//   - Length must be between 8 and 16 Unicode code points (runes) inclusive.
//   - At least two of the following character classes must be present:
//   - lower case letters (unicode.IsLower)
//   - upper case letters (unicode.IsUpper)
//   - digits (unicode.IsDigit)
//   - special characters (runes that are not letters nor digits; spaces count as special).
//   - Control characters are rejected.
//
// Returns nil on success, otherwise an error describing the validation failure.
func ValidatePassword(password string) error {
	if password == "" {
		return ErrEmptyPassword
	}
	if !utf8.ValidString(password) {
		return &ValidationError{Field: "password", Message: "password must be valid UTF-8"}
	}
	runes := []rune(password)
	if len(runes) < 8 || len(runes) > 16 {
		return &ValidationError{Field: "password", Message: "password length must be between 8 and 16 characters"}
	}

	var lower, upper, digit, special bool
	for _, char := range runes {
		if unicode.IsControl(char) {
			return &ValidationError{Field: "password", Message: "control characters are not allowed"}
		}
		switch {
		case unicode.IsLower(char):
			lower = true
		case unicode.IsUpper(char):
			upper = true
		case unicode.IsDigit(char):
			digit = true
		default:
			special = true
		}
	}

	classes := 0
	for _, present := range []bool{lower, upper, digit, special} {
		if present {
			classes++
		}
	}
	if classes < 2 {
		return &ValidationError{Field: "password", Message: "password must contain at least two character classes"}
	}
	return nil
}
