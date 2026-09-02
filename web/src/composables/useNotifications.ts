import { useToast } from 'primevue/usetoast'

export type NotificationType = 'success' | 'error' | 'warn' | 'info'

export interface NotificationOptions {
  title?: string
  message: string
  duration?: number
  closable?: boolean
}

/**
 * Centralized notification composable for consistent toast messages
 * across the application. Wraps PrimeVue's useToast with common patterns.
 */
export function useNotifications() {
  const toast = useToast()

  /**
   * Show a success notification
   */
  function success(options: NotificationOptions | string) {
    const opts = typeof options === 'string' ? { message: options } : options
    toast.add({
      severity: 'success',
      summary: opts.title || 'Success',
      detail: opts.message,
      life: opts.duration ?? 3000,
      closable: opts.closable ?? true,
    })
  }

  /**
   * Show an error notification
   */
  function error(options: NotificationOptions | string) {
    const opts = typeof options === 'string' ? { message: options } : options
    toast.add({
      severity: 'error',
      summary: opts.title || 'Error',
      detail: opts.message,
      life: opts.duration ?? 5000,
      closable: opts.closable ?? true,
    })
  }

  /**
   * Show a warning notification
   */
  function warn(options: NotificationOptions | string) {
    const opts = typeof options === 'string' ? { message: options } : options
    toast.add({
      severity: 'warn',
      summary: opts.title || 'Warning',
      detail: opts.message,
      life: opts.duration ?? 4000,
      closable: opts.closable ?? true,
    })
  }

  /**
   * Show an info notification
   */
  function info(options: NotificationOptions | string) {
    const opts = typeof options === 'string' ? { message: options } : options
    toast.add({
      severity: 'info',
      summary: opts.title || 'Info',
      detail: opts.message,
      life: opts.duration ?? 3000,
      closable: opts.closable ?? true,
    })
  }

  /**
   * Show a notification for a promise result
   */
  async function promise<T>(
    promise: Promise<T>,
    messages: {
      loading?: string
      success?: string | ((result: T) => string)
      error?: string | ((err: Error) => string)
    }
  ): Promise<T> {
    // Show loading toast if provided
    if (messages.loading) {
      toast.add({
        severity: 'info',
        summary: 'Loading',
        detail: messages.loading,
        life: 60000, // Long life, will be cleared on resolve/reject
        group: 'promise-loading',
      })
    }

    try {
      const result = await promise

      // Clear loading toast
      toast.removeGroup('promise-loading')

      // Show success toast
      const successMsg = typeof messages.success === 'function'
        ? messages.success(result)
        : messages.success

      if (successMsg) {
        success(successMsg)
      }

      return result
    } catch (err) {
      // Clear loading toast
      toast.removeGroup('promise-loading')

      // Narrow unknowns properly. Throwing non-Error values (strings,
      // objects, rejected promises) would previously render "undefined" in
      // the toast because `(err as Error).message` returned undefined.
      const errAsError =
        err instanceof Error
          ? err
          : new Error(typeof err === 'string' ? err : 'Operation failed')

      const errorMsg =
        typeof messages.error === 'function'
          ? messages.error(errAsError)
          : messages.error || errAsError.message

      error(errorMsg)
      throw err
    }
  }

  /**
   * Common notification patterns for lab actions
   */
  const lab = {
    sessionStarted: () => success({ title: 'Session Started', message: 'Your lab session is now active' }),
    sessionEnded: () => info({ title: 'Session Ended', message: 'Your lab session has been terminated' }),
    sessionSubmitted: () => success({ title: 'Lab Submitted', message: 'Your lab has been submitted for grading' }),
    checkpointPassed: (name: string) => success({ title: 'Checkpoint Passed', message: `You completed: ${name}` }),
    checkpointFailed: (name: string) => warn({ title: 'Checkpoint Failed', message: `Check your work: ${name}` }),
    connectionLost: () => error({ title: 'Connection Lost', message: 'Real-time updates paused. Attempting to reconnect...' }),
    connectionRestored: () => success({ title: 'Connected', message: 'Real-time updates restored' }),
  }

  /**
   * Common notification patterns for VM/Pod actions
   */
  const vm = {
    starting: (name: string) => info({ title: 'Starting VM', message: `Starting ${name}...` }),
    started: (name: string) => success({ title: 'VM Started', message: `${name} is now running` }),
    stopping: (name: string) => info({ title: 'Stopping VM', message: `Stopping ${name}...` }),
    stopped: (name: string) => info({ title: 'VM Stopped', message: `${name} has been stopped` }),
    consoleFailed: (name: string) => error({ title: 'Console Error', message: `Failed to connect to ${name} console` }),
    snapshotReverted: (vm: string, snapshot: string) => success({
      title: 'Snapshot Restored',
      message: `${vm} reverted to "${snapshot}"`
    }),
  }

  /**
   * Common notification patterns for achievements
   */
  const achievement = {
    unlocked: (name: string, points: number) => success({
      title: 'Achievement Unlocked!',
      message: `${name} (+${points} points)`,
      duration: 5000,
    }),
  }

  /**
   * Common notification patterns for pathway/module actions
   */
  const pathway = {
    moduleUnlocked: (name: string) => success({
      title: 'Module Unlocked!',
      message: `${name} is now available`,
      duration: 4000,
    }),
    enrolled: (name: string) => success({
      title: 'Enrolled!',
      message: `You've enrolled in ${name}`,
    }),
    completed: (name: string) => success({
      title: 'Pathway Completed!',
      message: `Congratulations on completing ${name}!`,
      duration: 5000,
    }),
    certificateIssued: (name: string) => success({
      title: 'Certificate Ready!',
      message: `Your certificate for ${name} is ready to view`,
      duration: 5000,
    }),
  }

  /**
   * Common notification patterns for data operations
   */
  const data = {
    saved: () => success('Changes saved successfully'),
    saveFailed: () => error('Failed to save changes'),
    loaded: () => info('Data refreshed'),
    loadFailed: () => error('Failed to load data'),
    copied: () => success({ title: 'Copied', message: 'Copied to clipboard', duration: 2000 }),
  }

  return {
    // Core methods
    success,
    error,
    warn,
    info,
    promise,

    // Domain-specific patterns
    lab,
    vm,
    achievement,
    pathway,
    data,

    // Access to underlying toast for advanced usage
    toast,
  }
}

export type UseNotificationsReturn = ReturnType<typeof useNotifications>
