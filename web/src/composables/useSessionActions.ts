import { ref, type Ref } from 'vue'
import type { useRouter } from 'vue-router'
import type { useConfirm as UseConfirm } from 'primevue/useconfirm'
import type { useToast as UseToast } from 'primevue/usetoast'
import type { useSessionStore as UseSessionStore } from '@/stores/session'
import type { UserAchievement } from '@/api'

export interface SessionActionOptions {
  sessionStore: ReturnType<typeof UseSessionStore>
  router: ReturnType<typeof useRouter>
  confirm: ReturnType<typeof UseConfirm>
  toast: ReturnType<typeof UseToast>
}

export interface UseSessionActionsReturn {
  showAchievementModal: Ref<boolean>
  unlockedAchievements: Ref<UserAchievement[]>
  handleEndSession: (sessionId: string) => void
  handleSubmitLab: (sessionId: string) => void
  closeAchievementModal: () => void
}

/**
 * Composable for session-related actions (end session, submit lab)
 */
export function useSessionActions(options: SessionActionOptions): UseSessionActionsReturn {
  const { sessionStore, router, confirm, toast } = options

  // Achievement modal state
  const showAchievementModal = ref(false)
  const unlockedAchievements = ref<UserAchievement[]>([])

  /**
   * Handles ending a session with confirmation dialog
   */
  function handleEndSession(sessionId: string): void {
    confirm.require({
      message: 'Are you sure you want to end this session? This will stop tracking your progress.',
      header: 'Confirm End Session',
      icon: 'pi pi-stop-circle',
      rejectProps: { label: 'Cancel', severity: 'secondary' },
      acceptProps: { label: 'End Session', severity: 'danger' },
      accept: async () => {
        const result = await sessionStore.endSession(sessionId)
        if (result) {
          router.push('/sessions')
        }
      }
    })
  }

  /**
   * Handles lab submission with confirmation dialog
   */
  function handleSubmitLab(sessionId: string): void {
    confirm.require({
      message: `Are you sure you want to submit your lab for grading? This action cannot be undone.

Your current progress:
- ${sessionStore.passedCheckpoints} of ${sessionStore.totalCheckpoints} checkpoints completed
- ${sessionStore.earnedPoints} of ${sessionStore.maxPoints} points earned (${sessionStore.percentage}%)`,
      header: 'Confirm Lab Submission',
      icon: 'pi pi-send',
      rejectProps: { label: 'Cancel', severity: 'secondary' },
      acceptProps: { label: 'Submit Lab', severity: 'success' },
      accept: async () => {
        await doSubmitLab(sessionId)
      }
    })
  }

  /**
   * Performs the actual lab submission
   */
  async function doSubmitLab(sessionId: string): Promise<void> {
    const result = await sessionStore.submitSession(sessionId)

    if (result) {
      // Show result toast
      toast.add({
        severity: result.passed ? 'success' : 'warn',
        summary: result.passed ? 'Lab Passed!' : 'Lab Not Passed',
        detail: `Score: ${result.percentage}% (${result.earnedPoints}/${result.maxPoints} points)`,
        life: 5000
      })

      // Show achievement modal if any were earned
      if (result.achievements && result.achievements.length > 0) {
        unlockedAchievements.value = result.achievements

        // Show toast notification for achievements
        const achievementCount = result.achievements.length
        const totalPoints = result.achievements.reduce(
          (sum, ua) => sum + (ua.achievement?.points || 0),
          0
        )
        toast.add({
          severity: 'success',
          summary: achievementCount === 1 ? 'Achievement Unlocked!' : `${achievementCount} Achievements Unlocked!`,
          detail: `You earned ${totalPoints} points!`,
          life: 4000
        })

        showAchievementModal.value = true
      }
    }
  }

  /**
   * Closes the achievement modal
   */
  function closeAchievementModal(): void {
    showAchievementModal.value = false
    unlockedAchievements.value = []
  }

  return {
    showAchievementModal,
    unlockedAchievements,
    handleEndSession,
    handleSubmitLab,
    closeAchievementModal,
  }
}
