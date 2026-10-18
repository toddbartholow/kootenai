package pathway

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/toddbartholow/kootenai/api/internal/models"
)

// Helper to create test logger
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestNewService(t *testing.T) {
	logger := testLogger()

	// NewService accepts nil for testing basic creation
	svc := NewService(nil, nil, logger)

	assert.NotNil(t, svc)
	assert.Equal(t, logger, svc.logger)
	assert.Nil(t, svc.pathwayRepo)
	assert.Nil(t, svc.enrollmentRepo)
}

func TestService_Enroll_NoRepos(t *testing.T) {
	logger := testLogger()

	// Service with nil repos
	svc := &Service{
		pathwayRepo:    nil,
		enrollmentRepo: nil,
		logger:         logger,
	}

	result, err := svc.Enroll(t.Context(), "user-123", "pathway-456")

	assert.ErrorIs(t, err, ErrRepoNotAvailable)
	assert.Nil(t, result)
}

func TestService_Unenroll_NoRepos(t *testing.T) {
	logger := testLogger()

	svc := &Service{
		pathwayRepo:    nil,
		enrollmentRepo: nil,
		logger:         logger,
	}

	err := svc.Unenroll(t.Context(), "user-123", "pathway-456")

	assert.ErrorIs(t, err, ErrRepoNotAvailable)
}

func TestService_ManualUnlock_NoRepos(t *testing.T) {
	logger := testLogger()

	svc := &Service{
		pathwayRepo:    nil,
		enrollmentRepo: nil,
		logger:         logger,
	}

	result, err := svc.ManualUnlock(t.Context(), "enrollment-123", "module-456")

	assert.ErrorIs(t, err, ErrRepoNotAvailable)
	assert.Nil(t, result)
}

func TestService_GetUnlockRequirements_NoRepos(t *testing.T) {
	logger := testLogger()

	svc := &Service{
		pathwayRepo:    nil,
		enrollmentRepo: nil,
		logger:         logger,
	}

	result, err := svc.GetUnlockRequirements(t.Context(), "enrollment-123", "module-456")

	assert.ErrorIs(t, err, ErrRepoNotAvailable)
	assert.Nil(t, result)
}

func TestService_RecordLabCompletion_NoRepos(t *testing.T) {
	logger := testLogger()

	svc := &Service{
		pathwayRepo:    nil,
		enrollmentRepo: nil,
		logger:         logger,
	}

	result, err := svc.RecordLabCompletion(t.Context(), "enrollment-123", "module-456", "lab-789", "session-abc", 100, true)

	assert.ErrorIs(t, err, ErrRepoNotAvailable)
	assert.Nil(t, result)
}

func TestService_GetEnrollmentByUserAndPathway_NoRepos(t *testing.T) {
	logger := testLogger()

	svc := &Service{
		pathwayRepo:    nil,
		enrollmentRepo: nil,
		logger:         logger,
	}

	result, err := svc.GetEnrollmentByUserAndPathway(t.Context(), "user-123", "pathway-456")

	assert.ErrorIs(t, err, ErrRepoNotAvailable)
	assert.Nil(t, result)
}

func TestEnrollResult_Format(t *testing.T) {
	enrollment := &models.PathwayEnrollment{
		ID:        "enrollment-123",
		UserID:    "user-456",
		PathwayID: "pathway-789",
		Status:    models.EnrollmentStatusEnrolled,
	}

	result := &EnrollResult{
		Enrollment: enrollment,
		IsNew:      true,
	}

	assert.Equal(t, "enrollment-123", result.Enrollment.ID)
	assert.Equal(t, "user-456", result.Enrollment.UserID)
	assert.Equal(t, "pathway-789", result.Enrollment.PathwayID)
	assert.True(t, result.IsNew)
}

func TestManualUnlockResult_Format(t *testing.T) {
	result := &ManualUnlockResult{
		ModuleID:     "module-123",
		EnrollmentID: "enrollment-456",
		Status:       "unlocked",
	}

	assert.Equal(t, "module-123", result.ModuleID)
	assert.Equal(t, "enrollment-456", result.EnrollmentID)
	assert.Equal(t, "unlocked", result.Status)
}

