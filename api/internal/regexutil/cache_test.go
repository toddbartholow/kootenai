package regexutil

import (
	"strings"
	"testing"
)

func TestCachedCompile_Basic(t *testing.T) {
	re, err := CachedCompile(`^hello\s+world$`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !re.MatchString("hello  world") {
		t.Error("expected match")
	}
}

func TestCachedCompile_CacheHit(t *testing.T) {
	pattern := `test-cache-hit-\d+`
	re1, err := CachedCompile(pattern)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	re2, err := CachedCompile(pattern)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if re1 != re2 {
		t.Error("expected same pointer for cached pattern")
	}
}

func TestCachedCompile_InvalidPattern(t *testing.T) {
	_, err := CachedCompile(`[invalid`)
	if err == nil {
		t.Error("expected error for invalid pattern")
	}
}

func TestCachedCompile_OverLength(t *testing.T) {
	long := strings.Repeat("a", MaxPatternLength+1)
	_, err := CachedCompile(long)
	if err == nil {
		t.Error("expected error for over-length pattern")
	}
	if !strings.Contains(err.Error(), "max length") {
		t.Errorf("expected max length error, got: %v", err)
	}
}

func TestCachedCompile_ExactMaxLength(t *testing.T) {
	pattern := strings.Repeat("a", MaxPatternLength)
	_, err := CachedCompile(pattern)
	if err != nil {
		t.Fatalf("pattern at exact max length should compile: %v", err)
	}
}
