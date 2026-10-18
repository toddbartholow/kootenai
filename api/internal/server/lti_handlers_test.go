package server

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	redisclient "github.com/toddbartholow/kootenai/api/internal/redis"
	"github.com/toddbartholow/kootenai/api/internal/testutil/mocks"
)

func ltiTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// -----------------------------------------------------------------------------
// LTI State Management Tests
// -----------------------------------------------------------------------------

func TestStoreLTIState_WithMockCache(t *testing.T) {
	t.Run("stores state successfully", func(t *testing.T) {
		mockCache := mocks.NewFakeLTIStateStore()
		manager, _ := NewCanvasManager(CanvasManagerConfig{
			LTIStateCache: mockCache,
			Logger:        ltiTestLogger(),
		})

		ctx := context.Background()
		state := "test-state-123"
		data := ltiState{
			Nonce:         "test-nonce",
			TargetLinkURI: "https://example.com/launch",
			CreatedAt:     time.Now(),
		}

		err := manager.storeLTIState(ctx, state, data)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Verify the mock was called correctly
		if len(mockCache.StoreCalls) != 1 {
			t.Fatalf("expected 1 store call, got %d", len(mockCache.StoreCalls))
		}

		call := mockCache.StoreCalls[0]
		if call.State != state {
			t.Errorf("expected state %q, got %q", state, call.State)
		}
		if call.Data.Nonce != data.Nonce {
			t.Errorf("expected nonce %q, got %q", data.Nonce, call.Data.Nonce)
		}
		if call.Data.TargetLinkURI != data.TargetLinkURI {
			t.Errorf("expected target link URI %q, got %q", data.TargetLinkURI, call.Data.TargetLinkURI)
		}
	})

	t.Run("returns error when store fails", func(t *testing.T) {
		mockCache := mocks.NewFakeLTIStateStore()
		mockCache.StoreErr = errors.New("redis connection failed")

		manager, _ := NewCanvasManager(CanvasManagerConfig{
			LTIStateCache: mockCache,
			Logger:        ltiTestLogger(),
		})

		ctx := context.Background()
		err := manager.storeLTIState(ctx, "test-state", ltiState{Nonce: "test"})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "redis connection failed" {
			t.Errorf("expected 'redis connection failed', got %q", err.Error())
		}
	})

	t.Run("falls back to in-memory when no cache available", func(t *testing.T) {
		manager, _ := NewCanvasManager(CanvasManagerConfig{
			Logger: ltiTestLogger(),
		})

		ctx := context.Background()
		state := "fallback-test-state"
		data := ltiState{
			Nonce:         "fallback-nonce",
			TargetLinkURI: "https://example.com/fallback",
			CreatedAt:     time.Now(),
		}

		err := manager.storeLTIState(ctx, state, data)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		// Verify we can retrieve it from in-memory store
		retrieved, exists := manager.getLTIState(ctx, state)
		if !exists {
			t.Fatal("expected state to exist in in-memory store")
		}
		if retrieved.Nonce != data.Nonce {
			t.Errorf("expected nonce %q, got %q", data.Nonce, retrieved.Nonce)
		}
	})
}

func TestGetLTIState_WithMockCache(t *testing.T) {
	t.Run("retrieves stored state successfully", func(t *testing.T) {
		mockCache := mocks.NewFakeLTIStateStore()
		manager, _ := NewCanvasManager(CanvasManagerConfig{
			LTIStateCache: mockCache,
			Logger:        ltiTestLogger(),
		})

		// Pre-store some state
		ctx := context.Background()
		state := "test-state-456"
		storedData := redisclient.LTIState{
			Nonce:         "stored-nonce",
			TargetLinkURI: "https://example.com/stored",
			CreatedAt:     time.Now(),
		}
		_ = mockCache.Store(ctx, state, storedData)
		mockCache.StoreCalls = nil // Reset call tracking

		// Now retrieve it
		retrieved, exists := manager.getLTIState(ctx, state)
		if !exists {
			t.Fatal("expected state to exist")
		}
		if retrieved.Nonce != storedData.Nonce {
			t.Errorf("expected nonce %q, got %q", storedData.Nonce, retrieved.Nonce)
		}
		if retrieved.TargetLinkURI != storedData.TargetLinkURI {
			t.Errorf("expected target link URI %q, got %q", storedData.TargetLinkURI, retrieved.TargetLinkURI)
		}

		// Verify the mock was called correctly
		if len(mockCache.GetCalls) != 1 {
			t.Fatalf("expected 1 get call, got %d", len(mockCache.GetCalls))
		}
		if mockCache.GetCalls[0] != state {
			t.Errorf("expected get call with %q, got %q", state, mockCache.GetCalls[0])
		}
	})

	t.Run("returns false when state not found", func(t *testing.T) {
		mockCache := mocks.NewFakeLTIStateStore()
		manager, _ := NewCanvasManager(CanvasManagerConfig{
			LTIStateCache: mockCache,
			Logger:        ltiTestLogger(),
		})

		ctx := context.Background()
		_, exists := manager.getLTIState(ctx, "non-existent-state")
		if exists {
			t.Error("expected state to not exist")
		}
	})

	t.Run("returns false on cache error", func(t *testing.T) {
		mockCache := mocks.NewFakeLTIStateStore()
		mockCache.GetErr = errors.New("cache unavailable")

		manager, _ := NewCanvasManager(CanvasManagerConfig{
			LTIStateCache: mockCache,
			Logger:        ltiTestLogger(),
		})

		ctx := context.Background()
		_, exists := manager.getLTIState(ctx, "test-state")
		if exists {
			t.Error("expected state to not exist when cache fails")
		}
	})

	t.Run("falls back to in-memory when no cache available", func(t *testing.T) {
		manager, _ := NewCanvasManager(CanvasManagerConfig{
			Logger: ltiTestLogger(),
		})

		ctx := context.Background()
		state := "inmemory-state"

		// Store first (uses in-memory)
		_ = manager.storeLTIState(ctx, state, ltiState{
			Nonce:     "inmemory-nonce",
			CreatedAt: time.Now(),
		})

		// Retrieve (uses in-memory)
		retrieved, exists := manager.getLTIState(ctx, state)
		if !exists {
			t.Fatal("expected state to exist in in-memory store")
		}
		if retrieved.Nonce != "inmemory-nonce" {
			t.Errorf("expected nonce 'inmemory-nonce', got %q", retrieved.Nonce)
		}
	})
}

