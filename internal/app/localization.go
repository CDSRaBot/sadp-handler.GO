package app

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/CDSRaBot/sadp-handler.GO/internal/i18n"
)

var defaultEnglishCatalog = map[string]string{
	"ui.help":    "discover, activate, network, language, help, version, quit",
	"ui.version": "version",
}

type LocalizationService struct {
	filesystem fs.FS
	english    map[string]string
	active     atomic.Pointer[i18n.LocaleState]
}

func NewLocalizationService(filesystem fs.FS, english map[string]string) (*LocalizationService, error) {
	if filesystem == nil {
		return nil, errors.New("locale filesystem is nil")
	}
	merged, err := i18n.MergeLocaleCatalog(english, english)
	if err != nil {
		return nil, err
	}
	service := &LocalizationService{filesystem: filesystem, english: cloneCatalog(english)}
	service.active.Store(&i18n.LocaleState{Code: "en", Messages: merged})
	return service, nil
}

func DefaultEnglishCatalog() map[string]string {
	return cloneCatalog(defaultEnglishCatalog)
}

func (s *LocalizationService) ListLocales(ctx context.Context) ([]string, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	return i18n.ListLocales(s.filesystem)
}

func (s *LocalizationService) SwitchLocale(ctx context.Context, code string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	catalog, err := i18n.LoadLocale(s.filesystem, code, s.english)
	if err != nil {
		return err
	}
	if err := contextError(ctx); err != nil {
		return err
	}
	return i18n.SwitchLocale(&s.active, i18n.LocaleState{Code: code, Messages: catalog})
}

func (s *LocalizationService) ExportLocaleTemplate(ctx context.Context, destination string) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if strings.TrimSpace(destination) == "" {
		return errors.New("destination is empty")
	}
	if filepath.Clean(destination) != destination {
		return errors.New("destination contains unsafe path components")
	}
	return i18n.ExportLocaleTemplate(s.english, destination)
}

func (s *LocalizationService) Active() i18n.LocaleState {
	state := s.active.Load()
	if state == nil {
		return i18n.LocaleState{}
	}
	return i18n.LocaleState{Code: state.Code, Messages: cloneCatalog(state.Messages)}
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("context is nil")
	}
	return ctx.Err()
}

func cloneCatalog(catalog map[string]string) map[string]string {
	copy := make(map[string]string, len(catalog))
	for key, value := range catalog {
		copy[key] = value
	}
	return copy
}
