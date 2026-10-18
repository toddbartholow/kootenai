// Package checkpoint provides checkpoint evaluation, filtering, and deduplication
package checkpoint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/toddbartholow/kootenai/api/internal/events"
)

// FilterAction determines what to do with a filtered event
type FilterAction string

const (
	// FilterActionAllow allows the event through
	FilterActionAllow FilterAction = "allow"
	// FilterActionDrop silently drops the event
	FilterActionDrop FilterAction = "drop"
	// FilterActionDedupe deduplicates the event within a time window
	FilterActionDedupe FilterAction = "dedupe"
)

// FilterRule defines a single event filter rule
type FilterRule struct {
	// Name is a human-readable name for the rule
	Name string `json:"name" yaml:"name"`
	// Description explains what this rule does
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	// EventTypes to match (empty matches all)
	EventTypes []string `json:"eventTypes,omitempty" yaml:"event_types,omitempty"`
	// VMNamePatterns to match (empty matches all)
	VMNamePatterns []string `json:"vmNamePatterns,omitempty" yaml:"vm_name_patterns,omitempty"`
	// MinWazuhLevel is the minimum Wazuh rule level to allow (0 = no filter)
	MinWazuhLevel int `json:"minWazuhLevel,omitempty" yaml:"min_wazuh_level,omitempty"`
	// MaxWazuhLevel is the maximum Wazuh rule level to allow (0 = no filter)
	MaxWazuhLevel int `json:"maxWazuhLevel,omitempty" yaml:"max_wazuh_level,omitempty"`
	// ExcludeGroups are Wazuh rule groups to exclude
	ExcludeGroups []string `json:"excludeGroups,omitempty" yaml:"exclude_groups,omitempty"`
	// IncludeGroups are the only Wazuh rule groups to include (if set)
	IncludeGroups []string `json:"includeGroups,omitempty" yaml:"include_groups,omitempty"`
	// Action to take when the rule matches
	Action FilterAction `json:"action" yaml:"action"`
	// DedupeWindow is the time window for deduplication (only used with ActionDedupe)
	DedupeWindow time.Duration `json:"dedupeWindow,omitempty" yaml:"dedupe_window,omitempty"`
	// Priority determines rule evaluation order (lower = higher priority)
	Priority int `json:"priority,omitempty" yaml:"priority,omitempty"`
	// Enabled determines if the rule is active
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// FilterConfig holds the complete filter configuration
type FilterConfig struct {
	// Enabled determines if filtering is active globally
	Enabled bool `json:"enabled" yaml:"enabled"`
	// DefaultAction is the action when no rule matches
	DefaultAction FilterAction `json:"defaultAction" yaml:"default_action"`
	// DefaultDedupeWindow for dedupe action when not specified per-rule
	DefaultDedupeWindow time.Duration `json:"defaultDedupeWindow" yaml:"default_dedupe_window"`
	// Rules is the list of filter rules
	Rules []FilterRule `json:"rules" yaml:"rules"`
	// MaxDedupeEntries limits memory usage for deduplication
	MaxDedupeEntries int `json:"maxDedupeEntries" yaml:"max_dedupe_entries"`
	// DedupeCleanupInterval is how often to clean old dedupe entries
	DedupeCleanupInterval time.Duration `json:"dedupeCleanupInterval" yaml:"dedupe_cleanup_interval"`
}

// DefaultFilterConfig returns sensible default filter configuration
func DefaultFilterConfig() FilterConfig {
	return FilterConfig{
		Enabled:               true,
		DefaultAction:         FilterActionAllow,
		DefaultDedupeWindow:   5 * time.Second,
		MaxDedupeEntries:      10000,
		DedupeCleanupInterval: 30 * time.Second,
		Rules: []FilterRule{
			// Drop low-level noise events
			{
				Name:          "drop-low-level",
				Description:   "Drop very low level Wazuh alerts (usually noise)",
				MinWazuhLevel: 0,
				MaxWazuhLevel: 2,
				Action:        FilterActionDrop,
				Priority:      100,
				Enabled:       true,
			},
			// Deduplicate frequent system events
			{
				Name:        "dedupe-syslog",
				Description: "Deduplicate frequent syslog events",
				EventTypes:  []string{"syslog"},
				Action:      FilterActionDedupe,
				Priority:    50,
				Enabled:     true,
			},
			// Deduplicate FIM events (same file within window)
			{
				Name:        "dedupe-fim",
				Description: "Deduplicate file integrity monitoring events",
				EventTypes:  []string{"fim_added", "fim_modified", "fim_deleted"},
				Action:      FilterActionDedupe,
				Priority:    40,
				Enabled:     true,
			},
		},
	}
}

// dedupeEntry tracks an event for deduplication
type dedupeEntry struct {
	hash     string
	lastSeen time.Time
	count    int
}

// FilterStats holds statistics about filtering
type FilterStats struct {
	TotalEvents   int64            `json:"totalEvents"`
	AllowedEvents int64            `json:"allowedEvents"`
	DroppedEvents int64            `json:"droppedEvents"`
	DedupedEvents int64            `json:"dedupedEvents"`
	ByRule        map[string]int64 `json:"byRule"`
}

// EventFilter filters and deduplicates events
type EventFilter struct {
	config    FilterConfig
	logger    *slog.Logger
	stats     FilterStats
	statsMu   sync.RWMutex
	dedupeMap map[string]*dedupeEntry
	dedupeMu  sync.RWMutex
	stopChan  chan struct{}
	wg        sync.WaitGroup
}

// NewEventFilter creates a new event filter
func NewEventFilter(config FilterConfig, logger *slog.Logger) *EventFilter {
	if config.MaxDedupeEntries == 0 {
		config.MaxDedupeEntries = 10000
	}
	if config.DedupeCleanupInterval == 0 {
		config.DedupeCleanupInterval = 30 * time.Second
	}
	if config.DefaultDedupeWindow == 0 {
		config.DefaultDedupeWindow = 5 * time.Second
	}

	return &EventFilter{
		config:    config,
		logger:    logger,
		stats:     FilterStats{ByRule: make(map[string]int64)},
		dedupeMap: make(map[string]*dedupeEntry),
		stopChan:  make(chan struct{}),
	}
}

// Start begins the background cleanup goroutine
func (f *EventFilter) Start() {
	f.wg.Add(1)
	go f.cleanupLoop()
	f.logger.Info("Event filter started",
		slog.Bool("enabled", f.config.Enabled),
		slog.Int("rules", len(f.config.Rules)),
	)
}

// Stop stops the background cleanup goroutine
func (f *EventFilter) Stop() {
	close(f.stopChan)
	f.wg.Wait()
	f.logger.Info("Event filter stopped")
}

// cleanupLoop periodically removes old dedupe entries
func (f *EventFilter) cleanupLoop() {
	defer f.wg.Done()

	ticker := time.NewTicker(f.config.DedupeCleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-f.stopChan:
			return
		case <-ticker.C:
			f.cleanupDedupeEntries()
		}
	}
}

