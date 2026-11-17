package route

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"unicode/utf8"
)

type ISAPIUserSecurity uint8

const (
	ISAPIUserSecurityAES128 ISAPIUserSecurity = 1
	ISAPIUserSecurityAES256 ISAPIUserSecurity = 2
	MaxISAPIUserFieldBytes                    = 4096
)

type ISAPIEncryptedField struct {
	Security   ISAPIUserSecurity
	IV         string
	Ciphertext string
}

// EncryptISAPIUserField implements the generic Base64/AES-CBC field format.
// The caller must independently verify the endpoint's schema and key source.
func EncryptISAPIUserField(scheme ISAPIUserSecurity, key []byte, value string) (ISAPIEncryptedField, error) {
	return encryptISAPIUserField(rand.Reader, scheme, key, value)
}

func encryptISAPIUserField(random io.Reader, scheme ISAPIUserSecurity, key []byte, value string) (ISAPIEncryptedField, error) {
	expectedKeySize := 0
	switch scheme {
	case ISAPIUserSecurityAES128:
		expectedKeySize = 16
	case ISAPIUserSecurityAES256:
		expectedKeySize = 32
	default:
		return ISAPIEncryptedField{}, &Error{Kind: ErrorUnsupported, Err: errors.New("ISAPI user security scheme is unsupported")}
	}
	if len(key) != expectedKeySize || !utf8.ValidString(value) || len(value) > MaxISAPIUserFieldBytes || random == nil {
		return ISAPIEncryptedField{}, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI user field input is invalid")}
	}
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(random, iv); err != nil {
		return ISAPIEncryptedField{}, &Error{Kind: ErrorTransport, Err: errors.New("ISAPI user field IV generation failed")}
	}
	ciphertext, err := encryptISAPIBase64CBC(key, iv, value)
	if err != nil {
		return ISAPIEncryptedField{}, err
	}
	return ISAPIEncryptedField{
		Security:   scheme,
		IV:         base64.StdEncoding.EncodeToString(iv),
		Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

func encryptISAPIBase64CBC(key, iv []byte, value string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil || len(iv) != aes.BlockSize {
		return nil, &Error{Kind: ErrorInvalidInput, Err: errors.New("ISAPI field key or IV is invalid")}
	}
	plaintext := []byte(base64.StdEncoding.EncodeToString([]byte(value)))
	padding := aes.BlockSize - len(plaintext)%aes.BlockSize
	for index := 0; index < padding; index++ {
		plaintext = append(plaintext, byte(padding))
	}
	ciphertext := make([]byte, len(plaintext))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, plaintext)
	return ciphertext, nil
}
