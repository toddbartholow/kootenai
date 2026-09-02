package repositories

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Mock implementations for LabTemplateRepository
type mockLabTemplateRepo struct {
	createFn    func(ctx context.Context, record *models.LabTemplateRecord) error
	getByIDFn   func(ctx context.Context, id string) (*models.LabTemplateRecord, error)
	getByNameFn func(ctx context.Context, name string) (*models.LabTemplateRecord, error)
	listFn      func(ctx context.Context, filter LabTemplateFilter) ([]*models.LabTemplateRecord, error)
	updateFn    func(ctx context.Context, record *models.LabTemplateRecord) error
	deleteFn    func(ctx context.Context, id string) error
	setActiveFn func(ctx context.Context, id string, active bool) error
}

func (m *mockLabTemplateRepo) Create(ctx context.Context, record *models.LabTemplateRecord) error {
	if m.createFn != nil {
		return m.createFn(ctx, record)
	}
	return nil
}

func (m *mockLabTemplateRepo) GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockLabTemplateRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.LabTemplateRecord, error) {
	return nil, nil
}

func (m *mockLabTemplateRepo) GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
	if m.getByNameFn != nil {
		return m.getByNameFn(ctx, name)
	}
	return nil, nil
}

func (m *mockLabTemplateRepo) List(ctx context.Context, filter LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
	if m.listFn != nil {
		return m.listFn(ctx, filter)
	}
	return nil, nil
}

func (m *mockLabTemplateRepo) Update(ctx context.Context, record *models.LabTemplateRecord) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, record)
	}
	return nil
}

func (m *mockLabTemplateRepo) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockLabTemplateRepo) SetActive(ctx context.Context, id string, active bool) error {
	if m.setActiveFn != nil {
		return m.setActiveFn(ctx, id, active)
	}
	return nil
}
func (m *mockLabTemplateRepo) CreateVersion(_ context.Context, _ *models.LabTemplateVersion) error {
	return nil
}
func (m *mockLabTemplateRepo) ListVersions(_ context.Context, _ string, _, _ int) ([]*models.LabTemplateVersion, error) {
	return nil, nil
}
func (m *mockLabTemplateRepo) GetVersionByNumber(_ context.Context, _ string, _ int) (*models.LabTemplateVersion, error) {
	return nil, nil
}
func (m *mockLabTemplateRepo) CountVersions(_ context.Context, _ string) (int, error) { return 0, nil }

var _ LabTemplateRepository = (*mockLabTemplateRepo)(nil)

// Mock implementations for PodRepository
type mockPodRepo struct {
	createFn            func(ctx context.Context, pod *models.Pod) error
	getByIDFn           func(ctx context.Context, id string) (*models.Pod, error)
	listFn              func(ctx context.Context, filter PodFilter) ([]*models.Pod, error)
	updateFn            func(ctx context.Context, pod *models.Pod) error
	updateStatusFn      func(ctx context.Context, id string, status models.PodStatus) error
	deleteFn            func(ctx context.Context, id string) error
	getExpiredFn        func(ctx context.Context) ([]*models.Pod, error)
	getOwnerIDFn        func(ctx context.Context, id string) (string, error)
	isOwnerFn           func(ctx context.Context, id, userID string) (bool, error)
	getOrganizationIDFn func(ctx context.Context, id string) (*string, error)
}

func (m *mockPodRepo) Create(ctx context.Context, pod *models.Pod) error {
	if m.createFn != nil {
		return m.createFn(ctx, pod)
	}
	return nil
}

func (m *mockPodRepo) GetByID(ctx context.Context, id string) (*models.Pod, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockPodRepo) List(ctx context.Context, filter PodFilter) ([]*models.Pod, error) {
	if m.listFn != nil {
		return m.listFn(ctx, filter)
	}
	return nil, nil
}

func (m *mockPodRepo) Update(ctx context.Context, pod *models.Pod) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, pod)
	}
	return nil
}