func TestDeleteLTIState_WithMockCache(t *testing.T) {
	t.Run("deletes state successfully", func(t *testing.T) {
		mockCache := mocks.NewFakeLTIStateStore()
		manager, _ := NewCanvasManager(CanvasManagerConfig{
			LTIStateCache: mockCache,
			Logger:        ltiTestLogger(),
		})

		// Pre-store some state
		ctx := context.Background()
		state := "to-delete-state"
		_ = mockCache.Store(ctx, state, redisclient.LTIState{Nonce: "delete-me"})
		mockCache.StoreCalls = nil

		// Delete it
		manager.deleteLTIState(ctx, state)

		// Verify the mock was called correctly
		if len(mockCache.DeleteCalls) != 1 {
			t.Fatalf("expected 1 delete call, got %d", len(mockCache.DeleteCalls))
		}
		if mockCache.DeleteCalls[0] != state {
			t.Errorf("expected delete call with %q, got %q", state, mockCache.DeleteCalls[0])
		}

		// Verify it's actually deleted
		_, exists := manager.getLTIState(ctx, state)
		if exists {
			t.Error("expected state to be deleted")
		}
	})

	t.Run("handles delete error gracefully", func(t *testing.T) {
		mockCache := mocks.NewFakeLTIStateStore()
		mockCache.DeleteErr = errors.New("delete failed")

		manager, _ := NewCanvasManager(CanvasManagerConfig{
			LTIStateCache: mockCache,
			Logger:        ltiTestLogger(),
		})

		ctx := context.Background()
		// Should not panic, just log the error
		manager.deleteLTIState(ctx, "test-state")

		// Verify delete was still called
		if len(mockCache.DeleteCalls) != 1 {
			t.Errorf("expected 1 delete call, got %d", len(mockCache.DeleteCalls))
		}
	})

	t.Run("falls back to in-memory when no cache available", func(t *testing.T) {
		manager, _ := NewCanvasManager(CanvasManagerConfig{
			Logger: ltiTestLogger(),
		})

		ctx := context.Background()
		state := "inmemory-delete-state"

		// Store first
		_ = manager.storeLTIState(ctx, state, ltiState{
			Nonce:     "to-delete",
			CreatedAt: time.Now(),
		})

		// Verify it exists
		_, exists := manager.getLTIState(ctx, state)
		if !exists {
			t.Fatal("expected state to exist before delete")
		}

		// Delete it
		manager.deleteLTIState(ctx, state)

		// Verify it's gone
		_, exists = manager.getLTIState(ctx, state)
		if exists {
			t.Error("expected state to be deleted from in-memory store")
		}
	})
}

func TestGetLTIStateCache_Helper(t *testing.T) {
	t.Run("returns injected cache when available", func(t *testing.T) {
		mockCache := mocks.NewFakeLTIStateStore()
		manager, _ := NewCanvasManager(CanvasManagerConfig{
			LTIStateCache: mockCache,
			Logger:        ltiTestLogger(),
		})

		cache := manager.getLTIStateCache()
		if cache == nil {
			t.Fatal("expected cache to be returned")
		}
		if cache != mockCache {
			t.Error("expected injected cache to be returned")
		}
	})

	t.Run("returns nil when no cache configured", func(t *testing.T) {
		manager, _ := NewCanvasManager(CanvasManagerConfig{
			Logger: ltiTestLogger(),
		})

		cache := manager.getLTIStateCache()
		if cache != nil {
			t.Error("expected nil cache when nothing configured")
		}
	})
}
