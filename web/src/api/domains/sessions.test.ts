/**
 * Tests for Sessions API module - getCheckpointHint
 */
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { sessionsApi } from './sessions'
import { api } from '../config'
import * as mockModule from '../shared/mock'

// Mock the api module
vi.mock('../config', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

describe('sessionsApi', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  describe('getCheckpointHint', () => {
    describe('with mock data', () => {
      beforeEach(() => {
        // Force mock mode
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(true)
      })

      it('returns mock hint for known checkpoint', async () => {
        // Use checkpoint ID that exists in mock data (cp-1, cp-2, cp-3)
        const result = await sessionsApi.getCheckpointHint('mock-hint-session-1', 'cp-1')

        expect(result).toBeDefined()
        expect(result.checkpointId).toBe('cp-1')
        expect(result.level).toBe(1)
        expect(result.maxLevel).toBe(3)
        expect(result.hint).toContain('home directory')
      })

      it('returns no hint available for unknown checkpoint', async () => {
        const result = await sessionsApi.getCheckpointHint('session-1', 'unknown-checkpoint')

        expect(result).toBeDefined()
        expect(result.checkpointId).toBe('unknown-checkpoint')
        expect(result.level).toBe(0)
        expect(result.maxLevel).toBe(0)
        expect(result.hint).toBe('No hint available')
        expect(result.penalty).toBe(0)
        expect(result.totalPenalty).toBe(0)
      })

      it('returns progressive hints with increasing level', async () => {
        // First hint request (uses cp-3 which has 3 levels)
        const result1 = await sessionsApi.getCheckpointHint('progressive-session', 'cp-3')
        expect(result1.level).toBe(1)

        // Second hint request (should return level 2)
        const result2 = await sessionsApi.getCheckpointHint('progressive-session', 'cp-3')
        expect(result2.level).toBe(2)
      })

      it('returns same hint without penalty when re-requesting already shown level', async () => {
        // First request hint level 1
        const result1 = await sessionsApi.getCheckpointHint('reshow-session', 'cp-1')
        expect(result1.level).toBe(1)
        expect(result1.penalty).toBe(0) // First hint has no penalty

        // Request level 1 again explicitly
        const result2 = await sessionsApi.getCheckpointHint('reshow-session', 'cp-1', 1)
        expect(result2.level).toBe(1)
        expect(result2.penalty).toBe(0) // No additional penalty for re-viewing
      })

      it('accepts optional level parameter for specific hint', async () => {
        const result = await sessionsApi.getCheckpointHint('level-test-session', 'cp-1', 2)

        expect(result.level).toBe(2)
      })

      it('does not call the actual API in mock mode', async () => {
        await sessionsApi.getCheckpointHint('no-api-call-session', 'cp-1')

        expect(api.post).not.toHaveBeenCalled()
      })
    })

    describe('with real API', () => {
      beforeEach(() => {
        // Force real API mode
        vi.spyOn(mockModule, 'USE_MOCK_DATA', 'get').mockReturnValue(false)
      })

      it('calls the correct API endpoint without level', async () => {
        const mockResponse = {
          data: {
            checkpointId: 'cp-1',
            level: 1,
            maxLevel: 3,
            hint: 'Look in the home directory',
            penalty: 0,
            totalPenalty: 0,
          },
        }
        vi.mocked(api.post).mockResolvedValue(mockResponse)

        const result = await sessionsApi.getCheckpointHint('session-123', 'cp-1')

        expect(api.post).toHaveBeenCalledWith('/sessions/session-123/checkpoints/cp-1/hint')
        expect(result.checkpointId).toBe('cp-1')
        expect(result.level).toBe(1)
        expect(result.hint).toBe('Look in the home directory')
      })

      it('calls the correct API endpoint with level parameter', async () => {
        const mockResponse = {
          data: {
            checkpointId: 'cp-1',
            level: 2,
            maxLevel: 3,
            hint: 'Check the ~/.config directory',
            penalty: 2,
            totalPenalty: 2,
          },
        }
        vi.mocked(api.post).mockResolvedValue(mockResponse)

        const result = await sessionsApi.getCheckpointHint('session-123', 'cp-1', 2)

        expect(api.post).toHaveBeenCalledWith('/sessions/session-123/checkpoints/cp-1/hint?level=2')
        expect(result.level).toBe(2)
        expect(result.penalty).toBe(2)
      })

      it('handles hint response with penalty', async () => {
        const mockResponse = {
          data: {
            checkpointId: 'cp-2',
            level: 3,
            maxLevel: 3,
            hint: 'Run: touch ~/.config',
            penalty: 5,
            totalPenalty: 10,
          },
        }
        vi.mocked(api.post).mockResolvedValue(mockResponse)

        const result = await sessionsApi.getCheckpointHint('session-456', 'cp-2')

        expect(result.penalty).toBe(5)
        expect(result.totalPenalty).toBe(10)
        expect(result.maxLevel).toBe(3)
      })

      it('handles API error', async () => {
        vi.mocked(api.post).mockRejectedValue(new Error('Checkpoint not found'))

        await expect(
          sessionsApi.getCheckpointHint('invalid-session', 'invalid-cp')
        ).rejects.toThrow('Checkpoint not found')
      })

      it('handles no hints available response', async () => {
        const mockResponse = {
          data: {
            checkpointId: 'cp-no-hints',
            level: 0,
            maxLevel: 0,
            hint: 'No hints available for this checkpoint',
            penalty: 0,
            totalPenalty: 0,
          },
        }
        vi.mocked(api.post).mockResolvedValue(mockResponse)

        const result = await sessionsApi.getCheckpointHint('session-789', 'cp-no-hints')

        expect(result.level).toBe(0)
        expect(result.maxLevel).toBe(0)
        expect(result.hint).toContain('No hints')
      })
    })
  })
})
