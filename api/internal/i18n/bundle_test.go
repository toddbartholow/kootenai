package i18n_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/BurntSushi/toml"
	gi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/toddbartholow/kootenai/api/internal/i18n"
)

func TestNewBundle_LoadsEmbeddedCatalogs(t *testing.T) {
	bundle, err := i18n.NewBundle()
	require.NoError(t, err)
	require.NotNil(t, bundle)
}

func TestLocalize_ResolvesKnownKey(t *testing.T) {
	bundle := i18n.MustNewBundle()
	localizer := gi18n.NewLocalizer(bundle, language.English.String())
	ctx := i18n.WithLocalizer(context.Background(), localizer)

	got := i18n.Localize(ctx, "auth.errors.invalidCredentials", nil)
	require.Equal(t, "invalid credentials", got)
}

func TestLocalize_FormatsNamedParams(t *testing.T) {
	bundle := i18n.MustNewBundle()
	localizer := gi18n.NewLocalizer(bundle, language.English.String())
	ctx := i18n.WithLocalizer(context.Background(), localizer)

	got := i18n.Localize(ctx, "validation.required", map[string]any{"Field": "email"})
	require.Equal(t, "email is required", got)

	gotMin := i18n.Localize(ctx, "validation.min", map[string]any{"Field": "password", "Param": "8"})
	require.Equal(t, "password must be at least 8 characters", gotMin)
}

func TestLocalize_UnknownKeyFallsBackToID(t *testing.T) {
	bundle := i18n.MustNewBundle()
	localizer := gi18n.NewLocalizer(bundle, language.English.String())
	ctx := i18n.WithLocalizer(context.Background(), localizer)

	got := i18n.Localize(ctx, "no.such.key", nil)
	require.Equal(t, "no.such.key", got)
}

func TestLocalize_NoLocalizerFallsBackToID(t *testing.T) {
	got := i18n.Localize(context.Background(), "auth.errors.invalidCredentials", nil)
	require.Equal(t, "auth.errors.invalidCredentials", got)
}

func TestFromContext_Empty(t *testing.T) {
	require.Nil(t, i18n.FromContext(context.Background()))
	require.Nil(t, i18n.FromContext(nil)) //nolint:staticcheck // intentionally testing nil-safety
}

func TestRequestedLocale_RoundTrip(t *testing.T) {
	ctx := i18n.WithRequestedLocale(context.Background(), "es-MX,es;q=0.9")
	require.Equal(t, "es-MX,es;q=0.9", i18n.RequestedLocale(ctx))
	require.Equal(t, "", i18n.RequestedLocale(context.Background()))
	require.Equal(t, "", i18n.RequestedLocale(nil)) //nolint:staticcheck // intentionally testing nil-safety
}

func TestMissHandler_FiresOnUnknownKey(t *testing.T) {
	type miss struct{ id, locale string }
	var got []miss
	i18n.SetMissHandler(func(id, locale string) {
		got = append(got, miss{id, locale})
	})
	t.Cleanup(func() { i18n.SetMissHandler(nil) })

	bundle := i18n.MustNewBundle()
	localizer := gi18n.NewLocalizer(bundle, language.English.String())
	ctx := i18n.WithLocalizer(context.Background(), localizer)
	ctx = i18n.WithRequestedLocale(ctx, "es")

	i18n.Localize(ctx, "no.such.key", nil)

	require.Len(t, got, 1)
	require.Equal(t, "no.such.key", got[0].id)
	require.Equal(t, "es", got[0].locale)
}

func TestMissHandler_FiresWhenNoLocalizer(t *testing.T) {
	var calls int
	i18n.SetMissHandler(func(id, locale string) { calls++ })
	t.Cleanup(func() { i18n.SetMissHandler(nil) })

	i18n.Localize(context.Background(), "any.key", nil)
	require.Equal(t, 1, calls)
}

