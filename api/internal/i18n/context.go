package i18n

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/nicksnyder/go-i18n/v2/i18n"
)

// ctxKey is the private type used for localizer context keys so callers
// cannot collide with our values.
type ctxKey int

const (
	localizerKey ctxKey = iota
	requestedLocaleKey
)

// WithLocalizer attaches a per-request Localizer to ctx. Invoked by the
// locale middleware after parsing Accept-Language.
func WithLocalizer(ctx context.Context, l *i18n.Localizer) context.Context {
	if l == nil {
		return ctx
	}
	return context.WithValue(ctx, localizerKey, l)
}

// FromContext returns the request-scoped Localizer or nil when none was set.
// Callers should prefer [Localize] which handles the nil case.
func FromContext(ctx context.Context) *i18n.Localizer {
	if ctx == nil {
		return nil
	}
	l, _ := ctx.Value(localizerKey).(*i18n.Localizer)
	return l
}

// WithRequestedLocale stashes the caller's raw Accept-Language value so that
// miss telemetry can attribute unresolved keys to a specific locale. A zero
// value is a no-op.
func WithRequestedLocale(ctx context.Context, lang string) context.Context {
	if lang == "" {
		return ctx
	}
	return context.WithValue(ctx, requestedLocaleKey, lang)
}

// RequestedLocale returns the raw Accept-Language attached via
// WithRequestedLocale, or "" when the middleware did not set one.
func RequestedLocale(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	s, _ := ctx.Value(requestedLocaleKey).(string)
	return s
}

// MissHandler is invoked on every catalog miss. Replace via SetMissHandler
// to wire additional observers (Prom counter, audit sink, etc). The default
// handler is a no-op; a structured slog.Warn fires regardless of handler.
type MissHandler func(messageID, requestedLocale string)

var missHandler atomic.Pointer[MissHandler]

// SetMissHandler installs a callback invoked on every Localize miss. Pass nil
// to clear. Safe to call concurrently; last writer wins.
func SetMissHandler(h MissHandler) {
	if h == nil {
		missHandler.Store(nil)
		return
	}
	missHandler.Store(&h)
}

// recordMiss fires the structured warning log and invokes any registered
// MissHandler. Intentionally private so the recording contract lives in this
// package.
func recordMiss(ctx context.Context, messageID string, err error) {
	locale := RequestedLocale(ctx)
	slog.Warn("i18n: missing translation",
		slog.String("message_id", messageID),
		slog.String("locale", locale),
		slog.Any("error", err),
	)
	if h := missHandler.Load(); h != nil {
		(*h)(messageID, locale)
	}
}

// Localize resolves messageID against the request-scoped Localizer, falling
// back to the message default (English) when the request has no localizer or
// the catalog lookup fails. templateData is interpolated into named template
// params like `{{.Field}}`.
//
// This helper never returns an empty string — if all fallbacks fail it
// returns messageID so a broken key surfaces visibly in responses rather than
// silently producing an empty error body. Every miss fires a structured
// slog.Warn and the registered MissHandler (if any) so operators can scrape
// catalog gaps from logs or metrics.
func Localize(ctx context.Context, messageID string, templateData map[string]any) string {
	l := FromContext(ctx)
	if l == nil {
		recordMiss(ctx, messageID, nil)
		return messageID
	}
	msg, err := l.Localize(&i18n.LocalizeConfig{
		MessageID:    messageID,
		TemplateData: templateData,
	})
	if err != nil || msg == "" {
		recordMiss(ctx, messageID, err)
		return messageID
	}
	return msg
}