func TestUnlockRequirement_Format(t *testing.T) {
	req := UnlockRequirement{
		ModuleID:   "module-123",
		ModuleName: "Introduction",
		Status:     "completed",
		Required:   true,
	}

	assert.Equal(t, "module-123", req.ModuleID)
	assert.Equal(t, "Introduction", req.ModuleName)
	assert.Equal(t, "completed", req.Status)
	assert.True(t, req.Required)
}

func TestUnlockRequirementsResult_Format(t *testing.T) {
	result := &UnlockRequirementsResult{
		ModuleID:      "module-456",
		ModuleName:    "Advanced Topics",
		UnlockType:    models.UnlockTypeSequential,
		CurrentStatus: "locked",
		Requirements: []UnlockRequirement{
			{
				ModuleID:   "module-123",
				ModuleName: "Introduction",
				Status:     "completed",
				Required:   true,
			},
		},
		Message: "Complete the previous module to unlock this one.",
	}

	assert.Equal(t, "module-456", result.ModuleID)
	assert.Equal(t, "Advanced Topics", result.ModuleName)
	assert.Equal(t, models.UnlockTypeSequential, result.UnlockType)
	assert.Equal(t, "locked", result.CurrentStatus)
	assert.Len(t, result.Requirements, 1)
	assert.Equal(t, "Complete the previous module to unlock this one.", result.Message)
}

func TestRecordLabCompletionResult_Format(t *testing.T) {
	result := &RecordLabCompletionResult{
		EnrollmentID:    "enrollment-123",
		ModuleID:        "module-456",
		LabTemplateID:   "lab-789",
		UnlockedModules: []string{"module-next-1", "module-next-2"},
	}

	assert.Equal(t, "enrollment-123", result.EnrollmentID)
	assert.Equal(t, "module-456", result.ModuleID)
	assert.Equal(t, "lab-789", result.LabTemplateID)
	assert.Len(t, result.UnlockedModules, 2)
	assert.Contains(t, result.UnlockedModules, "module-next-1")
	assert.Contains(t, result.UnlockedModules, "module-next-2")
}

func TestErrors(t *testing.T) {
	// Test that error messages are meaningful
	assert.Equal(t, "pathway not found", ErrPathwayNotFound.Error())
	assert.Equal(t, "enrollment not found", ErrEnrollmentNotFound.Error())
	assert.Equal(t, "module not found", ErrModuleNotFound.Error())
	assert.Equal(t, "already enrolled in pathway", ErrAlreadyEnrolled.Error())
	assert.Equal(t, "not enrolled in pathway", ErrNotEnrolled.Error())
	assert.Equal(t, "module does not belong to pathway", ErrModuleNotInPathway.Error())
	assert.Equal(t, "forbidden", ErrForbidden.Error())
	assert.Equal(t, "pathway repository not available", ErrRepoNotAvailable.Error())
}

// -----------------------------------------------------------------------------
// Mock Repository Implementations
// -----------------------------------------------------------------------------

type mockPathwayRepo struct {
	pathways       map[string]*models.Pathway
	modules        map[string]*models.PathwayModule
	bySlug         map[string]*models.Pathway
	getByIDErr     error
	getBySlugErr   error
	getModuleErr   error
	listModulesErr error
}

func newMockPathwayRepo() *mockPathwayRepo {
	return &mockPathwayRepo{
		pathways: make(map[string]*models.Pathway),
		modules:  make(map[string]*models.PathwayModule),
		bySlug:   make(map[string]*models.Pathway),
	}
}

func (m *mockPathwayRepo) Create(ctx context.Context, pathway *models.Pathway) error {
	m.pathways[pathway.ID] = pathway
	m.bySlug[pathway.Slug] = pathway
	return nil
}

func (m *mockPathwayRepo) GetByID(ctx context.Context, id string) (*models.Pathway, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	return m.pathways[id], nil
}