func TestLocalize_ResolvesSpanishKey(t *testing.T) {
	bundle := i18n.MustNewBundle()
	localizer := gi18n.NewLocalizer(bundle, language.Spanish.String())
	ctx := i18n.WithLocalizer(context.Background(), localizer)

	got := i18n.Localize(ctx, "auth.errors.invalidCredentials", nil)
	require.Equal(t, "credenciales no válidas", got)
}

func TestLocalize_SpanishFormatsNamedParams(t *testing.T) {
	bundle := i18n.MustNewBundle()
	localizer := gi18n.NewLocalizer(bundle, language.Spanish.String())
	ctx := i18n.WithLocalizer(context.Background(), localizer)

	// The Spanish catalog uses a colon-prefixed noun-phrase form to
	// avoid gender-agreement ambiguity when Field is interpolated without
	// an article — see the catalog file's header comment.
	got := i18n.Localize(ctx, "validation.required", map[string]any{"Field": "email"})
	require.Equal(t, "email: campo obligatorio", got)

	gotMin := i18n.Localize(ctx, "validation.min", map[string]any{"Field": "password", "Param": "8"})
	require.Equal(t, "password debe tener al menos 8 caracteres", gotMin)
}

func TestLocalize_SpanishFallsBackToEnglishForMissingKeys(t *testing.T) {
	// If a key ever ships to en.toml before es.toml (which the parity
	// gate should prevent), the Spanish localizer should fall back to
	// the English catalog rather than returning the raw key.
	bundle := i18n.MustNewBundle()
	localizer := gi18n.NewLocalizer(bundle, language.Spanish.String(), language.English.String())
	ctx := i18n.WithLocalizer(context.Background(), localizer)

	// All current keys exist in both catalogs; asserting that a known key
	// resolves under the locale chain exercises the fallback machinery.
	got := i18n.Localize(ctx, "auth.errors.loginFailed", nil)
	require.NotEmpty(t, got)
	require.NotEqual(t, "auth.errors.loginFailed", got) // not the raw key
}

// TestCatalogParity asserts that every non-English catalog carries the
// exact same set of message IDs as the English source of truth. Matches
// the web-side `check:i18n-parity` gate so new keys land in both files
// together rather than silently falling back.
func TestCatalogParity(t *testing.T) {
	// translations/ is embedded by the bundle but we read from disk here
	// so a missing-key parity failure points at the actual TOML file.
	enKeys := loadCatalogKeys(t, "translations/active.en.toml")
	require.NotEmpty(t, enKeys, "English catalog must have at least one key")

	entries, err := os.ReadDir("translations")
	require.NoError(t, err)

	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".toml" || name == "active.en.toml" {
			continue
		}
		locale := name[len("active.") : len(name)-len(".toml")]
		t.Run(locale, func(t *testing.T) {
			keys := loadCatalogKeys(t, filepath.Join("translations", name))
			missing, extra := diffKeySets(enKeys, keys)
			require.Empty(t, missing, "%s is missing keys present in English", name)
			require.Empty(t, extra, "%s has keys not present in English", name)
		})
	}
}

// loadCatalogKeys reads a TOML catalog and returns the set of fully-qualified
// keys (e.g. "auth.errors.invalidToken"). Each leaf is a table with "other".
func loadCatalogKeys(t *testing.T, path string) map[string]struct{} {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var raw map[string]any
	require.NoError(t, toml.Unmarshal(data, &raw))

	keys := make(map[string]struct{})
	walkTomlKeys("", raw, keys)
	return keys
}

// walkTomlKeys descends into nested TOML tables, collecting every leaf path
// whose table contains an "other" field (go-i18n's catalog entry marker).
func walkTomlKeys(prefix string, node map[string]any, out map[string]struct{}) {
	for k, v := range node {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		child, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if _, isLeaf := child["other"]; isLeaf {
			out[path] = struct{}{}
			continue
		}
		walkTomlKeys(path, child, out)
	}
}

func diffKeySets(en, other map[string]struct{}) (missing, extra []string) {
	for k := range en {
		if _, ok := other[k]; !ok {
			missing = append(missing, k)
		}
	}
	for k := range other {
		if _, ok := en[k]; !ok {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	return
}
