/**
 * Shared status utility functions for consistent styling across views.
 * Centralizes status-to-color/severity mappings to eliminate duplication.
 */
import { i18n } from '@/locales'

// PrimeVue Tag severity type
export type Severity = 'success' | 'info' | 'warn' | 'danger' | 'secondary' | 'contrast'

/**
 * Get severity for general status values (pods, sessions, etc.)
 * Used with PrimeVue Tag component
 */
export function getStatusSeverity(status: string): Severity | undefined {
  switch (status?.toLowerCase()) {
    // Success states
    case 'running':
    case 'active':
    case 'healthy':
    case 'connected':
    case 'ready':
    case 'ok':
    case 'passed':
      return 'success'

    // Info states
    case 'completed':
    case 'in_progress':
      return 'info'

    // Warning states
    case 'paused':
    case 'suspended':
    case 'provisioning':
    case 'pending':
    case 'starting':
    case 'degraded':
    case 'partial':
      return 'warn'

    // Danger states
    case 'error':
    case 'failed':
    case 'unhealthy':
    case 'disconnected':
      return 'danger'

    // Secondary/neutral states
    case 'stopped':
    case 'ended':
    case 'expired':
    case 'enrolled':
      return 'secondary'

    // Contrast states
    case 'graded':
      return 'contrast'

    default:
      return 'secondary'
  }
}

/**
 * Get severity for checkpoint status
 * Used with PrimeVue Tag component
 */
export function getCheckpointSeverity(status: string): Severity | undefined {
  switch (status?.toLowerCase()) {
    case 'passed':
      return 'success'
    case 'failed':
      return 'danger'
    case 'pending':
    default:
      return 'secondary'
  }
}

/**
 * Get Tailwind CSS color class for VM status indicator
 * Used for status dots/indicators
 */
export function getVMStatusColor(status: string): string {
  switch (status?.toLowerCase()) {
    case 'running':
      return 'text-green-500'
    case 'stopped':
      return 'text-red-500'
    case 'paused':
    case 'suspended':
      return 'text-yellow-500'
    default:
      return 'text-surface-400'
  }
}

/**
 * Get severity for difficulty level
 * Used with PrimeVue Tag component
 */
export function getDifficultySeverity(difficulty: string | undefined): Severity {
  switch (difficulty?.toLowerCase()) {
    case 'beginner':
      return 'success'
    case 'intermediate':
      return 'info'
    case 'advanced':
      return 'warn'
    case 'expert':
      return 'danger'
    default:
      return 'secondary'
  }
}

/**
 * Get Tailwind CSS text color class for difficulty
 * Used for inline text styling
 */
export function getDifficultyColor(difficulty: string | undefined): string {
  switch (difficulty?.toLowerCase()) {
    case 'beginner':
      return 'text-green-600 dark:text-green-400'
    case 'intermediate':
      return 'text-yellow-600 dark:text-yellow-400'
    case 'advanced':
      return 'text-orange-600 dark:text-orange-400'
    case 'expert':
      return 'text-red-600 dark:text-red-400'
    default:
      return 'text-surface-600 dark:text-surface-400'
  }
}

/**
 * Get Tailwind CSS background + text color class for difficulty badge
 * Used for badge/pill styling
 */
export function getDifficultyBadgeColor(difficulty: string | undefined): string {
  switch (difficulty?.toLowerCase()) {
    case 'beginner':
      return 'bg-green-100 text-green-700 dark:bg-green-900/50 dark:text-green-300'
    case 'intermediate':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300'
    case 'advanced':
      return 'bg-red-100 text-red-700 dark:bg-red-900/50 dark:text-red-300'
    case 'expert':
      return 'bg-purple-100 text-purple-700 dark:bg-purple-900/50 dark:text-purple-300'
    default:
      return 'bg-surface-100 text-surface-700 dark:bg-surface-700 dark:text-surface-300'
  }
}

/**
 * Get human-readable, localized label for a difficulty level.
 * Unrecognised values are returned verbatim (or a localized "Unknown").
 */
export function getDifficultyLabel(difficulty: string | undefined): string {
  const key = difficulty?.toLowerCase()
  switch (key) {
    case 'beginner':
    case 'intermediate':
    case 'advanced':
    case 'expert':
      return i18n.global.t(`difficulty.${key}`)
    default:
      return difficulty || i18n.global.t('common.unknown')
  }
}
