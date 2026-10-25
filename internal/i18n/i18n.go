package i18n

import (
	"errors"
	"io/fs"
	"sync/atomic"
)

var (
	ErrNoExternalLocales = errors.New("no external locales")
	ErrCatalogTooLarge   = errors.New("locale catalog exceeds 1 MiB")
)

type LocaleState struct {
	Code     string
	Messages map[string]string
}

func ValidateLocaleCode(string) error { return errors.New("not implemented") }

func DecodeLocaleCatalog([]byte) (map[string]string, error) {
	return nil, errors.New("not implemented")
}

func MergeLocaleCatalog(map[string]string, map[string]string) (map[string]string, error) {
	return nil, errors.New("not implemented")
}

func LoadLocale(fs.FS, string, map[string]string) (map[string]string, error) {
	return nil, errors.New("not implemented")
}

func ResolveLocale(string, string, bool, bool) (string, error) {
	return "", errors.New("not implemented")
}

func ListLocales(fs.FS) ([]string, error) {
	return nil, errors.New("not implemented")
}

func ExportLocaleTemplate(map[string]string, string) error {
	return errors.New("not implemented")
}

func SwitchLocale(*atomic.Pointer[LocaleState], LocaleState) error {
	return errors.New("not implemented")
}
