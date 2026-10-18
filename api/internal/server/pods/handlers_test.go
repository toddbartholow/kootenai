package pods

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/toddbartholow/kootenai/api/internal/auth"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/orchestrator"
	"github.com/toddbartholow/kootenai/api/internal/server/httputil"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
)

// -----------------------------------------------------------------------------
// canAccessPod Tests
// -----------------------------------------------------------------------------

func TestCanAccessPod(t *testing.T) {
	orgID := "org-123"
	otherOrgID := "org-other"
	userID := "user-456"
	otherUserID := "user-789"

	tests := []struct {
		name     string
		user     *auth.User
		pod      *models.Pod
		expected bool
	}{
		{
			name: "admin can access any pod",
			user: &auth.User{ID: userID, Roles: []string{"admin"}},
			pod: &models.Pod{
				OwnerID:        otherUserID,
				OrganizationID: &orgID,
			},
			expected: true,
		},
		{
			name: "instructor can access any pod",
			user: &auth.User{ID: userID, Roles: []string{"instructor"}},
			pod: &models.Pod{
				OwnerID:        otherUserID,
				OrganizationID: &orgID,
			},
			expected: true,
		},
		{
			name: "owner can access own pod via OwnerID",
			user: &auth.User{ID: userID, Roles: []string{"student"}},
			pod: &models.Pod{
				OwnerID: userID,
			},
			expected: true,
		},
		{
			name: "owner can access own pod via Owner name",
			user: &auth.User{ID: userID, Name: "Test User", Roles: []string{"student"}},
			pod: &models.Pod{
				Owner: "Test User",
			},
			expected: true,
		},
		{
			name: "user in same org can access pod",
			user: &auth.User{ID: userID, DefaultOrganizationID: orgID, Roles: []string{"student"}},
			pod: &models.Pod{
				OwnerID:        otherUserID,
				OrganizationID: &orgID,
			},
			expected: true,
		},
		{
			name: "user in different org cannot access pod",
			user: &auth.User{ID: userID, Name: "User A", DefaultOrganizationID: otherOrgID, Roles: []string{"student"}},
			pod: &models.Pod{
				OwnerID:        otherUserID,
				Owner:          "User B",
				OrganizationID: &orgID,
			},
			expected: false,
		},
		{
			name: "pod without org is accessible to anyone",
			user: &auth.User{ID: userID, DefaultOrganizationID: orgID, Roles: []string{"student"}},
			pod: &models.Pod{
				OwnerID:        otherUserID,
				OrganizationID: nil,
			},
			expected: true,
		},
		{
			name: "nil user cannot access pod without org",
			user: nil,
			pod: &models.Pod{
				OwnerID:        userID,
				OrganizationID: nil,
			},
			expected: false,
		},
		{
			name: "nil user cannot access pod with org",
			user: nil,
			pod: &models.Pod{
				OwnerID:        userID,
				OrganizationID: &orgID,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canAccessPod(tt.user, tt.pod)
			if result != tt.expected {
				t.Errorf("canAccessPod() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// canModifyPod Tests
// -----------------------------------------------------------------------------

func TestCanModifyPod(t *testing.T) {
	userID := "user-123"
	otherUserID := "user-456"

	tests := []struct {
		name     string
		user     *auth.User
		pod      *models.Pod
		expected bool
	}{
		{
			name: "nil user cannot modify",
			user: nil,
			pod: &models.Pod{
				OwnerID: userID,
			},
			expected: false,
		},
		{
			name: "admin can modify any pod",
			user: &auth.User{ID: otherUserID, Roles: []string{"admin"}},
			pod: &models.Pod{
				OwnerID: userID,
			},
			expected: true,
		},
		{
			name: "instructor can modify any pod",
			user: &auth.User{ID: otherUserID, Roles: []string{"instructor"}},
			pod: &models.Pod{
				OwnerID: userID,
			},
			expected: true,
		},
		{
			name: "owner can modify own pod via OwnerID",
			user: &auth.User{ID: userID, Roles: []string{"student"}},
			pod: &models.Pod{
				OwnerID: userID,
			},
			expected: true,
		},
		{
			name: "owner can modify own pod via Owner name",
			user: &auth.User{ID: userID, Name: "Test User", Roles: []string{"student"}},
			pod: &models.Pod{
				Owner: "Test User",
			},
			expected: true,
		},
		{
			name: "non-owner cannot modify pod",
			user: &auth.User{ID: otherUserID, Name: "Other User", Roles: []string{"student"}},
			pod: &models.Pod{
				OwnerID: userID,
				Owner:   "Original Owner",
			},
			expected: false,
		},
		{
			name: "user with no matching ID or name cannot modify",
			user: &auth.User{ID: otherUserID, Name: "Other User", Roles: []string{"student"}},
			pod: &models.Pod{
				OwnerID: userID,
				Owner:   "Test User",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canModifyPod(tt.user, tt.pod)
			if result != tt.expected {
				t.Errorf("canModifyPod() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Manager Handler Tests with FakeOrchestrator
// -----------------------------------------------------------------------------

// podTestLogger creates a no-op logger for pod handler tests
func podTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// newTestManager creates a Manager with mock orchestrator for testing
func newTestManager(mockOrch *mocks.FakeOrchestrator) *Manager {
	logger := podTestLogger()
	return NewManager(Config{
		Orchestrator: mockOrch,
		Logger:       logger,
		Responder:    httputil.NewResponder(logger),
	})
}

func TestManager_HandleListPods_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*mocks.FakeOrchestrator)
		queryParams    string
		expectedStatus int
		expectedCount  int
		expectError    bool
	}{
		{
			name: "returns all pods successfully",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{ID: "pod-1", Owner: "user1", Status: models.PodStatusRunning})
				m.AddPod(&models.Pod{ID: "pod-2", Owner: "user2", Status: models.PodStatusRunning})
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name: "returns empty list when no pods",
			setupMock: func(m *mocks.FakeOrchestrator) {
				// No pods added
			},
			expectedStatus: http.StatusOK,
			expectedCount:  0,
		},
		{
			name: "filters by owner",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{ID: "pod-1", Owner: "user1", OwnerID: "user1", Status: models.PodStatusRunning})
				m.AddPod(&models.Pod{ID: "pod-2", Owner: "user2", OwnerID: "user2", Status: models.PodStatusRunning})
			},
			queryParams:    "?owner=user1",
			expectedStatus: http.StatusOK,
			expectedCount:  1,
		},
		{
			name: "handles orchestrator error",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.ListPodsErr = errors.New("connection failed")
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleListPods()

			req := httptest.NewRequest(http.MethodGet, "/api/v1/pods"+tt.queryParams, nil)
			rr := httptest.NewRecorder()

			handler(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.expectError {
				var resp map[string]string
				json.NewDecoder(rr.Body).Decode(&resp)
				if _, ok := resp["error"]; !ok {
					t.Error("expected error response")
				}
			} else if !tt.expectError {
				var resp ListPodsResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if len(resp.Pods) != tt.expectedCount {
					t.Errorf("expected %d pods, got %d", tt.expectedCount, len(resp.Pods))
				}
			}
		})
	}
}

func TestManager_HandleGetPod_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		setupMock      func(*mocks.FakeOrchestrator)
		expectedStatus int
		expectError    bool
	}{
		{
			name:  "returns pod successfully",
			podID: "pod-123",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:     "pod-123",
					Owner:  "testuser",
					Status: models.PodStatusRunning,
					VMs: []models.PodVM{
						{Name: "vm1", Status: "running"},
					},
				})
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "returns 404 for non-existent pod",
			podID: "pod-nonexistent",
			setupMock: func(m *mocks.FakeOrchestrator) {
				// No pods added
			},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
		{
			name:  "handles orchestrator error as 404",
			podID: "pod-123",
			setupMock: func(m *mocks.FakeOrchestrator) {
				// Note: The handler treats all GetPod errors as "not found" (404)
				m.GetPodErr = errors.New("database error")
			},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleGetPod()

			// Use chi router to set URL params
			r := chi.NewRouter()
			r.Get("/pods/{podID}", handler)

			req := httptest.NewRequest(http.MethodGet, "/pods/"+tt.podID, nil)
			// Add auth context matching the pod owner
			ctx := auth.ContextWithUser(req.Context(), &auth.User{
				ID:   "user-1",
				Name: "testuser",
			})
			req = req.WithContext(ctx)
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			// Verify GetPod was called with correct ID
			if len(mockOrch.GetPodCalls) > 0 && mockOrch.GetPodCalls[0] != tt.podID {
				t.Errorf("expected GetPod called with %s, got %s", tt.podID, mockOrch.GetPodCalls[0])
			}
		})
	}

	// Test access denied for org-scoped pod
	t.Run("access denied for different org", func(t *testing.T) {
		mockOrch := mocks.NewFakeOrchestrator()
		orgID := "org-123"
		mockOrch.AddPod(&models.Pod{
			ID:             "pod-123",
			Owner:          "other-user",
			OwnerID:        "other-user-id",
			OrganizationID: &orgID,
			Status:         models.PodStatusRunning,
		})

		pm := newTestManager(mockOrch)
		handler := pm.handleGetPod()

		r := chi.NewRouter()
		r.Get("/pods/{podID}", handler)

		req := httptest.NewRequest(http.MethodGet, "/pods/pod-123", nil)
		// Add user from different org
		ctx := auth.ContextWithUser(req.Context(), &auth.User{
			ID:                    "different-user",
			Name:                  "Different User",
			Roles:                 []string{"student"},
			DefaultOrganizationID: "different-org",
		})
		req = req.WithContext(ctx)
		rr := httptest.NewRecorder()

		r.ServeHTTP(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected status %d, got %d", http.StatusForbidden, rr.Code)
		}
	})
}

func TestManager_HandleStartPod_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
	}{
		{
			name:  "starts pod successfully",
			podID: "pod-123",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusStopped,
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "handles start error",
			podID: "pod-123",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusStopped,
				})
				m.StartPodErr = errors.New("VM start failed")
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:  "admin can start any pod",
			podID: "pod-123",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "otheruser",
					OwnerID: "user-other",
					Status:  models.PodStatusStopped,
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "admin-123",
					Name:  "admin",
					Roles: []string{"admin"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleStartPod()

			r := chi.NewRouter()
			r.Post("/pods/{podID}/start", handler)

			req := httptest.NewRequest(http.MethodPost, "/pods/"+tt.podID+"/start", nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			// Verify StartPod was called (unless there was a permission error)
			if tt.expectedStatus == http.StatusOK && len(mockOrch.StartPodCalls) == 0 {
				t.Error("expected StartPod to be called")
			}
		})
	}
}

