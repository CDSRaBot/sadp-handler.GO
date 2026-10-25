package i18n

import (
	"errors"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
)

func TestValidateLocaleCode(t *testing.T) {
	for _, code := range []string{"en", "en-US", "pt_BR", "zh2"} {
		t.Run(code, func(t *testing.T) {
			if err := ValidateLocaleCode(code); err != nil {
				t.Fatalf("ValidateLocaleCode(%q) returned error: %v", code, err)
			}
		})
	}
	for _, code := range []string{"", " ", "en US", ".", "..", "../en", `en\US`, "en\x00", "рус"} {
		t.Run("reject_"+strings.ReplaceAll(code, "\x00", "NUL"), func(t *testing.T) {
			if err := ValidateLocaleCode(code); err == nil {
				t.Fatalf("ValidateLocaleCode(%q) returned nil", code)
			} else if code != "" && strings.TrimSpace(code) != "" && strings.Contains(err.Error(), code) {
				t.Fatalf("error disclosed locale input %q: %v", code, err)
			}
		})
	}
}

func TestDecodeLocaleCatalog(t *testing.T) {
	valid := []string{`{"hello":"Hello","bye":"Goodbye"}`, `{ "hello": "Hello" }`}
	for _, data := range valid {
		t.Run("valid", func(t *testing.T) {
			got, err := DecodeLocaleCatalog([]byte(data))
			if err != nil || len(got) == 0 {
				t.Fatalf("DecodeLocaleCatalog() = %#v, %v", got, err)
			}
		})
	}
	for _, data := range []string{
		`{}`, `[]`, `null`, `{"hello":1}`, `{"hello":{"nested":"value"}}`,
		`{"hello":"ok","hello":"again"}`, `{"hello":"ok"} {"extra":"value"}`,
		`{" ":"value"}`, `{"key":" "}`, `{broken`,
	} {
		t.Run("reject", func(t *testing.T) {
			got, err := DecodeLocaleCatalog([]byte(data))
			if err == nil || got != nil {
				t.Fatalf("DecodeLocaleCatalog(%q) = %#v, %v; want nil and error", data, got, err)
			}
		})
	}
	if _, err := DecodeLocaleCatalog(make([]byte, 1<<20+1)); !errors.Is(err, ErrCatalogTooLarge) {
		t.Fatalf("oversized catalog error = %v, want ErrCatalogTooLarge", err)
	}
}

func TestMergeLocaleCatalog(t *testing.T) {
	english := map[string]string{"hello": "Hello", "bye": "Goodbye"}
	translated := map[string]string{"hello": "Привет"}
	got, err := MergeLocaleCatalog(english, translated)
	if err != nil {
		t.Fatalf("MergeLocaleCatalog returned error: %v", err)
	}
	want := map[string]string{"hello": "Привет", "bye": "Goodbye"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("merged catalog = %#v, want %#v", got, want)
	}
	got["hello"] = "changed"
	if english["hello"] != "Hello" || translated["hello"] != "Привет" {
		t.Fatal("MergeLocaleCatalog aliased an input map")
	}
	if _, err := MergeLocaleCatalog(english, map[string]string{"unknown": "value"}); err == nil {
		t.Fatal("unknown translation key was accepted")
	}
	for _, tc := range []struct {
		name       string
		english    map[string]string
		translated map[string]string
	}{
		{"nil english", nil, translated},
		{"empty english", map[string]string{}, translated},
		{"nil translated", english, nil},
		{"invalid english", map[string]string{"hello": " "}, translated},
		{"invalid translated", english, map[string]string{"hello": ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := MergeLocaleCatalog(tc.english, tc.translated); err == nil || got != nil {
				t.Fatalf("MergeLocaleCatalog() = %#v, %v; want nil and error", got, err)
			}
		})
	}
}

func TestLoadLocale(t *testing.T) {
	english := map[string]string{"hello": "Hello", "bye": "Goodbye"}
	files := fstest.MapFS{
		"locales/ru.json": &fstest.MapFile{Data: []byte(`{"hello":"Привет"}`)},
	}
	got, err := LoadLocale(files, "ru", english)
	if err != nil {
		t.Fatalf("LoadLocale returned error: %v", err)
	}
	if !reflect.DeepEqual(got, map[string]string{"hello": "Привет", "bye": "Goodbye"}) {
		t.Fatalf("loaded catalog = %#v", got)
	}
	englishOnly, err := LoadLocale(fstest.MapFS{}, "en", english)
	if err != nil || !reflect.DeepEqual(englishOnly, english) {
		t.Fatalf("embedded English load = %#v, %v", englishOnly, err)
	}
	for _, code := range []string{"missing", "bad"} {
		t.Run(code, func(t *testing.T) {
			fsys := fstest.MapFS{}
			if code == "bad" {
				fsys["locales/bad.json"] = &fstest.MapFile{Data: []byte(`{"hello":`)}
			}
			got, err := LoadLocale(fsys, code, english)
			if err == nil || got != nil {
				t.Fatalf("LoadLocale() = %#v, %v; want nil and error", got, err)
			}
		})
	}
	oversized := fstest.MapFS{"locales/ru.json": &fstest.MapFile{Data: make([]byte, 1<<20+1)}}
	if _, err := LoadLocale(oversized, "ru", english); !errors.Is(err, ErrCatalogTooLarge) {
		t.Fatalf("oversized load error = %v, want ErrCatalogTooLarge", err)
	}
}

