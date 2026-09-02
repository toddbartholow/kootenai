/**
 * Mock data for sessions API
 * Loaded dynamically only when USE_MOCK_DATA is true
 */
import type { Session, CheckpointHintResponse } from './sessions'

// Mock checkpoint hints for progressive hint system
export const mockCheckpointHints: Record<string, { level: number; text: string; penalty: number }[]> = {
  'cp-1': [
    { level: 1, text: 'Check the home directory for configuration files', penalty: 0 },
    { level: 2, text: 'Use the ls -la command to see hidden files', penalty: 1 },
    { level: 3, text: 'Look for a .config file in the user home directory', penalty: 2 },
  ],
  'cp-2': [
    { level: 1, text: 'Review the firewall rules configuration', penalty: 0 },
    { level: 2, text: 'Try using iptables or nftables to list rules', penalty: 2 },
  ],
  'cp-3': [
    { level: 1, text: 'Investigate network services running on the system', penalty: 0 },
    { level: 2, text: 'Use netstat or ss to list listening ports', penalty: 1 },
    { level: 3, text: 'Check ports 80, 443, and 22', penalty: 2 },
  ],
}

// Track mock hint progress (in memory)
export const mockCheckpointHintProgress: Record<string, { level: number; totalPenalty: number }> = {}

export const mockSessions: Session[] = [
  {
    id: 'session-001',
    podId: 'pod-abc123',
    userId: 'demo@example.com',
    labTemplateId: 'lab-firewall-101',
    status: 'active',
    earnedPoints: 650,
    maxPoints: 1000,
    percentage: 65,
    passed: false,
    startedAt: new Date(Date.now() - 45 * 60 * 1000).toISOString(), // 45 minutes ago
  },
  {
    id: 'session-002',
    podId: 'pod-def456',
    userId: 'demo@example.com',
    labTemplateId: 'lab-ids-snort',
    status: 'active',
    earnedPoints: 400,
    maxPoints: 1500,
    percentage: 27,
    passed: false,
    startedAt: new Date(Date.now() - 25 * 60 * 1000).toISOString(), // 25 minutes ago
  },
  {
    id: 'session-003',
    podId: 'pod-old-001',
    userId: 'demo@example.com',
    labTemplateId: 'lab-network-fundamentals',
    status: 'completed',
    earnedPoints: 800,
    maxPoints: 800,
    percentage: 100,
    passed: true,
    startedAt: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000).toISOString(), // 3 days ago
    endedAt: new Date(Date.now() - 3 * 24 * 60 * 60 * 1000 + 28 * 60 * 1000).toISOString(),
  },
  {
    id: 'session-004',
    podId: 'pod-old-002',
    userId: 'demo@example.com',
    labTemplateId: 'lab-firewall-101',
    status: 'completed',
    earnedPoints: 920,
    maxPoints: 1000,
    percentage: 92,
    passed: true,
    startedAt: new Date(Date.now() - 5 * 24 * 60 * 60 * 1000).toISOString(), // 5 days ago
    endedAt: new Date(Date.now() - 5 * 24 * 60 * 60 * 1000 + 52 * 60 * 1000).toISOString(),
  },
  {
    id: 'session-005',
    podId: 'pod-old-003',
    userId: 'demo@example.com',
    labTemplateId: 'lab-vpn-config',
    status: 'completed',
    earnedPoints: 980,
    maxPoints: 1500,
    percentage: 65,
    passed: false,
    startedAt: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString(), // 7 days ago
    endedAt: new Date(Date.now() - 7 * 24 * 60 * 60 * 1000 + 90 * 60 * 1000).toISOString(),
  },
  {
    id: 'session-006',
    podId: 'pod-old-004',
    userId: 'demo@example.com',
    labTemplateId: 'lab-siem-basics',
    status: 'completed',
    earnedPoints: 1350,
    maxPoints: 1500,
    percentage: 90,
    passed: true,
    startedAt: new Date(Date.now() - 10 * 24 * 60 * 60 * 1000).toISOString(), // 10 days ago
    endedAt: new Date(Date.now() - 10 * 24 * 60 * 60 * 1000 + 85 * 60 * 1000).toISOString(),
  },
]

/**
 * Resolves a mock checkpoint hint for the given session/checkpoint
 */
export function resolveMockCheckpointHint(
  sessionId: string,
  checkpointId: string,
  level?: number,
): CheckpointHintResponse {
  const hints = mockCheckpointHints[checkpointId] || []
  const progressKey = `${sessionId}-${checkpointId}`

  if (hints.length === 0) {
    return {
      checkpointId,
      level: 0,
      maxLevel: 0,
      hint: 'No hint available',
      penalty: 0,
      totalPenalty: 0,
    }
  }

  // Get current progress
  const progress = mockCheckpointHintProgress[progressKey] || { level: 0, totalPenalty: 0 }
  const requestedLevel = level ?? (progress.level + 1)
  const maxLevel = hints.length

  // If already shown this level, return without penalty
  if (requestedLevel <= progress.level) {
    const hint = hints.find(h => h.level === requestedLevel) || hints[requestedLevel - 1]
    return {
      checkpointId,
      level: requestedLevel,
      maxLevel,
      hint: hint?.text || 'No hint available',
      penalty: 0,
      totalPenalty: progress.totalPenalty,
    }
  }

  // Get new hint and apply penalty
  const hint = hints.find(h => h.level === requestedLevel) || hints[requestedLevel - 1]
  if (hint) {
    mockCheckpointHintProgress[progressKey] = {
      level: requestedLevel,
      totalPenalty: progress.totalPenalty + hint.penalty,
    }
  }

  return {
    checkpointId,
    level: requestedLevel,
    maxLevel,
    hint: hint?.text || 'No more hints available',
    penalty: hint?.penalty || 0,
    totalPenalty: (progress.totalPenalty || 0) + (hint?.penalty || 0),
  }
}
