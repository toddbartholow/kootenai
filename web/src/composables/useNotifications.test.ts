import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useNotifications } from './useNotifications'

// Mock PrimeVue's useToast
const mockToastAdd = vi.fn()
const mockToastRemoveGroup = vi.fn()

vi.mock('primevue/usetoast', () => ({
  useToast: vi.fn(() => ({
    add: mockToastAdd,
    removeGroup: mockToastRemoveGroup,
  })),
}))

describe('useNotifications', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('success()', () => {
    it('should show success notification with string message', () => {
      const { success } = useNotifications()

      success('Operation completed')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Success',
        detail: 'Operation completed',
        life: 3000,
        closable: true,
      })
    })

    it('should show success notification with options object', () => {
      const { success } = useNotifications()

      success({ title: 'Custom Title', message: 'Custom message', duration: 5000 })

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Custom Title',
        detail: 'Custom message',
        life: 5000,
        closable: true,
      })
    })

    it('should respect closable option', () => {
      const { success } = useNotifications()

      success({ message: 'Test', closable: false })

      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({ closable: false })
      )
    })
  })

  describe('error()', () => {
    it('should show error notification with string message', () => {
      const { error } = useNotifications()

      error('Something went wrong')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'error',
        summary: 'Error',
        detail: 'Something went wrong',
        life: 5000,
        closable: true,
      })
    })

    it('should show error notification with options object', () => {
      const { error } = useNotifications()

      error({ title: 'API Error', message: 'Failed to fetch data', duration: 10000 })

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'error',
        summary: 'API Error',
        detail: 'Failed to fetch data',
        life: 10000,
        closable: true,
      })
    })
  })

  describe('warn()', () => {
    it('should show warning notification with string message', () => {
      const { warn } = useNotifications()

      warn('Be careful')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'warn',
        summary: 'Warning',
        detail: 'Be careful',
        life: 4000,
        closable: true,
      })
    })

    it('should show warning notification with custom title', () => {
      const { warn } = useNotifications()

      warn({ title: 'Caution', message: 'Check your input' })

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'warn',
        summary: 'Caution',
        detail: 'Check your input',
        life: 4000,
        closable: true,
      })
    })
  })

  describe('info()', () => {
    it('should show info notification with string message', () => {
      const { info } = useNotifications()

      info('For your information')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'info',
        summary: 'Info',
        detail: 'For your information',
        life: 3000,
        closable: true,
      })
    })

    it('should show info notification with options', () => {
      const { info } = useNotifications()

      info({ title: 'Notice', message: 'Please note', duration: 2000 })

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'info',
        summary: 'Notice',
        detail: 'Please note',
        life: 2000,
        closable: true,
      })
    })
  })

  describe('promise()', () => {
    it('should show loading toast and success on resolve', async () => {
      const { promise } = useNotifications()
      const testPromise = Promise.resolve('result')

      await promise(testPromise, {
        loading: 'Loading...',
        success: 'Loaded!',
      })

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'info',
        summary: 'Loading',
        detail: 'Loading...',
        life: 60000,
        group: 'promise-loading',
      })
      expect(mockToastRemoveGroup).toHaveBeenCalledWith('promise-loading')
      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Success',
        detail: 'Loaded!',
        life: 3000,
        closable: true,
      })
    })

    it('should use success function for dynamic message', async () => {
      const { promise } = useNotifications()
      const testPromise = Promise.resolve({ count: 5 })

      await promise(testPromise, {
        success: (result) => `Loaded ${result.count} items`,
      })

      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({ detail: 'Loaded 5 items' })
      )
    })

    it('should show error on reject', async () => {
      const { promise } = useNotifications()
      const testPromise = Promise.reject(new Error('Network error'))

      await expect(
        promise(testPromise, {
          loading: 'Loading...',
          error: 'Failed to load',
        })
      ).rejects.toThrow('Network error')

      expect(mockToastRemoveGroup).toHaveBeenCalledWith('promise-loading')
      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({
          severity: 'error',
          detail: 'Failed to load',
        })
      )
    })

    it('should use error function for dynamic message', async () => {
      const { promise } = useNotifications()
      const testPromise = Promise.reject(new Error('Custom error'))

      await expect(
        promise(testPromise, {
          error: (err) => `Error: ${err.message}`,
        })
      ).rejects.toThrow('Custom error')

      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({ detail: 'Error: Custom error' })
      )
    })

    it('should use error message if no error handler provided', async () => {
      const { promise } = useNotifications()
      const testPromise = Promise.reject(new Error('Fallback error'))

      await expect(promise(testPromise, {})).rejects.toThrow('Fallback error')

      expect(mockToastAdd).toHaveBeenCalledWith(
        expect.objectContaining({ detail: 'Fallback error' })
      )
    })

    it('should not show loading toast if not provided', async () => {
      const { promise } = useNotifications()
      const testPromise = Promise.resolve('result')

      await promise(testPromise, {
        success: 'Done',
      })

      expect(mockToastAdd).not.toHaveBeenCalledWith(
        expect.objectContaining({ group: 'promise-loading' })
      )
    })
  })

  describe('lab patterns', () => {
    it('should show session started notification', () => {
      const { lab } = useNotifications()

      lab.sessionStarted()

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Session Started',
        detail: 'Your lab session is now active',
        life: 3000,
        closable: true,
      })
    })

    it('should show session ended notification', () => {
      const { lab } = useNotifications()

      lab.sessionEnded()

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'info',
        summary: 'Session Ended',
        detail: 'Your lab session has been terminated',
        life: 3000,
        closable: true,
      })
    })

    it('should show session submitted notification', () => {
      const { lab } = useNotifications()

      lab.sessionSubmitted()

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Lab Submitted',
        detail: 'Your lab has been submitted for grading',
        life: 3000,
        closable: true,
      })
    })

    it('should show checkpoint passed notification', () => {
      const { lab } = useNotifications()

      lab.checkpointPassed('Configure Router')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Checkpoint Passed',
        detail: 'You completed: Configure Router',
        life: 3000,
        closable: true,
      })
    })

    it('should show checkpoint failed notification', () => {
      const { lab } = useNotifications()

      lab.checkpointFailed('Setup VLAN')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'warn',
        summary: 'Checkpoint Failed',
        detail: 'Check your work: Setup VLAN',
        life: 4000,
        closable: true,
      })
    })

    it('should show connection lost notification', () => {
      const { lab } = useNotifications()

      lab.connectionLost()

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'error',
        summary: 'Connection Lost',
        detail: 'Real-time updates paused. Attempting to reconnect...',
        life: 5000,
        closable: true,
      })
    })

    it('should show connection restored notification', () => {
      const { lab } = useNotifications()

      lab.connectionRestored()

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Connected',
        detail: 'Real-time updates restored',
        life: 3000,
        closable: true,
      })
    })
  })

  describe('vm patterns', () => {
    it('should show VM starting notification', () => {
      const { vm } = useNotifications()

      vm.starting('webserver-01')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'info',
        summary: 'Starting VM',
        detail: 'Starting webserver-01...',
        life: 3000,
        closable: true,
      })
    })

    it('should show VM started notification', () => {
      const { vm } = useNotifications()

      vm.started('webserver-01')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'VM Started',
        detail: 'webserver-01 is now running',
        life: 3000,
        closable: true,
      })
    })

    it('should show VM stopping notification', () => {
      const { vm } = useNotifications()

      vm.stopping('database-01')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'info',
        summary: 'Stopping VM',
        detail: 'Stopping database-01...',
        life: 3000,
        closable: true,
      })
    })

    it('should show VM stopped notification', () => {
      const { vm } = useNotifications()

      vm.stopped('database-01')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'info',
        summary: 'VM Stopped',
        detail: 'database-01 has been stopped',
        life: 3000,
        closable: true,
      })
    })

    it('should show console failed notification', () => {
      const { vm } = useNotifications()

      vm.consoleFailed('router-01')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'error',
        summary: 'Console Error',
        detail: 'Failed to connect to router-01 console',
        life: 5000,
        closable: true,
      })
    })

    it('should show snapshot reverted notification', () => {
      const { vm } = useNotifications()

      vm.snapshotReverted('router-01', 'initial-config')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Snapshot Restored',
        detail: 'router-01 reverted to "initial-config"',
        life: 3000,
        closable: true,
      })
    })
  })

  describe('achievement patterns', () => {
    it('should show achievement unlocked notification', () => {
      const { achievement } = useNotifications()

      achievement.unlocked('First Lab Complete', 50)

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Achievement Unlocked!',
        detail: 'First Lab Complete (+50 points)',
        life: 5000,
        closable: true,
      })
    })
  })

  describe('pathway patterns', () => {
    it('should show module unlocked notification', () => {
      const { pathway } = useNotifications()

      pathway.moduleUnlocked('Advanced Networking')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Module Unlocked!',
        detail: 'Advanced Networking is now available',
        life: 4000,
        closable: true,
      })
    })

    it('should show enrolled notification', () => {
      const { pathway } = useNotifications()

      pathway.enrolled('Network Security')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Enrolled!',
        detail: "You've enrolled in Network Security",
        life: 3000,
        closable: true,
      })
    })

    it('should show pathway completed notification', () => {
      const { pathway } = useNotifications()

      pathway.completed('Network Administration')

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Pathway Completed!',
        detail: 'Congratulations on completing Network Administration!',
        life: 5000,
        closable: true,
      })
    })
  })

  describe('data patterns', () => {
    it('should show saved notification', () => {
      const { data } = useNotifications()

      data.saved()

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Success',
        detail: 'Changes saved successfully',
        life: 3000,
        closable: true,
      })
    })

    it('should show save failed notification', () => {
      const { data } = useNotifications()

      data.saveFailed()

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'error',
        summary: 'Error',
        detail: 'Failed to save changes',
        life: 5000,
        closable: true,
      })
    })

    it('should show loaded notification', () => {
      const { data } = useNotifications()

      data.loaded()

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'info',
        summary: 'Info',
        detail: 'Data refreshed',
        life: 3000,
        closable: true,
      })
    })

    it('should show load failed notification', () => {
      const { data } = useNotifications()

      data.loadFailed()

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'error',
        summary: 'Error',
        detail: 'Failed to load data',
        life: 5000,
        closable: true,
      })
    })

    it('should show copied notification', () => {
      const { data } = useNotifications()

      data.copied()

      expect(mockToastAdd).toHaveBeenCalledWith({
        severity: 'success',
        summary: 'Copied',
        detail: 'Copied to clipboard',
        life: 2000,
        closable: true,
      })
    })
  })

  describe('toast access', () => {
    it('should expose underlying toast instance', () => {
      const { toast } = useNotifications()

      expect(toast.add).toBe(mockToastAdd)
      expect(toast.removeGroup).toBe(mockToastRemoveGroup)
    })
  })
})
