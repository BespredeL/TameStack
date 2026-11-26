package i18n

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

// Args holds named placeholder values for translation interpolation (e.g. "{name}").
type Args map[string]any

var (
	mu           sync.RWMutex
	activeLang   = "ru"
	dictionaries = map[string]map[string]string{}
)

func init() {
	entries, err := localeFS.ReadDir("locales")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		lang := strings.TrimSuffix(entry.Name(), ".json")
		raw, err := localeFS.ReadFile(path.Join("locales", entry.Name()))
		if err != nil {
			continue
		}
		var tree map[string]any
		if err := json.Unmarshal(raw, &tree); err != nil {
			continue
		}
		flat := make(map[string]string)
		flatten("", tree, flat)
		dictionaries[lang] = flat
	}
}

func flatten(prefix string, node map[string]any, out map[string]string) {
	for k, v := range node {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		switch val := v.(type) {
		case string:
			out[fullKey] = val
		case map[string]any:
			flatten(fullKey, val, out)
		}
	}
}

// NormalizeLang normalizes a locale string and falls back to "ru" if unknown.
func NormalizeLang(lang string) string {
	trimmed := strings.ToLower(strings.TrimSpace(lang))
	if trimmed == "" {
		return "ru"
	}
	mu.RLock()
	_, ok := dictionaries[trimmed]
	mu.RUnlock()
	if ok {
		return trimmed
	}
	if strings.HasPrefix(trimmed, "en") {
		return "en"
	}
	return "ru"
}

// SetLanguage updates the active backend language used by Tr/Err/Errf.
func SetLanguage(lang string) {
	norm := NormalizeLang(lang)
	mu.Lock()
	activeLang = norm
	mu.Unlock()
}

// GetLanguage returns the currently active backend language.
func GetLanguage() string {
	mu.RLock()
	lang := activeLang
	mu.RUnlock()
	return lang
}

// SupportedLanguages returns the sorted list of available locale codes.
func SupportedLanguages() []string {
	mu.RLock()
	defer mu.RUnlock()
	langs := make([]string, 0, len(dictionaries))
	for k := range dictionaries {
		langs = append(langs, k)
	}
	sort.Strings(langs)
	return langs
}

// T translates key for the given language with optional {placeholder} arguments.
func T(lang, key string, args ...Args) string {
	mu.RLock()
	var tmpl string
	var found bool
	if dict, ok := dictionaries[strings.ToLower(strings.TrimSpace(lang))]; ok {
		tmpl, found = dict[key]
	}
	if !found {
		if dict, ok := dictionaries["en"]; ok {
			tmpl, found = dict[key]
		}
	}
	if !found {
		if dict, ok := dictionaries["ru"]; ok {
			tmpl, found = dict[key]
		}
	}
	mu.RUnlock()

	if !found {
		return key
	}

	if len(args) > 0 && args[0] != nil {
		for k, v := range args[0] {
			tmpl = strings.ReplaceAll(tmpl, "{"+k+"}", fmt.Sprint(v))
		}
	}
	return tmpl
}

// Tr translates key using the currently active backend language.
func Tr(key string, args ...Args) string {
	return T(GetLanguage(), key, args...)
}

// Err returns a localized error for the active backend language.
func Err(key string, args ...Args) error {
	return errors.New(Tr(key, args...))
}

// Errf returns a localized error wrapping an underlying error with %w.
func Errf(key string, args Args, wrapErr error) error {
	if wrapErr == nil {
		return Err(key, args)
	}
	return fmt.Errorf("%s: %w", Tr(key, args), wrapErr)
}
