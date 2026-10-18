// Package i18n provides the shared go-i18n Bundle, catalog loading, and
// request-context helpers used by middleware and handlers to emit localized
// user-facing messages.
//
// Operator-facing logs (slog) are NOT localized; only strings that travel
// back to the client as an HTTP response body pass through the Localizer.
package i18n

import (
	"embed"
	"fmt"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

// DefaultLanguage is the fallback used when a request does not express a
// preference (or the expressed preference does not match any catalog).
var DefaultLanguage = language.English

//go:embed translations/*.toml
var translationsFS embed.FS

// NewBundle constructs a go-i18n Bundle with every embedded catalog loaded.
// The default language is registered first so unresolved keys fall back to it.
func NewBundle() (*i18n.Bundle, error) {
	bundle := i18n.NewBundle(DefaultLanguage)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)

	entries, err := translationsFS.ReadDir("translations")
	if err != nil {
		return nil, fmt.Errorf("read embedded translations dir: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".toml" {
			continue
		}
		data, err := translationsFS.ReadFile(filepath.Join("translations", name))
		if err != nil {
			return nil, fmt.Errorf("read embedded catalog %s: %w", name, err)
		}
		if _, err := bundle.ParseMessageFileBytes(data, name); err != nil {
			return nil, fmt.Errorf("parse catalog %s: %w", name, err)
		}
	}
	return bundle, nil
}

// MustNewBundle is the panic-on-failure variant for startup paths.
// Bundle construction only fails on developer error (bad embedded catalog),
// so crashing during server initialization is the right behaviour.
func MustNewBundle() *i18n.Bundle {
	b, err := NewBundle()
	if err != nil {
		panic(fmt.Sprintf("i18n: bundle init failed: %v", err))
	}
	return b
}