func TestResolveLocale(t *testing.T) {
	tests := []struct {
		name, settings, cli, want string
		hasSettings, hasCLI       bool
	}{
		{"default", "", "", "en", false, false},
		{"settings", "ru", "", "ru", true, false},
		{"cli", "ru", "de", "de", true, true},
		{"invalid lower priority ignored", "bad/locale", "de", "de", true, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveLocale(tc.settings, tc.cli, tc.hasSettings, tc.hasCLI)
			if err != nil || got != tc.want {
				t.Fatalf("ResolveLocale() = %q, %v; want %q, nil", got, err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name, settings, cli string
		hasSettings, hasCLI bool
	}{{"empty settings", "", "", true, false}, {"invalid cli", "ru", "bad/locale", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := ResolveLocale(tc.settings, tc.cli, tc.hasSettings, tc.hasCLI); err == nil || got != "" {
				t.Fatalf("ResolveLocale() = %q, %v; want empty result and error", got, err)
			}
		})
	}
}

func TestListLocales(t *testing.T) {
	fsys := fstest.MapFS{
		"locales/zh.json":        &fstest.MapFile{Data: []byte(`not validated here`)},
		"locales/en.json":        &fstest.MapFile{Data: []byte(`ignored`)},
		"locales/readme.txt":     &fstest.MapFile{Data: []byte(`ignored`)},
		"locales/nested/de.json": &fstest.MapFile{Data: []byte(`ignored`)},
	}
	got, err := ListLocales(fsys)
	if err != nil || !reflect.DeepEqual(got, []string{"en", "zh"}) {
		t.Fatalf("ListLocales() = %#v, %v", got, err)
	}
	got[0] = "changed"
	again, err := ListLocales(fstest.MapFS{})
	if !errors.Is(err, ErrNoExternalLocales) || !reflect.DeepEqual(again, []string{"en"}) {
		t.Fatalf("empty ListLocales() = %#v, %v", again, err)
	}
	if _, err := ListLocales(nil); err == nil {
		t.Fatal("nil filesystem was accepted")
	}
	if _, err := ListLocales(fstest.MapFS{"locales/bad!.json": &fstest.MapFile{}}); err == nil {
		t.Fatal("invalid locale filename was accepted")
	}
}

func TestExportLocaleTemplate(t *testing.T) {
	dir := t.TempDir()
	destination := dir + "/en.json"
	english := map[string]string{"z": "last", "a": "first"}
	if err := ExportLocaleTemplate(english, destination); err != nil {
		t.Fatalf("ExportLocaleTemplate returned error: %v", err)
	}
	data, err := fs.ReadFile(osDirFS(dir), "en.json")
	if err != nil || string(data) != "{\n  \"a\": \"first\",\n  \"z\": \"last\"\n}\n" {
		t.Fatalf("exported template = %q, %v", data, err)
	}
	if err := ExportLocaleTemplate(english, destination); err == nil {
		t.Fatal("existing destination was overwritten")
	}
	if err := ExportLocaleTemplate(nil, dir+"/invalid.json"); err == nil {
		t.Fatal("invalid catalog was accepted")
	}
}

func TestSwitchLocale(t *testing.T) {
	var active atomic.Pointer[LocaleState]
	previous := &LocaleState{Code: "en", Messages: map[string]string{"hello": "Hello"}}
	active.Store(previous)
	candidate := LocaleState{Code: "ru", Messages: map[string]string{"hello": "Привет"}}
	if err := SwitchLocale(&active, candidate); err != nil {
		t.Fatalf("SwitchLocale returned error: %v", err)
	}
	candidate.Messages["hello"] = "changed"
	if got := active.Load(); got.Code != "ru" || got.Messages["hello"] != "Привет" {
		t.Fatalf("published state aliases candidate: %#v", got)
	}
	before := active.Load()
	if err := SwitchLocale(&active, LocaleState{Code: "bad/locale", Messages: map[string]string{"hello": "x"}}); err == nil {
		t.Fatal("invalid state was accepted")
	}
	if active.Load() != before {
		t.Fatal("active state changed after failed switch")
	}
	if err := SwitchLocale(nil, candidate); err == nil {
		t.Fatal("nil atomic pointer was accepted")
	}
}

func TestSwitchLocalePublishesCompletePairs(t *testing.T) {
	var active atomic.Pointer[LocaleState]
	active.Store(&LocaleState{Code: "en", Messages: map[string]string{"key": "English"}})
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					state := active.Load()
					if state == nil || state.Messages["key"] == "" {
						t.Error("reader observed incomplete state")
						return
					}
				}
			}
		}()
	}
	for i := 0; i < 100; i++ {
		code, message := "en", "English"
		if i%2 == 1 {
			code, message = "ru", "Русский"
		}
		if err := SwitchLocale(&active, LocaleState{Code: code, Messages: map[string]string{"key": message}}); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	wg.Wait()
}

func osDirFS(dir string) fs.FS {
	return os.DirFS(dir)
}