// cleanupDedupeEntries removes expired dedupe entries
func (f *EventFilter) cleanupDedupeEntries() {
	f.dedupeMu.Lock()
	defer f.dedupeMu.Unlock()

	now := time.Now()
	maxWindow := f.config.DefaultDedupeWindow
	for _, rule := range f.config.Rules {
		if rule.DedupeWindow > maxWindow {
			maxWindow = rule.DedupeWindow
		}
	}

	// Remove entries older than the max window
	expiry := now.Add(-maxWindow * 2) // 2x window for safety
	for hash, entry := range f.dedupeMap {
		if entry.lastSeen.Before(expiry) {
			delete(f.dedupeMap, hash)
		}
	}

	// If still over limit, remove oldest entries
	if len(f.dedupeMap) > f.config.MaxDedupeEntries {
		// Find oldest entries (simple approach - in production use a proper LRU)
		excess := len(f.dedupeMap) - f.config.MaxDedupeEntries
		removed := 0
		for hash, entry := range f.dedupeMap {
			if entry.lastSeen.Before(now.Add(-f.config.DefaultDedupeWindow)) {
				delete(f.dedupeMap, hash)
				removed++
				if removed >= excess {
					break
				}
			}
		}
	}
}

// ShouldProcess returns true if the event should be processed
func (f *EventFilter) ShouldProcess(event *events.VMEvent) bool {
	action, _ := f.Filter(event)
	return action == FilterActionAllow
}