func (m *mockPodRepo) UpdateStatus(ctx context.Context, id string, status models.PodStatus) error {
	if m.updateStatusFn != nil {
		return m.updateStatusFn(ctx, id, status)
	}
	return nil
}

func (m *mockPodRepo) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockPodRepo) GetExpired(ctx context.Context) ([]*models.Pod, error) {
	if m.getExpiredFn != nil {
		return m.getExpiredFn(ctx)
	}
	return nil, nil
}

func (m *mockPodRepo) GetOwnerID(ctx context.Context, id string) (string, error) {
	if m.getOwnerIDFn != nil {
		return m.getOwnerIDFn(ctx, id)
	}
	return "", nil
}

func (m *mockPodRepo) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	if m.isOwnerFn != nil {
		return m.isOwnerFn(ctx, id, userID)
	}
	return false, nil
}

func (m *mockPodRepo) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	if m.getOrganizationIDFn != nil {
		return m.getOrganizationIDFn(ctx, id)
	}
	return nil, nil
}

var _ PodRepository = (*mockPodRepo)(nil)

// Mock implementations for UserRepository
type mockUserRepo struct {
	createFn                  func(ctx context.Context, user *models.User) error
	getByIDFn                 func(ctx context.Context, id string) (*models.User, error)
	getByExternalIDFn         func(ctx context.Context, externalID string) (*models.User, error)
	getByUsernameFn           func(ctx context.Context, username string) (*models.User, error)
	getByEmailFn              func(ctx context.Context, email string) (*models.User, error)
	getOrCreateByUsernameFn   func(ctx context.Context, username string) (*models.User, error)
	updateFn                  func(ctx context.Context, user *models.User) error
	updateLastLoginFn         func(ctx context.Context, id string) error
	listFn                    func(ctx context.Context, opts UserListOptions) ([]*models.User, int, error)
	deleteFn                  func(ctx context.Context, id string) error
	getByEmailForAuthFn       func(ctx context.Context, email string) (*models.User, error)
	getByIDWithPasswordFn     func(ctx context.Context, id string) (*models.User, error)
	updatePasswordFn          func(ctx context.Context, id, passwordHash string, mustChange bool) error
	clearMustChangePasswordFn func(ctx context.Context, id string) error
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	if m.createFn != nil {
		return m.createFn(ctx, user)
	}
	return nil
}

func (m *mockUserRepo) GetByID(ctx context.Context, id string) (*models.User, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockUserRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.User, error) {
	return nil, nil
}

func (m *mockUserRepo) GetByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	if m.getByExternalIDFn != nil {
		return m.getByExternalIDFn(ctx, externalID)
	}
	return nil, nil
}

func (m *mockUserRepo) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	if m.getByUsernameFn != nil {
		return m.getByUsernameFn(ctx, username)
	}
	return nil, nil
}

func (m *mockUserRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.getByEmailFn != nil {
		return m.getByEmailFn(ctx, email)
	}
	return nil, nil
}

func (m *mockUserRepo) GetOrCreateByUsername(ctx context.Context, username string) (*models.User, error) {
	if m.getOrCreateByUsernameFn != nil {
		return m.getOrCreateByUsernameFn(ctx, username)
	}
	return nil, nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *models.User) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, user)
	}
	return nil
}

func (m *mockUserRepo) UpdateLastLogin(ctx context.Context, id string) error {
	if m.updateLastLoginFn != nil {
		return m.updateLastLoginFn(ctx, id)
	}
	return nil
}

func (m *mockUserRepo) List(ctx context.Context, opts UserListOptions) ([]*models.User, int, error) {
	if m.listFn != nil {
		return m.listFn(ctx, opts)
	}
	return nil, 0, nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockUserRepo) GetByEmailForAuth(ctx context.Context, email string) (*models.User, error) {
	if m.getByEmailForAuthFn != nil {
		return m.getByEmailForAuthFn(ctx, email)
	}
	return nil, nil
}

func (m *mockUserRepo) GetByIDWithPassword(ctx context.Context, id string) (*models.User, error) {
	if m.getByIDWithPasswordFn != nil {
		return m.getByIDWithPasswordFn(ctx, id)
	}
	return nil, nil
}

