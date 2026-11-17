package route

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"unicode/utf8"
)

type ISAPISessionLoginParams struct {
	Username     string
	Password     string
	Salt         string
	Challenge    string
	Irreversible bool
	Iterations   int
}

// BuildISAPISessionLoginProof implements the captured session-login proof
// only. The caller must establish the device profile and challenge lifetime.
func BuildISAPISessionLoginProof(params ISAPISessionLoginParams) (string, error) {
	if params.Username == "" || params.Password == "" || params.Challenge == "" ||
		(params.Irreversible && params.Salt == "") || params.Iterations < 2 ||
		params.Iterations > MaxISAPISecurityIterations {
		return "", &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI session login inputs are invalid")}
	}
	for _, value := range []string{params.Username, params.Password, params.Salt, params.Challenge} {
		if !utf8.ValidString(value) || len(value) > MaxISAPIUserFieldBytes {
			return "", &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI session login inputs are invalid")}
		}
	}
	var proof string
	var remaining int
	if params.Irreversible {
		proof = isapiSHA256Hex(isapiSHA256Hex(params.Username+params.Salt+params.Password) + params.Challenge)
		remaining = params.Iterations - 2
	} else {
		proof = isapiSHA256Hex(params.Password) + params.Challenge
		remaining = params.Iterations - 1
	}
	for i := 0; i < remaining; i++ {
		proof = isapiSHA256Hex(proof)
	}
	return proof, nil
}

func isapiSHA256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