// Filter evaluates an event against all rules and returns the action
func (f *EventFilter) Filter(event *events.VMEvent) (action FilterAction, ruleName string) {
	f.statsMu.Lock()
	f.stats.TotalEvents++
	f.statsMu.Unlock()

	if !f.config.Enabled {
		f.statsMu.Lock()
		f.stats.AllowedEvents++
		f.statsMu.Unlock()
		return FilterActionAllow, ""
	}

	// Find matching rule (highest priority first)
	var matchedRule *FilterRule
	for i := range f.config.Rules {
		rule := &f.config.Rules[i]
		if !rule.Enabled {
			continue
		}
		if f.ruleMatches(rule, event) {
			if matchedRule == nil || rule.Priority < matchedRule.Priority {
				matchedRule = rule
			}
		}
	}

	// Apply matched rule or default action
	action = f.config.DefaultAction
	ruleName = ""
	if matchedRule != nil {
		action = matchedRule.Action
		ruleName = matchedRule.Name
	}

	// Handle deduplication
	if action == FilterActionDedupe {
		window := f.config.DefaultDedupeWindow
		if matchedRule != nil && matchedRule.DedupeWindow > 0 {
			window = matchedRule.DedupeWindow
		}

		if f.isDuplicate(event, window) {
			f.statsMu.Lock()
			f.stats.DedupedEvents++
			if ruleName != "" {
				f.stats.ByRule[ruleName]++
			}
			f.statsMu.Unlock()
			return FilterActionDrop, ruleName
		}
		// Not a duplicate, allow through
		action = FilterActionAllow
	}

	// Update stats
	f.statsMu.Lock()
	switch action {
	case FilterActionAllow:
		f.stats.AllowedEvents++
	case FilterActionDrop:
		f.stats.DroppedEvents++
	}
	if ruleName != "" {
		f.stats.ByRule[ruleName]++
	}
	f.statsMu.Unlock()

	return action, ruleName
}

