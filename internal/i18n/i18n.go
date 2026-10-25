package i18n

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
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

func ValidateLocaleCode(code string) error {
	if code == "" {
		return errors.New("locale code is empty")
	}
	for i := 0; i < len(code); i++ {
		char := code[i]
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return errors.New("invalid locale code")
	}
	return nil
}

func DecodeLocaleCatalog(data []byte) (map[string]string, error) {
	if len(data) > 1<<20 {
		return nil, ErrCatalogTooLarge
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return nil, fmt.Errorf("decode locale catalog: %w", err)
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return nil, errors.New("locale catalog must be a JSON object")
	}
	catalog := make(map[string]string)
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, fmt.Errorf("decode locale key: %w", err)
		}
		key, ok := keyToken.(string)
		if !ok || !validMessagePart(key) {
			return nil, errors.New("locale catalog contains an invalid key")
		}
		if _, exists := catalog[key]; exists {
			return nil, errors.New("locale catalog contains a duplicate key")
		}
		var value string
		if err := decoder.Decode(&value); err != nil {
			return nil, fmt.Errorf("decode locale value: %w", err)
		}
		if !validMessagePart(value) {
			return nil, errors.New("locale catalog contains an invalid value")
		}
		catalog[key] = value
	}
	if _, err := decoder.Token(); err != nil {
		return nil, fmt.Errorf("close locale catalog: %w", err)
	}
	if len(catalog) == 0 {
		return nil, errors.New("locale catalog is empty")
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return nil, errors.New("locale catalog contains trailing data")
		}
		return nil, fmt.Errorf("read locale catalog tail: %w", err)
	}
	return catalog, nil
}

func MergeLocaleCatalog(english, translated map[string]string) (map[string]string, error) {
	if err := validateCatalog(english); err != nil {
		return nil, fmt.Errorf("validate English catalog: %w", err)
	}
	if translated == nil {
		return nil, errors.New("translated catalog is nil")
	}
	for key, value := range translated {
		if !validMessagePart(key) || !validMessagePart(value) {
			return nil, errors.New("translated catalog contains an invalid entry")
		}
		if _, ok := english[key]; !ok {
			return nil, errors.New("translated catalog contains an unknown key")
		}
	}
	merged := make(map[string]string, len(english))
	for key, value := range english {
		merged[key] = value
	}
	for key, value := range translated {
		merged[key] = value
	}
	return merged, nil
}

func LoadLocale(filesystem fs.FS, langCode string, english map[string]string) (result map[string]string, resultErr error) {
	if filesystem == nil {
		return nil, errors.New("locale filesystem is nil")
	}
	if err := ValidateLocaleCode(langCode); err != nil {
		return nil, fmt.Errorf("validate locale: %w", err)
	}
	if err := validateCatalog(english); err != nil {
		return nil, fmt.Errorf("validate English catalog: %w", err)
	}
	if langCode == "en" {
		return MergeLocaleCatalog(english, english)
	}
	file, err := filesystem.Open("locales/" + langCode + ".json")
	if err != nil {
		return nil, fmt.Errorf("open locale catalog: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			result = nil
			resultErr = errors.Join(resultErr, fmt.Errorf("close locale catalog: %w", err))
		}
	}()
	data, err := io.ReadAll(io.LimitReader(file, 1<<20+1))
	if err != nil {
		return nil, fmt.Errorf("read locale catalog: %w", err)
	}
	if len(data) > 1<<20 {
		return nil, ErrCatalogTooLarge
	}
	translated, err := DecodeLocaleCatalog(data)
	if err != nil {
		return nil, fmt.Errorf("decode locale %s: %w", langCode, err)
	}
	return MergeLocaleCatalog(english, translated)
}

func ResolveLocale(settingsLocale, cliLocale string, hasSettings, hasCLI bool) (string, error) {
	code := "en"
	if hasSettings {
		code = settingsLocale
	}
	if hasCLI {
		code = cliLocale
	}
	if err := ValidateLocaleCode(code); err != nil {
		return "", fmt.Errorf("resolve locale: %w", err)
	}
	return code, nil
}

func ListLocales(filesystem fs.FS) ([]string, error) {
	if filesystem == nil {
		return nil, errors.New("locale filesystem is nil")
	}
	entries, err := fs.ReadDir(filesystem, "locales")
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return []string{"en"}, ErrNoExternalLocales
		}
		return nil, fmt.Errorf("list locale directory: %w", err)
	}
	locales := map[string]struct{}{"en": {}}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		code := strings.TrimSuffix(entry.Name(), ".json")
		if code == "en" {
			continue
		}
		if err := ValidateLocaleCode(code); err != nil {
			return nil, fmt.Errorf("validate locale filename: %w", err)
		}
		locales[code] = struct{}{}
	}
	result := make([]string, 0, len(locales))
	for code := range locales {
		result = append(result, code)
	}
	sort.Strings(result)
	if len(result) == 1 {
		return result, ErrNoExternalLocales
	}
	return result, nil
}

func ExportLocaleTemplate(english map[string]string, destination string) error {
	if destination == "" {
		return errors.New("destination is empty")
	}
	if err := validateCatalog(english); err != nil {
		return fmt.Errorf("validate English catalog: %w", err)
	}
	data, err := json.MarshalIndent(english, "", "  ")
	if err != nil {
		return fmt.Errorf("encode locale template: %w", err)
	}
	data = append(data, '\n')
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return fmt.Errorf("create locale template: %w", err)
	}
	created := true
	cleanup := func() error {
		if !created {
			return nil
		}
		return os.Remove(destination)
	}
	if n, err := file.Write(data); err != nil || n != len(data) {
		if err == nil {
			err = io.ErrShortWrite
		}
		closeErr := file.Close()
		return errors.Join(fmt.Errorf("write locale template: %w", err), closeErr, cleanup())
	}
	if err := file.Close(); err != nil {
		return errors.Join(fmt.Errorf("close locale template: %w", err), cleanup())
	}
	created = false
	return nil
}

func SwitchLocale(active *atomic.Pointer[LocaleState], candidate LocaleState) error {
	if active == nil {
		return errors.New("active locale pointer is nil")
	}
	if err := ValidateLocaleCode(candidate.Code); err != nil {
		return fmt.Errorf("validate candidate locale: %w", err)
	}
	if err := validateCatalog(candidate.Messages); err != nil {
		return fmt.Errorf("validate candidate catalog: %w", err)
	}
	messages := make(map[string]string, len(candidate.Messages))
	for key, value := range candidate.Messages {
		messages[key] = value
	}
	active.Store(&LocaleState{Code: candidate.Code, Messages: messages})
	return nil
}

func validMessagePart(value string) bool {
	return value != "" && strings.TrimSpace(value) != ""
}

func validateCatalog(catalog map[string]string) error {
	if len(catalog) == 0 {
		return errors.New("catalog is empty")
	}
	for key, value := range catalog {
		if !validMessagePart(key) || !validMessagePart(value) {
			return errors.New("catalog contains an invalid entry")
		}
	}
	return nil
}
