package wazuh

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/database/repositories"
	"github.com/toddbartholow/kootenai/api/internal/models"
	"github.com/toddbartholow/kootenai/api/internal/regexutil"
)

// Config holds configuration for the Wazuh service
type Config struct {
	// WebhookSecret for validating incoming webhooks
	WebhookSecret string `yaml:"webhook_secret"`

	// RequireSignature enforces webhook HMAC signature validation (should be true in production)
	RequireSignature bool `yaml:"require_signature"`

	// AllowedIPs is a list of IP addresses/CIDRs allowed to send webhook requests
	AllowedIPs []string `yaml:"allowed_ips"`

	// ManagerURL is the Wazuh Manager API URL (e.g., "https://wazuh.example.com:55000")
	ManagerURL string `yaml:"manager_url"`

	// ManagerUsername for API authentication
	ManagerUsername string `yaml:"manager_username"`

	// ManagerPassword for API authentication
	ManagerPassword string `yaml:"manager_password"`

	// InsecureSkipVerify disables TLS certificate verification (not recommended for production)
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`

	// AgentGroupPrefix is prepended to pod IDs for agent group names
	AgentGroupPrefix string `yaml:"agent_group_prefix"`

	// EventBufferSize for async event processing
	EventBufferSize int `yaml:"event_buffer_size"`

	// ProcessingWorkers number of concurrent event processors
	ProcessingWorkers int `yaml:"processing_workers"`
}

// DefaultConfig returns the default Wazuh configuration
func DefaultConfig() Config {
	return Config{
		AgentGroupPrefix:  "kootenai-",
		EventBufferSize:   1000,
		ProcessingWorkers: 4,
	}
}

// Service handles Wazuh integration for the Kootenai platform
type Service struct {
	config         Config
	logger         *slog.Logger
	apiClient      *APIClient
	eventRepo      repositories.EventRepository
	sessionRepo    repositories.SessionRepository
	checkpointRepo repositories.CheckpointProgressRepository
	podRepo        repositories.PodRepository
	templateRepo   repositories.LabTemplateRepository

	// agentPodMapping maps Wazuh agent IDs to pod IDs
	agentPodMapping   map[string]string
	agentPodMappingMu sync.RWMutex

	// checkpointCache caches checkpoint matchers per template
	checkpointCache   map[string][]CheckpointMatcher
	checkpointCacheMu sync.RWMutex

	// eventChan for async event processing
	eventChan chan *Alert
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// NewService creates a new Wazuh integration service
func NewService(
	config Config,
	logger *slog.Logger,
	eventRepo repositories.EventRepository,
	sessionRepo repositories.SessionRepository,
	checkpointRepo repositories.CheckpointProgressRepository,
	podRepo repositories.PodRepository,
	templateRepo repositories.LabTemplateRepository,
) *Service {
	if config.EventBufferSize == 0 {
		config.EventBufferSize = 1000
	}
	if config.ProcessingWorkers == 0 {
		config.ProcessingWorkers = 4
	}

	// Initialize API client if manager URL is configured
	var apiClient *APIClient
	if config.ManagerURL != "" {
		apiClient = NewAPIClient(APIClientConfig{
			ManagerURL:         config.ManagerURL,
			Username:           config.ManagerUsername,
			Password:           config.ManagerPassword,
			InsecureSkipVerify: config.InsecureSkipVerify,
		})
		logger.Info("Wazuh API client initialized", "manager_url", config.ManagerURL)
	} else {
		logger.Warn("Wazuh Manager URL not configured, agent registration will use local mapping only")
	}

	return &Service{
		config:          config,
		logger:          logger,
		apiClient:       apiClient,
		eventRepo:       eventRepo,
		sessionRepo:     sessionRepo,
		checkpointRepo:  checkpointRepo,
		podRepo:         podRepo,
		templateRepo:    templateRepo,
		agentPodMapping: make(map[string]string),
		checkpointCache: make(map[string][]CheckpointMatcher),
		eventChan:       make(chan *Alert, config.EventBufferSize),
		stopChan:        make(chan struct{}),
	}
}

// Start begins the background event processing workers
func (s *Service) Start() {
	for i := 0; i < s.config.ProcessingWorkers; i++ {
		s.wg.Add(1)
		go s.processEvents(i)
	}
	s.logger.Info("Wazuh service started", "workers", s.config.ProcessingWorkers)
}

// Stop gracefully shuts down the service
func (s *Service) Stop() {
	close(s.stopChan)
	s.wg.Wait()
	s.logger.Info("Wazuh service stopped")
}

// processEvents is a worker that processes events from the channel
func (s *Service) processEvents(workerID int) {
	defer s.wg.Done()

	for {
		select {
		case <-s.stopChan:
			return
		case alert := <-s.eventChan:
			if err := s.handleAlert(context.Background(), alert); err != nil {
				s.logger.Error("failed to handle alert",
					"worker", workerID,
					"error", err,
					"agent_id", alert.Agent.ID,
				)
			}
		}
	}
}

// ProcessAlert queues an alert for processing
func (s *Service) ProcessAlert(ctx context.Context, alert *Alert) error {
	select {
	case s.eventChan <- alert:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Buffer full, process synchronously
		s.logger.Warn("event buffer full, processing synchronously")
		return s.handleAlert(ctx, alert)
	}
}

// handleAlert processes a single Wazuh alert
func (s *Service) handleAlert(ctx context.Context, alert *Alert) error {
	// Look up the pod for this agent
	podID, err := s.getPodForAgent(ctx, alert.Agent.ID)
	if err != nil {
		return fmt.Errorf("looking up pod for agent %s: %w", alert.Agent.ID, err)
	}
	if podID == "" {
		s.logger.Debug("no pod mapping for agent", "agent_id", alert.Agent.ID)
		return nil
	}

	// Get active session for the pod
	sessions, err := s.sessionRepo.GetByPodID(ctx, podID)
	if err != nil {
		return fmt.Errorf("getting sessions for pod %s: %w", podID, err)
	}

	var activeSession *models.Session
	for _, session := range sessions {
		if session.EndedAt == nil {
			activeSession = session
			break
		}
	}

	if activeSession == nil {
		s.logger.Debug("no active session for pod", "pod_id", podID)
		return nil
	}

	// Convert alert to event
	event, err := s.alertToEvent(alert, podID, activeSession.ID)
	if err != nil {
		return fmt.Errorf("converting alert to event: %w", err)
	}

	// Store the event
	if err := s.eventRepo.Create(ctx, event); err != nil {
		return fmt.Errorf("storing event: %w", err)
	}

	// Match against checkpoints
	matchedCheckpoints, err := s.matchCheckpoints(ctx, activeSession, alert)
	if err != nil {
		return fmt.Errorf("matching checkpoints: %w", err)
	}

	// Mark event as processed
	if err := s.eventRepo.MarkProcessed(ctx, event.ID, matchedCheckpoints); err != nil {
		return fmt.Errorf("marking event processed: %w", err)
	}

	// Update checkpoint progress for matches
	for _, checkpointID := range matchedCheckpoints {
		eventIDStr := fmt.Sprintf("%d", event.ID)
		if err := s.checkpointRepo.MarkPassed(ctx, activeSession.ID, checkpointID, &eventIDStr); err != nil {
			s.logger.Error("failed to mark checkpoint passed",
				"checkpoint_id", checkpointID,
				"session_id", activeSession.ID,
				"error", err,
			)
		} else {
			s.logger.Info("checkpoint completed",
				"checkpoint_id", checkpointID,
				"session_id", activeSession.ID,
				"trigger_event", event.ID,
			)
		}
	}

	return nil
}

// alertToEvent converts a Wazuh alert to an Event model
func (s *Service) alertToEvent(alert *Alert, podID, sessionID string) (*models.Event, error) {
	eventType := "wazuh_alert"
	if alert.Syscheck != nil {
		eventType = "fim_" + alert.Syscheck.Event
	}

	data, err := json.Marshal(alert)
	if err != nil {
		return nil, fmt.Errorf("marshaling alert data: %w", err)
	}

	// Parse timestamp string to time.Time
	timestamp, _ := time.Parse(time.RFC3339, alert.Timestamp)
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	return &models.Event{
		Timestamp:     timestamp,
		PodID:         podID,
		SessionID:     sessionID,
		VMName:        alert.Agent.Name,
		AgentID:       alert.Agent.ID,
		EventType:     eventType,
		Source:        "wazuh",
		WazuhAlertID:  alert.ID,
		WazuhRuleID:   alert.Rule.ID,
		WazuhRuleDesc: alert.Rule.Description,
		WazuhLevel:    alert.Rule.Level,
		Data:          data,
		Processed:     false,
	}, nil
}

// matchCheckpoints checks if an alert matches any checkpoint definitions
func (s *Service) matchCheckpoints(ctx context.Context, session *models.Session, alert *Alert) ([]string, error) {
	// Get checkpoint matchers for this template
	matchers, err := s.getCheckpointMatchers(ctx, session.LabTemplateID)
	if err != nil {
		return nil, err
	}

	var matched []string
	for _, matcher := range matchers {
		if s.alertMatchesMatcher(alert, &matcher) {
			matched = append(matched, matcher.CheckpointID)
		}
	}

	return matched, nil
}

// getCheckpointMatchers retrieves or builds checkpoint matchers for a template
func (s *Service) getCheckpointMatchers(ctx context.Context, templateID string) ([]CheckpointMatcher, error) {
	s.checkpointCacheMu.RLock()
	matchers, ok := s.checkpointCache[templateID]
	s.checkpointCacheMu.RUnlock()
	if ok {
		return matchers, nil
	}

	// Load template and build matchers
	template, err := s.templateRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("loading template: %w", err)
	}
	if template == nil {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	// Parse checkpoints from template
	var checkpoints []models.Checkpoint
	if len(template.Checkpoints) > 0 {
		if err := json.Unmarshal(template.Checkpoints, &checkpoints); err != nil {
			return nil, fmt.Errorf("parsing checkpoints: %w", err)
		}
	}

	// Build matchers from checkpoints
	matchers = s.buildMatchers(checkpoints)

	s.checkpointCacheMu.Lock()
	s.checkpointCache[templateID] = matchers
	s.checkpointCacheMu.Unlock()

	return matchers, nil
}

// buildMatchers converts checkpoint triggers to matchers
func (s *Service) buildMatchers(checkpoints []models.Checkpoint) []CheckpointMatcher {
	var matchers []CheckpointMatcher

	for _, cp := range checkpoints {
		matcher := CheckpointMatcher{
			CheckpointID: cp.ID,
		}

		for _, trigger := range cp.Triggers {
			switch trigger.Type {
			case models.TriggerTypeFileExists, models.TriggerTypeFileContent, models.TriggerTypeFileDeleted:
				if trigger.Match.Path != "" {
					matcher.FilePaths = append(matcher.FilePaths, trigger.Match.Path)
				}
				if trigger.Type == models.TriggerTypeFileDeleted {
					matcher.FileEvents = append(matcher.FileEvents, "deleted")
				} else if trigger.Type == models.TriggerTypeFileExists {
					matcher.FileEvents = append(matcher.FileEvents, "added")
				}
				if trigger.Match.Regex != "" {
					matcher.ContentMatch = trigger.Match.Regex
				}

			case models.TriggerTypePermission:
				if trigger.Match.Path != "" {
					matcher.FilePaths = append(matcher.FilePaths, trigger.Match.Path)
				}
				matcher.FileEvents = append(matcher.FileEvents, "modified")

			case models.TriggerTypeCommandExecuted:
				if trigger.Match.Pattern != "" {
					matcher.LogMatch = trigger.Match.Pattern
				}

			case models.TriggerTypeService:
				// Match service state changes via rule groups
				matcher.RuleGroups = append(matcher.RuleGroups, "systemd")

			case models.TriggerTypePackage:
				matcher.RuleGroups = append(matcher.RuleGroups, "dpkg", "yum", "rpm")

			case models.TriggerTypeUserCreated:
				matcher.RuleGroups = append(matcher.RuleGroups, "authentication_success", "adduser")
			}
		}

		if len(matcher.FilePaths) > 0 || len(matcher.RuleIDs) > 0 ||
			len(matcher.RuleGroups) > 0 || matcher.LogMatch != "" {
			matchers = append(matchers, matcher)
		}
	}

	return matchers
}

// alertMatchesMatcher checks if an alert matches a checkpoint matcher
func (s *Service) alertMatchesMatcher(alert *Alert, matcher *CheckpointMatcher) bool {
	matchCount := 0
	totalConditions := 0

	// Check rule IDs
	if len(matcher.RuleIDs) > 0 {
		totalConditions++
		for _, ruleID := range matcher.RuleIDs {
			if alert.Rule.ID == ruleID {
				matchCount++
				break
			}
		}
	}

	// Check rule groups
	if len(matcher.RuleGroups) > 0 {
		totalConditions++
		for _, group := range matcher.RuleGroups {
			for _, alertGroup := range alert.Rule.Groups {
				if strings.EqualFold(alertGroup, group) {
					matchCount++
					break
				}
			}
		}
	}

	// Check minimum level
	if matcher.MinLevel > 0 {
		totalConditions++
		if alert.Rule.Level >= matcher.MinLevel {
			matchCount++
		}
	}

	// Check file paths (FIM events)
	if len(matcher.FilePaths) > 0 && alert.Syscheck != nil {
		totalConditions++
		for _, pattern := range matcher.FilePaths {
			matched, _ := filepath.Match(pattern, alert.Syscheck.Path)
			if matched || strings.HasPrefix(alert.Syscheck.Path, pattern) {
				matchCount++
				break
			}
		}
	}

	// Check file events
	if len(matcher.FileEvents) > 0 && alert.Syscheck != nil {
		totalConditions++
		for _, event := range matcher.FileEvents {
			if strings.EqualFold(alert.Syscheck.Event, event) {
				matchCount++
				break
			}
		}
	}

	// Check log match
	if matcher.LogMatch != "" {
		totalConditions++
		re, err := regexutil.CachedCompile(matcher.LogMatch)
		if err == nil && re.MatchString(alert.FullLog) {
			matchCount++
		}
	}

	// Determine if we matched
	if totalConditions == 0 {
		return false
	}

	if matcher.RequireAll {
		return matchCount == totalConditions
	}
	return matchCount > 0
}

// RegisterAgent registers a new agent with the Wazuh Manager and maps it to a pod
func (s *Service) RegisterAgent(ctx context.Context, reg *AgentRegistration) (*AgentKey, error) {
	// If API client is not configured, fall back to local mapping
	if s.apiClient == nil {
		s.logger.Warn("Wazuh API client not configured, using local agent mapping")
		return s.registerAgentLocal(reg)
	}

	// Register agent with Wazuh Manager API
	resp, err := s.apiClient.RegisterAgent(ctx, reg.Name, reg.IP)
	if err != nil {
		return nil, fmt.Errorf("registering agent with Wazuh Manager: %w", err)
	}

	// Store the agent-to-pod mapping
	s.agentPodMappingMu.Lock()
	s.agentPodMapping[resp.ID] = reg.PodID
	s.agentPodMappingMu.Unlock()

	s.logger.Info("registered agent with Wazuh Manager",
		"agent_id", resp.ID,
		"pod_id", reg.PodID,
		"name", reg.Name,
	)

	// Add agent to pod-specific group if configured
	if s.config.AgentGroupPrefix != "" && reg.GroupID == "" {
		groupID := s.config.AgentGroupPrefix + reg.PodID
		// Try to create group (ignore error if it already exists)
		_ = s.apiClient.CreateGroup(ctx, groupID)
		if err := s.apiClient.AddAgentToGroup(ctx, resp.ID, groupID); err != nil {
			s.logger.Warn("failed to add agent to group",
				"agent_id", resp.ID,
				"group_id", groupID,
				"error", err,
			)
		}
	} else if reg.GroupID != "" {
		if err := s.apiClient.AddAgentToGroup(ctx, resp.ID, reg.GroupID); err != nil {
			s.logger.Warn("failed to add agent to group",
				"agent_id", resp.ID,
				"group_id", reg.GroupID,
				"error", err,
			)
		}
	}

	return &AgentKey{
		ID:  resp.ID,
		Key: resp.Key,
	}, nil
}

// registerAgentLocal creates a local-only agent mapping (fallback when API is not configured)
func (s *Service) registerAgentLocal(reg *AgentRegistration) (*AgentKey, error) {
	s.agentPodMappingMu.Lock()
	defer s.agentPodMappingMu.Unlock()

	// Generate a local agent ID
	agentID := fmt.Sprintf("local-%s-%s", reg.PodID, reg.Name)
	s.agentPodMapping[agentID] = reg.PodID

	s.logger.Info("registered agent locally (no Wazuh Manager)",
		"agent_id", agentID,
		"pod_id", reg.PodID,
		"name", reg.Name,
	)

	return &AgentKey{
		ID:  agentID,
		Key: "", // No key for local-only agents
	}, nil
}

// UnregisterAgent removes an agent from the Wazuh Manager and local mapping
func (s *Service) UnregisterAgent(ctx context.Context, agentID string) error {
	// Remove from local mapping first
	s.agentPodMappingMu.Lock()
	podID, exists := s.agentPodMapping[agentID]
	delete(s.agentPodMapping, agentID)
	s.agentPodMappingMu.Unlock()

	// If API client is configured, delete from Wazuh Manager
	if s.apiClient != nil && !strings.HasPrefix(agentID, "local-") {
		if err := s.apiClient.DeleteAgent(ctx, agentID); err != nil {
			s.logger.Error("failed to delete agent from Wazuh Manager",
				"agent_id", agentID,
				"error", err,
			)
			return fmt.Errorf("deleting agent from Wazuh Manager: %w", err)
		}

		// Clean up pod-specific group if it exists
		if s.config.AgentGroupPrefix != "" && exists {
			groupID := s.config.AgentGroupPrefix + podID
			// Ignore errors - group may have other agents or not exist
			_ = s.apiClient.DeleteGroup(ctx, groupID)
		}
	}

	s.logger.Info("unregistered agent",
		"agent_id", agentID,
		"pod_id", podID,
	)

	return nil
}

// GetAgentStatus retrieves the current status of an agent from the Wazuh Manager
func (s *Service) GetAgentStatus(ctx context.Context, agentID string) (*AgentInfo, error) {
	if s.apiClient == nil {
		return nil, fmt.Errorf("Wazuh API client not configured")
	}

	if strings.HasPrefix(agentID, "local-") {
		return nil, fmt.Errorf("cannot get status for local-only agent")
	}

	apiInfo, err := s.apiClient.GetAgent(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("getting agent status: %w", err)
	}

	// Parse timestamps
	registeredAt, _ := time.Parse(time.RFC3339, apiInfo.DateAdd)
	lastKeepAlive, _ := time.Parse(time.RFC3339, apiInfo.LastKeepAlive)

	return &AgentInfo{
		ID:            apiInfo.ID,
		Name:          apiInfo.Name,
		IP:            apiInfo.IP,
		Status:        apiInfo.Status,
		Group:         apiInfo.Group,
		OS:            apiInfo.OS,
		Version:       apiInfo.Version,
		RegisteredAt:  registeredAt,
		LastKeepAlive: lastKeepAlive,
	}, nil
}

// CheckPodAgents queries the Wazuh Manager for the status of all agents associated with a pod.
// Agent names follow the convention {podID}-{vmName}.
// Returns a map of vmName → status (e.g., "active", "disconnected", "never_connected").
// If the Wazuh Manager is unreachable, returns nil with an error.
func (s *Service) CheckPodAgents(ctx context.Context, podID string, vmNames []string) (map[string]string, error) {
	if s.apiClient == nil {
		return nil, fmt.Errorf("Wazuh API client not configured")
	}

	results := make(map[string]string, len(vmNames))
	for _, vmName := range vmNames {
		agentName := podID + "-" + vmName
		agent, err := s.apiClient.GetAgentByName(ctx, agentName)
		if err != nil {
			s.logger.Debug("Agent not found in Wazuh Manager",
				"agentName", agentName,
				"error", err,
			)
			results[vmName] = "not_found"
			continue
		}
		results[vmName] = agent.Status
	}

	return results, nil
}

// MapAgentToPod creates a mapping between a Wazuh agent and a lab pod
func (s *Service) MapAgentToPod(agentID, podID string) {
	s.agentPodMappingMu.Lock()
	defer s.agentPodMappingMu.Unlock()
	s.agentPodMapping[agentID] = podID
}

// UnmapAgent removes an agent mapping
func (s *Service) UnmapAgent(agentID string) {
	s.agentPodMappingMu.Lock()
	defer s.agentPodMappingMu.Unlock()
	delete(s.agentPodMapping, agentID)
}

// getPodForAgent looks up the pod ID for a given agent
func (s *Service) getPodForAgent(ctx context.Context, agentID string) (string, error) {
	s.agentPodMappingMu.RLock()
	podID, ok := s.agentPodMapping[agentID]
	s.agentPodMappingMu.RUnlock()

	if ok {
		return podID, nil
	}

	// Try to find by agent name pattern (agent-<podID>)
	if strings.HasPrefix(agentID, "agent-") {
		podID = strings.TrimPrefix(agentID, "agent-")
		// Verify pod exists
		pod, err := s.podRepo.GetByID(ctx, podID)
		if err != nil {
			return "", err
		}
		if pod != nil {
			s.MapAgentToPod(agentID, podID)
			return podID, nil
		}
	}

	return "", nil
}

// InvalidateCheckpointCache clears the checkpoint cache for a template
func (s *Service) InvalidateCheckpointCache(templateID string) {
	s.checkpointCacheMu.Lock()
	defer s.checkpointCacheMu.Unlock()
	delete(s.checkpointCache, templateID)
}

// HealthStatus represents the health of the Wazuh integration
type HealthStatus struct {
	Healthy          bool       `json:"healthy"`
	ManagerReachable bool       `json:"managerReachable"`
	ManagerVersion   string     `json:"managerVersion,omitempty"`
	EventQueueSize   int        `json:"eventQueueSize"`
	EventQueueMax    int        `json:"eventQueueMax"`
	AgentCount       int        `json:"agentCount"`
	LastEventAt      *time.Time `json:"lastEventAt,omitempty"`
	Error            string     `json:"error,omitempty"`
}

// HealthCheck returns the current health status
func (s *Service) HealthCheck(ctx context.Context) HealthStatus {
	s.agentPodMappingMu.RLock()
	agentCount := len(s.agentPodMapping)
	s.agentPodMappingMu.RUnlock()

	status := HealthStatus{
		EventQueueSize: len(s.eventChan),
		EventQueueMax:  s.config.EventBufferSize,
		AgentCount:     agentCount,
	}

	// Check Wazuh Manager connectivity if API client is configured
	if s.apiClient != nil {
		info, err := s.apiClient.GetManagerInfo(ctx)
		if err != nil {
			status.ManagerReachable = false
			status.Healthy = false
			status.Error = fmt.Sprintf("failed to reach Wazuh Manager: %v", err)
		} else {
			status.ManagerReachable = true
			status.ManagerVersion = info.Version
			status.Healthy = true
		}
	} else {
		// No API client configured - consider healthy but note manager is not configured
		status.Healthy = true
		status.ManagerReachable = false
		status.Error = "Wazuh Manager API not configured"
	}

	return status
}