func TestManager_HandleStopPod_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
	}{
		{
			name:  "stops pod successfully",
			podID: "pod-123",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "handles stop error",
			podID: "pod-123",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
				})
				m.StopPodErr = errors.New("VM stop failed")
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleStopPod()

			r := chi.NewRouter()
			r.Post("/pods/{podID}/stop", handler)

			req := httptest.NewRequest(http.MethodPost, "/pods/"+tt.podID+"/stop", nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManager_HandleResetVM_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		vmName         string
		body           string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
	}{
		{
			name:   "resets VM successfully",
			podID:  "pod-123",
			vmName: "vm1",
			body:   `{"snapshot": "baseline"}`,
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "handles reset error",
			podID:  "pod-123",
			vmName: "vm1",
			body:   `{"snapshot": "baseline"}`,
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
				m.ResetPodVMErr = fmt.Errorf("snapshot: %w", orchestrator.ErrNotFound)
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:   "requires snapshot name",
			podID:  "pod-123",
			vmName: "vm1",
			body:   `{}`,
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleResetVM()

			r := chi.NewRouter()
			r.Post("/pods/{podID}/vms/{vmName}/reset", handler)

			req := httptest.NewRequest(http.MethodPost, "/pods/"+tt.podID+"/vms/"+tt.vmName+"/reset", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			// Verify mock was called correctly for successful resets
			if tt.expectedStatus == http.StatusOK {
				if len(mockOrch.ResetPodVMCalls) != 1 {
					t.Errorf("expected 1 ResetPodVM call, got %d", len(mockOrch.ResetPodVMCalls))
				} else {
					call := mockOrch.ResetPodVMCalls[0]
					if call.PodID != tt.podID {
						t.Errorf("expected podID %s, got %s", tt.podID, call.PodID)
					}
					if call.VMName != tt.vmName {
						t.Errorf("expected vmName %s, got %s", tt.vmName, call.VMName)
					}
				}
			}
		})
	}
}

