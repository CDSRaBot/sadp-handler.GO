package logic

import (
	"crypto/sha256"
	"encoding/hex"
)

// CalculateHash calculates the challenge-response digest required by the target device protocol (P3.5).
// It computes SHA-256 over the UTF-8 bytes of password + challenge (in that exact order)
// and returns the digest as a 64-character lowercase hexadecimal string.
//
// An error is returned if either password or challenge is empty.
// In accordance with security policy, errors never disclose the password, challenge, or digest.
func CalculateHash(password, challenge string) (string, error) {
	if password == "" {
		return "", ErrEmptyPassword
	}
	if challenge == "" {
		return "", ErrEmptyChallenge
	}

	h := sha256.New()
	h.Write([]byte(password))
	h.Write([]byte(challenge))
	digest := h.Sum(nil)

	return hex.EncodeToString(digest), nil
}
