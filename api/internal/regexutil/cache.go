// Package regexutil provides a shared, thread-safe regex compilation cache.
package regexutil

import (
	"fmt"
	"regexp"
	"sync"
)

// MaxPatternLength is the maximum allowed regex pattern length to prevent ReDoS.
const MaxPatternLength = 4096

var cache sync.Map

// CachedCompile compiles and caches regex patterns. Thread-safe.
func CachedCompile(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > MaxPatternLength {
		return nil, fmt.Errorf("regex pattern exceeds max length (%d)", MaxPatternLength)
	}
	if cached, ok := cache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	cache.Store(pattern, re)
	return re, nil
}