func (m *mockPathwayRepo) GetByIDs(ctx context.Context, ids []string) ([]*models.Pathway, error) {
	result := make([]*models.Pathway, 0)
	for _, id := range ids {
		if p := m.pathways[id]; p != nil {
			result = append(result, p)
		}
	}
	return result, nil
}

func (m *mockPathwayRepo) GetBySlug(ctx context.Context, slug string) (*models.Pathway, error) {
	if m.getBySlugErr != nil {
		return nil, m.getBySlugErr
	}
	return m.bySlug[slug], nil
}

func (m *mockPathwayRepo) List(ctx context.Context, opts models.PathwayListOptions) ([]*models.Pathway, error) {
	result := make([]*models.Pathway, 0)
	for _, p := range m.pathways {
		result = append(result, p)
	}
	return result, nil
}

func (m *mockPathwayRepo) Update(ctx context.Context, pathway *models.Pathway) error {
	return nil
}

func (m *mockPathwayRepo) Delete(ctx context.Context, id string) error {
	return nil
}

func (m *mockPathwayRepo) UpdateStatus(ctx context.Context, id string, status models.PathwayStatus) error {
	return nil
}

func (m *mockPathwayRepo) GetWithModules(ctx context.Context, id string) (*models.Pathway, error) {
	return m.pathways[id], nil
}

func (m *mockPathwayRepo) GetWithModulesBySlug(ctx context.Context, slug string) (*models.Pathway, error) {
	return m.bySlug[slug], nil
}

func (m *mockPathwayRepo) CreateModule(ctx context.Context, module *models.PathwayModule) error {
	m.modules[module.ID] = module
	return nil
}

func (m *mockPathwayRepo) GetModuleByID(ctx context.Context, id string) (*models.PathwayModule, error) {
	if m.getModuleErr != nil {
		return nil, m.getModuleErr
	}
	return m.modules[id], nil
}

func (m *mockPathwayRepo) ListModules(ctx context.Context, pathwayID string) ([]*models.PathwayModule, error) {
	if m.listModulesErr != nil {
		return nil, m.listModulesErr
	}
	result := make([]*models.PathwayModule, 0)
	for _, mod := range m.modules {
		if mod.PathwayID == pathwayID {
			result = append(result, mod)
		}
	}
	return result, nil
}

func (m *mockPathwayRepo) UpdateModule(ctx context.Context, module *models.PathwayModule) error {
	return nil
}

func (m *mockPathwayRepo) DeleteModule(ctx context.Context, id string) error {
	return nil
}

func (m *mockPathwayRepo) ReorderModules(ctx context.Context, pathwayID string, moduleIDs []string) error {
	return nil
}

func (m *mockPathwayRepo) AddLabToModule(ctx context.Context, moduleLab *models.ModuleLab) error {
	return nil
}

func (m *mockPathwayRepo) RemoveLabFromModule(ctx context.Context, moduleID, labTemplateID string) error {
	return nil
}

func (m *mockPathwayRepo) ListModuleLabs(ctx context.Context, moduleID string) ([]*models.ModuleLab, error) {
	return nil, nil
}

func (m *mockPathwayRepo) ReorderModuleLabs(ctx context.Context, moduleID string, labTemplateIDs []string) error {
	return nil
}

func (m *mockPathwayRepo) GetStats(ctx context.Context, pathwayID string) (*models.PathwayStats, error) {
	return nil, nil
}

type mockEnrollmentRepo struct {
	enrollments          map[string]*models.PathwayEnrollment
	byUserAndPathway     map[string]*models.PathwayEnrollment
	moduleProgress       map[string]*models.ModuleProgress
	createErr            error
	deleteErr            error
	getByIDErr           error
	getByUserErr         error
	getWithProgressErr   error
	unlockErr            error
	recordLabErr         error
	recalcModuleErr      error
	recalcEnrollErr      error
	getModuleProgressErr error
	unlockNextErr        error
}

