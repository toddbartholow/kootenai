package namegen

import (
	"regexp"
	"testing"
)

func TestGenerate(t *testing.T) {
	// Generate a name
	name := Generate()

	// Should match pattern adjective-verb-noun
	pattern := regexp.MustCompile(`^[a-z]+-[a-z]+-[a-z]+$`)
	if !pattern.MatchString(name) {
		t.Errorf("Generated name %q does not match expected pattern", name)
	}

	// Should have exactly 2 hyphens (3 parts)
	parts := regexp.MustCompile(`-`).Split(name, -1)
	if len(parts) != 3 {
		t.Errorf("Expected 3 parts, got %d in name %q", len(parts), name)
	}
}

func TestGenerateUniqueness(t *testing.T) {
	// Generate 100 names and check for duplicates
	names := make(map[string]bool)
	for range 100 {
		name := Generate()
		if names[name] {
			// Duplicates are possible but should be rare
			t.Logf("Note: Got duplicate name %q (this is possible but rare)", name)
		}
		names[name] = true
	}
}

func TestGenerateWithSuffix(t *testing.T) {
	name := generateWithSuffix("test")
	pattern := regexp.MustCompile(`^[a-z]+-[a-z]+-[a-z]+-test$`)
	if !pattern.MatchString(name) {
		t.Errorf("Generated name %q does not match expected pattern with suffix", name)
	}
}

func TestGenerateUnique(t *testing.T) {
	name := generateUnique()

	// Should match pattern adjective-verb-noun-xxxx
	pattern := regexp.MustCompile(`^[a-z]+-[a-z]+-[a-z]+-[a-z0-9]{4}$`)
	if !pattern.MatchString(name) {
		t.Errorf("Generated unique name %q does not match expected pattern", name)
	}
}

func TestRandomSuffix(t *testing.T) {
	suffix := randomSuffix(4)
	if len(suffix) != 4 {
		t.Errorf("Expected suffix length 4, got %d", len(suffix))
	}

	// Should only contain lowercase letters and digits
	pattern := regexp.MustCompile(`^[a-z0-9]+$`)
	if !pattern.MatchString(suffix) {
		t.Errorf("Suffix %q contains invalid characters", suffix)
	}
}

func BenchmarkGenerate(b *testing.B) {
	for b.Loop() {
		Generate()
	}
}
