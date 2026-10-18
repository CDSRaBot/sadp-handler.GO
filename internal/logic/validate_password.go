package logic

import (
    "errors"
)

// ValidatePassword checks whether the given password satisfies the compatibility policy.
// Policy (see docs/spec/procedures/logic/validate_password.md):
//   * Length must be between 8 and 16 Unicode code points (runes) inclusive.
//   * At least two of the following character classes must be present:
//       - lower case letters (unicode.IsLower)
//       - upper case letters (unicode.IsUpper)
//       - digits (unicode.IsDigit)
//       - special characters (runes that are not letters nor digits; spaces count as special).
//   * Control characters are rejected.
// Returns nil on success, otherwise an error describing the validation failure.
func ValidatePassword(password string) error {
    // TODO: implement validation according to specification.
    return errors.New("not implemented")
}
