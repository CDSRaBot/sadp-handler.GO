package logic_test

import (
	"errors"
	"testing"

	"github.com/CDSRaBot/sadp-handler.GO/internal/logic"
)

// Тесты, проверяющие, что функция возвращает ошибку в недопустимых случаях.
func TestValidatePassword_Errors(t *testing.T) {
	cases := []struct {
		name    string
		pwd     string
		wantErr bool
	}{
		{"empty password", "", true},
		{"too short", "Ab1!", true},              // length 4
		{"too long", "Abcdefghijklmnop1!", true}, // 18 chars
		{"only lower", "abcdefgh", true},
		{"only upper", "ABCDEFGH", true},
		{"only digit", "12345678", true},
		{"only special", "!@#$%^&*", true},
		{"lower + upper", "abCDefgh", false},
		{"lower + digit", "ab12cdef", false},
		{"lower + special", "ab!@cdef", false},
		{"upper + digit", "AB12CDEF", false},
		{"upper + special", "AB!@CDEF", false},
		{"digit + special", "12!@34AB", false},
		{"unicode letters", "ПримерПароля", true},
		{"unicode letters + digit", "Пароль123", true},
		{"accented letter", "Abcdefé1", true},
		{"unicode digit", "Abcdef١!", true},
		{"unicode symbol", "Abcdef1€", true},
		{"space as special", "Abcdef1 ", true},
		{"ascii specials first range", "Abcdef1!/", false},
		{"ascii specials second range", "Abcdef1:@", false},
		{"ascii specials third range", "Abcdef1[\\]^_`", false},
		{"ascii specials fourth range", "Abcdef1{~", false},
		{"control char", "Abcdef\x00", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := logic.ValidatePassword(tc.pwd)
			if (err != nil) != tc.wantErr {
				t.Fatalf("expected error=%v, got %v", tc.wantErr, err)
			}
			if tc.wantErr {
				var ve *logic.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("expected *logic.ValidationError, got %T", err)
				}
			}
		})
	}
}

// Тесты, покрывающие граничные длины и все четыре класса символов.
func TestValidatePassword_BoundariesAndAllClasses(t *testing.T) {
	cases := []struct {
		name    string
		pwd     string
		wantErr bool
	}{
		{"len=7", "Abc1!de", true},               // 7 ASCII characters
		{"len=8 ok", "Abc1!def", false},          // ровно 8 – успешно
		{"len=16 ok", "Abcdef123456!@#$", false}, // ровно 16 – успешно
		{"len=17", "Abcdef123456!@#$x", true},    // 17 ASCII characters
		{"all four classes", "Abc1!def", false},  // lower, upper, digit, special
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := logic.ValidatePassword(tc.pwd)
			if (err != nil) != tc.wantErr {
				t.Fatalf("expected error=%v, got %v", tc.wantErr, err)
			}
		})
	}
}