func newMockEnrollmentRepo() *mockEnrollmentRepo {
	return &mockEnrollmentRepo{
		enrollments:      make(map[string]*models.PathwayEnrollment),
		byUserAndPathway: make(map[string]*models.PathwayEnrollment),
		moduleProgress:   make(map[string]*models.ModuleProgress),
	}
}

func (m *mockEnrollmentRepo) Create(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.enrollments[enrollment.ID] = enrollment
	m.byUserAndPathway[enrollment.UserID+":"+enrollment.PathwayID] = enrollment
	return nil
}

func (m *mockEnrollmentRepo) GetByID(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	return m.enrollments[id], nil
}

func (m *mockEnrollmentRepo) GetByUserAndPathway(ctx context.Context, userID, pathwayID string) (*models.PathwayEnrollment, error) {
	if m.getByUserErr != nil {
		return nil, m.getByUserErr
	}
	return m.byUserAndPathway[userID+":"+pathwayID], nil
}

func (m *mockEnrollmentRepo) List(ctx context.Context, opts models.EnrollmentListOptions) ([]*models.PathwayEnrollment, error) {
	result := make([]*models.PathwayEnrollment, 0)
	for _, e := range m.enrollments {
		result = append(result, e)
	}
	return result, nil
}

func (m *mockEnrollmentRepo) Update(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	return nil
}

func (m *mockEnrollmentRepo) Delete(ctx context.Context, id string) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	delete(m.enrollments, id)
	return nil
}

func (m *mockEnrollmentRepo) UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus) error {
	return nil
}

func (m *mockEnrollmentRepo) GetWithProgress(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	if m.getWithProgressErr != nil {
		return nil, m.getWithProgressErr
	}
	enrollment := m.enrollments[id]
	if enrollment != nil {
		// Add module progress
		progress := make([]*models.ModuleProgress, 0)
		for _, mp := range m.moduleProgress {
			if mp.EnrollmentID == id {
				progress = append(progress, mp)
			}
		}
		enrollment.ModuleProgress = progress
	}
	return enrollment, nil
}

func (m *mockEnrollmentRepo) InitializeModuleProgress(ctx context.Context, enrollmentID, pathwayID string) error {
	return nil
}

func (m *mockEnrollmentRepo) GetModuleProgress(ctx context.Context, enrollmentID, moduleID string) (*models.ModuleProgress, error) {
	if m.getModuleProgressErr != nil {
		return nil, m.getModuleProgressErr
	}
	return m.moduleProgress[enrollmentID+":"+moduleID], nil
}

func (m *mockEnrollmentRepo) ListModuleProgress(ctx context.Context, enrollmentID string) ([]*models.ModuleProgress, error) {
	result := make([]*models.ModuleProgress, 0)
	for _, mp := range m.moduleProgress {
		if mp.EnrollmentID == enrollmentID {
			result = append(result, mp)
		}
	}
	return result, nil
}

func (m *mockEnrollmentRepo) UpdateModuleProgress(ctx context.Context, progress *models.ModuleProgress) error {
	return nil
}

func (m *mockEnrollmentRepo) UnlockModule(ctx context.Context, enrollmentID, moduleID string) error {
	if m.unlockErr != nil {
		return m.unlockErr
	}
	return nil
}

func (m *mockEnrollmentRepo) UnlockNextModules(ctx context.Context, enrollmentID, completedModuleID string) ([]string, error) {
	if m.unlockNextErr != nil {
		return nil, m.unlockNextErr
	}
	return []string{"module-next"}, nil
}

func (m *mockEnrollmentRepo) GetLabProgress(ctx context.Context, enrollmentID, labTemplateID string) (*models.LabProgress, error) {
	return nil, nil
}

func (m *mockEnrollmentRepo) ListLabProgress(ctx context.Context, enrollmentID, moduleID string) ([]*models.LabProgress, error) {
	return nil, nil
}

func (m *mockEnrollmentRepo) UpdateLabProgress(ctx context.Context, progress *models.LabProgress) error {
	return nil
}