func TestManager_HandleListSnapshots_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		vmName         string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
		expectedCount  int
	}{
		{
			name:   "lists snapshots successfully",
			podID:  "pod-123",
			vmName: "vm1",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
				m.Snapshots = []orchestrator.VMSnapshot{
					{Name: "baseline", Description: "Initial state"},
					{Name: "checkpoint1", Description: "After step 1"},
				}
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
			expectedCount:  2,
		},
		{
			name:   "handles error listing snapshots",
			podID:  "pod-123",
			vmName: "vm1",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
				m.ListVMSnapshotsErr = errors.New("failed to list snapshots")
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleListSnapshots()

			r := chi.NewRouter()
			r.Get("/pods/{podID}/vms/{vmName}/snapshots", handler)

			req := httptest.NewRequest(http.MethodGet, "/pods/"+tt.podID+"/vms/"+tt.vmName+"/snapshots", nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if tt.expectedStatus == http.StatusOK {
				var resp ListSnapshotsResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.Count != tt.expectedCount {
					t.Errorf("expected count %d, got %d", tt.expectedCount, resp.Count)
				}
			}
		})
	}
}

func TestManager_HandleStartVM_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		vmName         string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
	}{
		{
			name:   "starts VM successfully",
			podID:  "pod-123",
			vmName: "vm1",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "stopped"}},
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "handles start error",
			podID:  "pod-123",
			vmName: "vm1",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "stopped"}},
				})
				m.StartVMErr = errors.New("start failed")
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleStartVM()

			r := chi.NewRouter()
			r.Post("/pods/{podID}/vms/{vmName}/start", handler)

			req := httptest.NewRequest(http.MethodPost, "/pods/"+tt.podID+"/vms/"+tt.vmName+"/start", nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManager_HandleStopVM_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		vmName         string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
	}{
		{
			name:   "stops VM successfully",
			podID:  "pod-123",
			vmName: "vm1",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "handles stop error",
			podID:  "pod-123",
			vmName: "vm1",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
				m.StopVMErr = errors.New("stop failed")
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleStopVM()

			r := chi.NewRouter()
			r.Post("/pods/{podID}/vms/{vmName}/stop", handler)

			req := httptest.NewRequest(http.MethodPost, "/pods/"+tt.podID+"/vms/"+tt.vmName+"/stop", nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManager_HandleSuspendVM_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		vmName         string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
	}{
		{
			name:   "suspends VM successfully",
			podID:  "pod-123",
			vmName: "vm1",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "handles suspend error",
			podID:  "pod-123",
			vmName: "vm1",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
				m.SuspendVMErr = errors.New("suspend failed")
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleSuspendVM()

			r := chi.NewRouter()
			r.Post("/pods/{podID}/vms/{vmName}/suspend", handler)

			req := httptest.NewRequest(http.MethodPost, "/pods/"+tt.podID+"/vms/"+tt.vmName+"/suspend", nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManager_HandleResumeVM_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		vmName         string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
	}{
		{
			name:   "resumes VM successfully",
			podID:  "pod-123",
			vmName: "vm1",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "suspended"}},
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:   "handles resume error",
			podID:  "pod-123",
			vmName: "vm1",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "suspended"}},
				})
				m.ResumeVMErr = errors.New("resume failed")
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleResumeVM()

			r := chi.NewRouter()
			r.Post("/pods/{podID}/vms/{vmName}/resume", handler)

			req := httptest.NewRequest(http.MethodPost, "/pods/"+tt.podID+"/vms/"+tt.vmName+"/resume", nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManager_HandleDeletePod_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
	}{
		{
			name:  "deletes pod successfully",
			podID: "pod-123",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK, // Handler returns 200 with status body
		},
		{
			name:  "handles delete error",
			podID: "pod-123",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
				})
				m.DestroyPodErr = errors.New("delete failed")
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:  "admin can delete any pod",
			podID: "pod-123",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "otheruser",
					OwnerID: "other-user-id",
					Status:  models.PodStatusRunning,
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "admin-user",
					Name:  "admin",
					Roles: []string{"admin"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK, // Handler returns 200 with status body
		},
		{
			name:  "returns 404 for non-existent pod",
			podID: "nonexistent",
			setupMock: func(m *mocks.FakeOrchestrator) {
				// No pods added
			},
			setupContext:   nil,
			expectedStatus: http.StatusNotFound,
		},
		{
			name:  "access denied for different user non-admin",
			podID: "pod-456",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-456",
					Owner:   "other-user",
					OwnerID: "other-user-id",
					Status:  models.PodStatusRunning,
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "different-user",
					Name:  "Different User",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleDeletePod()

			r := chi.NewRouter()
			r.Delete("/pods/{podID}", handler)

			req := httptest.NewRequest(http.MethodDelete, "/pods/"+tt.podID, nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManager_HandleCreateSnapshot_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		vmName         string
		body           string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
	}{
		{
			name:   "creates snapshot successfully",
			podID:  "pod-123",
			vmName: "vm1",
			body:   `{"name": "mysnapshot", "description": "Test snapshot"}`,
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:   "requires snapshot name",
			podID:  "pod-123",
			vmName: "vm1",
			body:   `{"description": "Test snapshot"}`,
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "handles create snapshot error",
			podID:  "pod-123",
			vmName: "vm1",
			body:   `{"name": "mysnapshot"}`,
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
				m.CreateVMSnapshotErr = errors.New("snapshot failed")
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleCreateSnapshot()

			r := chi.NewRouter()
			r.Post("/pods/{podID}/vms/{vmName}/snapshots", handler)

			req := httptest.NewRequest(http.MethodPost, "/pods/"+tt.podID+"/vms/"+tt.vmName+"/snapshots", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestManager_HandleDeleteSnapshot_WithFakeOrchestrator(t *testing.T) {
	tests := []struct {
		name           string
		podID          string
		vmName         string
		snapshotName   string
		setupMock      func(*mocks.FakeOrchestrator)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
	}{
		{
			name:         "deletes snapshot successfully",
			podID:        "pod-123",
			vmName:       "vm1",
			snapshotName: "mysnapshot",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK, // Handler returns 200 with status body
		},
		{
			name:         "handles delete snapshot error",
			podID:        "pod-123",
			vmName:       "vm1",
			snapshotName: "mysnapshot",
			setupMock: func(m *mocks.FakeOrchestrator) {
				m.AddPod(&models.Pod{
					ID:      "pod-123",
					Owner:   "testuser",
					OwnerID: "user-123",
					Status:  models.PodStatusRunning,
					VMs:     []models.PodVM{{Name: "vm1", Status: "running"}},
				})
				m.DeleteVMSnapshotErr = fmt.Errorf("snapshot: %w", orchestrator.ErrNotFound)
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusNotFound, // "not found" in error triggers 404
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			tt.setupMock(mockOrch)

			pm := newTestManager(mockOrch)

			handler := pm.handleDeleteSnapshot()

			r := chi.NewRouter()
			r.Delete("/pods/{podID}/vms/{vmName}/snapshots/{snapshotName}", handler)

			req := httptest.NewRequest(http.MethodDelete, "/pods/"+tt.podID+"/vms/"+tt.vmName+"/snapshots/"+tt.snapshotName, nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}
		})
	}
}

// -----------------------------------------------------------------------------
// HandleCreatePod Tests with Mock Repositories
// -----------------------------------------------------------------------------

// newTestManagerWithRepos creates a Manager with mock repos for testing
func newTestManagerWithRepos(mockOrch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository, userRepo *mocks.FakeUserRepository) *Manager {
	logger := podTestLogger()
	return NewManager(Config{
		Orchestrator:    mockOrch,
		LabTemplateRepo: labRepo,
		UserRepo:        userRepo,
		Logger:          logger,
		Responder:       httputil.NewResponder(logger),
	})
}

func TestManager_HandleCreatePod_WithMockRepos(t *testing.T) {
	// Create a valid template spec as JSON
	validSpec := json.RawMessage(`{
		"platform": "proxmox",
		"vms": [
			{
				"name": "vm1",
				"template": "ubuntu-22.04",
				"startOnCreate": true
			}
		]
	}`)

	tests := []struct {
		name           string
		body           string
		setupMocks     func(*mocks.FakeOrchestrator, *mocks.FakeLabTemplateRepository, *mocks.FakeUserRepository)
		expectedStatus int
		expectError    bool
	}{
		{
			name: "creates pod successfully",
			body: `{"labTemplate": "test-lab", "owner": "testuser"}`,
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository, userRepo *mocks.FakeUserRepository) {
				labRepo.AddTemplate(&models.LabTemplateRecord{
					ID:       "template-1",
					Name:     "test-lab",
					Version:  "1.0",
					Platform: models.PlatformProxmox,
					Spec:     validSpec,
					IsActive: true,
				})
				userRepo.AddUser(&models.User{
					ID:       "user-1",
					Username: "testuser",
					Email:    "test@example.com",
				})
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name:           "invalid JSON body",
			body:           `{invalid json`,
			setupMocks:     func(*mocks.FakeOrchestrator, *mocks.FakeLabTemplateRepository, *mocks.FakeUserRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "missing required fields",
			body:           `{}`,
			setupMocks:     func(*mocks.FakeOrchestrator, *mocks.FakeLabTemplateRepository, *mocks.FakeUserRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "template not found by name or ID",
			body: `{"labTemplate": "nonexistent", "owner": "testuser"}`,
			setupMocks: func(*mocks.FakeOrchestrator, *mocks.FakeLabTemplateRepository, *mocks.FakeUserRepository) {
				// No templates added
			},
			expectedStatus: http.StatusNotFound,
			expectError:    true,
		},
		{
			name: "template found by ID when name lookup fails",
			body: `{"labTemplate": "template-uuid", "owner": "testuser"}`,
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository, userRepo *mocks.FakeUserRepository) {
				labRepo.AddTemplate(&models.LabTemplateRecord{
					ID:       "template-uuid",
					Name:     "test-lab-different-name",
					Version:  "1.0",
					Platform: models.PlatformProxmox,
					Spec:     validSpec,
					IsActive: true,
				})
				userRepo.AddUser(&models.User{
					ID:       "user-1",
					Username: "testuser",
					Email:    "test@example.com",
				})
			},
			expectedStatus: http.StatusCreated,
		},
		{
			name: "inactive template returns 400",
			body: `{"labTemplate": "test-lab", "owner": "testuser"}`,
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository, userRepo *mocks.FakeUserRepository) {
				labRepo.AddTemplate(&models.LabTemplateRecord{
					ID:       "template-1",
					Name:     "test-lab",
					Version:  "1.0",
					Platform: models.PlatformProxmox,
					Spec:     validSpec,
					IsActive: false, // inactive
				})
			},
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "template repo error on name lookup",
			body: `{"labTemplate": "test-lab", "owner": "testuser"}`,
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository, userRepo *mocks.FakeUserRepository) {
				labRepo.GetByNameErr = errors.New("database connection error")
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name: "user repo error",
			body: `{"labTemplate": "test-lab", "owner": "testuser"}`,
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository, userRepo *mocks.FakeUserRepository) {
				labRepo.AddTemplate(&models.LabTemplateRecord{
					ID:       "template-1",
					Name:     "test-lab",
					Version:  "1.0",
					Platform: models.PlatformProxmox,
					Spec:     validSpec,
					IsActive: true,
				})
				userRepo.GetOrCreateByUsernameErr = errors.New("user service unavailable")
			},
			expectedStatus: http.StatusInternalServerError, // safeErrorResponse returns 500 for generic errors
			expectError:    true,
		},
		{
			name: "orchestrator create error",
			body: `{"labTemplate": "test-lab", "owner": "testuser"}`,
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository, userRepo *mocks.FakeUserRepository) {
				labRepo.AddTemplate(&models.LabTemplateRecord{
					ID:       "template-1",
					Name:     "test-lab",
					Version:  "1.0",
					Platform: models.PlatformProxmox,
					Spec:     validSpec,
					IsActive: true,
				})
				userRepo.AddUser(&models.User{
					ID:       "user-1",
					Username: "testuser",
					Email:    "test@example.com",
				})
				orch.CreatePodErr = errors.New("provisioning failed")
			},
			expectedStatus: http.StatusInternalServerError,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			mockLabRepo := mocks.NewFakeLabTemplateRepository()
			mockUserRepo := mocks.NewFakeUserRepository()

			tt.setupMocks(mockOrch, mockLabRepo, mockUserRepo)

			pm := newTestManagerWithRepos(mockOrch, mockLabRepo, mockUserRepo)

			handler := pm.handleCreatePod()

			req := httptest.NewRequest(http.MethodPost, "/api/v1/pods", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			handler(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if tt.expectError {
				var resp map[string]interface{}
				if err := json.NewDecoder(rr.Body).Decode(&resp); err == nil {
					if _, ok := resp["error"]; !ok {
						t.Error("expected error response")
					}
				}
			}
		})
	}
}

func TestManager_HandleCreatePod_NoLabRepo(t *testing.T) {
	mockOrch := mocks.NewFakeOrchestrator()
	mockUserRepo := mocks.NewFakeUserRepository()

	// Create Manager without lab template repo
	pm := NewManager(Config{
		Orchestrator: mockOrch,
		UserRepo:     mockUserRepo,
		Logger:       podTestLogger(),
	})

	handler := pm.handleCreatePod()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/pods", strings.NewReader(`{"labTemplate": "test", "owner": "user"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

func TestManager_HandleCreatePod_NoUserRepo(t *testing.T) {
	mockOrch := mocks.NewFakeOrchestrator()
	mockLabRepo := mocks.NewFakeLabTemplateRepository()

	// Add a valid template
	validSpec := json.RawMessage(`{
		"platform": "proxmox",
		"vms": [{"name": "vm1", "template": "ubuntu-22.04"}]
	}`)
	mockLabRepo.AddTemplate(&models.LabTemplateRecord{
		ID:       "template-1",
		Name:     "test-lab",
		Version:  "1.0",
		Platform: models.PlatformProxmox,
		Spec:     validSpec,
		IsActive: true,
	})

	// Create Manager without user repo
	pm := NewManager(Config{
		Orchestrator:    mockOrch,
		LabTemplateRepo: mockLabRepo,
		Logger:          podTestLogger(),
	})

	handler := pm.handleCreatePod()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/pods", strings.NewReader(`{"labTemplate": "test-lab", "owner": "user"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

// -----------------------------------------------------------------------------
// handleCreatePodAsync Tests
// -----------------------------------------------------------------------------

func TestManager_HandleCreatePodAsync_NoAsyncProvisioner(t *testing.T) {
	mockOrch := mocks.NewFakeOrchestrator()

	// Create Manager without async provisioner
	pm := NewManager(Config{
		Orchestrator: mockOrch,
		Logger:       podTestLogger(),
	})

	handler := pm.handleCreatePodAsync()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/pods/async", strings.NewReader(`{"labTemplate": "test", "owner": "user"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}

	var response map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Handler invoked directly (not routed through server.Router) so the
	// locale middleware never runs; Localize() falls back to the raw
	// message ID. The English text for this ID is
	// "async provisioning not configured".
	if response["error"] != "pod.errors.asyncProvisioningNotConfigured" {
		t.Errorf("unexpected error: %s", response["error"])
	}
}

// -----------------------------------------------------------------------------
// HandleGetTopology Tests
// -----------------------------------------------------------------------------

func TestManager_HandleGetTopology_WithMockRepos(t *testing.T) {
	// Create a template spec with network topology as JSON
	// Note: disk is an int (MB), not a string
	topologySpec := json.RawMessage(`{
		"platform": "proxmox",
		"network": {
			"segments": [
				{
					"name": "management",
					"vlan": 100,
					"subnet": "10.0.1.0/24",
					"gateway": "10.0.1.1",
					"dhcp": true
				},
				{
					"name": "internal",
					"vlan": 200,
					"subnet": "10.0.2.0/24"
				}
			]
		},
		"vms": [
			{
				"name": "web-server",
				"template": "ubuntu-22.04",
				"resources": {
					"cpu": 2,
					"memory": 2048,
					"disk": 20
				},
				"networks": [
					{"segment": "management", "ip": "10.0.1.10"},
					{"segment": "internal", "ip": "10.0.2.10"}
				]
			},
			{
				"name": "database",
				"template": "ubuntu-22.04",
				"resources": {
					"cpu": 4,
					"memory": 4096,
					"disk": 50
				},
				"networks": [
					{"segment": "internal", "ip": "10.0.2.20"}
				]
			}
		]
	}`)

	tests := []struct {
		name           string
		podID          string
		setupMocks     func(*mocks.FakeOrchestrator, *mocks.FakeLabTemplateRepository)
		setupContext   func(*http.Request) *http.Request
		expectedStatus int
		validateResp   func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:  "returns topology successfully",
			podID: "pod-123",
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository) {
				orch.AddPod(&models.Pod{
					ID:            "pod-123",
					Owner:         "testuser",
					OwnerID:       "user-123",
					LabTemplate:   "network-lab",
					LabTemplateID: "template-1",
					Status:        models.PodStatusRunning,
					VMs: []models.PodVM{
						{Name: "web-server", Status: "running", PlatformID: "vm-101", IPAddress: "10.0.1.10"},
						{Name: "database", Status: "running", PlatformID: "vm-102", IPAddress: "10.0.2.20"},
					},
				})
				labRepo.AddTemplate(&models.LabTemplateRecord{
					ID:       "template-1",
					Name:     "network-lab",
					Version:  "1.0",
					Platform: models.PlatformProxmox,
					Spec:     topologySpec,
					IsActive: true,
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp TopologyResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				if resp.PodID != "pod-123" {
					t.Errorf("expected podID pod-123, got %s", resp.PodID)
				}
				if len(resp.Segments) != 2 {
					t.Errorf("expected 2 segments, got %d", len(resp.Segments))
				}
				if len(resp.VMs) != 2 {
					t.Errorf("expected 2 VMs, got %d", len(resp.VMs))
				}
				// Validate network segment details
				foundMgmt := false
				for _, seg := range resp.Segments {
					if seg.Name == "management" {
						foundMgmt = true
						if seg.VLAN != 100 {
							t.Errorf("expected VLAN 100 for management, got %d", seg.VLAN)
						}
					}
				}
				if !foundMgmt {
					t.Error("management segment not found")
				}
			},
		},
		{
			name:  "returns 404 for non-existent pod",
			podID: "nonexistent",
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository) {
				// No pods added
			},
			expectedStatus: http.StatusNotFound,
		},
		{
			name:  "returns 403 when user cannot access pod",
			podID: "pod-123",
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository) {
				orgID := "org-abc"
				orch.AddPod(&models.Pod{
					ID:             "pod-123",
					Owner:          "other-user",
					OwnerID:        "other-user-id",
					OrganizationID: &orgID,
					LabTemplate:    "network-lab",
					Status:         models.PodStatusRunning,
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:                    "different-user",
					Name:                  "Different User",
					Roles:                 []string{"student"},
					DefaultOrganizationID: "different-org",
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusForbidden,
		},
		{
			name:  "returns default segment when template not found",
			podID: "pod-123",
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository) {
				orch.AddPod(&models.Pod{
					ID:          "pod-123",
					Owner:       "testuser",
					OwnerID:     "user-123",
					LabTemplate: "missing-template",
					Status:      models.PodStatusRunning,
					VMs: []models.PodVM{
						{Name: "vm1", Status: "running", IPAddress: "10.0.0.5"},
					},
				})
				// No template added to labRepo
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "user-123",
					Name:  "testuser",
					Roles: []string{"student"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, rr *httptest.ResponseRecorder) {
				var resp TopologyResponse
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				// Handler creates a default segment when template not found
				if len(resp.Segments) != 1 {
					t.Errorf("expected 1 default segment, got %d", len(resp.Segments))
				}
				if resp.Segments[0].Name != "default" {
					t.Errorf("expected default segment name, got %s", resp.Segments[0].Name)
				}
				// VMs should be populated from pod state
				if len(resp.VMs) != 1 {
					t.Errorf("expected 1 VM from pod state, got %d", len(resp.VMs))
				}
			},
		},
		{
			name:  "admin can access any pod's topology",
			podID: "pod-123",
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository) {
				orch.AddPod(&models.Pod{
					ID:            "pod-123",
					Owner:         "other-user",
					OwnerID:       "other-user-id",
					LabTemplate:   "network-lab",
					LabTemplateID: "template-1",
					Status:        models.PodStatusRunning,
				})
				labRepo.AddTemplate(&models.LabTemplateRecord{
					ID:       "template-1",
					Name:     "network-lab",
					Version:  "1.0",
					Platform: models.PlatformProxmox,
					Spec:     topologySpec,
					IsActive: true,
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "admin-user",
					Name:  "admin",
					Roles: []string{"admin"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:  "instructor can access any pod's topology",
			podID: "pod-123",
			setupMocks: func(orch *mocks.FakeOrchestrator, labRepo *mocks.FakeLabTemplateRepository) {
				orch.AddPod(&models.Pod{
					ID:            "pod-123",
					Owner:         "student-user",
					OwnerID:       "student-user-id",
					LabTemplate:   "network-lab",
					LabTemplateID: "template-1",
					Status:        models.PodStatusRunning,
				})
				labRepo.AddTemplate(&models.LabTemplateRecord{
					ID:       "template-1",
					Name:     "network-lab",
					Version:  "1.0",
					Platform: models.PlatformProxmox,
					Spec:     topologySpec,
					IsActive: true,
				})
			},
			setupContext: func(r *http.Request) *http.Request {
				ctx := auth.ContextWithUser(r.Context(), &auth.User{
					ID:    "instructor-user",
					Name:  "instructor",
					Roles: []string{"instructor"},
				})
				return r.WithContext(ctx)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockOrch := mocks.NewFakeOrchestrator()
			mockLabRepo := mocks.NewFakeLabTemplateRepository()

			tt.setupMocks(mockOrch, mockLabRepo)

			pm := newTestManagerWithRepos(mockOrch, mockLabRepo, nil)

			handler := pm.handleGetTopology()

			r := chi.NewRouter()
			r.Get("/pods/{podID}/topology", handler)

			req := httptest.NewRequest(http.MethodGet, "/pods/"+tt.podID+"/topology", nil)
			if tt.setupContext != nil {
				req = tt.setupContext(req)
			}
			rr := httptest.NewRecorder()

			r.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d: %s", tt.expectedStatus, rr.Code, rr.Body.String())
			}

			if tt.validateResp != nil && rr.Code == http.StatusOK {
				tt.validateResp(t, rr)
			}
		})
	}
}

func TestManager_HandleGetTopology_NoLabRepo(t *testing.T) {
	mockOrch := mocks.NewFakeOrchestrator()
	mockOrch.AddPod(&models.Pod{
		ID:          "pod-123",
		Owner:       "testuser",
		OwnerID:     "user-123",
		LabTemplate: "test-lab",
		Status:      models.PodStatusRunning,
	})

	// Create Manager without lab template repo
	pm := NewManager(Config{
		Orchestrator: mockOrch,
		Logger:       podTestLogger(),
	})

	handler := pm.handleGetTopology()

	r := chi.NewRouter()
	r.Get("/pods/{podID}/topology", handler)

	req := httptest.NewRequest(http.MethodGet, "/pods/pod-123/topology", nil)
	ctx := auth.ContextWithUser(req.Context(), &auth.User{
		ID:    "user-123",
		Name:  "testuser",
		Roles: []string{"student"},
	})
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status %d, got %d: %s", http.StatusServiceUnavailable, rr.Code, rr.Body.String())
	}
}

// Suppress unused variable warnings
var _ = context.Background
