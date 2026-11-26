package i18n

import (
	"errors"
	"strings"
	"testing"
)

func TestLocaleParity(t *testing.T) {
	langs := SupportedLanguages()
	if len(langs) < 2 {
		t.Fatalf("expected at least 2 locales (ru, en), got %v", langs)
	}

	ruKeys := dictionaries["ru"]
	enKeys := dictionaries["en"]
	if len(ruKeys) == 0 || len(enKeys) == 0 {
		t.Fatal("ru and en dictionaries must not be empty")
	}

	for k := range ruKeys {
		if _, ok := enKeys[k]; !ok {
			t.Errorf("key %q exists in ru.json but is missing in en.json", k)
		}
	}
	for k := range enKeys {
		if _, ok := ruKeys[k]; !ok {
			t.Errorf("key %q exists in en.json but is missing in ru.json", k)
		}
	}
}

func TestTranslateAndInterpolation(t *testing.T) {
	gotEN := T("en", "users.taskCreateTitle", Args{"username": "alice"})
	if gotEN != "Create user alice" {
		t.Errorf("unexpected EN translation: %q", gotEN)
	}

	gotRU := T("ru", "users.taskCreateTitle", Args{"username": "alice"})
	if gotRU != "Создание пользователя alice" {
		t.Errorf("unexpected RU translation: %q", gotRU)
	}

	missing := T("en", "nonexistent.key")
	if missing != "nonexistent.key" {
		t.Errorf("expected key fallback, got %q", missing)
	}
}

func TestActiveLanguageAndErrf(t *testing.T) {
	prev := GetLanguage()
	defer SetLanguage(prev)

	SetLanguage("en")
	if GetLanguage() != "en" {
		t.Fatalf("expected active lang 'en', got %q", GetLanguage())
	}

	baseErr := errors.New("boom")
	wrapped := Errf("common.metricsCollectError", nil, baseErr)
	if !errors.Is(wrapped, baseErr) {
		t.Errorf("expected wrapped error to match baseErr")
	}
	if !strings.Contains(wrapped.Error(), "failed to collect metrics") {
		t.Errorf("expected EN error message, got %q", wrapped.Error())
	}
}