func (m *mockEnrollmentRepo) RecordLabAttempt(ctx context.Context, enrollmentID, moduleID, labTemplateID, sessionID string, score int, passed bool) error {
	if m.recordLabErr != nil {
		return m.recordLabErr
	}
	return nil
}

func (m *mockEnrollmentRepo) RecalculateModuleProgress(ctx context.Context, enrollmentID, moduleID string) error {
	if m.recalcModuleErr != nil {
		return m.recalcModuleErr
	}
	return nil
}

func (m *mockEnrollmentRepo) RecalculateEnrollmentProgress(ctx context.Context, enrollmentID string) error {
	if m.recalcEnrollErr != nil {
		return m.recalcEnrollErr
	}
	return nil
}

// -----------------------------------------------------------------------------
// Service Tests with Mocked Repositories
// -----------------------------------------------------------------------------

func TestService_Enroll_Success(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Name: "Security Basics",
		Slug: "security-basics",
	}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.Enroll(t.Context(), "user-123", "pathway-1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsNew)
	assert.NotEmpty(t, result.Enrollment.ID)
	assert.Equal(t, "user-123", result.Enrollment.UserID)
	assert.Equal(t, "pathway-1", result.Enrollment.PathwayID)
	assert.Equal(t, models.EnrollmentStatusEnrolled, result.Enrollment.Status)
}

func TestService_Enroll_BySlug(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathway := &models.Pathway{
		ID:   "pathway-1",
		Name: "Security Basics",
		Slug: "security-basics",
	}
	pathwayRepo.pathways["pathway-1"] = pathway
	pathwayRepo.bySlug["security-basics"] = pathway

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.Enroll(t.Context(), "user-123", "security-basics")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsNew)
	assert.Equal(t, "pathway-1", result.Enrollment.PathwayID)
}

func TestService_Enroll_AlreadyEnrolled(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Name: "Security Basics",
	}

	existingEnrollment := &models.PathwayEnrollment{
		ID:        "existing-enrollment",
		UserID:    "user-123",
		PathwayID: "pathway-1",
		Status:    models.EnrollmentStatusInProgress,
	}
	enrollmentRepo.enrollments["existing-enrollment"] = existingEnrollment
	enrollmentRepo.byUserAndPathway["user-123:pathway-1"] = existingEnrollment

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.Enroll(t.Context(), "user-123", "pathway-1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsNew)
	assert.Equal(t, "existing-enrollment", result.Enrollment.ID)
}

func TestService_Enroll_PathwayNotFound(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.Enroll(t.Context(), "user-123", "nonexistent-pathway")

	assert.ErrorIs(t, err, ErrPathwayNotFound)
	assert.Nil(t, result)
}

