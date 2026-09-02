package sessions

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/checkpoint"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
)

// fakeSessionRecorder counts the calls the handlers make into the metrics API.
type fakeSessionRecorder struct {
	created int
	ended   int
	bulk    int64
}

func (f *fakeSessionRecorder) SessionCreated()       { f.created++ }
func (f *fakeSessionRecorder) SessionEnded()         { f.ended++ }
func (f *fakeSessionRecorder) SessionsEnded(n int64) { f.bulk += n }

func newRecordingManager(t *testing.T, repo *mocks.FakeSessionRepository) (*Manager, *fakeSessionRecorder) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	rec := &fakeSessionRecorder{}
	return NewManager(Config{
		SessionRepo: repo,
		Evaluator:   checkpoint.NewEvaluator(logger),
		Logger:      logger,
		Responder:   httputil.NewResponder(logger),
		Metrics:     rec,
	}), rec
}

// endRequest builds a request routed as the real chi mux would, so
// chi.URLParam finds the session ID.
func endRequest(t *testing.T, m *Manager, sessionID string, user *auth.User) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Post("/sessions/{sessionID}/end", m.handleEndSession())

	req := httptest.NewRequest(http.MethodPost, "/sessions/"+sessionID+"/end", nil)
	if user != nil {
		req = req.WithContext(auth.ContextWithUser(req.Context(), user))
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestEndSessionRecordsOneEnd(t *testing.T) {
	repo := mocks.NewFakeSessionRepository()
	if err := repo.Create(context.Background(), &models.Session{ID: "s1", UserID: "u1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	m, rec := newRecordingManager(t, repo)

	if w := endRequest(t, m, "s1", &auth.User{ID: "u1"}); w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	if rec.ended != 1 {
		t.Errorf("ended count = %d, want 1", rec.ended)
	}
}

// A retry or a double-click must not decrement twice. The repository treats a
// repeat end as a no-op, so only the transition is an outcome.
func TestEndSessionDoesNotDoubleCount(t *testing.T) {
	repo := mocks.NewFakeSessionRepository()
	ended := time.Now()
	if err := repo.Create(context.Background(), &models.Session{ID: "s1", UserID: "u1", EndedAt: &ended}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	m, rec := newRecordingManager(t, repo)

	endRequest(t, m, "s1", &auth.User{ID: "u1"})

	if rec.ended != 0 {
		t.Errorf("ended count = %d, want 0 for an already-ended session", rec.ended)
	}
}

// GetByID returns (nil, nil) for a missing row. Before the guard, the
// ownership check was skipped, the repo writes failed into the log, and the
// handler answered 200 while decrementing — reachable with any random UUID.
func TestEndSessionOnMissingSessionIs404AndRecordsNothing(t *testing.T) {
	repo := mocks.NewFakeSessionRepository()
	m, rec := newRecordingManager(t, repo)

	w := endRequest(t, m, "does-not-exist", &auth.User{ID: "u1", Roles: []string{"instructor"}})

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	if rec.ended != 0 {
		t.Errorf("ended count = %d, want 0", rec.ended)
	}
}

// The bulk sweep ends an unbounded number of sessions in one statement, so it
// reports the count rather than calling SessionEnded in a loop.
func TestCleanupStaleSessionsRecordsBulkEnd(t *testing.T) {
	repo := mocks.NewFakeSessionRepository()
	repo.EndStaleSessionsCount = 5
	m, rec := newRecordingManager(t, repo)

	r := chi.NewRouter()
	r.Post("/admin/sessions/cleanup", m.handleCleanupStaleSessions())
	req := httptest.NewRequest(http.MethodPost, "/admin/sessions/cleanup", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", w.Code, w.Body.String())
	}
	if rec.bulk != 5 {
		t.Errorf("bulk end count = %d, want 5", rec.bulk)
	}
}

// A nil recorder is the default in most tests; the handlers must not panic.
func TestSessionHandlersWithoutRecorderDoNotPanic(t *testing.T) {
	repo := mocks.NewFakeSessionRepository()
	if err := repo.Create(context.Background(), &models.Session{ID: "s1", UserID: "u1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	m := NewManager(Config{
		SessionRepo: repo,
		Evaluator:   checkpoint.NewEvaluator(logger),
		Logger:      logger,
		Responder:   httputil.NewResponder(logger),
	})

	endRequest(t, m, "s1", &auth.User{ID: "u1"})
}
