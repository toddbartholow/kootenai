// Package resilience provides circuit breaker and retry patterns for external API calls
package resilience

import (
	"encoding/json"
	"net/http"
	"sync"
)

// Registry maintains a collection of circuit breakers for monitoring
type Registry struct {
	mu       sync.RWMutex
	breakers map[string]*CircuitBreaker
}

// GlobalRegistry is the default circuit breaker registry
var GlobalRegistry = NewRegistry()

// NewRegistry creates a new circuit breaker registry
func NewRegistry() *Registry {
	return &Registry{
		breakers: make(map[string]*CircuitBreaker),
	}
}

// Register adds a circuit breaker to the registry
func (r *Registry) Register(cb *CircuitBreaker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.breakers[cb.name] = cb
}

// Unregister removes a circuit breaker from the registry
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.breakers, name)
}

// Get retrieves a circuit breaker by name
func (r *Registry) Get(name string) (*CircuitBreaker, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cb, ok := r.breakers[name]
	return cb, ok
}

// Stats returns statistics for all registered circuit breakers
func (r *Registry) Stats() map[string]CircuitStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := make(map[string]CircuitStats, len(r.breakers))
	for name, cb := range r.breakers {
		stats[name] = cb.Stats()
	}
	return stats
}

// List returns the names of all registered circuit breakers
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.breakers))
	for name := range r.breakers {
		names = append(names, name)
	}
	return names
}

// HealthStatus represents the overall health of circuit breakers
type HealthStatus struct {
	Healthy   bool                    `json:"healthy"`
	OpenCount int                     `json:"openCount"`
	Total     int                     `json:"total"`
	Circuits  map[string]CircuitStats `json:"circuits"`
}

// Health returns the overall health status of all circuit breakers
func (r *Registry) Health() HealthStatus {
	stats := r.Stats()

	status := HealthStatus{
		Healthy:  true,
		Total:    len(stats),
		Circuits: stats,
	}

	for _, s := range stats {
		if s.State == StateOpen.String() {
			status.OpenCount++
			status.Healthy = false
		}
	}

	return status
}

// HTTPHandler returns an HTTP handler that exposes circuit breaker stats
func (r *Registry) HTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		health := r.Health()
		w.Header().Set("Content-Type", "application/json")

		status := http.StatusOK
		if !health.Healthy {
			status = http.StatusServiceUnavailable
		}
		w.WriteHeader(status)

		if err := json.NewEncoder(w).Encode(health); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// NewCircuitBreakerWithRegistry creates a circuit breaker and registers it
func NewCircuitBreakerWithRegistry(name string, config CircuitBreakerConfig, registry *Registry) *CircuitBreaker {
	cb := NewCircuitBreaker(name, config, nil)
	if registry != nil {
		registry.Register(cb)
	}
	return cb
}

// NewResilientClientWithRegistry creates a resilient client and registers its circuit breaker
func NewResilientClientWithRegistry(config ResilientClientConfig, registry *Registry) *ResilientClient {
	rc := NewResilientClient(config)
	if registry != nil {
		registry.Register(rc.circuitBreaker)
	}
	return rc
}