func TestService_Enroll_CreateError(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID:   "pathway-1",
		Name: "Security Basics",
	}
	enrollmentRepo.createErr = errors.New("database error")

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.Enroll(t.Context(), "user-123", "pathway-1")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestService_Enroll_GetByUserError(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.pathways["pathway-1"] = &models.Pathway{ID: "pathway-1"}
	enrollmentRepo.getByUserErr = errors.New("database error")

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.Enroll(t.Context(), "user-123", "pathway-1")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestService_Enroll_PathwayResolveError(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.getByIDErr = errors.New("database error")

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.Enroll(t.Context(), "user-123", "pathway-1")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestService_Unenroll_Success(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.pathways["pathway-1"] = &models.Pathway{
		ID: "pathway-1",
	}

	enrollment := &models.PathwayEnrollment{
		ID:        "enrollment-1",
		UserID:    "user-123",
		PathwayID: "pathway-1",
	}
	enrollmentRepo.enrollments["enrollment-1"] = enrollment
	enrollmentRepo.byUserAndPathway["user-123:pathway-1"] = enrollment

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	err := svc.Unenroll(t.Context(), "user-123", "pathway-1")

	assert.NoError(t, err)
}

func TestService_Unenroll_NotEnrolled(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.pathways["pathway-1"] = &models.Pathway{ID: "pathway-1"}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	err := svc.Unenroll(t.Context(), "user-123", "pathway-1")

	assert.ErrorIs(t, err, ErrNotEnrolled)
}

func TestService_Unenroll_PathwayNotFound(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	err := svc.Unenroll(t.Context(), "user-123", "nonexistent")

	assert.ErrorIs(t, err, ErrPathwayNotFound)
}

func TestService_Unenroll_DeleteError(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.pathways["pathway-1"] = &models.Pathway{ID: "pathway-1"}

	enrollment := &models.PathwayEnrollment{ID: "enrollment-1", UserID: "user-123", PathwayID: "pathway-1"}
	enrollmentRepo.enrollments["enrollment-1"] = enrollment
	enrollmentRepo.byUserAndPathway["user-123:pathway-1"] = enrollment
	enrollmentRepo.deleteErr = errors.New("database error")

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	err := svc.Unenroll(t.Context(), "user-123", "pathway-1")

	assert.Error(t, err)
}

func TestService_ManualUnlock_Success(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.modules["module-1"] = &models.PathwayModule{
		ID:        "module-1",
		PathwayID: "pathway-1",
		Name:      "Module 1",
	}

	enrollmentRepo.enrollments["enrollment-1"] = &models.PathwayEnrollment{
		ID:        "enrollment-1",
		UserID:    "user-123",
		PathwayID: "pathway-1",
	}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.ManualUnlock(t.Context(), "enrollment-1", "module-1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "module-1", result.ModuleID)
	assert.Equal(t, "enrollment-1", result.EnrollmentID)
	assert.Equal(t, "unlocked", result.Status)
}

func TestService_ManualUnlock_EnrollmentNotFound(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.ManualUnlock(t.Context(), "nonexistent", "module-1")

	assert.ErrorIs(t, err, ErrEnrollmentNotFound)
	assert.Nil(t, result)
}

func TestService_ManualUnlock_ModuleNotFound(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	enrollmentRepo.enrollments["enrollment-1"] = &models.PathwayEnrollment{
		ID:        "enrollment-1",
		PathwayID: "pathway-1",
	}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.ManualUnlock(t.Context(), "enrollment-1", "nonexistent")

	assert.ErrorIs(t, err, ErrModuleNotFound)
	assert.Nil(t, result)
}

func TestService_ManualUnlock_ModuleNotInPathway(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.modules["module-1"] = &models.PathwayModule{
		ID:        "module-1",
		PathwayID: "pathway-2", // Different pathway
	}

	enrollmentRepo.enrollments["enrollment-1"] = &models.PathwayEnrollment{
		ID:        "enrollment-1",
		PathwayID: "pathway-1",
	}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.ManualUnlock(t.Context(), "enrollment-1", "module-1")

	assert.ErrorIs(t, err, ErrModuleNotInPathway)
	assert.Nil(t, result)
}

func TestService_ManualUnlock_UnlockError(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.modules["module-1"] = &models.PathwayModule{
		ID:        "module-1",
		PathwayID: "pathway-1",
	}
	enrollmentRepo.enrollments["enrollment-1"] = &models.PathwayEnrollment{
		ID:        "enrollment-1",
		PathwayID: "pathway-1",
	}
	enrollmentRepo.unlockErr = errors.New("unlock error")

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.ManualUnlock(t.Context(), "enrollment-1", "module-1")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestService_GetUnlockRequirements_Sequential(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.modules["module-1"] = &models.PathwayModule{
		ID:           "module-1",
		PathwayID:    "pathway-1",
		Name:         "Module 1",
		DisplayOrder: 1,
		UnlockType:   models.UnlockTypeAlways,
	}
	pathwayRepo.modules["module-2"] = &models.PathwayModule{
		ID:           "module-2",
		PathwayID:    "pathway-1",
		Name:         "Module 2",
		DisplayOrder: 2,
		UnlockType:   models.UnlockTypeSequential,
	}

	enrollmentRepo.enrollments["enrollment-1"] = &models.PathwayEnrollment{
		ID:        "enrollment-1",
		PathwayID: "pathway-1",
	}
	enrollmentRepo.moduleProgress["enrollment-1:module-1"] = &models.ModuleProgress{
		EnrollmentID: "enrollment-1",
		ModuleID:     "module-1",
		Status:       models.ModuleStatusCompleted,
	}
	enrollmentRepo.moduleProgress["enrollment-1:module-2"] = &models.ModuleProgress{
		EnrollmentID: "enrollment-1",
		ModuleID:     "module-2",
		Status:       models.ModuleStatusLocked,
	}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.GetUnlockRequirements(t.Context(), "enrollment-1", "module-2")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "module-2", result.ModuleID)
	assert.Equal(t, "Module 2", result.ModuleName)
	assert.Equal(t, models.UnlockTypeSequential, result.UnlockType)
	assert.Equal(t, "locked", result.CurrentStatus)
	assert.Len(t, result.Requirements, 1)
	assert.Equal(t, "completed", result.Requirements[0].Status)
	assert.Contains(t, result.Message, "Previous module is complete")
}

func TestService_GetUnlockRequirements_AllPrevious(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.modules["module-1"] = &models.PathwayModule{
		ID: "module-1", PathwayID: "pathway-1", Name: "Module 1", DisplayOrder: 1,
	}
	pathwayRepo.modules["module-2"] = &models.PathwayModule{
		ID: "module-2", PathwayID: "pathway-1", Name: "Module 2", DisplayOrder: 2,
	}
	pathwayRepo.modules["module-3"] = &models.PathwayModule{
		ID: "module-3", PathwayID: "pathway-1", Name: "Module 3", DisplayOrder: 3, UnlockType: models.UnlockTypeAllPrevious,
	}

	enrollmentRepo.enrollments["enrollment-1"] = &models.PathwayEnrollment{
		ID: "enrollment-1", PathwayID: "pathway-1",
	}
	enrollmentRepo.moduleProgress["enrollment-1:module-1"] = &models.ModuleProgress{
		EnrollmentID: "enrollment-1", ModuleID: "module-1", Status: models.ModuleStatusCompleted,
	}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.GetUnlockRequirements(t.Context(), "enrollment-1", "module-3")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, models.UnlockTypeAllPrevious, result.UnlockType)
	assert.Len(t, result.Requirements, 2) // module-1 and module-2
	assert.Contains(t, result.Message, "Complete all previous modules")
}

func TestService_GetUnlockRequirements_Manual(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.modules["module-1"] = &models.PathwayModule{
		ID: "module-1", PathwayID: "pathway-1", Name: "Module 1", DisplayOrder: 1, UnlockType: models.UnlockTypeManual,
	}

	enrollmentRepo.enrollments["enrollment-1"] = &models.PathwayEnrollment{
		ID: "enrollment-1", PathwayID: "pathway-1",
	}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.GetUnlockRequirements(t.Context(), "enrollment-1", "module-1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, models.UnlockTypeManual, result.UnlockType)
	assert.Contains(t, result.Message, "instructor approval")
}

func TestService_GetUnlockRequirements_Always(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.modules["module-1"] = &models.PathwayModule{
		ID: "module-1", PathwayID: "pathway-1", Name: "Module 1", DisplayOrder: 1, UnlockType: models.UnlockTypeAlways,
	}

	enrollmentRepo.enrollments["enrollment-1"] = &models.PathwayEnrollment{
		ID: "enrollment-1", PathwayID: "pathway-1",
	}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.GetUnlockRequirements(t.Context(), "enrollment-1", "module-1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, models.UnlockTypeAlways, result.UnlockType)
	assert.Contains(t, result.Message, "always available")
}

func TestService_GetUnlockRequirements_EnrollmentNotFound(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.GetUnlockRequirements(t.Context(), "nonexistent", "module-1")

	assert.ErrorIs(t, err, ErrEnrollmentNotFound)
	assert.Nil(t, result)
}

func TestService_GetUnlockRequirements_ModuleNotFound(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	enrollmentRepo.enrollments["enrollment-1"] = &models.PathwayEnrollment{
		ID: "enrollment-1", PathwayID: "pathway-1",
	}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.GetUnlockRequirements(t.Context(), "enrollment-1", "nonexistent")

	assert.ErrorIs(t, err, ErrModuleNotFound)
	assert.Nil(t, result)
}

func TestService_RecordLabCompletion_Success(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	// Set up module progress so we can test unlocking
	enrollmentRepo.moduleProgress["enrollment-1:module-1"] = &models.ModuleProgress{
		EnrollmentID: "enrollment-1",
		ModuleID:     "module-1",
		Status:       models.ModuleStatusCompleted,
	}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.RecordLabCompletion(t.Context(), "enrollment-1", "module-1", "lab-1", "session-1", 100, true)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "enrollment-1", result.EnrollmentID)
	assert.Equal(t, "module-1", result.ModuleID)
	assert.Equal(t, "lab-1", result.LabTemplateID)
	assert.Contains(t, result.UnlockedModules, "module-next")
}

func TestService_RecordLabCompletion_NotPassed(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.RecordLabCompletion(t.Context(), "enrollment-1", "module-1", "lab-1", "session-1", 50, false)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.UnlockedModules) // No unlocking when not passed
}

func TestService_RecordLabCompletion_RecordError(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()
	enrollmentRepo.recordLabErr = errors.New("record error")

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.RecordLabCompletion(t.Context(), "enrollment-1", "module-1", "lab-1", "session-1", 100, true)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestService_GetEnrollmentByUserAndPathway_Success(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.pathways["pathway-1"] = &models.Pathway{ID: "pathway-1"}

	enrollment := &models.PathwayEnrollment{
		ID:        "enrollment-1",
		UserID:    "user-123",
		PathwayID: "pathway-1",
	}
	enrollmentRepo.enrollments["enrollment-1"] = enrollment
	enrollmentRepo.byUserAndPathway["user-123:pathway-1"] = enrollment

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.GetEnrollmentByUserAndPathway(t.Context(), "user-123", "pathway-1")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "enrollment-1", result.ID)
}

func TestService_GetEnrollmentByUserAndPathway_PathwayNotFound(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	result, err := svc.GetEnrollmentByUserAndPathway(t.Context(), "user-123", "nonexistent")

	assert.ErrorIs(t, err, ErrPathwayNotFound)
	assert.Nil(t, result)
}

func TestService_ResolvePathwayID_ByID(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathwayRepo.pathways["pathway-1"] = &models.Pathway{ID: "pathway-1", Slug: "test"}

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	id, err := svc.resolvePathwayID(t.Context(), "pathway-1")

	assert.NoError(t, err)
	assert.Equal(t, "pathway-1", id)
}

func TestService_ResolvePathwayID_BySlug(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	pathway := &models.Pathway{ID: "pathway-1", Slug: "test-slug"}
	pathwayRepo.bySlug["test-slug"] = pathway

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	id, err := svc.resolvePathwayID(t.Context(), "test-slug")

	assert.NoError(t, err)
	assert.Equal(t, "pathway-1", id)
}

func TestService_ResolvePathwayID_NotFound(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	id, err := svc.resolvePathwayID(t.Context(), "nonexistent")

	assert.NoError(t, err)
	assert.Empty(t, id)
}

func TestService_ResolvePathwayID_GetBySlugError(t *testing.T) {
	logger := testLogger()
	pathwayRepo := newMockPathwayRepo()
	enrollmentRepo := newMockEnrollmentRepo()
	pathwayRepo.getBySlugErr = errors.New("slug error")

	svc := NewService(pathwayRepo, enrollmentRepo, logger)

	id, err := svc.resolvePathwayID(t.Context(), "nonexistent")

	assert.Error(t, err)
	assert.Empty(t, id)
}
