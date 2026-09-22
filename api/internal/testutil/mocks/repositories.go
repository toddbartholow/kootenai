// Package mocks provides mock implementations for testing
package mocks

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
)

// -----------------------------------------------------------------------------
// Mock Pod Repository
// -----------------------------------------------------------------------------

// FakePodRepository is a mock implementation of PodRepository
type FakePodRepository struct {
	mu   sync.RWMutex
	pods map[string]*models.Pod

	// Error injection
	CountActiveErr  error
	CreateErr       error
	GetByIDErr      error
	ListErr         error
	UpdateErr       error
	UpdateStatusErr error
	DeleteErr       error
	GetExpiredErr   error

	// Call tracking
	CreateCalls       []models.Pod
	GetByIDCalls      []string
	UpdateStatusCalls []UpdateStatusCall
}

// UpdateStatusCall records a call to UpdateStatus
type UpdateStatusCall struct {
	ID     string
	Status models.PodStatus
}

// NewFakePodRepository creates a new mock pod repository
func NewFakePodRepository() *FakePodRepository {
	return &FakePodRepository{
		pods: make(map[string]*models.Pod),
	}
}

func (r *FakePodRepository) Create(ctx context.Context, pod *models.Pod) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.CreateCalls = append(r.CreateCalls, *pod)
	r.pods[pod.ID] = pod
	return nil
}

func (r *FakePodRepository) GetByID(ctx context.Context, id string) (*models.Pod, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.GetByIDCalls = append(r.GetByIDCalls, id)
	pod, ok := r.pods[id]
	if !ok {
		return nil, nil
	}
	return pod, nil
}

func (r *FakePodRepository) List(ctx context.Context, filter repositories.PodFilter) ([]*models.Pod, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Pod, 0)
	for _, pod := range r.pods {
		if filter.OwnerID != "" && pod.Owner != filter.OwnerID {
			continue
		}
		if filter.Status != "" && pod.Status != filter.Status {
			continue
		}
		if filter.Platform != "" && string(pod.Platform) != filter.Platform {
			continue
		}
		result = append(result, pod)
	}
	return result, nil
}

func (r *FakePodRepository) Update(ctx context.Context, pod *models.Pod) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pods[pod.ID] = pod
	return nil
}

func (r *FakePodRepository) UpdateStatus(ctx context.Context, id string, status models.PodStatus) error {
	if r.UpdateStatusErr != nil {
		return r.UpdateStatusErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.UpdateStatusCalls = append(r.UpdateStatusCalls, UpdateStatusCall{ID: id, Status: status})
	if pod, ok := r.pods[id]; ok {
		pod.Status = status
	}
	return nil
}

func (r *FakePodRepository) Delete(ctx context.Context, id string) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pods, id)
	return nil
}

// CountActive mirrors the real repository's predicate: every pod except
// destroyed. A destroying pod still holds its VMs.
func (r *FakePodRepository) CountActive(ctx context.Context) (int64, error) {
	if r.CountActiveErr != nil {
		return 0, r.CountActiveErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var n int64
	for _, p := range r.pods {
		if p.Status != models.PodStatusDestroyed {
			n++
		}
	}
	return n, nil
}

func (r *FakePodRepository) GetExpired(ctx context.Context) ([]*models.Pod, error) {
	if r.GetExpiredErr != nil {
		return nil, r.GetExpiredErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Pod, 0)
	now := time.Now()
	for _, pod := range r.pods {
		if pod.ExpiresAt != nil && pod.ExpiresAt.Before(now) {
			result = append(result, pod)
		}
	}
	return result, nil
}

// AddPod adds a pod directly to the mock (for test setup)
func (r *FakePodRepository) AddPod(pod *models.Pod) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pods[pod.ID] = pod
}

// GetOwnerID returns the owner ID for a pod
func (r *FakePodRepository) GetOwnerID(ctx context.Context, id string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pod, ok := r.pods[id]
	if !ok {
		return "", nil
	}
	return pod.OwnerID, nil
}

// IsOwner checks if a user owns a pod
func (r *FakePodRepository) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pod, ok := r.pods[id]
	if !ok {
		return false, nil
	}
	return pod.OwnerID == userID, nil
}

// GetOrganizationID returns the organization ID for a pod
func (r *FakePodRepository) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pod, ok := r.pods[id]
	if !ok {
		return nil, nil
	}
	return pod.OrganizationID, nil
}

// -----------------------------------------------------------------------------
// Mock Session Repository
// -----------------------------------------------------------------------------

// FakeSessionRepository is a mock implementation of SessionRepository
type FakeSessionRepository struct {
	mu       sync.RWMutex
	sessions map[string]*models.Session

	// Error injection
	// EndStaleSessionsCount is what EndStaleSessions reports as swept.
	EndStaleSessionsCount int64
	// CountActiveErr, when set, makes CountActive fail.
	CountActiveErr error

	CreateErr              error
	GetByIDErr             error
	GetByPodIDErr          error
	GetActiveByUserIDErr   error
	ListErr                error
	UpdateErr              error
	EndErr                 error
	UpdateGradeErr         error
	MarkGradeSyncedErr     error
	MarkGradeSyncFailedErr error
}

// NewFakeSessionRepository creates a new mock session repository
func NewFakeSessionRepository() *FakeSessionRepository {
	return &FakeSessionRepository{
		sessions: make(map[string]*models.Session),
	}
}

func (r *FakeSessionRepository) Create(ctx context.Context, session *models.Session) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	session.StartedAt = time.Now()
	r.sessions[session.ID] = session
	return nil
}

func (r *FakeSessionRepository) GetByID(ctx context.Context, id string) (*models.Session, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[id]
	if !ok {
		return nil, nil
	}
	return session, nil
}

func (r *FakeSessionRepository) GetByPodID(ctx context.Context, podID string) ([]*models.Session, error) {
	if r.GetByPodIDErr != nil {
		return nil, r.GetByPodIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Session, 0)
	for _, session := range r.sessions {
		if session.PodID == podID {
			result = append(result, session)
		}
	}
	return result, nil
}

func (r *FakeSessionRepository) GetActiveByUserID(ctx context.Context, userID string) ([]*models.Session, error) {
	if r.GetActiveByUserIDErr != nil {
		return nil, r.GetActiveByUserIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Session, 0)
	for _, session := range r.sessions {
		if session.UserID == userID && session.EndedAt == nil {
			result = append(result, session)
		}
	}
	return result, nil
}

