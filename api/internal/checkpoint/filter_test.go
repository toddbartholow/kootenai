package checkpoint

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/toddbartholow/kootenai/api/internal/events"
)

func TestNewEventFilter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := DefaultFilterConfig()

	filter := NewEventFilter(config, logger)

	if filter == nil {
		t.Fatal("NewEventFilter returned nil")
	}

	if filter.config.MaxDedupeEntries != 10000 {
		t.Errorf("expected max dedupe entries 10000, got %d", filter.config.MaxDedupeEntries)
	}

	if filter.config.DefaultAction != FilterActionAllow {
		t.Errorf("expected default action 'allow', got %s", filter.config.DefaultAction)
	}
}

func TestEventFilter_ShouldProcess(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       true,
		DefaultAction: FilterActionAllow,
		Rules: []FilterRule{
			{
				Name:          "drop-low-level",
				MaxWazuhLevel: 2,
				Action:        FilterActionDrop,
				Priority:      10,
				Enabled:       true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	tests := []struct {
		name        string
		event       *events.VMEvent
		shouldAllow bool
	}{
		{
			name: "allow high level event",
			event: &events.VMEvent{
				EventType: "test",
				RuleLevel: 5,
			},
			shouldAllow: true,
		},
		{
			name: "drop low level event",
			event: &events.VMEvent{
				EventType: "test",
				RuleLevel: 2,
			},
			shouldAllow: false,
		},
		{
			name: "drop very low level event",
			event: &events.VMEvent{
				EventType: "test",
				RuleLevel: 1,
			},
			shouldAllow: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filter.ShouldProcess(tt.event)
			if result != tt.shouldAllow {
				t.Errorf("ShouldProcess() = %v, want %v", result, tt.shouldAllow)
			}
		})
	}
}

func TestEventFilter_Deduplication(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:             true,
		DefaultAction:       FilterActionAllow,
		DefaultDedupeWindow: 100 * time.Millisecond,
		Rules: []FilterRule{
			{
				Name:         "dedupe-all",
				Action:       FilterActionDedupe,
				Priority:     10,
				Enabled:      true,
				DedupeWindow: 100 * time.Millisecond,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	event := &events.VMEvent{
		PodID:     "pod-1",
		VMName:    "vm-1",
		EventType: "test",
		RuleID:    100,
		Data:      json.RawMessage(`{"key": "value"}`),
	}

	// First event should be allowed
	action1, _ := filter.Filter(event)
	if action1 != FilterActionAllow {
		t.Errorf("first event should be allowed, got %s", action1)
	}

	// Second identical event within window should be dropped
	action2, _ := filter.Filter(event)
	if action2 != FilterActionDrop {
		t.Errorf("duplicate event should be dropped, got %s", action2)
	}

	// Third identical event within window should also be dropped
	action3, _ := filter.Filter(event)
	if action3 != FilterActionDrop {
		t.Errorf("duplicate event should be dropped, got %s", action3)
	}

	// This sleep is intentional and cannot be replaced with require.Eventually
	// (audit 7B): Filter()'s dedupe map records a fresh lastSeen timestamp on
	// every call, so polling Filter() would indefinitely extend the window
	// instead of letting it expire. The only way to observe the post-window
	// state without a clock-injection refactor is a wall-clock wait past the
	// 100ms DedupeWindow. Kept at 150ms for a 50ms safety margin.
	time.Sleep(150 * time.Millisecond)

	// After window, event should be allowed again
	action4, _ := filter.Filter(event)
	if action4 != FilterActionAllow {
		t.Errorf("event after window should be allowed, got %s", action4)
	}
}

func TestEventFilter_EventTypeFiltering(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       true,
		DefaultAction: FilterActionAllow,
		Rules: []FilterRule{
			{
				Name:       "drop-syslog",
				EventTypes: []string{"syslog"},
				Action:     FilterActionDrop,
				Priority:   10,
				Enabled:    true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	// Syslog events should be dropped
	action, _ := filter.Filter(&events.VMEvent{EventType: "syslog"})
	if action != FilterActionDrop {
		t.Errorf("syslog event should be dropped, got %s", action)
	}

	// Other events should be allowed
	action, _ = filter.Filter(&events.VMEvent{EventType: "fim_added"})
	if action != FilterActionAllow {
		t.Errorf("non-syslog event should be allowed, got %s", action)
	}
}

func TestEventFilter_VMNameFiltering(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       true,
		DefaultAction: FilterActionAllow,
		Rules: []FilterRule{
			{
				Name:           "allow-webservers",
				VMNamePatterns: []string{"web-*"},
				Action:         FilterActionAllow,
				Priority:       5,
				Enabled:        true,
			},
			{
				Name:     "drop-all-else",
				Action:   FilterActionDrop,
				Priority: 100,
				Enabled:  true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	// Webserver events should be allowed
	action, _ := filter.Filter(&events.VMEvent{VMName: "web-01"})
	if action != FilterActionAllow {
		t.Errorf("webserver event should be allowed, got %s", action)
	}

	// Other VMs should be dropped
	action, _ = filter.Filter(&events.VMEvent{VMName: "db-01"})
	if action != FilterActionDrop {
		t.Errorf("non-webserver event should be dropped, got %s", action)
	}
}

func TestEventFilter_GroupFiltering(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       true,
		DefaultAction: FilterActionAllow,
		Rules: []FilterRule{
			{
				Name:          "drop-noise-groups",
				ExcludeGroups: []string{"pam", "audit"},
				Action:        FilterActionDrop,
				Priority:      10,
				Enabled:       true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	// Events with excluded groups should be dropped
	action, _ := filter.Filter(&events.VMEvent{
		Groups: []string{"pam", "authentication"},
	})
	if action != FilterActionDrop {
		t.Errorf("event with excluded group should be dropped, got %s", action)
	}

	// Events without excluded groups should be allowed
	action, _ = filter.Filter(&events.VMEvent{
		Groups: []string{"systemd", "service"},
	})
	if action != FilterActionAllow {
		t.Errorf("event without excluded group should be allowed, got %s", action)
	}
}

func TestEventFilter_IncludeGroups(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       true,
		DefaultAction: FilterActionDrop,
		Rules: []FilterRule{
			{
				Name:          "allow-security",
				IncludeGroups: []string{"syscheck", "authentication"},
				Action:        FilterActionAllow,
				Priority:      10,
				Enabled:       true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	// Events with included groups should be allowed
	action, _ := filter.Filter(&events.VMEvent{
		Groups: []string{"syscheck", "file"},
	})
	if action != FilterActionAllow {
		t.Errorf("event with included group should be allowed, got %s", action)
	}

	// Events without included groups should be dropped (default action)
	action, _ = filter.Filter(&events.VMEvent{
		Groups: []string{"random", "other"},
	})
	if action != FilterActionDrop {
		t.Errorf("event without included group should be dropped, got %s", action)
	}
}

func TestEventFilter_Stats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:             true,
		DefaultAction:       FilterActionAllow,
		DefaultDedupeWindow: 1 * time.Second,
		Rules: []FilterRule{
			{
				Name:       "drop-test",
				EventTypes: []string{"drop-me"},
				Action:     FilterActionDrop,
				Priority:   10,
				Enabled:    true,
			},
			{
				Name:       "dedupe-test",
				EventTypes: []string{"dedupe-me"},
				Action:     FilterActionDedupe,
				Priority:   10,
				Enabled:    true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	// Process some events
	filter.Filter(&events.VMEvent{EventType: "allow-me"})
	filter.Filter(&events.VMEvent{EventType: "drop-me"})
	filter.Filter(&events.VMEvent{EventType: "dedupe-me", PodID: "a", VMName: "b"})
	filter.Filter(&events.VMEvent{EventType: "dedupe-me", PodID: "a", VMName: "b"}) // duplicate

	stats := filter.GetStats()

	if stats.TotalEvents != 4 {
		t.Errorf("expected 4 total events, got %d", stats.TotalEvents)
	}

	if stats.AllowedEvents != 2 {
		t.Errorf("expected 2 allowed events, got %d", stats.AllowedEvents)
	}

	if stats.DroppedEvents != 1 {
		t.Errorf("expected 1 dropped event, got %d", stats.DroppedEvents)
	}

	if stats.DedupedEvents != 1 {
		t.Errorf("expected 1 deduped event, got %d", stats.DedupedEvents)
	}
}

func TestEventFilter_AddRemoveRule(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       true,
		DefaultAction: FilterActionAllow,
		Rules:         []FilterRule{},
	}
	filter := NewEventFilter(config, logger)

	// Initially no rules, event should be allowed
	action, _ := filter.Filter(&events.VMEvent{EventType: "test"})
	if action != FilterActionAllow {
		t.Errorf("expected allow with no rules, got %s", action)
	}

	// Add a rule to drop test events
	filter.AddRule(FilterRule{
		Name:       "drop-test",
		EventTypes: []string{"test"},
		Action:     FilterActionDrop,
		Priority:   10,
		Enabled:    true,
	})

	// Now event should be dropped
	action, ruleName := filter.Filter(&events.VMEvent{EventType: "test"})
	if action != FilterActionDrop {
		t.Errorf("expected drop after adding rule, got %s", action)
	}
	if ruleName != "drop-test" {
		t.Errorf("expected rule name 'drop-test', got '%s'", ruleName)
	}

	// Remove the rule
	removed := filter.RemoveRule("drop-test")
	if !removed {
		t.Error("RemoveRule should return true")
	}

	// Event should be allowed again
	action, _ = filter.Filter(&events.VMEvent{EventType: "test"})
	if action != FilterActionAllow {
		t.Errorf("expected allow after removing rule, got %s", action)
	}
}

func TestEventFilter_DisabledFilter(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       false,
		DefaultAction: FilterActionDrop,
		Rules: []FilterRule{
			{
				Name:     "drop-all",
				Action:   FilterActionDrop,
				Priority: 10,
				Enabled:  true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	// When filter is disabled, all events should be allowed
	action, _ := filter.Filter(&events.VMEvent{EventType: "test"})
	if action != FilterActionAllow {
		t.Errorf("expected allow when filter disabled, got %s", action)
	}
}

func TestEventFilter_RulePriority(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       true,
		DefaultAction: FilterActionAllow,
		Rules: []FilterRule{
			{
				Name:     "high-priority-allow",
				Priority: 1,
				Action:   FilterActionAllow,
				Enabled:  true,
			},
			{
				Name:     "low-priority-drop",
				Priority: 100,
				Action:   FilterActionDrop,
				Enabled:  true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	// High priority rule should take precedence
	action, ruleName := filter.Filter(&events.VMEvent{EventType: "test"})
	if action != FilterActionAllow {
		t.Errorf("expected allow from high priority rule, got %s", action)
	}
	if ruleName != "high-priority-allow" {
		t.Errorf("expected rule 'high-priority-allow', got '%s'", ruleName)
	}
}

func TestEventFilter_FilterWithDetails(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:             true,
		DefaultAction:       FilterActionAllow,
		DefaultDedupeWindow: 1 * time.Second,
		Rules: []FilterRule{
			{
				Name:     "dedupe-test",
				Action:   FilterActionDedupe,
				Priority: 10,
				Enabled:  true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	event := &events.VMEvent{
		PodID:     "pod-1",
		VMName:    "vm-1",
		EventType: "test",
	}

	// First call
	result1 := filter.FilterWithDetails(event)
	if result1.Action != FilterActionAllow {
		t.Errorf("first event should be allowed, got %s", result1.Action)
	}
	if result1.Hash == "" {
		t.Error("hash should not be empty")
	}

	// Second call (duplicate)
	result2 := filter.FilterWithDetails(event)
	if result2.Action != FilterActionDrop {
		t.Errorf("duplicate should be dropped, got %s", result2.Action)
	}
	if result2.Hash != result1.Hash {
		t.Error("hash should be identical for duplicate events")
	}
}

func TestEventFilter_CleanupDedupeEntries(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:               true,
		DefaultAction:         FilterActionAllow,
		DefaultDedupeWindow:   10 * time.Millisecond,
		MaxDedupeEntries:      5,
		DedupeCleanupInterval: 10 * time.Millisecond,
		Rules: []FilterRule{
			{
				Name:         "dedupe-test",
				Action:       FilterActionDedupe,
				DedupeWindow: 10 * time.Millisecond,
				Priority:     10,
				Enabled:      true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	// Add many events
	for i := 0; i < 10; i++ {
		filter.Filter(&events.VMEvent{
			PodID:     "pod-1",
			VMName:    "vm-1",
			EventType: "test",
			RuleID:    i,
		})
	}

	// Poll until entries expire and cleanup removes them
	require.Eventually(t, func() bool {
		filter.cleanupDedupeEntries()
		filter.dedupeMu.RLock()
		count := len(filter.dedupeMap)
		filter.dedupeMu.RUnlock()
		return count <= config.MaxDedupeEntries
	}, 1*time.Second, 10*time.Millisecond, "expected entries to be cleaned up after expiry")
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		match   bool
	}{
		{"*", "anything", true},
		{"web-*", "web-01", true},
		{"web-*", "db-01", false},
		{"*-server", "web-server", true},
		{"*-server", "web-client", false},
		{"exact", "exact", true},
		{"exact", "different", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.value, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.value)
			if result != tt.match {
				t.Errorf("matchPattern(%s, %s) = %v, want %v", tt.pattern, tt.value, result, tt.match)
			}
		})
	}
}

func TestDefaultFilterConfig(t *testing.T) {
	config := DefaultFilterConfig()

	if !config.Enabled {
		t.Error("expected filter to be enabled by default")
	}

	if config.DefaultAction != FilterActionAllow {
		t.Errorf("expected default action 'allow', got %s", config.DefaultAction)
	}

	if len(config.Rules) < 3 {
		t.Errorf("expected at least 3 default rules, got %d", len(config.Rules))
	}

	// Check that default rules exist
	foundDropLowLevel := false
	foundDedupeSyslog := false
	foundDedupeFim := false

	for _, rule := range config.Rules {
		switch rule.Name {
		case "drop-low-level":
			foundDropLowLevel = true
		case "dedupe-syslog":
			foundDedupeSyslog = true
		case "dedupe-fim":
			foundDedupeFim = true
		}
	}

	if !foundDropLowLevel {
		t.Error("expected 'drop-low-level' rule in defaults")
	}
	if !foundDedupeSyslog {
		t.Error("expected 'dedupe-syslog' rule in defaults")
	}
	if !foundDedupeFim {
		t.Error("expected 'dedupe-fim' rule in defaults")
	}
}

func TestEventFilter_ResetStats(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       true,
		DefaultAction: FilterActionAllow,
		Rules: []FilterRule{
			{
				Name:     "test-rule",
				Action:   FilterActionAllow,
				Priority: 10,
				Enabled:  true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	// Process some events to generate stats
	event := &events.VMEvent{EventType: "test", RuleLevel: 5}
	filter.Filter(event)
	filter.Filter(event)

	// Verify stats exist
	stats := filter.GetStats()
	if stats.TotalEvents == 0 {
		t.Error("expected stats to be populated")
	}

	// Reset stats
	filter.ResetStats()

	// Verify stats are reset
	stats = filter.GetStats()
	if stats.TotalEvents != 0 {
		t.Errorf("expected TotalEvents to be 0 after reset, got %d", stats.TotalEvents)
	}
	if stats.AllowedEvents != 0 {
		t.Errorf("expected AllowedEvents to be 0 after reset, got %d", stats.AllowedEvents)
	}
}

func TestEventFilter_SetEnabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       true,
		DefaultAction: FilterActionAllow,
		Rules: []FilterRule{
			{
				Name:     "drop-all",
				Action:   FilterActionDrop,
				Priority: 100,
				Enabled:  true,
			},
		},
	}
	filter := NewEventFilter(config, logger)

	event := &events.VMEvent{EventType: "test", RuleLevel: 5}

	// With filtering enabled, events should be dropped
	action, _ := filter.Filter(event)
	if action != FilterActionDrop {
		t.Errorf("expected action 'drop' when enabled, got %s", action)
	}

	// Disable filtering
	filter.SetEnabled(false)

	// With filtering disabled, events should be allowed
	action, _ = filter.Filter(event)
	if action != FilterActionAllow {
		t.Errorf("expected action 'allow' when disabled, got %s", action)
	}

	// Re-enable filtering
	filter.SetEnabled(true)

	// Verify filtering is back to normal
	action, _ = filter.Filter(event)
	if action != FilterActionDrop {
		t.Errorf("expected action 'drop' when re-enabled, got %s", action)
	}
}

func TestEventFilter_GetConfig(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:          true,
		DefaultAction:    FilterActionDrop,
		MaxDedupeEntries: 5000,
		Rules: []FilterRule{
			{Name: "rule1", Action: FilterActionAllow, Enabled: true},
			{Name: "rule2", Action: FilterActionDrop, Enabled: false},
		},
	}
	filter := NewEventFilter(config, logger)

	gotConfig := filter.GetConfig()

	if gotConfig.Enabled != config.Enabled {
		t.Errorf("expected Enabled %v, got %v", config.Enabled, gotConfig.Enabled)
	}
	if gotConfig.DefaultAction != config.DefaultAction {
		t.Errorf("expected DefaultAction %s, got %s", config.DefaultAction, gotConfig.DefaultAction)
	}
	if gotConfig.MaxDedupeEntries != config.MaxDedupeEntries {
		t.Errorf("expected MaxDedupeEntries %d, got %d", config.MaxDedupeEntries, gotConfig.MaxDedupeEntries)
	}
	if len(gotConfig.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(gotConfig.Rules))
	}
}

func TestEventFilter_RemoveRule_NotFound(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	config := FilterConfig{
		Enabled:       true,
		DefaultAction: FilterActionAllow,
		Rules: []FilterRule{
			{Name: "rule1", Action: FilterActionAllow, Enabled: true},
		},
	}
	filter := NewEventFilter(config, logger)

	// Try to remove a non-existent rule
	removed := filter.RemoveRule("nonexistent")
	if removed {
		t.Error("expected RemoveRule to return false for non-existent rule")
	}

	// Verify original rules are intact
	gotConfig := filter.GetConfig()
	if len(gotConfig.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(gotConfig.Rules))
	}
}
