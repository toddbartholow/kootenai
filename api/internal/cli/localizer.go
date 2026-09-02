// Package cli — this file hosts the CLI-side i18n resolver. labctl picks its
// locale from a `--locale` flag, falling back to POSIX `LC_ALL`/`LANG` env
// vars, then to English. Only locales in SupportedTags have translations;
// everything else falls back to English with a one-time stderr warning.
//
// Per ADR-0001 the CLI ships English by default (operator tool) and
// localized output is opt-in. See issue #119.
package cli

import (
	"fmt"
	"os"
	"strings"
	"sync"

	gi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	appi18n "github.com/toddbartholow/kootenai/api/internal/i18n"
)

// SupportedTags lists every BCP-47 tag the labctl catalog currently carries.
// Matches the set in api/internal/i18n/translations/active.*.toml — keep in
// sync when adding a new locale.
var SupportedTags = []language.Tag{
	language.English,
	language.Spanish,
}

var (
	localizerMu sync.RWMutex
	localizer   *gi18n.Localizer
)

// T resolves messageID against the CLI's active locale and returns the
// rendered string. When no localizer is installed (tests, early startup)
// or the key is missing it returns messageID so broken keys surface
// visibly rather than producing empty output. Mirrors the signature of
// i18n.Localize in the HTTP path.
func T(messageID string, templateData map[string]any) string {
	localizerMu.RLock()
	l := localizer
	localizerMu.RUnlock()
	if l == nil {
		return messageID
	}
	msg, err := l.Localize(&gi18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})
	if err != nil || msg == "" {
		return messageID
	}
	return msg
}

// SetLocalizer installs the package-level Localizer used by T(). Called
// from labctl startup after parsing --locale / LANG; tests can call it
// directly with a hand-built localizer to exercise specific catalogs.
// Pass nil to clear (mostly useful for test cleanup).
func SetLocalizer(l *gi18n.Localizer) {
	localizerMu.Lock()
	defer localizerMu.Unlock()
	localizer = l
}

// PrintErr writes the localized "Error:" prefix followed by err to stderr.
// Centralized so every call site uses the same "{prefix} {err}" format and
// only the prefix needs a translation (Go error strings are not routed
// through the catalog — they surface verbatim).
func PrintErr(err error) {
	fmt.Fprintf(os.Stderr, "%s %v\n", T("labctl.errors.prefix", nil), err)
}

// ResolveLocale picks the effective locale for this CLI invocation.
// Precedence:
//
//  1. flagVal (--locale) if non-empty
//  2. LC_ALL env var
//  3. LANG env var
//  4. English (default)
//
// The `matched` return is always a tag from SupportedTags. When the
// resolved input doesn't match a supported locale, `warning` carries a
// non-empty human-readable sentence suitable for a one-shot stderr print.
// Supported inputs cover both BCP-47 (`es-ES`) and POSIX (`es_ES.UTF-8`)
// spellings — see parseLocaleInput.
func ResolveLocale(flagVal string) (matched language.Tag, warning string) {
	raw := pickRawLocale(flagVal)
	if raw == "" {
		return language.English, ""
	}
	parsed, err := parseLocaleInput(raw)
	if err != nil {
		return language.English, fmt.Sprintf(
			"labctl: could not parse locale %q — falling back to English", raw,
		)
	}
	matcher := language.NewMatcher(SupportedTags)
	_, idx, confidence := matcher.Match(parsed)
	pick := SupportedTags[idx]
	if confidence == language.No {
		return pick, fmt.Sprintf(
			"labctl: locale %q is not supported — falling back to English", raw,
		)
	}
	return pick, ""
}

// pickRawLocale walks the precedence chain (flag → LC_ALL → LANG).
// POSIX "C" / "POSIX" locales are treated as "no preference" since they
// carry no language info and would otherwise match English trivially
// (which is fine, but means we'd never fall back to the default branch —
// cosmetic difference only, but skipping keeps the trace cleaner).
func pickRawLocale(flagVal string) string {
	flagVal = strings.TrimSpace(flagVal)
	if flagVal != "" {
		return flagVal
	}
	for _, key := range []string{"LC_ALL", "LANG"} {
		v := strings.TrimSpace(os.Getenv(key))
		if v == "" || v == "C" || v == "POSIX" {
			continue
		}
		return v
	}
	return ""
}

// parseLocaleInput accepts both BCP-47 ("es", "es-ES", "en-US") and POSIX
// locale names ("es_ES.UTF-8", "en_US@euro"). The `.CHARSET` and `@modifier`
// suffixes are stripped and `_` is rewritten to `-` before handing off to
// language.Parse.
func parseLocaleInput(s string) (language.Tag, error) {
	// Strip charset + modifier: es_ES.UTF-8@euro → es_ES.
	if i := strings.IndexAny(s, ".@"); i >= 0 {
		s = s[:i]
	}
	// POSIX uses '_' as the region separator; BCP-47 uses '-'.
	s = strings.ReplaceAll(s, "_", "-")
	return language.Parse(s)
}

// InstallLocalizer is the startup convenience: resolve + build the bundle +
// install. Returns the matched tag + any warning to surface on stderr.
// Separate from SetLocalizer so tests can wire custom bundles.
func InstallLocalizer(flagVal string) (language.Tag, string) {
	tag, warning := ResolveLocale(flagVal)
	bundle := appi18n.MustNewBundle()
	l := gi18n.NewLocalizer(bundle, tag.String(), language.English.String())
	SetLocalizer(l)
	return tag, warning
}

// ExtractLocaleFlag scans argv, removes any --locale / -locale token(s),
// and returns the remainder alongside the raw flag value (empty when
// unset). Must run BEFORE the per-subcommand FlagSet parses, so the
// pattern `labctl --locale es lab list` works even though the outer
// command doesn't own a FlagSet.
//
// Accepted forms:
//
//	--locale es
//	-locale es
//	--locale=es
//	-locale=es
//
// A trailing bare `--locale` with no following token is silently
// dropped — the resolver then falls back to env/default.
func ExtractLocaleFlag(argv []string) (remainder []string, flagVal string) {
	out := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		a := argv[i]
		switch {
		case strings.HasPrefix(a, "--locale="):
			flagVal = strings.TrimPrefix(a, "--locale=")
		case strings.HasPrefix(a, "-locale="):
			flagVal = strings.TrimPrefix(a, "-locale=")
		case a == "--locale" || a == "-locale":
			if i+1 < len(argv) {
				flagVal = argv[i+1]
				i++
			}
		default:
			out = append(out, a)
		}
	}
	return out, flagVal
}
