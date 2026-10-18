package server

import (
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/server/consoleaccess"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/server/labs"
	"github.com/toddbartholow/kootenai/api/internal/server/organizations"
	"github.com/toddbartholow/kootenai/api/internal/server/pods"
	"github.com/toddbartholow/kootenai/api/internal/server/sessions"
	"github.com/toddbartholow/kootenai/api/internal/server/users"
)

// -----------------------------------------------------------------------------
// Lab Manager Tests
// -----------------------------------------------------------------------------

func TestNewLabManager(t *testing.T) {
	logger := newTestLogger()

	mgr := labs.NewManager(labs.Config{
		Logger: logger,
	})

	if mgr == nil {
		t.Fatal("labs.NewManager() returned nil")
	}
}

func TestLabManager_SetupRoutes(t *testing.T) {
	logger := newTestLogger()
	mgr := labs.NewManager(labs.Config{
		Logger:    logger,
		Responder: httputil.NewResponder(logger),
	})

	// Should not panic
	router := chi.NewRouter()
	mgr.SetupRoutes(router)
}

// -----------------------------------------------------------------------------
// Pod Manager Tests
// -----------------------------------------------------------------------------

func TestNewPodManager(t *testing.T) {
	logger := newTestLogger()

	mgr := pods.NewManager(pods.Config{
		Logger: logger,
	})

	if mgr == nil {
		t.Fatal("pods.NewManager() returned nil")
	}
}

func TestPodManager_SetupRoutes(t *testing.T) {
	logger := newTestLogger()
	mgr := pods.NewManager(pods.Config{
		Logger:    logger,
		Responder: httputil.NewResponder(logger),
	})

	// Should not panic
	router := chi.NewRouter()
	mgr.SetupRoutes(router)
}

// -----------------------------------------------------------------------------
// Console Manager Tests
// -----------------------------------------------------------------------------

func TestNewConsoleManager(t *testing.T) {
	logger := newTestLogger()

	mgr := consoleaccess.NewManager(consoleaccess.Config{
		Logger: logger,
	})

	if mgr == nil {
		t.Fatal("consoleaccess.NewManager() returned nil")
	}
}

func TestConsoleManager_SetupProxmoxRoutes(t *testing.T) {
	logger := newTestLogger()
	mgr := consoleaccess.NewManager(consoleaccess.Config{
		Logger:    logger,
		Responder: httputil.NewResponder(logger),
	})

	// Should not panic
	router := chi.NewRouter()
	mgr.SetupProxmoxRoutes(router)
}

// -----------------------------------------------------------------------------
// User Manager Tests
// -----------------------------------------------------------------------------

func TestNewUserManager(t *testing.T) {
	logger := newTestLogger()

	mgr := users.NewManager(users.Config{
		Logger: logger,
	})

	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}
}

func TestUserManager_SetupRoutes(t *testing.T) {
	logger := newTestLogger()
	mgr := users.NewManager(users.Config{Logger: logger})

	// Should not panic
	router := chi.NewRouter()
	mgr.SetupRoutes(router)
}

// -----------------------------------------------------------------------------
// Pathway Manager Tests
// -----------------------------------------------------------------------------

func TestNewPathwayManager(t *testing.T) {
	logger := newTestLogger()

	mgr := NewPathwayManager(PathwayManagerConfig{
		Logger: logger,
	})

	if mgr == nil {
		t.Fatal("NewPathwayManager() returned nil")
	}
	if mgr.logger != logger {
		t.Error("NewPathwayManager() did not set logger")
	}
}

func TestPathwayManager_SetupRoutes(t *testing.T) {
	logger := newTestLogger()
	mgr := NewPathwayManager(PathwayManagerConfig{
		Logger:    logger,
		Responder: httputil.NewResponder(logger),
	})

	// Should not panic
	router := chi.NewRouter()
	mgr.SetupRoutes(router, PathwayExternalHandlers{})
}

// -----------------------------------------------------------------------------
// Session Manager Tests
// -----------------------------------------------------------------------------

func TestNewSessionManager(t *testing.T) {
	logger := newTestLogger()

	mgr := sessions.NewManager(sessions.Config{
		Logger: logger,
	})

	if mgr == nil {
		t.Fatal("NewManager() returned nil")
	}
}

// -----------------------------------------------------------------------------
// Organization Manager Tests
// -----------------------------------------------------------------------------

func TestNewOrganizationManager(t *testing.T) {
	logger := newTestLogger()

	mgr := organizations.NewManager(organizations.Config{
		Logger: logger,
	})

	if mgr == nil {
		t.Fatal("organizations.NewManager() returned nil")
	}
}
