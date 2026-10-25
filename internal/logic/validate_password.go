package logic

import (
	"unicode/utf8"
)

// ValidatePassword checks whether the given password satisfies the compatibility policy.
// Policy (see docs/spec/procedures/logic/validate_password.md):
//   - Length must be between 8 and 16 ASCII characters inclusive.
//   - At least two of the following character classes must be present:
//   - lower case letters (a-z)
//   - upper case letters (A-Z)
//   - digits (0-9)
//   - printable ASCII special characters.
//   - All non-ASCII, whitespace, and control characters are rejected.
//
// Returns nil on success, otherwise an error describing the validation failure.
func ValidatePassword(password string) error {
	if password == "" {
		return ErrEmptyPassword
	}
	if !utf8.ValidString(password) {
		return &ValidationError{Field: "password", Message: "password must be valid UTF-8"}
	}
	if len(password) < 8 || len(password) > 16 {
		return &ValidationError{Field: "password", Message: "password length must be between 8 and 16 characters"}
	}

	var lower, upper, digit, special bool
	for i := 0; i < len(password); i++ {
		char := password[i]
		switch {
		case char >= 'a' && char <= 'z':
			lower = true
		case char >= 'A' && char <= 'Z':
			upper = true
		case char >= '0' && char <= '9':
			digit = true
		case (char >= '!' && char <= '/') ||
			(char >= ':' && char <= '@') ||
			(char >= '[' && char <= '`') ||
			(char >= '{' && char <= '~'):
			special = true
		default:
			return &ValidationError{Field: "password", Message: "password contains unsupported characters"}
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
