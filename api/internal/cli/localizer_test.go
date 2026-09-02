package cli_test

import (
	"testing"

	gi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	"github.com/toddbartholow/kootenai/api/internal/cli"
	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
)

// TestExtractLocaleFlag covers every accepted --locale spelling plus the
// passthrough case. The CLI does this pre-dispatch so `--locale es` works
// regardless of which subcommand eventually owns the FlagSet; any miss
// here would turn into a confusing "unknown flag" error from the inner
// FlagSet.
func TestExtractLocaleFlag(t *testing.T) {
	tests := []struct {
		name        string
		argv        []string
		wantRemain  []string
		wantFlagVal string
	}{
		{
			name:        "space form long",
			argv:        []string{"--locale", "es", "lab", "list"},
			wantRemain:  []string{"lab", "list"},
			wantFlagVal: "es",
		},
		{
			name:        "space form short",
			argv:        []string{"-locale", "es", "lab", "list"},
			wantRemain:  []string{"lab", "list"},
			wantFlagVal: "es",
		},
		{
			name:        "equals form long",
			argv:        []string{"--locale=es", "lab", "list"},
			wantRemain:  []string{"lab", "list"},
			wantFlagVal: "es",
		},
		{
			name:        "equals form short",
			argv:        []string{"-locale=es", "lab", "list"},
			wantRemain:  []string{"lab", "list"},
			wantFlagVal: "es",
		},
		{
			name:        "flag in the middle of argv",
			argv:        []string{"lab", "--locale", "es", "list"},
			wantRemain:  []string{"lab", "list"},
			wantFlagVal: "es",
		},
		{
			name:        "no flag — passthrough",
			argv:        []string{"lab", "list"},
			wantRemain:  []string{"lab", "list"},
			wantFlagVal: "",
		},
		{
			name:        "bare trailing --locale with no value is dropped",
			argv:        []string{"version", "--locale"},
			wantRemain:  []string{"version"},
			wantFlagVal: "",
		},
		{
			name:        "empty argv",
			argv:        []string{},
			wantRemain:  []string{},
			wantFlagVal: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			remain, flagVal := cli.ExtractLocaleFlag(tc.argv)
			require.Equal(t, tc.wantRemain, remain)
			require.Equal(t, tc.wantFlagVal, flagVal)
		})
	}
}

// TestResolveLocale_Precedence asserts the flag → LC_ALL → LANG → default
// precedence. Env vars are isolated via t.Setenv so tests don't interfere
// with each other or the surrounding shell.
func TestResolveLocale_Precedence(t *testing.T) {
	t.Run("flag beats env", func(t *testing.T) {
		t.Setenv("LC_ALL", "fr_FR.UTF-8")
		t.Setenv("LANG", "de_DE.UTF-8")
		tag, warn := cli.ResolveLocale("es")
		require.Equal(t, language.Spanish, tag)
		require.Empty(t, warn)
	})

	t.Run("LC_ALL beats LANG", func(t *testing.T) {
		t.Setenv("LC_ALL", "es_ES.UTF-8")
		t.Setenv("LANG", "fr_FR.UTF-8")
		tag, warn := cli.ResolveLocale("")
		require.Equal(t, language.Spanish, tag)
		require.Empty(t, warn)
	})

	t.Run("LANG used when LC_ALL unset", func(t *testing.T) {
		t.Setenv("LC_ALL", "")
		t.Setenv("LANG", "es_ES.UTF-8")
		tag, warn := cli.ResolveLocale("")
		require.Equal(t, language.Spanish, tag)
		require.Empty(t, warn)
	})

	t.Run("C locale treated as no preference", func(t *testing.T) {
		t.Setenv("LC_ALL", "C")
		t.Setenv("LANG", "POSIX")
		tag, warn := cli.ResolveLocale("")
		require.Equal(t, language.English, tag)
		require.Empty(t, warn)
	})

	t.Run("no input defaults to English", func(t *testing.T) {
		t.Setenv("LC_ALL", "")
		t.Setenv("LANG", "")
		tag, warn := cli.ResolveLocale("")
		require.Equal(t, language.English, tag)
		require.Empty(t, warn)
	})
}

// TestResolveLocale_POSIXForms asserts that POSIX locale names (with
// charset/modifier suffixes and underscore region separator) reduce to
// the same canonical BCP-47 tag as plain BCP-47 input.
func TestResolveLocale_POSIXForms(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "POSIX with charset", input: "es_ES.UTF-8"},
		{name: "POSIX with modifier", input: "es_ES@euro"},
		{name: "POSIX with charset + modifier", input: "es_ES.UTF-8@euro"},
		{name: "POSIX bare", input: "es_ES"},
		{name: "BCP-47 with region", input: "es-ES"},
		{name: "BCP-47 bare", input: "es"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tag, warn := cli.ResolveLocale(tc.input)
			require.Equal(t, language.Spanish, tag)
			require.Empty(t, warn, "supported-locale input should not warn")
		})
	}
}

// TestResolveLocale_UnsupportedWarns exercises the fallback-with-warning
// path that issue #119 calls out as an acceptance criterion:
//
//	labctl --locale xx lab list  → falls back to English with a warning.
//
// The warning text is returned (not printed) so the caller owns the
// "one-time" stderr emission contract.
func TestResolveLocale_UnsupportedWarns(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantTag      language.Tag
		warnContains string
	}{
		{
			name:         "unsupported locale",
			input:        "fr",
			wantTag:      language.English,
			warnContains: `locale "fr" is not supported`,
		},
		{
			name:         "unsupported POSIX locale",
			input:        "de_DE.UTF-8",
			wantTag:      language.English,
			warnContains: `locale "de_DE.UTF-8" is not supported`,
		},
		{
			name:         "unparseable garbage",
			input:        "xx-yy-zz-not-a-tag",
			wantTag:      language.English,
			warnContains: "could not parse",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Clear env so only flagVal drives the decision.
			t.Setenv("LC_ALL", "")
			t.Setenv("LANG", "")
			tag, warn := cli.ResolveLocale(tc.input)
			require.Equal(t, tc.wantTag, tag)
			require.Contains(t, warn, tc.warnContains)
		})
	}
}

// TestT_NoLocalizerReturnsID guards the fallback behavior: if the
// package-level localizer is unset (tests forgot to install, startup
// failed, etc.) T should return the raw message ID rather than panic or
// return "".
func TestT_NoLocalizerReturnsID(t *testing.T) {
	// Save + clear + restore so this test doesn't leak state into
	// sibling tests that rely on a prior InstallLocalizer call.
	t.Cleanup(func() { cli.InstallLocalizer("en") })
	cli.SetLocalizer(nil)

	got := cli.T("labctl.lab.list.colName", nil)
	require.Equal(t, "labctl.lab.list.colName", got)
}

// TestT_ResolvesSpanish round-trips a real catalog key through T with the
// Spanish localizer installed. Belt-and-braces alongside the integration
// test — if the bundle can't find a key or the locale tag doesn't match,
// this fails closer to the source than the main package's output tests.
func TestT_ResolvesSpanish(t *testing.T) {
	bundle := appi18n.MustNewBundle()
	l := gi18n.NewLocalizer(bundle, language.Spanish.String(), language.English.String())
	cli.SetLocalizer(l)
	t.Cleanup(func() { cli.InstallLocalizer("en") })

	require.Equal(t, "NOMBRE", cli.T("labctl.lab.list.colName", nil))
	require.Equal(t,
		"No se encontraron plantillas en /tmp/x",
		cli.T("labctl.lab.list.empty", map[string]any{"Dir": "/tmp/x"}),
	)
}