func (m *mockUserRepo) UpdatePassword(ctx context.Context, id, passwordHash string, mustChange bool) error {
	if m.updatePasswordFn != nil {
		return m.updatePasswordFn(ctx, id, passwordHash, mustChange)
	}
	return nil
}

func (m *mockUserRepo) ClearMustChangePassword(ctx context.Context, id string) error {
	if m.clearMustChangePasswordFn != nil {
		return m.clearMustChangePasswordFn(ctx, id)
	}
	return nil
}

func (m *mockUserRepo) GetPreferredLocale(ctx context.Context, id string) (*string, error) {
	return nil, nil
}

func (m *mockUserRepo) UpdatePreferredLocale(ctx context.Context, id string, locale *string) error {
	return nil
}

var _ UserRepository = (*mockUserRepo)(nil)

// ========== CachedLabTemplateRepo Tests ==========

func TestNewCachedLabTemplateRepo(t *testing.T) {
	mockRepo := &mockLabTemplateRepo{}

	t.Run("with_logger", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		if cached == nil {
			t.Fatal("expected non-nil cached repo")
		}
	})

	t.Run("nil_logger_uses_default", func(t *testing.T) {
		cached := NewCachedLabTemplateRepo(mockRepo, nil, nil)
		if cached == nil {
			t.Fatal("expected non-nil cached repo")
		}
	})
}

