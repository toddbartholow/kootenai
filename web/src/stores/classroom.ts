import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import {
  classroomApi,
  type ClassroomSimulation,
  type ClassroomFeedback,
  type FeedbackSummary,
  type CreateSimulationRequest,
  type StudentActivity,
} from '@/api/domains/classroom'
import { createErrorState, type ErrorState } from '@/types/errors'

export const useClassroomStore = defineStore('classroom', () => {
  const simulations = ref<ClassroomSimulation[]>([])
  const currentSimulation = ref<ClassroomSimulation | null>(null)
  const activities = ref<StudentActivity[]>([])
  const activitiesTotal = ref(0)
  const feedback = ref<ClassroomFeedback[]>([])
  const feedbackTotal = ref(0)
  const feedbackSummary = ref<FeedbackSummary | null>(null)
  const loading = ref(false)
  const error = ref<ErrorState | null>(null)

  const hasSimulations = computed(() => simulations.value.length > 0)
  const isRunning = computed(() => currentSimulation.value?.status === 'running')

  async function fetchSimulations() {
    loading.value = true
    error.value = null
    try {
      simulations.value = await classroomApi.list()
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to fetch simulations')
    } finally {
      loading.value = false
    }
  }

  async function fetchSimulation(id: string) {
    loading.value = true
    error.value = null
    try {
      currentSimulation.value = await classroomApi.get(id)
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to fetch simulation')
    } finally {
      loading.value = false
    }
  }

  async function createSimulation(req: CreateSimulationRequest) {
    loading.value = true
    error.value = null
    try {
      const sim = await classroomApi.create(req)
      simulations.value.unshift(sim)
      currentSimulation.value = sim
      return sim
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to create simulation')
      return null
    } finally {
      loading.value = false
    }
  }

  async function startSimulation(id: string) {
    try {
      await classroomApi.start(id)
      if (currentSimulation.value?.id === id) {
        currentSimulation.value.status = 'running'
      }
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to start simulation')
      throw e
    }
  }

  async function pauseSimulation(id: string) {
    try {
      await classroomApi.pause(id)
      if (currentSimulation.value?.id === id) {
        currentSimulation.value.status = 'paused'
      }
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to pause simulation')
      throw e
    }
  }

  async function stopSimulation(id: string) {
    try {
      await classroomApi.stop(id)
      if (currentSimulation.value?.id === id) {
        currentSimulation.value.status = 'completed'
      }
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to stop simulation')
      throw e
    }
  }

  async function deleteSimulation(id: string) {
    try {
      await classroomApi.delete(id)
      simulations.value = simulations.value.filter((s) => s.id !== id)
      if (currentSimulation.value?.id === id) {
        currentSimulation.value = null
      }
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to delete simulation')
      throw e
    }
  }

  async function fetchActivities(simulationId: string, limit = 50, offset = 0) {
    try {
      const result = await classroomApi.listActivities(simulationId, limit, offset)
      activities.value = result.activities || []
      activitiesTotal.value = result.total
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to fetch activities')
    }
  }

  async function fetchFeedback(
    simulationId: string,
    opts: { type?: string | undefined; labId?: string | undefined; limit?: number | undefined; offset?: number | undefined } = {},
  ) {
    try {
      const result = await classroomApi.listFeedback(simulationId, opts)
      feedback.value = result.feedback || []
      feedbackTotal.value = result.total
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to fetch feedback')
    }
  }

  async function fetchFeedbackSummary(simulationId: string) {
    try {
      feedbackSummary.value = await classroomApi.getFeedbackSummary(simulationId)
    } catch (e: unknown) {
      error.value = createErrorState(e, 'Failed to fetch feedback summary')
    }
  }

  /**
   * Resets the store state to initial values
   */
  function reset(): void {
    simulations.value = []
    currentSimulation.value = null
    activities.value = []
    activitiesTotal.value = 0
    feedback.value = []
    feedbackTotal.value = 0
    feedbackSummary.value = null
    loading.value = false
    error.value = null
  }

  return {
    simulations,
    currentSimulation,
    activities,
    activitiesTotal,
    loading,
    error,
    hasSimulations,
    isRunning,
    fetchSimulations,
    fetchSimulation,
    createSimulation,
    startSimulation,
    pauseSimulation,
    stopSimulation,
    deleteSimulation,
    fetchActivities,
    feedback,
    feedbackTotal,
    feedbackSummary,
    fetchFeedback,
    fetchFeedbackSummary,
    reset,
  }
})