func (r *FakeSessionRepository) List(ctx context.Context, filter repositories.SessionFilter) ([]*models.Session, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Session, 0)
	for _, session := range r.sessions {
		if filter.UserID != "" && session.UserID != filter.UserID {
			continue
		}
		if filter.PodID != "" && session.PodID != filter.PodID {
			continue
		}
		if filter.Active != nil && *filter.Active && session.EndedAt != nil {
			continue
		}
		result = append(result, session)
	}
	return result, nil
}

func (r *FakeSessionRepository) Update(ctx context.Context, session *models.Session) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
	return nil
}

func (r *FakeSessionRepository) End(ctx context.Context, id string) error {
	if r.EndErr != nil {
		return r.EndErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if session, ok := r.sessions[id]; ok {
		now := time.Now()
		session.EndedAt = &now
	}
	return nil
}

func (r *FakeSessionRepository) UpdateGrade(ctx context.Context, id string, earnedPoints int, passed bool) error {
	if r.UpdateGradeErr != nil {
		return r.UpdateGradeErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if session, ok := r.sessions[id]; ok {
		session.EarnedPoints = earnedPoints
		session.Passed = passed
	}
	return nil
}

func (r *FakeSessionRepository) MarkGradeSynced(ctx context.Context, id string, syncedAt time.Time) error {
	if r.MarkGradeSyncedErr != nil {
		return r.MarkGradeSyncedErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if session, ok := r.sessions[id]; ok {
		session.GradeSyncedAt = &syncedAt
	}
	return nil
}

func (r *FakeSessionRepository) MarkGradeSyncFailed(ctx context.Context, id, errorMsg string) error {
	if r.MarkGradeSyncFailedErr != nil {
		return r.MarkGradeSyncFailedErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if session, ok := r.sessions[id]; ok {
		session.GradeSyncError = errorMsg
	}
	return nil
}

// AddSession adds a session directly to the mock (for test setup)
func (r *FakeSessionRepository) AddSession(session *models.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
}

// GetUserID returns the user ID for a session
func (r *FakeSessionRepository) GetUserID(ctx context.Context, id string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[id]
	if !ok {
		return "", nil
	}
	return session.UserID, nil
}

// IsOwner checks if a user owns a session
func (r *FakeSessionRepository) IsOwner(ctx context.Context, id, userID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[id]
	if !ok {
		return false, nil
	}
	return session.UserID == userID, nil
}

// GetOrganizationID returns the organization ID for a session
func (r *FakeSessionRepository) GetOrganizationID(ctx context.Context, id string) (*string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.sessions[id]
	if !ok {
		return nil, nil
	}
	return session.OrganizationID, nil
}

// ListAll returns all sessions
func (r *FakeSessionRepository) ListAll(ctx context.Context) ([]*models.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Session, 0, len(r.sessions))
	for _, session := range r.sessions {
		result = append(result, session)
	}
	return result, nil
}

// EndStaleSessionsCount is the number the fake reports as swept, so tests can
// exercise the bulk-end metric path.
func (r *FakeSessionRepository) EndStaleSessions(ctx context.Context, maxAge time.Duration) (int64, error) {
	return r.EndStaleSessionsCount, nil
}

func (r *FakeSessionRepository) DeleteEndedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	return 0, nil
}

// CountActive counts stored sessions that have not ended, mirroring the real
// repository's `ended_at IS NULL` predicate so reconcile tests are meaningful.
func (r *FakeSessionRepository) CountActive(ctx context.Context) (int64, error) {
	if r.CountActiveErr != nil {
		return 0, r.CountActiveErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var n int64
	for _, s := range r.sessions {
		if s.EndedAt == nil {
			n++
		}
	}
	return n, nil
}

// Delete removes a session by ID
func (r *FakeSessionRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, id)
	return nil
}

// GetUserStats returns aggregated user statistics
func (r *FakeSessionRepository) GetUserStats(ctx context.Context, userID string) (*repositories.UserSessionStats, error) {
	return &repositories.UserSessionStats{}, nil
}

// CountCompletedLabsByUser returns a map of user ID to completed lab count
func (r *FakeSessionRepository) CountCompletedLabsByUser(ctx context.Context) (map[string]int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make(map[string]int)
	for _, s := range r.sessions {
		if s.EndedAt != nil && s.Passed {
			result[s.UserID]++
		}
	}
	return result, nil
}

// ListWithLabNames returns sessions with lab template names
func (r *FakeSessionRepository) ListWithLabNames(ctx context.Context, filter repositories.SessionFilter) ([]*repositories.SessionWithLabName, error) {
	return nil, nil
}

// -----------------------------------------------------------------------------
// Mock Lab Template Repository
// -----------------------------------------------------------------------------

// FakeLabTemplateRepository is a mock implementation of LabTemplateRepository
type FakeLabTemplateRepository struct {
	mu        sync.RWMutex
	templates map[string]*models.LabTemplateRecord

	// Error injection
	CreateErr    error
	GetByIDErr   error
	GetByNameErr error
	ListErr      error
	UpdateErr    error
	DeleteErr    error
	SetActiveErr error
}

// NewFakeLabTemplateRepository creates a new mock lab template repository
func NewFakeLabTemplateRepository() *FakeLabTemplateRepository {
	return &FakeLabTemplateRepository{
		templates: make(map[string]*models.LabTemplateRecord),
	}
}

func (r *FakeLabTemplateRepository) Create(ctx context.Context, record *models.LabTemplateRecord) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	record.CreatedAt = time.Now()
	record.UpdatedAt = time.Now()
	r.templates[record.ID] = record
	return nil
}

func (r *FakeLabTemplateRepository) GetByID(ctx context.Context, id string) (*models.LabTemplateRecord, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	record, ok := r.templates[id]
	if !ok {
		return nil, nil
	}
	return record, nil
}

func (r *FakeLabTemplateRepository) GetByIDs(ctx context.Context, ids []string) ([]*models.LabTemplateRecord, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.LabTemplateRecord, 0)
	for _, id := range ids {
		if record, ok := r.templates[id]; ok {
			result = append(result, record)
		}
	}
	return result, nil
}

func (r *FakeLabTemplateRepository) GetByName(ctx context.Context, name string) (*models.LabTemplateRecord, error) {
	if r.GetByNameErr != nil {
		return nil, r.GetByNameErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, record := range r.templates {
		if record.Name == name {
			return record, nil
		}
	}
	return nil, nil
}

func (r *FakeLabTemplateRepository) List(ctx context.Context, filter repositories.LabTemplateFilter) ([]*models.LabTemplateRecord, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.LabTemplateRecord, 0)
	for _, record := range r.templates {
		if filter.Platform != "" && string(record.Platform) != filter.Platform {
			continue
		}
		if filter.Active != nil && record.IsActive != *filter.Active {
			continue
		}
		result = append(result, record)
	}
	return result, nil
}

