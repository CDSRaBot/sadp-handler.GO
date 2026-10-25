package app

import (
	"context"
	"errors"
	"testing"
	"testing/fstest"
)

func TestLocalizationServiceLoadsAndSwitchesLocales(t *testing.T) {
	files := fstest.MapFS{
		"locales/ru.json": &fstest.MapFile{Data: []byte(`{"ui.help":"помощь","ui.version":"версия"}`)},
	}
	service, err := NewLocalizationService(files, DefaultEnglishCatalog())
	if err != nil {
		t.Fatal(err)
	}
	locales, err := service.ListLocales(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(locales) != 2 || locales[0] != "en" || locales[1] != "ru" {
		t.Fatalf("locales = %#v", locales)
	}
	if err := service.SwitchLocale(context.Background(), "ru"); err != nil {
		t.Fatal(err)
	}
	if state := service.Active(); state.Code != "ru" || state.Messages["ui.help"] != "помощь" {
		t.Fatalf("active state = %#v", state)
	}
}

func TestLocalizationServicePropagatesCancellation(t *testing.T) {
	service, err := NewLocalizationService(fstest.MapFS{}, DefaultEnglishCatalog())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.SwitchLocale(ctx, "en"); !errors.Is(err, context.Canceled) {
		t.Fatalf("SwitchLocale error = %v", err)
	}
	if _, err := service.ListLocales(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListLocales error = %v", err)
	}
}
