// Package mocks provides mock implementations for testing
package mocks

import (
	"context"
	"sync"

	"github.com/toddbartholow/kootenai/api/internal/events"
	natsclient "github.com/toddbartholow/kootenai/api/internal/nats"
)

// Compile-time check that FakeNATSClient implements natsclient.ClientInterface
var _ natsclient.ClientInterface = (*FakeNATSClient)(nil)

// FakeNATSClient is a mock implementation of the NATS client for testing
type FakeNATSClient struct {
	mu sync.RWMutex

	// Connection state
	connected bool

	// Error injection
	PublishErr                error
	PublishEventErr           error
	PublishCheckpointErr      error
	PublishSessionErr         error
	PublishGradeErr           error
	PublishAchievementJobErr  error
	PublishGradeSyncJobErr    error
	PublishEventProcessingErr error

	// Call tracking
	PublishCalls                []PublishCall
	PublishEventCalls           []*events.VMEvent
	PublishCheckpointCalls      []*events.CheckpointUpdate
	PublishSessionCalls         []*events.SessionEvent
	PublishGradeCalls           []*events.GradeUpdate
	PublishAchievementJobCalls  []*events.AchievementEvaluationJob
	PublishGradeSyncJobCalls    []*events.GradeSyncJob
	PublishEventProcessingCalls []*events.EventProcessingJob
	CloseCalls                  int
	IsConnectedCalls            int
}

// PublishCall records a raw publish call
type PublishCall struct {
	Subject string
	Data    []byte
}

// NewFakeNATSClient creates a new mock NATS client
func NewFakeNATSClient() *FakeNATSClient {
	return &FakeNATSClient{
		connected: true, // Default to connected
	}
}

// Publish publishes raw data to a subject
func (m *FakeNATSClient) Publish(ctx context.Context, subject string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishCalls = append(m.PublishCalls, PublishCall{Subject: subject, Data: data})

	if m.PublishErr != nil {
		return m.PublishErr
	}

	return nil
}

// PublishEvent publishes a VM event
func (m *FakeNATSClient) PublishEvent(ctx context.Context, event *events.VMEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishEventCalls = append(m.PublishEventCalls, event)

	if m.PublishEventErr != nil {
		return m.PublishEventErr
	}

	return nil
}

// PublishCheckpoint publishes a checkpoint update
func (m *FakeNATSClient) PublishCheckpoint(ctx context.Context, update *events.CheckpointUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishCheckpointCalls = append(m.PublishCheckpointCalls, update)

	if m.PublishCheckpointErr != nil {
		return m.PublishCheckpointErr
	}

	return nil
}

// PublishSession publishes a session event
func (m *FakeNATSClient) PublishSession(ctx context.Context, event *events.SessionEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishSessionCalls = append(m.PublishSessionCalls, event)

	if m.PublishSessionErr != nil {
		return m.PublishSessionErr
	}

	return nil
}

// PublishGrade publishes a grade update
func (m *FakeNATSClient) PublishGrade(ctx context.Context, update *events.GradeUpdate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishGradeCalls = append(m.PublishGradeCalls, update)

	if m.PublishGradeErr != nil {
		return m.PublishGradeErr
	}

	return nil
}

// PublishAchievementJob publishes an achievement evaluation job
func (m *FakeNATSClient) PublishAchievementJob(ctx context.Context, job *events.AchievementEvaluationJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishAchievementJobCalls = append(m.PublishAchievementJobCalls, job)

	if m.PublishAchievementJobErr != nil {
		return m.PublishAchievementJobErr
	}

	return nil
}

// PublishGradeSyncJob publishes a grade sync job
func (m *FakeNATSClient) PublishGradeSyncJob(ctx context.Context, job *events.GradeSyncJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishGradeSyncJobCalls = append(m.PublishGradeSyncJobCalls, job)

	if m.PublishGradeSyncJobErr != nil {
		return m.PublishGradeSyncJobErr
	}

	return nil
}

// PublishEventProcessingJob publishes an event processing job
func (m *FakeNATSClient) PublishEventProcessingJob(ctx context.Context, job *events.EventProcessingJob) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PublishEventProcessingCalls = append(m.PublishEventProcessingCalls, job)

	if m.PublishEventProcessingErr != nil {
		return m.PublishEventProcessingErr
	}

	return nil
}

// IsConnected returns whether the mock is "connected"
func (m *FakeNATSClient) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.IsConnectedCalls++
	return m.connected
}

// Close simulates closing the connection
func (m *FakeNATSClient) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CloseCalls++
	m.connected = false
}

// SetConnected sets the mock connection state
func (m *FakeNATSClient) SetConnected(connected bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connected = connected
}

// Reset clears all call tracking and resets state
func (m *FakeNATSClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.connected = true
	m.PublishCalls = nil
	m.PublishEventCalls = nil
	m.PublishCheckpointCalls = nil
	m.PublishSessionCalls = nil
	m.PublishGradeCalls = nil
	m.PublishAchievementJobCalls = nil
	m.PublishGradeSyncJobCalls = nil
	m.PublishEventProcessingCalls = nil
	m.CloseCalls = 0
	m.IsConnectedCalls = 0
}