func (r *FakeLabTemplateRepository) Update(ctx context.Context, record *models.LabTemplateRecord) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	record.UpdatedAt = time.Now()
	r.templates[record.ID] = record
	return nil
}

func (r *FakeLabTemplateRepository) Delete(ctx context.Context, id string) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.templates, id)
	return nil
}

func (r *FakeLabTemplateRepository) SetActive(ctx context.Context, id string, active bool) error {
	if r.SetActiveErr != nil {
		return r.SetActiveErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if record, ok := r.templates[id]; ok {
		record.IsActive = active
		record.UpdatedAt = time.Now()
	}
	return nil
}

// AddTemplate adds a template directly to the mock (for test setup)
func (r *FakeLabTemplateRepository) AddTemplate(record *models.LabTemplateRecord) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.templates[record.ID] = record
}

// CreateVersion is a no-op mock
func (r *FakeLabTemplateRepository) CreateVersion(_ context.Context, _ *models.LabTemplateVersion) error {
	return nil
}

// ListVersions returns empty list
func (r *FakeLabTemplateRepository) ListVersions(_ context.Context, _ string, _, _ int) ([]*models.LabTemplateVersion, error) {
	return nil, nil
}

// GetVersionByNumber returns nil
func (r *FakeLabTemplateRepository) GetVersionByNumber(_ context.Context, _ string, _ int) (*models.LabTemplateVersion, error) {
	return nil, nil
}

// CountVersions returns 0
func (r *FakeLabTemplateRepository) CountVersions(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// -----------------------------------------------------------------------------
// Mock Checkpoint Progress Repository
// -----------------------------------------------------------------------------

// FakeCheckpointProgressRepository is a mock implementation of CheckpointProgressRepository
type FakeCheckpointProgressRepository struct {
	mu       sync.RWMutex
	progress map[string]*models.CheckpointProgress // key: sessionID:checkpointID

	// Error injection
	CreateErr                    error
	GetBySessionIDErr            error
	GetBySessionAndCheckpointErr error
	UpdateErr                    error
	MarkPassedErr                error
	MarkFailedErr                error
	ResetForSessionErr           error
}

// NewFakeCheckpointProgressRepository creates a new mock checkpoint progress repository
func NewFakeCheckpointProgressRepository() *FakeCheckpointProgressRepository {
	return &FakeCheckpointProgressRepository{
		progress: make(map[string]*models.CheckpointProgress),
	}
}

func (r *FakeCheckpointProgressRepository) key(sessionID, checkpointID string) string {
	return sessionID + ":" + checkpointID
}

func (r *FakeCheckpointProgressRepository) Create(ctx context.Context, progress *models.CheckpointProgress) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress[r.key(progress.SessionID, progress.CheckpointID)] = progress
	return nil
}

func (r *FakeCheckpointProgressRepository) GetBySessionID(ctx context.Context, sessionID string) ([]*models.CheckpointProgress, error) {
	if r.GetBySessionIDErr != nil {
		return nil, r.GetBySessionIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.CheckpointProgress, 0)
	for key, p := range r.progress {
		if len(key) > len(sessionID) && key[:len(sessionID)] == sessionID {
			result = append(result, p)
		}
	}
	return result, nil
}

