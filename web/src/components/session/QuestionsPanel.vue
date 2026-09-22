<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useQuestionsStore } from '@/stores/questions'
import type { QuestionProgress } from '@/api'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Checkbox from 'primevue/checkbox'
import RadioButton from 'primevue/radiobutton'
import Message from '@volt/Message.vue'
import ProgressBar from '@volt/ProgressBar.vue'

const { t } = useI18n()

const props = defineProps<{
  sessionId: string
}>()

const store = useQuestionsStore()

// Track which question is expanded
const expandedQuestion = ref<string | null>(null)

// Track answer inputs for each question
const answerInputs = ref<Record<string, string | string[]>>({})

// Track shown hints by question and level
const shownHints = ref<Record<string, { level: number; text: string }[]>>({})

// Progress bar color based on percentage
const progressColor = computed(() => {
  const pct = store.percentage
  if (pct >= 90) return undefined // default green
  if (pct >= 70) return 'bg-yellow-500'
  if (pct >= 50) return 'bg-orange-500'
  return 'bg-red-500'
})

// Get status icon for a question
function getStatusIcon(question: QuestionProgress): string {
  switch (question.status) {
    case 'correct':
      return 'pi pi-check-circle'
    case 'incorrect':
      return 'pi pi-times-circle'
    case 'partial':
      return 'pi pi-minus-circle'
    default:
      return question.isLocked ? 'pi pi-lock' : 'pi pi-circle'
  }
}

// Get status color for a question
function getStatusColor(question: QuestionProgress): string {
  switch (question.status) {
    case 'correct':
      return 'text-green-500'
    case 'incorrect':
      return 'text-red-500'
    case 'partial':
      return 'text-yellow-500'
    default:
      return question.isLocked ? 'text-surface-400' : 'text-surface-500'
  }
}

// Toggle question expansion
function toggleQuestion(questionId: string, isLocked: boolean) {
  if (isLocked) return
  expandedQuestion.value = expandedQuestion.value === questionId ? null : questionId
}

// Handle answer submission
async function handleSubmit(question: QuestionProgress) {
  const answer = answerInputs.value[question.id]
  if (!answer) return

  const payload = question.type === 'text'
    ? { responseText: answer as string }
    : { selectedOptions: Array.isArray(answer) ? answer : [answer] }

  const result = await store.submitAnswer(props.sessionId, question.id, payload)

  // If correct, collapse the question
  if (result?.isCorrect) {
    expandedQuestion.value = null
    // Clear the answer input
    delete answerInputs.value[question.id]
  }
}

// Handle showing next progressive hint
async function handleShowHint(question: QuestionProgress) {
  const response = await store.showHint(props.sessionId, question.id)
  if (response) {
    // Initialize array if needed
    if (!shownHints.value[question.id]) {
      shownHints.value[question.id] = []
    }
    const hints = shownHints.value[question.id]!
    // Add hint if not already present
    const existing = hints.find(h => h.level === response.level)
    if (!existing) {
      hints.push({
        level: response.level,
        text: response.hint
      })
      // Sort by level
      hints.sort((a, b) => a.level - b.level)
    }
  }
}

// Get the next hint level that would be revealed
function getNextHintLevel(question: QuestionProgress): number {
  return question.hintLevelShown + 1
}

// Check if more hints are available
function hasMoreHints(question: QuestionProgress): boolean {
  return question.hintAvailable && (question.hintCount ?? 0) > question.hintLevelShown
}

// Get hint penalty message for next hint
function _getHintPenaltyMessage(question: QuestionProgress): string | null {
  // Only show penalty warning if there are hints and more levels available
  if (!hasMoreHints(question)) return null
  // Penalty varies by hint level, but we don't know the specific penalty until we request it
  // For now, just indicate that a penalty may apply
  return 'May deduct points'
}

// Handle checkbox toggle for multi-select
function toggleOption(questionId: string, optionId: string, checked: boolean) {
  const current = (answerInputs.value[questionId] as string[]) || []
  if (checked) {
    answerInputs.value[questionId] = [...current, optionId]
  } else {
    answerInputs.value[questionId] = current.filter(id => id !== optionId)
  }
}

