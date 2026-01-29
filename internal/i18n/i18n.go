package i18n

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Language represents a supported language
type Language string

const (
	English  Language = "en"
	Korean   Language = "ko"
	Japanese Language = "ja"
	Chinese  Language = "zh"
)

// Translator handles internationalization
type Translator struct {
	mu           sync.RWMutex
	currentLang  Language
	translations map[Language]map[string]string
	fallback     Language
}

// Global translator instance
var global *Translator
var once sync.Once

// Global returns the global translator instance
func Global() *Translator {
	once.Do(func() {
		global = NewTranslator()
		global.LoadBuiltin()
	})
	return global
}

// NewTranslator creates a new translator
func NewTranslator() *Translator {
	return &Translator{
		currentLang:  English,
		translations: make(map[Language]map[string]string),
		fallback:     English,
	}
}

// SetLanguage sets the current language
func (t *Translator) SetLanguage(lang Language) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.currentLang = lang
}

// GetLanguage returns the current language
func (t *Translator) GetLanguage() Language {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentLang
}

// T translates a key to the current language
func (t *Translator) T(key string, args ...any) string {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// Try current language
	if trans, ok := t.translations[t.currentLang]; ok {
		if val, ok := trans[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(val, args...)
			}
			return val
		}
	}

	// Try fallback language
	if trans, ok := t.translations[t.fallback]; ok {
		if val, ok := trans[key]; ok {
			if len(args) > 0 {
				return fmt.Sprintf(val, args...)
			}
			return val
		}
	}

	// Return key if not found
	return key
}

// LoadBuiltin loads built-in translations
func (t *Translator) LoadBuiltin() {
	t.LoadTranslations(English, englishTranslations)
	t.LoadTranslations(Korean, koreanTranslations)
	t.LoadTranslations(Japanese, japaneseTranslations)
	t.LoadTranslations(Chinese, chineseTranslations)
}

// LoadTranslations loads translations for a language
func (t *Translator) LoadTranslations(lang Language, translations map[string]string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.translations[lang] == nil {
		t.translations[lang] = make(map[string]string)
	}

	for k, v := range translations {
		t.translations[lang][k] = v
	}
}

// LoadFromFile loads translations from a JSON file
func (t *Translator) LoadFromFile(lang Language, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		return err
	}

	t.LoadTranslations(lang, translations)
	return nil
}

// LoadFromDir loads all translation files from a directory
func (t *Translator) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		// Extract language from filename (e.g., "en.json" -> "en")
		lang := Language(strings.TrimSuffix(entry.Name(), ".json"))
		path := filepath.Join(dir, entry.Name())

		if err := t.LoadFromFile(lang, path); err != nil {
			// Log error but continue loading other files
			continue
		}
	}

	return nil
}

// AvailableLanguages returns all available languages
func (t *Translator) AvailableLanguages() []Language {
	t.mu.RLock()
	defer t.mu.RUnlock()

	langs := make([]Language, 0, len(t.translations))
	for lang := range t.translations {
		langs = append(langs, lang)
	}
	return langs
}

// Shorthand global functions

// T translates a key using the global translator
func T(key string, args ...any) string {
	return Global().T(key, args...)
}

// SetLanguage sets the language for the global translator
func SetLanguage(lang Language) {
	Global().SetLanguage(lang)
}

// GetLanguage returns the current language
func GetLanguage() Language {
	return Global().GetLanguage()
}

// DetectLanguage attempts to detect language from environment
func DetectLanguage() Language {
	// Check LANG environment variable
	langEnv := os.Getenv("LANG")
	if langEnv == "" {
		langEnv = os.Getenv("LC_ALL")
	}
	if langEnv == "" {
		langEnv = os.Getenv("LC_MESSAGES")
	}

	langEnv = strings.ToLower(langEnv)

	switch {
	case strings.HasPrefix(langEnv, "ko"):
		return Korean
	case strings.HasPrefix(langEnv, "ja"):
		return Japanese
	case strings.HasPrefix(langEnv, "zh"):
		return Chinese
	default:
		return English
	}
}
