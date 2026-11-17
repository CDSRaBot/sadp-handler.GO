package route

import (
	"crypto/aes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

const (
	MaxISAPISecurityIterations = 100000
	MaxISAPIRecoveryFields     = 16
)

type ISAPIRecoveryCryptoParams struct {
	Username     string
	Password     string
	Salt         string
	Irreversible bool
	Iterations   int
}

type ISAPIRecoveryEncryptedFields struct {
	IV     string
	Values []string
}

// EncryptISAPIRecoveryFields implements only the captured security=1
// question-configuration format. Callers must verify the device profile and
// establish an authenticated operation before using the result.
func EncryptISAPIRecoveryFields(params ISAPIRecoveryCryptoParams, values []string) (ISAPIRecoveryEncryptedFields, error) {
	return encryptISAPIRecoveryFields(rand.Reader, params, values)
}

func encryptISAPIRecoveryFields(random io.Reader, params ISAPIRecoveryCryptoParams, values []string) (ISAPIRecoveryEncryptedFields, error) {
	if random == nil || params.Username == "" || params.Password == "" ||
		params.Irreversible && params.Salt == "" || params.Iterations < 1 ||
		params.Iterations > MaxISAPISecurityIterations || len(values) == 0 || len(values) > MaxISAPIRecoveryFields {
		return ISAPIRecoveryEncryptedFields{}, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI recovery encryption inputs are invalid")}
	}
	for _, value := range append([]string{params.Username, params.Password, params.Salt}, values...) {
		if !utf8.ValidString(value) || len(value) > MaxISAPIUserFieldBytes {
			return ISAPIRecoveryEncryptedFields{}, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI recovery encryption inputs are invalid")}
		}
	}
	key := deriveISAPIRecoveryKey(params)
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(random, iv); err != nil {
		return ISAPIRecoveryEncryptedFields{}, &Error{Kind: ErrorTransport, Err: errors.New("ISAPI recovery IV generation failed")}
	}
	escape := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	result := ISAPIRecoveryEncryptedFields{IV: hex.EncodeToString(iv), Values: make([]string, 0, len(values))}
	for _, value := range values {
		ciphertext, err := encryptISAPIBase64CBC(key[:], iv, escape.Replace(value))
		if err != nil {
			return ISAPIRecoveryEncryptedFields{}, err
		}
		result.Values = append(result.Values, hex.EncodeToString(ciphertext))
	}
	return result, nil
}

func deriveISAPIRecoveryKey(params ISAPIRecoveryCryptoParams) [16]byte {
	base := params.Password
	if params.Irreversible {
		base = isapiSHA256Hex(params.Username + params.Salt + params.Password)
	}
	hash := sha256.Sum256([]byte(base + "AaBbCcDd1234!@#$"))
	for iteration := 1; iteration < params.Iterations; iteration++ {
		hash = sha256.Sum256([]byte(hex.EncodeToString(hash[:])))
	}
	var key [16]byte
	copy(key[:], hash[:len(key)])
	return key
}