// Check if an option is selected
function isOptionSelected(questionId: string, optionId: string): boolean {
  const answer = answerInputs.value[questionId]
  if (Array.isArray(answer)) {
    return answer.includes(optionId)
  }
  return answer === optionId
}

// Fetch questions on mount
onMounted(() => {
  store.fetchQuestions(props.sessionId)
})

// Refetch if sessionId changes
watch(() => props.sessionId, (newId) => {
  if (newId) {
    store.fetchQuestions(newId)
  }
})
</script>

<template>
  <Card class="h-full flex flex-col">
    <template #title>
      <div class="flex items-center justify-between">
        <span class="text-lg font-semibold">{{ t('questions.cardTitle') }}</span>
        <span class="text-sm font-normal text-surface-500">
          {{ t('questions.answeredCount', { answered: store.answeredCount, total: store.totalCount }) }}
        </span>
      </div>
    </template>

    <template #content>
      <div class="flex flex-col gap-4 h-full">
        <!-- Loading state -->
        <div v-if="store.loading" class="flex items-center justify-center py-8">
          <i class="pi pi-spin pi-spinner text-2xl text-primary-500" />
        </div>

        <!-- No questions -->
        <div v-else-if="!store.hasQuestions" class="flex flex-col items-center justify-center py-8 text-surface-500">
          <i class="pi pi-inbox text-4xl mb-2" />
          <p>{{ t('questions.empty') }}</p>
        </div>

        <!-- Questions list -->
        <template v-else>
          <!-- Progress summary -->
          <div class="space-y-2">
            <div class="flex justify-between text-sm">
              <span class="text-surface-600 dark:text-surface-400">
                {{ t('questions.pointsSummary', { earned: store.earnedPoints, total: store.totalPoints }) }}
              </span>
              <span class="font-medium text-surface-900 dark:text-surface-100">
                {{ store.percentage }}%
              </span>
            </div>
            <ProgressBar :value="store.percentage" :class="progressColor" />
          </div>

          <!-- Error message -->
          <Message v-if="store.error" severity="error" :closable="true" @close="store.clearError">
            {{ store.error.message }}
          </Message>

          <!-- Questions -->
          <div class="space-y-3 flex-1 overflow-y-auto">
            <div
              v-for="question in store.questions"
              :key="question.id"
              :class="[
                'border rounded-lg overflow-hidden transition-all',
                question.status === 'correct'
                  ? 'border-green-300 dark:border-green-700 bg-green-50 dark:bg-green-900/20'
                  : question.isLocked
                    ? 'border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-800/50 opacity-60'
                    : 'border-surface-200 dark:border-surface-700 hover:border-primary-300 dark:hover:border-primary-600'
              ]"
            >
              <!-- Question Header -->
              <button
                @click="toggleQuestion(question.id, question.isLocked)"
                :disabled="question.isLocked"
                class="w-full p-4 text-left flex items-center justify-between gap-3"
                :class="{ 'cursor-not-allowed': question.isLocked }"
              >
                <div class="flex items-center gap-3 flex-1 min-w-0">
                  <i :class="[getStatusIcon(question), getStatusColor(question), 'text-lg shrink-0']" />
                  <span class="font-medium text-surface-900 dark:text-surface-100 truncate">
                    {{ question.description }}
                  </span>
                </div>
                <div class="flex items-center gap-2 shrink-0">
                  <span class="text-sm text-surface-500">
                    {{ t('assessment.tree.pointsFraction', { earned: question.earnedPoints, max: question.points }) }}
                  </span>
                  <i
                    v-if="!question.isLocked && question.status !== 'correct'"
                    :class="[
                      'pi text-surface-400 transition-transform',
                      expandedQuestion === question.id ? 'pi-chevron-up' : 'pi-chevron-down'
                    ]"
                  />
                </div>
              </button>

              <!-- Expanded Answer Form -->
              <div
                v-if="expandedQuestion === question.id && !question.isLocked && question.status !== 'correct'"
                class="p-4 border-t border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-800/30"
              >
                <!-- Text Input -->
                <div v-if="question.type === 'text'" class="space-y-3">
                  <InputText
                    v-model="answerInputs[question.id] as string"
                    :placeholder="t('questions.answerPlaceholder')"
                    class="w-full"
                    @keyup.enter="handleSubmit(question)"
                  />
                </div>

                <!-- Multiple Choice -->
                <div v-else-if="question.type === 'multiple_choice'" class="space-y-2">
                  <div
                    v-for="option in question.options"
                    :key="option.id"
                    class="flex items-center gap-3 p-2 rounded hover:bg-surface-100 dark:hover:bg-surface-700/50"
                  >
                    <!-- Multi-select with checkboxes -->
                    <Checkbox
                      v-if="question.multiSelect"
                      :modelValue="isOptionSelected(question.id, option.id)"
                      @update:modelValue="(val: boolean) => toggleOption(question.id, option.id, val)"
                      :binary="true"
                      :inputId="`${question.id}-${option.id}`"
                    />
                    <!-- Single-select with radio buttons -->
                    <RadioButton
                      v-else
                      v-model="answerInputs[question.id]"
                      :value="option.id"
                      :inputId="`${question.id}-${option.id}`"
                    />
                    <label
                      :for="`${question.id}-${option.id}`"
                      class="cursor-pointer text-surface-700 dark:text-surface-300"
                    >
                      {{ option.text }}
                    </label>
                  </div>
                </div>

                <!-- Progressive Hints -->
                <div v-if="question.hintCount && question.hintCount > 0" class="mt-3 space-y-2">
                  <!-- Display all revealed hints -->
                  <div
                    v-for="hint in (shownHints[question.id] || [])"
                    :key="hint.level"
                    class="p-3 bg-yellow-50 dark:bg-yellow-900/20 border border-yellow-200 dark:border-yellow-800 rounded-md"
                  >
                    <div class="flex items-start gap-2">
                      <i class="pi pi-lightbulb text-yellow-500 mt-0.5 shrink-0" />
                      <div class="flex-1">
                        <span class="text-xs font-medium text-yellow-600 dark:text-yellow-400">
                          {{ t('questions.hint.label', { level: hint.level, total: question.hintCount }) }}
                        </span>
                        <p class="text-sm text-yellow-800 dark:text-yellow-200 mt-1">
                          {{ hint.text }}
                        </p>
                      </div>
                    </div>
                  </div>

                  <!-- Hint penalty warning -->
                  <div
                    v-if="question.hintPenaltyApplied > 0"
                    class="flex items-center gap-2 text-xs text-orange-600 dark:text-orange-400"
                  >
                    <i class="pi pi-info-circle" />
                    <span>{{ t('questions.hint.pointsDeducted', question.hintPenaltyApplied) }}</span>
                  </div>

                  <!-- Get Next Hint button -->
                  <Button
                    v-if="hasMoreHints(question)"
                    @click="handleShowHint(question)"
                    :label="t('questions.hint.getNext', { level: getNextHintLevel(question), total: question.hintCount })"
                    icon="pi pi-lightbulb"
                    severity="secondary"
                    size="small"
                    text
                    class="mt-2"
                  />
                  <span
                    v-else-if="question.hintLevelShown > 0"
                    class="text-xs text-surface-500 italic"
                  >
                    {{ t('questions.hint.allRevealed') }}
                  </span>
                </div>

                <!-- Attempt count -->
                <div v-if="question.attemptCount > 0" class="mt-2 text-sm text-surface-500">
                  {{ t('questions.attempts', { count: question.attemptCount }) }}
                </div>

                <!-- Submit Button -->
                <div class="mt-4 flex justify-end">
                  <Button
                    @click="handleSubmit(question)"
                    :loading="store.submitting === question.id"
                    :disabled="!answerInputs[question.id] || (Array.isArray(answerInputs[question.id]) && (answerInputs[question.id] as string[]).length === 0)"
                    :label="t('questions.submit')"
                    icon="pi pi-check"
                  />
                </div>
              </div>
            </div>
          </div>
        </template>
      </div>
    </template>
  </Card>
</template>