// ruleMatches checks if a rule matches an event
func (f *EventFilter) ruleMatches(rule *FilterRule, event *events.VMEvent) bool {
	// Check event types
	if len(rule.EventTypes) > 0 {
		matched := false
		for _, et := range rule.EventTypes {
			if et == event.EventType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check VM name patterns
	if len(rule.VMNamePatterns) > 0 {
		matched := false
		for _, pattern := range rule.VMNamePatterns {
			if matchPattern(pattern, event.VMName) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check Wazuh level
	if rule.MinWazuhLevel > 0 && event.RuleLevel < rule.MinWazuhLevel {
		return false
	}
	if rule.MaxWazuhLevel > 0 && event.RuleLevel > rule.MaxWazuhLevel {
		return false
	}

	// Check exclude groups - if set, the rule matches when the event HAS one of these groups
	// (semantically: "drop events that belong to these groups")
	if len(rule.ExcludeGroups) > 0 {
		hasExcludedGroup := false
		for _, group := range rule.ExcludeGroups {
			for _, eventGroup := range event.Groups {
				if group == eventGroup {
					hasExcludedGroup = true
					break
				}
			}
			if hasExcludedGroup {
				break
			}
		}
		if !hasExcludedGroup {
			return false // Rule doesn't match if event doesn't have any excluded groups
		}
	}

	// Check include groups (if specified, at least one must match)
	if len(rule.IncludeGroups) > 0 {
		matched := false
		for _, group := range rule.IncludeGroups {
			for _, eventGroup := range event.Groups {
				if group == eventGroup {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

// isDuplicate checks if an event is a duplicate within the time window
func (f *EventFilter) isDuplicate(event *events.VMEvent, window time.Duration) bool {
	hash := f.hashEvent(event)

	f.dedupeMu.Lock()
	defer f.dedupeMu.Unlock()

	now := time.Now()

	entry, exists := f.dedupeMap[hash]
	if exists && now.Sub(entry.lastSeen) <= window {
		// Duplicate found within window
		entry.lastSeen = now
		entry.count++
		return true
	}

	// Not a duplicate, record this event
	f.dedupeMap[hash] = &dedupeEntry{
		hash:     hash,
		lastSeen: now,
		count:    1,
	}

	return false
}

// hashEvent creates a unique hash for event deduplication
func (f *EventFilter) hashEvent(event *events.VMEvent) string {
	// Create a hash from key event properties (not timestamp/ID)
	data := struct {
		PodID     string
		VMName    string
		EventType string
		RuleID    int
		Data      json.RawMessage
	}{
		PodID:     event.PodID,
		VMName:    event.VMName,
		EventType: event.EventType,
		RuleID:    event.RuleID,
		Data:      event.Data,
	}

	jsonBytes, _ := json.Marshal(data)
	hash := sha256.Sum256(jsonBytes)
	return hex.EncodeToString(hash[:])
}

// GetStats returns the current filter statistics
func (f *EventFilter) GetStats() FilterStats {
	f.statsMu.RLock()
	defer f.statsMu.RUnlock()

	// Deep copy
	stats := FilterStats{
		TotalEvents:   f.stats.TotalEvents,
		AllowedEvents: f.stats.AllowedEvents,
		DroppedEvents: f.stats.DroppedEvents,
		DedupedEvents: f.stats.DedupedEvents,
		ByRule:        make(map[string]int64),
	}
	for k, v := range f.stats.ByRule {
		stats.ByRule[k] = v
	}
	return stats
}

// ResetStats resets the filter statistics
func (f *EventFilter) ResetStats() {
	f.statsMu.Lock()
	defer f.statsMu.Unlock()
	f.stats = FilterStats{ByRule: make(map[string]int64)}
}

// AddRule adds a new filter rule
func (f *EventFilter) AddRule(rule FilterRule) {
	f.config.Rules = append(f.config.Rules, rule)
}

// RemoveRule removes a filter rule by name
func (f *EventFilter) RemoveRule(name string) bool {
	for i, rule := range f.config.Rules {
		if rule.Name == name {
			f.config.Rules = append(f.config.Rules[:i], f.config.Rules[i+1:]...)
			return true
		}
	}
	return false
}

// SetEnabled enables or disables filtering globally
func (f *EventFilter) SetEnabled(enabled bool) {
	f.config.Enabled = enabled
}

// GetConfig returns the current filter configuration
func (f *EventFilter) GetConfig() FilterConfig {
	return f.config
}

// matchPattern performs simple wildcard matching
func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	// Simple prefix/suffix matching for now
	if pattern != "" && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(value) >= len(prefix) && value[:len(prefix)] == prefix
	}
	if pattern != "" && pattern[0] == '*' {
		suffix := pattern[1:]
		return len(value) >= len(suffix) && value[len(value)-len(suffix):] == suffix
	}
	return pattern == value
}

// FilterResult provides detailed information about a filter decision
type FilterResult struct {
	Action      FilterAction `json:"action"`
	RuleName    string       `json:"ruleName,omitempty"`
	IsDuplicate bool         `json:"isDuplicate"`
	Hash        string       `json:"hash,omitempty"`
}

// FilterWithDetails returns detailed information about the filter decision
func (f *EventFilter) FilterWithDetails(event *events.VMEvent) FilterResult {
	result := FilterResult{
		Hash: f.hashEvent(event),
	}

	action, ruleName := f.Filter(event)
	result.Action = action
	result.RuleName = ruleName
	result.IsDuplicate = (action == FilterActionDrop && ruleName != "")

	return result
}