func (r *FakeCheckpointProgressRepository) GetBySessionAndCheckpoint(ctx context.Context, sessionID, checkpointID string) (*models.CheckpointProgress, error) {
	if r.GetBySessionAndCheckpointErr != nil {
		return nil, r.GetBySessionAndCheckpointErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.progress[r.key(sessionID, checkpointID)]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (r *FakeCheckpointProgressRepository) Update(ctx context.Context, progress *models.CheckpointProgress) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.progress[r.key(progress.SessionID, progress.CheckpointID)] = progress
	return nil
}

func (r *FakeCheckpointProgressRepository) MarkPassed(ctx context.Context, sessionID, checkpointID string, triggerEventID *string) error {
	if r.MarkPassedErr != nil {
		return r.MarkPassedErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.progress[r.key(sessionID, checkpointID)]; ok {
		p.Status = "passed"
		now := time.Now()
		p.PassedAt = &now
		p.TriggerEventID = triggerEventID
	}
	return nil
}

func (r *FakeCheckpointProgressRepository) MarkFailed(ctx context.Context, sessionID, checkpointID string) error {
	if r.MarkFailedErr != nil {
		return r.MarkFailedErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if p, ok := r.progress[r.key(sessionID, checkpointID)]; ok {
		p.Status = "failed"
	}
	return nil
}

func (r *FakeCheckpointProgressRepository) ResetForSession(ctx context.Context, sessionID string) error {
	if r.ResetForSessionErr != nil {
		return r.ResetForSessionErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for key := range r.progress {
		if len(key) > len(sessionID) && key[:len(sessionID)] == sessionID {
			delete(r.progress, key)
		}
	}
	return nil
}

// -----------------------------------------------------------------------------
// Mock User Repository
// -----------------------------------------------------------------------------

// FakeUserRepository is a mock implementation of UserRepository
type FakeUserRepository struct {
	mu    sync.RWMutex
	users map[string]*models.User // key: ID

	// Error injection
	CreateErr                  error
	GetByIDErr                 error
	GetByIDWithPasswordErr     error
	GetByUsernameErr           error
	GetByEmailErr              error
	GetByEmailForAuthErr       error
	GetOrCreateByUsernameErr   error
	ListErr                    error
	UpdateErr                  error
	DeleteErr                  error
	UpdatePasswordErr          error
	ClearMustChangePasswordErr error
}

// NewFakeUserRepository creates a new mock user repository
func NewFakeUserRepository() *FakeUserRepository {
	return &FakeUserRepository{
		users: make(map[string]*models.User),
	}
}

func (r *FakeUserRepository) Create(ctx context.Context, user *models.User) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	r.users[user.ID] = user
	return nil
}

func (r *FakeUserRepository) GetByID(ctx context.Context, id string) (*models.User, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[id]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (r *FakeUserRepository) GetByIDs(ctx context.Context, ids []string) ([]*models.User, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.User, 0)
	for _, id := range ids {
		if user, ok := r.users[id]; ok {
			result = append(result, user)
		}
	}
	return result, nil
}

func (r *FakeUserRepository) GetByExternalID(ctx context.Context, externalID string) (*models.User, error) {
	if r.GetByUsernameErr != nil {
		return nil, r.GetByUsernameErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, user := range r.users {
		if user.ExternalID == externalID {
			return user, nil
		}
	}
	return nil, nil
}

func (r *FakeUserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	if r.GetByUsernameErr != nil {
		return nil, r.GetByUsernameErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, user := range r.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, nil
}

func (r *FakeUserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	if r.GetByEmailErr != nil {
		return nil, r.GetByEmailErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, nil
}

func (r *FakeUserRepository) GetOrCreateByUsername(ctx context.Context, username string) (*models.User, error) {
	if r.GetOrCreateByUsernameErr != nil {
		return nil, r.GetOrCreateByUsernameErr
	}

	// Try to get existing user
	user, err := r.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	// If found, return it
	if user != nil {
		return user, nil
	}

	// Create new user
	r.mu.Lock()
	defer r.mu.Unlock()
	newUser := &models.User{
		ID:        "user-" + username,
		Username:  username,
		Email:     username + "@example.com",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	r.users[newUser.ID] = newUser
	return newUser, nil
}

func (r *FakeUserRepository) List(ctx context.Context, opts repositories.UserListOptions) ([]*models.User, int, error) {
	if r.ListErr != nil {
		return nil, 0, r.ListErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.User, 0, len(r.users))
	for _, user := range r.users {
		// Apply filters
		if opts.Role != "" && user.Role != opts.Role {
			continue
		}
		if opts.IsActive != nil && user.IsActive != *opts.IsActive {
			continue
		}
		result = append(result, user)
	}
	return result, len(result), nil
}

func (r *FakeUserRepository) UpdateLastLogin(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if user, ok := r.users[id]; ok {
		now := time.Now()
		user.LastLoginAt = &now
		user.UpdatedAt = now
	}
	return nil
}

func (r *FakeUserRepository) Update(ctx context.Context, user *models.User) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	user.UpdatedAt = time.Now()
	r.users[user.ID] = user
	return nil
}

func (r *FakeUserRepository) Delete(ctx context.Context, id string) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.users, id)
	return nil
}

// AddUser adds a user directly to the mock (for test setup)
func (r *FakeUserRepository) AddUser(user *models.User) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.users[user.ID] = user
}

// GetByIDWithPassword returns user with password hash
func (r *FakeUserRepository) GetByIDWithPassword(ctx context.Context, id string) (*models.User, error) {
	if r.GetByIDWithPasswordErr != nil {
		return nil, r.GetByIDWithPasswordErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[id]
	if !ok {
		return nil, nil
	}
	return user, nil
}

// GetByEmailForAuth returns user by email with password hash
func (r *FakeUserRepository) GetByEmailForAuth(ctx context.Context, email string) (*models.User, error) {
	if r.GetByEmailForAuthErr != nil {
		return nil, r.GetByEmailForAuthErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, nil
}

// UpdatePassword updates user's password hash
func (r *FakeUserRepository) UpdatePassword(ctx context.Context, id, passwordHash string, mustChange bool) error {
	if r.UpdatePasswordErr != nil {
		return r.UpdatePasswordErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if user, ok := r.users[id]; ok {
		user.PasswordHash = passwordHash
		user.MustChangePassword = mustChange
		now := time.Now()
		user.PasswordUpdatedAt = &now
		user.UpdatedAt = now
	}
	return nil
}

// ClearMustChangePassword clears the must_change_password flag
func (r *FakeUserRepository) ClearMustChangePassword(ctx context.Context, id string) error {
	if r.ClearMustChangePasswordErr != nil {
		return r.ClearMustChangePasswordErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if user, ok := r.users[id]; ok {
		user.MustChangePassword = false
		user.UpdatedAt = time.Now()
	}
	return nil
}

// GetPreferredLocale returns the user's stored preferred_locale, or nil.
func (r *FakeUserRepository) GetPreferredLocale(ctx context.Context, id string) (*string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	user, ok := r.users[id]
	if !ok {
		return nil, nil
	}
	return user.PreferredLocale, nil
}

// UpdatePreferredLocale sets (or clears with nil) the user's preferred_locale.
func (r *FakeUserRepository) UpdatePreferredLocale(ctx context.Context, id string, locale *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	user, ok := r.users[id]
	if !ok {
		return fmt.Errorf("user not found: %s", id)
	}
	user.PreferredLocale = locale
	user.UpdatedAt = time.Now()
	return nil
}

// -----------------------------------------------------------------------------
// Mock Pathway Repository
// -----------------------------------------------------------------------------

// FakePathwayRepository is a mock implementation of PathwayRepository
type FakePathwayRepository struct {
	mu       sync.RWMutex
	pathways map[string]*models.Pathway
	modules  map[string]*models.PathwayModule
	labs     map[string]*models.ModuleLab // key: moduleID:labTemplateID

	// Error injection
	CreateErr               error
	GetByIDErr              error
	GetBySlugErr            error
	ListErr                 error
	UpdateErr               error
	DeleteErr               error
	UpdateStatusErr         error
	GetWithModulesErr       error
	GetWithModulesBySlugErr error
	CreateModuleErr         error
	GetModuleByIDErr        error
	ListModulesErr          error
	UpdateModuleErr         error
	DeleteModuleErr         error
	AddLabToModuleErr       error
	RemoveLabFromModuleErr  error
	ListModuleLabsErr       error
	GetStatsErr             error
}

// NewFakePathwayRepository creates a new mock pathway repository
func NewFakePathwayRepository() *FakePathwayRepository {
	return &FakePathwayRepository{
		pathways: make(map[string]*models.Pathway),
		modules:  make(map[string]*models.PathwayModule),
		labs:     make(map[string]*models.ModuleLab),
	}
}

func (r *FakePathwayRepository) Create(ctx context.Context, pathway *models.Pathway) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	pathway.CreatedAt = time.Now()
	pathway.UpdatedAt = time.Now()
	r.pathways[pathway.ID] = pathway
	return nil
}

func (r *FakePathwayRepository) GetByID(ctx context.Context, id string) (*models.Pathway, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	pathway, ok := r.pathways[id]
	if !ok {
		return nil, nil
	}
	return pathway, nil
}

func (r *FakePathwayRepository) GetByIDs(ctx context.Context, ids []string) ([]*models.Pathway, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Pathway, 0)
	for _, id := range ids {
		if pathway, ok := r.pathways[id]; ok {
			result = append(result, pathway)
		}
	}
	return result, nil
}

func (r *FakePathwayRepository) GetBySlug(ctx context.Context, slug string) (*models.Pathway, error) {
	if r.GetBySlugErr != nil {
		return nil, r.GetBySlugErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, pathway := range r.pathways {
		if pathway.Slug == slug {
			return pathway, nil
		}
	}
	return nil, nil
}

func (r *FakePathwayRepository) List(ctx context.Context, opts models.PathwayListOptions) ([]*models.Pathway, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Pathway, 0)
	for _, pathway := range r.pathways {
		// Apply filters
		if opts.Status != "" && pathway.Status != opts.Status {
			continue
		}
		if opts.IsFeatured != nil && pathway.IsFeatured != *opts.IsFeatured {
			continue
		}
		if opts.Difficulty != "" && pathway.Difficulty != opts.Difficulty {
			continue
		}
		result = append(result, pathway)
	}
	return result, nil
}

func (r *FakePathwayRepository) Update(ctx context.Context, pathway *models.Pathway) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	pathway.UpdatedAt = time.Now()
	r.pathways[pathway.ID] = pathway
	return nil
}

func (r *FakePathwayRepository) Delete(ctx context.Context, id string) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pathways, id)
	return nil
}

func (r *FakePathwayRepository) UpdateStatus(ctx context.Context, id string, status models.PathwayStatus) error {
	if r.UpdateStatusErr != nil {
		return r.UpdateStatusErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if pathway, ok := r.pathways[id]; ok {
		pathway.Status = status
		pathway.UpdatedAt = time.Now()
	}
	return nil
}

func (r *FakePathwayRepository) GetWithModules(ctx context.Context, id string) (*models.Pathway, error) {
	if r.GetWithModulesErr != nil {
		return nil, r.GetWithModulesErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	pathway, ok := r.pathways[id]
	if !ok {
		return nil, nil
	}
	// Copy pathway and add modules
	result := *pathway
	result.Modules = make([]*models.PathwayModule, 0)
	for _, module := range r.modules {
		if module.PathwayID == id {
			result.Modules = append(result.Modules, module)
		}
	}
	return &result, nil
}

func (r *FakePathwayRepository) GetWithModulesBySlug(ctx context.Context, slug string) (*models.Pathway, error) {
	if r.GetWithModulesBySlugErr != nil {
		return nil, r.GetWithModulesBySlugErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, pathway := range r.pathways {
		if pathway.Slug == slug {
			result := *pathway
			result.Modules = make([]*models.PathwayModule, 0)
			for _, module := range r.modules {
				if module.PathwayID == pathway.ID {
					result.Modules = append(result.Modules, module)
				}
			}
			return &result, nil
		}
	}
	return nil, nil
}

func (r *FakePathwayRepository) CreateModule(ctx context.Context, module *models.PathwayModule) error {
	if r.CreateModuleErr != nil {
		return r.CreateModuleErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	module.CreatedAt = time.Now()
	r.modules[module.ID] = module
	return nil
}

func (r *FakePathwayRepository) GetModuleByID(ctx context.Context, id string) (*models.PathwayModule, error) {
	if r.GetModuleByIDErr != nil {
		return nil, r.GetModuleByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	module, ok := r.modules[id]
	if !ok {
		return nil, nil
	}
	return module, nil
}

func (r *FakePathwayRepository) ListModules(ctx context.Context, pathwayID string) ([]*models.PathwayModule, error) {
	if r.ListModulesErr != nil {
		return nil, r.ListModulesErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.PathwayModule, 0)
	for _, module := range r.modules {
		if module.PathwayID == pathwayID {
			result = append(result, module)
		}
	}
	return result, nil
}

func (r *FakePathwayRepository) UpdateModule(ctx context.Context, module *models.PathwayModule) error {
	if r.UpdateModuleErr != nil {
		return r.UpdateModuleErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modules[module.ID] = module
	return nil
}

func (r *FakePathwayRepository) DeleteModule(ctx context.Context, id string) error {
	if r.DeleteModuleErr != nil {
		return r.DeleteModuleErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.modules, id)
	return nil
}

func (r *FakePathwayRepository) ReorderModules(ctx context.Context, pathwayID string, moduleIDs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, id := range moduleIDs {
		if module, ok := r.modules[id]; ok && module.PathwayID == pathwayID {
			module.DisplayOrder = i
		}
	}
	return nil
}

func (r *FakePathwayRepository) AddLabToModule(ctx context.Context, moduleLab *models.ModuleLab) error {
	if r.AddLabToModuleErr != nil {
		return r.AddLabToModuleErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := moduleLab.ModuleID + ":" + moduleLab.LabTemplateID
	r.labs[key] = moduleLab
	return nil
}

func (r *FakePathwayRepository) RemoveLabFromModule(ctx context.Context, moduleID, labTemplateID string) error {
	if r.RemoveLabFromModuleErr != nil {
		return r.RemoveLabFromModuleErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := moduleID + ":" + labTemplateID
	delete(r.labs, key)
	return nil
}

func (r *FakePathwayRepository) ListModuleLabs(ctx context.Context, moduleID string) ([]*models.ModuleLab, error) {
	if r.ListModuleLabsErr != nil {
		return nil, r.ListModuleLabsErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.ModuleLab, 0)
	for _, lab := range r.labs {
		if lab.ModuleID == moduleID {
			result = append(result, lab)
		}
	}
	return result, nil
}

func (r *FakePathwayRepository) ReorderModuleLabs(ctx context.Context, moduleID string, labTemplateIDs []string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, labID := range labTemplateIDs {
		key := moduleID + ":" + labID
		if lab, ok := r.labs[key]; ok {
			lab.DisplayOrder = i
		}
	}
	return nil
}

func (r *FakePathwayRepository) GetStats(ctx context.Context, pathwayID string) (*models.PathwayStats, error) {
	if r.GetStatsErr != nil {
		return nil, r.GetStatsErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	moduleCount := 0
	for _, module := range r.modules {
		if module.PathwayID == pathwayID {
			moduleCount++
		}
	}

	return &models.PathwayStats{
		PathwayID:       pathwayID,
		ModuleCount:     moduleCount,
		LabCount:        0,
		TotalPoints:     0,
		EnrollmentCount: 0,
		CompletionCount: 0,
		CompletionRate:  0,
	}, nil
}

// AddPathway adds a pathway directly to the mock (for test setup)
func (r *FakePathwayRepository) AddPathway(pathway *models.Pathway) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pathways[pathway.ID] = pathway
}

// AddModule adds a module directly to the mock (for test setup)
func (r *FakePathwayRepository) AddModule(module *models.PathwayModule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.modules[module.ID] = module
}

// -----------------------------------------------------------------------------
// Mock Enrollment Repository
// -----------------------------------------------------------------------------

// FakeEnrollmentRepository is a mock implementation of EnrollmentRepository
type FakeEnrollmentRepository struct {
	mu          sync.RWMutex
	enrollments map[string]*models.PathwayEnrollment
	progress    map[string]*models.ModuleProgress // key: enrollmentID:moduleID

	// Error injection
	CreateErr                   error
	GetByIDErr                  error
	GetByUserAndPathwayErr      error
	ListErr                     error
	UpdateErr                   error
	DeleteErr                   error
	UpdateStatusErr             error
	GetWithProgressErr          error
	InitializeModuleProgressErr error
	GetModuleProgressErr        error
	ListModuleProgressErr       error
	UpdateModuleProgressErr     error
	UnlockModuleErr             error
}

// NewFakeEnrollmentRepository creates a new mock enrollment repository
func NewFakeEnrollmentRepository() *FakeEnrollmentRepository {
	return &FakeEnrollmentRepository{
		enrollments: make(map[string]*models.PathwayEnrollment),
		progress:    make(map[string]*models.ModuleProgress),
	}
}

func (r *FakeEnrollmentRepository) Create(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	enrollment.EnrolledAt = time.Now()
	r.enrollments[enrollment.ID] = enrollment
	return nil
}

func (r *FakeEnrollmentRepository) GetByID(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	enrollment, ok := r.enrollments[id]
	if !ok {
		return nil, nil
	}
	return enrollment, nil
}

func (r *FakeEnrollmentRepository) GetByUserAndPathway(ctx context.Context, userID, pathwayID string) (*models.PathwayEnrollment, error) {
	if r.GetByUserAndPathwayErr != nil {
		return nil, r.GetByUserAndPathwayErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, enrollment := range r.enrollments {
		if enrollment.UserID == userID && enrollment.PathwayID == pathwayID {
			return enrollment, nil
		}
	}
	return nil, nil
}

func (r *FakeEnrollmentRepository) List(ctx context.Context, opts models.EnrollmentListOptions) ([]*models.PathwayEnrollment, error) {
	if r.ListErr != nil {
		return nil, r.ListErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.PathwayEnrollment, 0)
	for _, enrollment := range r.enrollments {
		if opts.UserID != "" && enrollment.UserID != opts.UserID {
			continue
		}
		if opts.PathwayID != "" && enrollment.PathwayID != opts.PathwayID {
			continue
		}
		if opts.Status != "" && enrollment.Status != opts.Status {
			continue
		}
		result = append(result, enrollment)
	}
	return result, nil
}

func (r *FakeEnrollmentRepository) Update(ctx context.Context, enrollment *models.PathwayEnrollment) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	enrollment.LastActivityAt = &now
	r.enrollments[enrollment.ID] = enrollment
	return nil
}

func (r *FakeEnrollmentRepository) Delete(ctx context.Context, id string) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.enrollments, id)
	return nil
}

func (r *FakeEnrollmentRepository) UpdateStatus(ctx context.Context, id string, status models.EnrollmentStatus) error {
	if r.UpdateStatusErr != nil {
		return r.UpdateStatusErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if enrollment, ok := r.enrollments[id]; ok {
		enrollment.Status = status
		now := time.Now()
		enrollment.LastActivityAt = &now
	}
	return nil
}

func (r *FakeEnrollmentRepository) GetWithProgress(ctx context.Context, id string) (*models.PathwayEnrollment, error) {
	if r.GetWithProgressErr != nil {
		return nil, r.GetWithProgressErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	enrollment, ok := r.enrollments[id]
	if !ok {
		return nil, nil
	}
	return enrollment, nil
}

func (r *FakeEnrollmentRepository) InitializeModuleProgress(ctx context.Context, enrollmentID, pathwayID string) error {
	if r.InitializeModuleProgressErr != nil {
		return r.InitializeModuleProgressErr
	}
	return nil
}

func (r *FakeEnrollmentRepository) GetModuleProgress(ctx context.Context, enrollmentID, moduleID string) (*models.ModuleProgress, error) {
	if r.GetModuleProgressErr != nil {
		return nil, r.GetModuleProgressErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	key := enrollmentID + ":" + moduleID
	progress, ok := r.progress[key]
	if !ok {
		return nil, nil
	}
	return progress, nil
}

func (r *FakeEnrollmentRepository) ListModuleProgress(ctx context.Context, enrollmentID string) ([]*models.ModuleProgress, error) {
	if r.ListModuleProgressErr != nil {
		return nil, r.ListModuleProgressErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.ModuleProgress, 0)
	for key, progress := range r.progress {
		if len(key) > len(enrollmentID)+1 && key[:len(enrollmentID)] == enrollmentID {
			result = append(result, progress)
		}
	}
	return result, nil
}

func (r *FakeEnrollmentRepository) UpdateModuleProgress(ctx context.Context, progress *models.ModuleProgress) error {
	if r.UpdateModuleProgressErr != nil {
		return r.UpdateModuleProgressErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := progress.EnrollmentID + ":" + progress.ModuleID
	r.progress[key] = progress
	return nil
}

func (r *FakeEnrollmentRepository) UnlockModule(ctx context.Context, enrollmentID, moduleID string) error {
	if r.UnlockModuleErr != nil {
		return r.UnlockModuleErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := enrollmentID + ":" + moduleID
	if progress, ok := r.progress[key]; ok {
		now := time.Now()
		progress.UnlockedAt = &now
	}
	return nil
}

func (r *FakeEnrollmentRepository) UnlockNextModules(ctx context.Context, enrollmentID, completedModuleID string) ([]string, error) {
	// Mock implementation - returns empty list (no modules unlocked)
	return nil, nil
}

func (r *FakeEnrollmentRepository) GetLabProgress(ctx context.Context, enrollmentID, labTemplateID string) (*models.LabProgress, error) {
	return nil, nil
}

func (r *FakeEnrollmentRepository) ListLabProgress(ctx context.Context, enrollmentID, moduleID string) ([]*models.LabProgress, error) {
	return nil, nil
}

func (r *FakeEnrollmentRepository) UpdateLabProgress(ctx context.Context, progress *models.LabProgress) error {
	return nil
}

func (r *FakeEnrollmentRepository) RecordLabAttempt(ctx context.Context, enrollmentID, moduleID, labTemplateID, sessionID string, score int, passed bool) error {
	return nil
}

func (r *FakeEnrollmentRepository) RecalculateModuleProgress(ctx context.Context, enrollmentID, moduleID string) error {
	return nil
}

func (r *FakeEnrollmentRepository) RecalculateEnrollmentProgress(ctx context.Context, enrollmentID string) error {
	return nil
}

// AddEnrollment adds an enrollment directly to the mock (for test setup)
func (r *FakeEnrollmentRepository) AddEnrollment(enrollment *models.PathwayEnrollment) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enrollments[enrollment.ID] = enrollment
}

// -----------------------------------------------------------------------------
// Mock Audit Log Repository
// -----------------------------------------------------------------------------

// FakeAuditLogRepository is a mock implementation of AuditLogRepository
type FakeAuditLogRepository struct {
	mu      sync.RWMutex
	entries []*models.AuditEntry

	// Error injection
	CreateErr error
	QueryErr  error
}

// NewFakeAuditLogRepository creates a new mock audit log repository
func NewFakeAuditLogRepository() *FakeAuditLogRepository {
	return &FakeAuditLogRepository{
		entries: make([]*models.AuditEntry, 0),
	}
}

func (r *FakeAuditLogRepository) Create(ctx context.Context, entry *models.AuditEntry) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	entry.ID = int64(len(r.entries) + 1)
	entry.Timestamp = time.Now()
	r.entries = append(r.entries, entry)
	return nil
}

func (r *FakeAuditLogRepository) Query(ctx context.Context, filter repositories.AuditFilter) ([]*models.AuditEntry, error) {
	if r.QueryErr != nil {
		return nil, r.QueryErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*models.AuditEntry, 0)
	for _, entry := range r.entries {
		// Apply filters
		if filter.ActorID != "" && entry.ActorID != filter.ActorID {
			continue
		}
		if filter.Action != "" && entry.Action != filter.Action {
			continue
		}
		if filter.ResourceType != "" && entry.ResourceType != filter.ResourceType {
			continue
		}
		if filter.ResourceID != "" && entry.ResourceID != filter.ResourceID {
			continue
		}
		if filter.StartTime != nil && entry.Timestamp.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && entry.Timestamp.After(*filter.EndTime) {
			continue
		}
		result = append(result, entry)
	}

	// Apply limit
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

// AddEntry adds an entry directly to the mock (for test setup)
func (r *FakeAuditLogRepository) AddEntry(entry *models.AuditEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, entry)
}

// -----------------------------------------------------------------------------
// Mock Event Repository
// -----------------------------------------------------------------------------

// FakeEventRepository is a mock implementation of EventRepository
type FakeEventRepository struct {
	mu     sync.RWMutex
	events []*models.Event

	// Error injection
	CreateErr         error
	GetByIDErr        error
	GetByPodIDErr     error
	GetBySessionIDErr error
	GetUnprocessedErr error
	MarkProcessedErr  error
	QueryErr          error
}

// NewFakeEventRepository creates a new mock event repository
func NewFakeEventRepository() *FakeEventRepository {
	return &FakeEventRepository{
		events: make([]*models.Event, 0),
	}
}

func (r *FakeEventRepository) Create(ctx context.Context, event *models.Event) error {
	if r.CreateErr != nil {
		return r.CreateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	event.ID = int64(len(r.events) + 1)
	r.events = append(r.events, event)
	return nil
}

func (r *FakeEventRepository) GetByID(ctx context.Context, id int64) (*models.Event, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, event := range r.events {
		if event.ID == id {
			return event, nil
		}
	}
	return nil, nil
}

func (r *FakeEventRepository) GetByPodID(ctx context.Context, podID string, limit int) ([]*models.Event, error) {
	if r.GetByPodIDErr != nil {
		return nil, r.GetByPodIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Event, 0)
	for _, event := range r.events {
		if event.PodID == podID {
			result = append(result, event)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (r *FakeEventRepository) GetBySessionID(ctx context.Context, sessionID string, limit int) ([]*models.Event, error) {
	if r.GetBySessionIDErr != nil {
		return nil, r.GetBySessionIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Event, 0)
	for _, event := range r.events {
		if event.SessionID == sessionID {
			result = append(result, event)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (r *FakeEventRepository) GetUnprocessed(ctx context.Context, limit int) ([]*models.Event, error) {
	if r.GetUnprocessedErr != nil {
		return nil, r.GetUnprocessedErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Event, 0)
	for _, event := range r.events {
		if !event.Processed {
			result = append(result, event)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}

func (r *FakeEventRepository) MarkProcessed(ctx context.Context, id int64, matchedCheckpoints []string) error {
	if r.MarkProcessedErr != nil {
		return r.MarkProcessedErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, event := range r.events {
		if event.ID == id {
			event.Processed = true
			event.MatchedCheckpoints = matchedCheckpoints
			break
		}
	}
	return nil
}

func (r *FakeEventRepository) Query(ctx context.Context, filter repositories.EventFilter) ([]*models.Event, error) {
	if r.QueryErr != nil {
		return nil, r.QueryErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.Event, 0)
	for _, event := range r.events {
		if filter.PodID != "" && event.PodID != filter.PodID {
			continue
		}
		if filter.SessionID != "" && event.SessionID != filter.SessionID {
			continue
		}
		if filter.EventType != "" && event.EventType != filter.EventType {
			continue
		}
		if filter.VMName != "" && event.VMName != filter.VMName {
			continue
		}
		if filter.StartTime != nil && event.Timestamp.Before(*filter.StartTime) {
			continue
		}
		if filter.EndTime != nil && event.Timestamp.After(*filter.EndTime) {
			continue
		}
		result = append(result, event)
	}
	// Apply limit and offset
	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}
	return result, nil
}

// AddEvent adds an event directly to the mock (for test setup)
func (r *FakeEventRepository) AddEvent(event *models.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if event.ID == 0 {
		event.ID = int64(len(r.events) + 1)
	}
	r.events = append(r.events, event)
}

// -----------------------------------------------------------------------------
// Mock Organization Membership Repository
// -----------------------------------------------------------------------------

// FakeOrganizationMembershipRepository is a mock implementation of
// OrganizationMembershipRepository.
//
// GetPrimaryOrganization mirrors the real query's predicate
// (`is_primary = true AND accepted_at IS NOT NULL`), so a membership written
// without an AcceptedAt is invisible to it — the same way it would be in
// Postgres.
type FakeOrganizationMembershipRepository struct {
	mu          sync.RWMutex
	memberships map[string]*models.OrganizationMembership // key: ID

	// Created records every membership passed to Create, in call order, even
	// when CreateErr is set. Tests assert on the count and on field values.
	Created []*models.OrganizationMembership

	// Error injection
	CreateErr                 error
	GetByIDErr                error
	GetByOrgAndUserErr        error
	ListByOrganizationErr     error
	ListByUserErr             error
	UpdateErr                 error
	UpdateRoleErr             error
	DeleteErr                 error
	AcceptInvitationErr       error
	SetPrimaryErr             error
	GetPrimaryOrganizationErr error
}

var _ repositories.OrganizationMembershipRepository = (*FakeOrganizationMembershipRepository)(nil)

// NewFakeOrganizationMembershipRepository creates a new mock organization
// membership repository.
func NewFakeOrganizationMembershipRepository() *FakeOrganizationMembershipRepository {
	return &FakeOrganizationMembershipRepository{
		memberships: make(map[string]*models.OrganizationMembership),
	}
}

func (r *FakeOrganizationMembershipRepository) Create(ctx context.Context, membership *models.OrganizationMembership) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Record the attempt before the error check so tests can assert on what a
	// failing Create was asked to write.
	r.Created = append(r.Created, membership)
	if r.CreateErr != nil {
		return r.CreateErr
	}
	membership.CreatedAt = time.Now()
	membership.UpdatedAt = time.Now()
	r.memberships[membership.ID] = membership
	return nil
}

func (r *FakeOrganizationMembershipRepository) GetByID(ctx context.Context, id string) (*models.OrganizationMembership, error) {
	if r.GetByIDErr != nil {
		return nil, r.GetByIDErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	membership, ok := r.memberships[id]
	if !ok {
		return nil, nil
	}
	return membership, nil
}

func (r *FakeOrganizationMembershipRepository) GetByOrgAndUser(ctx context.Context, orgID, userID string) (*models.OrganizationMembership, error) {
	if r.GetByOrgAndUserErr != nil {
		return nil, r.GetByOrgAndUserErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, membership := range r.memberships {
		if membership.OrganizationID == orgID && membership.UserID == userID {
			return membership, nil
		}
	}
	return nil, nil
}

func (r *FakeOrganizationMembershipRepository) ListByOrganization(ctx context.Context, orgID string, filter repositories.MembershipFilter) ([]*models.OrganizationMembership, error) {
	if r.ListByOrganizationErr != nil {
		return nil, r.ListByOrganizationErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.OrganizationMembership, 0)
	for _, membership := range r.memberships {
		if membership.OrganizationID != orgID {
			continue
		}
		if filter.Role != "" && membership.Role != filter.Role {
			continue
		}
		if !filter.IncludePending && membership.AcceptedAt == nil {
			continue
		}
		result = append(result, membership)
	}
	return result, nil
}

func (r *FakeOrganizationMembershipRepository) ListByUser(ctx context.Context, userID string) ([]*models.OrganizationMembership, error) {
	if r.ListByUserErr != nil {
		return nil, r.ListByUserErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*models.OrganizationMembership, 0)
	for _, membership := range r.memberships {
		if membership.UserID == userID {
			result = append(result, membership)
		}
	}
	return result, nil
}

func (r *FakeOrganizationMembershipRepository) Update(ctx context.Context, membership *models.OrganizationMembership) error {
	if r.UpdateErr != nil {
		return r.UpdateErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.memberships[membership.ID]; !ok {
		return fmt.Errorf("membership not found: %s", membership.ID)
	}
	membership.UpdatedAt = time.Now()
	r.memberships[membership.ID] = membership
	return nil
}

func (r *FakeOrganizationMembershipRepository) UpdateRole(ctx context.Context, id string, role models.OrgRole) error {
	if r.UpdateRoleErr != nil {
		return r.UpdateRoleErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	membership, ok := r.memberships[id]
	if !ok {
		return fmt.Errorf("membership not found: %s", id)
	}
	membership.Role = role
	membership.UpdatedAt = time.Now()
	return nil
}

func (r *FakeOrganizationMembershipRepository) Delete(ctx context.Context, id string) error {
	if r.DeleteErr != nil {
		return r.DeleteErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.memberships, id)
	return nil
}

func (r *FakeOrganizationMembershipRepository) AcceptInvitation(ctx context.Context, token string) (*models.OrganizationMembership, error) {
	if r.AcceptInvitationErr != nil {
		return nil, r.AcceptInvitationErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, membership := range r.memberships {
		if membership.InvitationToken != nil && *membership.InvitationToken == token {
			now := time.Now()
			membership.AcceptedAt = &now
			membership.InvitationToken = nil
			membership.UpdatedAt = now
			return membership, nil
		}
	}
	return nil, fmt.Errorf("invitation not found")
}

func (r *FakeOrganizationMembershipRepository) SetPrimary(ctx context.Context, userID, orgID string) error {
	if r.SetPrimaryErr != nil {
		return r.SetPrimaryErr
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, membership := range r.memberships {
		if membership.UserID != userID {
			continue
		}
		membership.IsPrimary = membership.OrganizationID == orgID
		membership.UpdatedAt = time.Now()
	}
	return nil
}

func (r *FakeOrganizationMembershipRepository) GetPrimaryOrganization(ctx context.Context, userID string) (*models.Organization, error) {
	if r.GetPrimaryOrganizationErr != nil {
		return nil, r.GetPrimaryOrganizationErr
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, membership := range r.memberships {
		// Mirrors the real query: is_primary = true AND accepted_at IS NOT NULL.
		if membership.UserID == userID && membership.IsPrimary && membership.AcceptedAt != nil {
			return &models.Organization{ID: membership.OrganizationID}, nil
		}
	}
	return nil, nil
}

// AddMembership adds a membership directly to the mock (for test setup),
// bypassing Create so it is not recorded in Created.
func (r *FakeOrganizationMembershipRepository) AddMembership(membership *models.OrganizationMembership) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.memberships[membership.ID] = membership
}

// CreateCount returns how many times Create was invoked.
func (r *FakeOrganizationMembershipRepository) CreateCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.Created)
}