func TestCachedLabTemplateRepo_Create(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		called := false
		mockRepo := &mockLabTemplateRepo{
			createFn: func(ctx context.Context, record *models.LabTemplateRecord) error {
				called = true
				return nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		err := cached.Create(ctx, &models.LabTemplateRecord{ID: "template-1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !called {
			t.Error("expected underlying repo to be called")
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			createFn: func(ctx context.Context, record *models.LabTemplateRecord) error {
				return errors.New("create failed")
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		err := cached.Create(ctx, &models.LabTemplateRecord{ID: "template-1"})

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedLabTemplateRepo_GetByID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success_no_cache", func(t *testing.T) {
		expected := &models.LabTemplateRecord{ID: "template-1", Name: "Test Template"}
		mockRepo := &mockLabTemplateRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
				return expected, nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		result, err := cached.GetByID(ctx, "template-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.ID != expected.ID {
			t.Errorf("expected ID %s, got %s", expected.ID, result.ID)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
				return nil, nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		result, err := cached.GetByID(ctx, "nonexistent")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil result")
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
				return nil, errors.New("db error")
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		_, err := cached.GetByID(ctx, "template-1")

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedLabTemplateRepo_GetByName(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success_no_cache", func(t *testing.T) {
		expected := &models.LabTemplateRecord{ID: "template-1", Name: "Test Template"}
		mockRepo := &mockLabTemplateRepo{
			getByNameFn: func(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
				return expected, nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		result, err := cached.GetByName(ctx, "Test Template")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Name != expected.Name {
			t.Errorf("expected name %s, got %s", expected.Name, result.Name)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			getByNameFn: func(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
				return nil, nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		result, err := cached.GetByName(ctx, "nonexistent")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil result")
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			getByNameFn: func(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
				return nil, errors.New("db error")
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		_, err := cached.GetByName(ctx, "Test Template")

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedLabTemplateRepo_List(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expected := []*models.LabTemplateRecord{
			{ID: "template-1", Name: "Template 1"},
			{ID: "template-2", Name: "Template 2"},
		}
		mockRepo := &mockLabTemplateRepo{
			listFn: func(ctx context.Context, filter LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
				return expected, nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		result, err := cached.List(ctx, LabTemplateFilter{})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 templates, got %d", len(result))
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			listFn: func(ctx context.Context, filter LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
				return nil, errors.New("list failed")
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		_, err := cached.List(ctx, LabTemplateFilter{})

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedLabTemplateRepo_Update(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			updateFn: func(ctx context.Context, record *models.LabTemplateRecord) error {
				return nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		err := cached.Update(ctx, &models.LabTemplateRecord{ID: "template-1", Name: "Updated"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			updateFn: func(ctx context.Context, record *models.LabTemplateRecord) error {
				return errors.New("update failed")
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		err := cached.Update(ctx, &models.LabTemplateRecord{ID: "template-1"})

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedLabTemplateRepo_Delete(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success_with_existing_record", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
				return &models.LabTemplateRecord{ID: id, Name: "Test"}, nil
			},
			deleteFn: func(ctx context.Context, id string) error {
				return nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		err := cached.Delete(ctx, "template-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("success_without_existing_record", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
				return nil, nil
			},
			deleteFn: func(ctx context.Context, id string) error {
				return nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		err := cached.Delete(ctx, "template-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
				return nil, nil
			},
			deleteFn: func(ctx context.Context, id string) error {
				return errors.New("delete failed")
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		err := cached.Delete(ctx, "template-1")

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedLabTemplateRepo_SetActive(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success_with_existing_record", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
				return &models.LabTemplateRecord{ID: id, Name: "Test"}, nil
			},
			setActiveFn: func(ctx context.Context, id string, active bool) error {
				return nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		err := cached.SetActive(ctx, "template-1", true)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("success_without_existing_record", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
				return nil, nil
			},
			setActiveFn: func(ctx context.Context, id string, active bool) error {
				return nil
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		err := cached.SetActive(ctx, "template-1", false)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockLabTemplateRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
				return nil, nil
			},
			setActiveFn: func(ctx context.Context, id string, active bool) error {
				return errors.New("setActive failed")
			},
		}

		cached := NewCachedLabTemplateRepo(mockRepo, nil, logger)
		err := cached.SetActive(ctx, "template-1", true)

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestHashFilter(t *testing.T) {
	t.Run("deterministic", func(t *testing.T) {
		filter := LabTemplateFilter{
			Active: boolPtr(true),
		}

		hash1 := hashFilter(filter)
		hash2 := hashFilter(filter)

		if hash1 != hash2 {
			t.Errorf("hash should be deterministic: %s != %s", hash1, hash2)
		}
	})

	t.Run("different_filters_different_hashes", func(t *testing.T) {
		filter1 := LabTemplateFilter{Active: boolPtr(true)}
		filter2 := LabTemplateFilter{Active: boolPtr(false)}

		hash1 := hashFilter(filter1)
		hash2 := hashFilter(filter2)

		if hash1 == hash2 {
			t.Error("different filters should produce different hashes")
		}
	})
}

// ========== CachedPodRepo Tests ==========

func TestNewCachedPodRepo(t *testing.T) {
	mockRepo := &mockPodRepo{}

	t.Run("with_logger", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		cached := NewCachedPodRepo(mockRepo, nil, logger)
		if cached == nil {
			t.Fatal("expected non-nil cached repo")
		}
	})

	t.Run("nil_logger_uses_default", func(t *testing.T) {
		cached := NewCachedPodRepo(mockRepo, nil, nil)
		if cached == nil {
			t.Fatal("expected non-nil cached repo")
		}
	})
}

func TestCachedPodRepo_Create(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		called := false
		mockRepo := &mockPodRepo{
			createFn: func(ctx context.Context, pod *models.Pod) error {
				called = true
				return nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		err := cached.Create(ctx, &models.Pod{ID: "pod-1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !called {
			t.Error("expected underlying repo to be called")
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			createFn: func(ctx context.Context, pod *models.Pod) error {
				return errors.New("create failed")
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		err := cached.Create(ctx, &models.Pod{ID: "pod-1"})

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedPodRepo_GetByID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success_no_cache", func(t *testing.T) {
		expected := &models.Pod{ID: "pod-1", OwnerID: "user-1"}
		mockRepo := &mockPodRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.Pod, error) {
				return expected, nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		result, err := cached.GetByID(ctx, "pod-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.ID != expected.ID {
			t.Errorf("expected ID %s, got %s", expected.ID, result.ID)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.Pod, error) {
				return nil, nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		result, err := cached.GetByID(ctx, "nonexistent")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil result")
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.Pod, error) {
				return nil, errors.New("db error")
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		_, err := cached.GetByID(ctx, "pod-1")

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedPodRepo_List(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expected := []*models.Pod{
			{ID: "pod-1"},
			{ID: "pod-2"},
		}
		mockRepo := &mockPodRepo{
			listFn: func(ctx context.Context, filter PodFilter) ([]*models.Pod, error) {
				return expected, nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		result, err := cached.List(ctx, PodFilter{})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 pods, got %d", len(result))
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			listFn: func(ctx context.Context, filter PodFilter) ([]*models.Pod, error) {
				return nil, errors.New("list failed")
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		_, err := cached.List(ctx, PodFilter{})

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedPodRepo_Update(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			updateFn: func(ctx context.Context, pod *models.Pod) error {
				return nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		err := cached.Update(ctx, &models.Pod{ID: "pod-1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			updateFn: func(ctx context.Context, pod *models.Pod) error {
				return errors.New("update failed")
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		err := cached.Update(ctx, &models.Pod{ID: "pod-1"})

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedPodRepo_UpdateStatus(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			updateStatusFn: func(ctx context.Context, id string, status models.PodStatus) error {
				return nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		err := cached.UpdateStatus(ctx, "pod-1", models.PodStatusRunning)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			updateStatusFn: func(ctx context.Context, id string, status models.PodStatus) error {
				return errors.New("update status failed")
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		err := cached.UpdateStatus(ctx, "pod-1", models.PodStatusRunning)

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedPodRepo_Delete(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			deleteFn: func(ctx context.Context, id string) error {
				return nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		err := cached.Delete(ctx, "pod-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			deleteFn: func(ctx context.Context, id string) error {
				return errors.New("delete failed")
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		err := cached.Delete(ctx, "pod-1")

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedPodRepo_GetExpired(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expected := []*models.Pod{{ID: "pod-1"}, {ID: "pod-2"}}
		mockRepo := &mockPodRepo{
			getExpiredFn: func(ctx context.Context) ([]*models.Pod, error) {
				return expected, nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		result, err := cached.GetExpired(ctx)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 pods, got %d", len(result))
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			getExpiredFn: func(ctx context.Context) ([]*models.Pod, error) {
				return nil, errors.New("get expired failed")
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		_, err := cached.GetExpired(ctx)

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedPodRepo_GetOwnerID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			getOwnerIDFn: func(ctx context.Context, id string) (string, error) {
				return "user-123", nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		ownerID, err := cached.GetOwnerID(ctx, "pod-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if ownerID != "user-123" {
			t.Errorf("expected user-123, got %s", ownerID)
		}
	})
}

func TestCachedPodRepo_IsOwner(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("is_owner", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			isOwnerFn: func(ctx context.Context, id, userID string) (bool, error) {
				return true, nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		isOwner, err := cached.IsOwner(ctx, "pod-1", "user-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !isOwner {
			t.Error("expected true")
		}
	})

	t.Run("not_owner", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			isOwnerFn: func(ctx context.Context, id, userID string) (bool, error) {
				return false, nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		isOwner, err := cached.IsOwner(ctx, "pod-1", "user-2")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if isOwner {
			t.Error("expected false")
		}
	})
}

func TestCachedPodRepo_GetOrganizationID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("with_org", func(t *testing.T) {
		orgID := "org-123"
		mockRepo := &mockPodRepo{
			getOrganizationIDFn: func(ctx context.Context, id string) (*string, error) {
				return &orgID, nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		result, err := cached.GetOrganizationID(ctx, "pod-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if *result != orgID {
			t.Errorf("expected %s, got %s", orgID, *result)
		}
	})

	t.Run("nil_org", func(t *testing.T) {
		mockRepo := &mockPodRepo{
			getOrganizationIDFn: func(ctx context.Context, id string) (*string, error) {
				return nil, nil
			},
		}

		cached := NewCachedPodRepo(mockRepo, nil, logger)
		result, err := cached.GetOrganizationID(ctx, "pod-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil")
		}
	})
}

// ========== CachedUserRepo Tests ==========

func TestNewCachedUserRepo(t *testing.T) {
	mockRepo := &mockUserRepo{}

	t.Run("with_logger", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		cached := NewCachedUserRepo(mockRepo, nil, logger)
		if cached == nil {
			t.Fatal("expected non-nil cached repo")
		}
	})

	t.Run("nil_logger_uses_default", func(t *testing.T) {
		cached := NewCachedUserRepo(mockRepo, nil, nil)
		if cached == nil {
			t.Fatal("expected non-nil cached repo")
		}
	})
}

func TestCachedUserRepo_Create(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		called := false
		mockRepo := &mockUserRepo{
			createFn: func(ctx context.Context, user *models.User) error {
				called = true
				return nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.Create(ctx, &models.User{ID: "user-1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !called {
			t.Error("expected underlying repo to be called")
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			createFn: func(ctx context.Context, user *models.User) error {
				return errors.New("create failed")
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.Create(ctx, &models.User{ID: "user-1"})

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedUserRepo_GetByID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success_no_cache", func(t *testing.T) {
		expected := &models.User{ID: "user-1", Username: "testuser"}
		mockRepo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.User, error) {
				return expected, nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		result, err := cached.GetByID(ctx, "user-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.ID != expected.ID {
			t.Errorf("expected ID %s, got %s", expected.ID, result.ID)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.User, error) {
				return nil, nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		result, err := cached.GetByID(ctx, "nonexistent")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil result")
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.User, error) {
				return nil, errors.New("db error")
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		_, err := cached.GetByID(ctx, "user-1")

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedUserRepo_GetByExternalID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success_no_cache", func(t *testing.T) {
		expected := &models.User{ID: "user-1", ExternalID: "ext-123"}
		mockRepo := &mockUserRepo{
			getByExternalIDFn: func(ctx context.Context, externalID string) (*models.User, error) {
				return expected, nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		result, err := cached.GetByExternalID(ctx, "ext-123")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.ID != expected.ID {
			t.Errorf("expected ID %s, got %s", expected.ID, result.ID)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			getByExternalIDFn: func(ctx context.Context, externalID string) (*models.User, error) {
				return nil, nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		result, err := cached.GetByExternalID(ctx, "nonexistent")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != nil {
			t.Error("expected nil result")
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			getByExternalIDFn: func(ctx context.Context, externalID string) (*models.User, error) {
				return nil, errors.New("db error")
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		_, err := cached.GetByExternalID(ctx, "ext-123")

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedUserRepo_GetByUsername(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expected := &models.User{ID: "user-1", Username: "testuser"}
		mockRepo := &mockUserRepo{
			getByUsernameFn: func(ctx context.Context, username string) (*models.User, error) {
				return expected, nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		result, err := cached.GetByUsername(ctx, "testuser")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Username != expected.Username {
			t.Errorf("expected username %s, got %s", expected.Username, result.Username)
		}
	})
}

func TestCachedUserRepo_GetByEmail(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expected := &models.User{ID: "user-1", Email: "test@example.com"}
		mockRepo := &mockUserRepo{
			getByEmailFn: func(ctx context.Context, email string) (*models.User, error) {
				return expected, nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		result, err := cached.GetByEmail(ctx, "test@example.com")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Email != expected.Email {
			t.Errorf("expected email %s, got %s", expected.Email, result.Email)
		}
	})
}

func TestCachedUserRepo_GetOrCreateByUsername(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expected := &models.User{ID: "user-1", Username: "testuser"}
		mockRepo := &mockUserRepo{
			getOrCreateByUsernameFn: func(ctx context.Context, username string) (*models.User, error) {
				return expected, nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		result, err := cached.GetOrCreateByUsername(ctx, "testuser")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Username != expected.Username {
			t.Errorf("expected username %s, got %s", expected.Username, result.Username)
		}
	})
}

func TestCachedUserRepo_Update(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			updateFn: func(ctx context.Context, user *models.User) error {
				return nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.Update(ctx, &models.User{ID: "user-1"})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			updateFn: func(ctx context.Context, user *models.User) error {
				return errors.New("update failed")
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.Update(ctx, &models.User{ID: "user-1"})

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedUserRepo_UpdateLastLogin(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			updateLastLoginFn: func(ctx context.Context, id string) error {
				return nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.UpdateLastLogin(ctx, "user-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			updateLastLoginFn: func(ctx context.Context, id string) error {
				return errors.New("update failed")
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.UpdateLastLogin(ctx, "user-1")

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedUserRepo_List(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expected := []*models.User{{ID: "user-1"}, {ID: "user-2"}}
		mockRepo := &mockUserRepo{
			listFn: func(ctx context.Context, opts UserListOptions) ([]*models.User, int, error) {
				return expected, 2, nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		result, count, err := cached.List(ctx, UserListOptions{})

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 users, got %d", len(result))
		}
		if count != 2 {
			t.Errorf("expected count 2, got %d", count)
		}
	})
}

func TestCachedUserRepo_Delete(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success_with_existing_user", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.User, error) {
				return &models.User{ID: id, Username: "test"}, nil
			},
			deleteFn: func(ctx context.Context, id string) error {
				return nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.Delete(ctx, "user-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("success_without_existing_user", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.User, error) {
				return nil, nil
			},
			deleteFn: func(ctx context.Context, id string) error {
				return nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.Delete(ctx, "user-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			getByIDFn: func(ctx context.Context, id string) (*models.User, error) {
				return nil, nil
			},
			deleteFn: func(ctx context.Context, id string) error {
				return errors.New("delete failed")
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.Delete(ctx, "user-1")

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedUserRepo_GetByEmailForAuth(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expected := &models.User{ID: "user-1", Email: "test@example.com"}
		mockRepo := &mockUserRepo{
			getByEmailForAuthFn: func(ctx context.Context, email string) (*models.User, error) {
				return expected, nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		result, err := cached.GetByEmailForAuth(ctx, "test@example.com")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.Email != expected.Email {
			t.Errorf("expected email %s, got %s", expected.Email, result.Email)
		}
	})
}

func TestCachedUserRepo_GetByIDWithPassword(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		expected := &models.User{ID: "user-1"}
		mockRepo := &mockUserRepo{
			getByIDWithPasswordFn: func(ctx context.Context, id string) (*models.User, error) {
				return expected, nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		result, err := cached.GetByIDWithPassword(ctx, "user-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result.ID != expected.ID {
			t.Errorf("expected ID %s, got %s", expected.ID, result.ID)
		}
	})
}

func TestCachedUserRepo_UpdatePassword(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			updatePasswordFn: func(ctx context.Context, id, passwordHash string, mustChange bool) error {
				return nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.UpdatePassword(ctx, "user-1", "newhashedpassword", false)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			updatePasswordFn: func(ctx context.Context, id, passwordHash string, mustChange bool) error {
				return errors.New("update password failed")
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.UpdatePassword(ctx, "user-1", "newhashedpassword", false)

		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestCachedUserRepo_ClearMustChangePassword(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			clearMustChangePasswordFn: func(ctx context.Context, id string) error {
				return nil
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.ClearMustChangePassword(ctx, "user-1")

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("repo_error", func(t *testing.T) {
		mockRepo := &mockUserRepo{
			clearMustChangePasswordFn: func(ctx context.Context, id string) error {
				return errors.New("clear failed")
			},
		}

		cached := NewCachedUserRepo(mockRepo, nil, logger)
		err := cached.ClearMustChangePassword(ctx, "user-1")

		if err == nil {
			t.Error("expected error")
		}
	})
}

// Helper functions - boolPtr and stringPtr already defined in other test files

func (m *mockPodRepo) CountActive(ctx context.Context) (int64, error) {
	return 0, nil
}
