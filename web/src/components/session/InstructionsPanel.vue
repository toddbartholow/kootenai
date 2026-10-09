<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useSessionStore } from '@/stores/session'
import { labTemplatesApi, type LabInstructions, type InstructionStep } from '@/api'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import ProgressBar from '@volt/ProgressBar.vue'

const { t } = useI18n()

const props = defineProps<{
  labTemplateId: string
  labTemplateName?: string
}>()

const emit = defineEmits<{
  close: []
}>()

const sessionStore = useSessionStore()

// State
const instructions = ref<LabInstructions | null>(null)
const loading = ref(true)
const error = ref<string | null>(null)
const currentStepIndex = ref(0)
const expandedSections = ref<Set<string>>(new Set(['overview']))

// Computed
const currentStep = computed<InstructionStep | null>(() => {
  if (!instructions.value?.steps) return null
  return instructions.value.steps[currentStepIndex.value] || null
})

const totalSteps = computed(() => instructions.value?.steps?.length || 0)

const progressPercentage = computed(() => {
  if (totalSteps.value === 0) return 0
  return Math.round(((currentStepIndex.value + 1) / totalSteps.value) * 100)
})

// Check if a step's objective has been completed
function isStepCompleted(step: InstructionStep): boolean {
  if (!step.objective_id) return false
  return sessionStore.checkpoints.some(cp => cp.id === step.objective_id && cp.status === 'passed')
}

// Get the current checkpoint status for a step
function getStepStatus(step: InstructionStep): 'completed' | 'current' | 'pending' {
  if (isStepCompleted(step)) return 'completed'
  const stepIndex = instructions.value?.steps?.findIndex(s => s.id === step.id) ?? -1
  if (stepIndex === currentStepIndex.value) return 'current'
  return 'pending'
}

// Navigation
function goToStep(index: number) {
  if (index >= 0 && index < totalSteps.value) {
    currentStepIndex.value = index
  }
}

function nextStep() {
  if (currentStepIndex.value < totalSteps.value - 1) {
    currentStepIndex.value++
  }
}

function prevStep() {
  if (currentStepIndex.value > 0) {
    currentStepIndex.value--
  }
}

// Section toggling
function toggleSection(section: string) {
  if (expandedSections.value.has(section)) {
    expandedSections.value.delete(section)
  } else {
    expandedSections.value.add(section)
  }
}

// Load instructions. Defensive: skip the fetch when labTemplateId is
// missing (can happen when this panel mounts before the session has
// hydrated). Previously an empty string would be sent to the API, which
// quietly 404'd.
async function loadInstructions() {
  if (!props.labTemplateId) {
    instructions.value = null
    loading.value = false
    return
  }
  try {
    loading.value = true
    error.value = null
    instructions.value = await labTemplatesApi.getInstructions(props.labTemplateId)
  } catch (err) {
    console.error('Failed to load instructions:', err)
    error.value = t('instructions.loadFailed')
  } finally {
    loading.value = false
  }
}

onMounted(loadInstructions)

watch(() => props.labTemplateId, loadInstructions)
</script>

<template>
  <div class="instructions-panel h-full flex flex-col">
    <!-- Header -->
    <div
      class="flex items-center justify-between p-4 border-b border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-800"
    >
      <div class="flex items-center gap-3">
        <i class="pi pi-book text-primary-500 text-xl" />
        <div>
          <h2 class="font-semibold text-surface-900 dark:text-surface-100">
            {{ t('instructions.headerTitle') }}
          </h2>
          <p class="text-sm text-surface-500">
            {{ labTemplateName || t('instructions.fallbackSubtitle') }}
          </p>
        </div>
      </div>
      <Button @click="emit('close')" icon="pi pi-times" text rounded severity="secondary" />
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="flex-1 flex items-center justify-center">
      <div class="text-center">
        <i class="pi pi-spin pi-spinner text-3xl text-primary-500 mb-3" />
        <p class="text-surface-500">{{ t('instructions.loading') }}</p>
      </div>
    </div>

    <!-- Error State -->
    <div v-else-if="error" class="flex-1 flex items-center justify-center p-4">
      <div class="text-center">
        <i class="pi pi-exclamation-circle text-4xl text-red-500 mb-3" />
        <p class="text-surface-700 dark:text-surface-300 mb-2">{{ error }}</p>
        <Button
          @click="loadInstructions"
          :label="t('instructions.tryAgain')"
          icon="pi pi-refresh"
          size="small"
        />
      </div>
    </div>

    <!-- Content -->
    <div v-else-if="instructions" class="flex-1 overflow-y-auto">
      <!-- Progress Bar -->
      <div class="p-4 border-b border-surface-200 dark:border-surface-700">
        <div class="flex items-center justify-between text-sm mb-2">
          <span class="text-surface-500">{{ t('instructions.progressLabel') }}</span>
          <span class="font-medium text-surface-700 dark:text-surface-300">
            {{
              t('instructions.stepOfTotal', { current: currentStepIndex + 1, total: totalSteps })
            }}
          </span>
        </div>
        <ProgressBar :value="progressPercentage" :showValue="false" class="h-2" />
      </div>

      <!-- Step Navigation Sidebar -->
      <div class="flex">
        <!-- Step List -->
        <div
          class="w-16 border-r border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-800"
        >
          <div class="py-2">
            <button
              v-for="(step, index) in instructions.steps"
              :key="step.id"
              @click="goToStep(index)"
              :class="[
                'w-full py-3 px-2 flex flex-col items-center gap-1 transition-all',
                'hover:bg-surface-100 dark:hover:bg-surface-700',
                index === currentStepIndex
                  ? 'bg-primary-50 dark:bg-primary-900/30 border-r-2 border-primary-500'
                  : '',
              ]"
            >
              <div
                :class="[
                  'w-8 h-8 rounded-full flex items-center justify-center text-sm font-medium',
                  getStepStatus(step) === 'completed'
                    ? 'bg-green-100 dark:bg-green-900/50 text-green-600'
                    : index === currentStepIndex
                      ? 'bg-primary-500 text-white'
                      : 'bg-surface-200 dark:bg-surface-600 text-surface-500',
                ]"
              >
                <i v-if="getStepStatus(step) === 'completed'" class="pi pi-check text-xs" />
                <span v-else>{{ index + 1 }}</span>
              </div>
            </button>
          </div>
        </div>

        <!-- Step Content -->
        <div class="flex-1 p-6 overflow-y-auto">
          <!-- Overview Section (collapsible) -->
          <div v-if="currentStepIndex === 0 && instructions.overview" class="mb-6">
            <button
              @click="toggleSection('overview')"
              class="w-full flex items-center justify-between p-3 bg-blue-50 dark:bg-blue-900/20 rounded-lg text-left"
            >
              <div class="flex items-center gap-2">
                <i class="pi pi-info-circle text-blue-500" />
                <span class="font-medium text-blue-700 dark:text-blue-300">{{
                  t('instructions.sections.overview')
                }}</span>
              </div>
              <i
                :class="[
                  'pi transition-transform',
                  expandedSections.has('overview') ? 'pi-chevron-up' : 'pi-chevron-down',
                ]"
              />
            </button>
            <div
              v-if="expandedSections.has('overview')"
              class="mt-3 prose prose-sm dark:prose-invert max-w-none"
            >
              <div v-html="renderMarkdown(instructions.overview)" />

              <!-- Learning Objectives -->
              <div v-if="instructions.learning_objectives?.length" class="mt-4">
                <h4 class="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
                  <i class="pi pi-check-circle mr-2 text-green-500" />
                  {{ t('instructions.sections.learningObjectives') }}
                </h4>
                <ul
                  class="list-disc list-inside text-sm text-surface-600 dark:text-surface-400 space-y-1"
                >
                  <li v-for="(obj, i) in instructions.learning_objectives" :key="i">
                    {{ obj }}
                  </li>
                </ul>
              </div>

              <!-- Prerequisites -->
              <div v-if="instructions.prerequisites?.length" class="mt-4">
                <h4 class="text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
                  <i class="pi pi-book mr-2 text-orange-500" />
                  {{ t('instructions.sections.prerequisites') }}
                </h4>
                <ul
                  class="list-disc list-inside text-sm text-surface-600 dark:text-surface-400 space-y-1"
                >
                  <li v-for="(prereq, i) in instructions.prerequisites" :key="i">
                    {{ prereq }}
                  </li>
                </ul>
              </div>
            </div>
          </div>

          <!-- Current Step -->
          <div v-if="currentStep">
            <!-- Step Header -->
            <div class="flex items-start gap-3 mb-4">
              <div
                :class="[
                  'w-10 h-10 rounded-full flex items-center justify-center text-lg font-bold flex-shrink-0',
                  isStepCompleted(currentStep)
                    ? 'bg-green-100 dark:bg-green-900/50 text-green-600'
                    : 'bg-primary-100 dark:bg-primary-900/50 text-primary-600',
                ]"
              >
                <i v-if="isStepCompleted(currentStep)" class="pi pi-check" />
                <span v-else>{{ currentStepIndex + 1 }}</span>
              </div>
              <div class="flex-1">
                <h3 class="text-lg font-semibold text-surface-900 dark:text-surface-100">
                  {{ currentStep.title }}
                </h3>
                <div v-if="isStepCompleted(currentStep)" class="flex items-center gap-2 mt-1">
                  <Tag :value="t('instructions.completedTag')" severity="success" />
                </div>
              </div>
            </div>

            <!-- Step Content (Markdown) -->
            <div class="prose prose-sm dark:prose-invert max-w-none step-content">
              <div v-html="renderMarkdown(currentStep.content)" />
            </div>
          </div>

          <!-- Summary Section (show on last step) -->
          <div
            v-if="currentStepIndex === totalSteps - 1 && instructions.summary"
            class="mt-8 pt-6 border-t border-surface-200 dark:border-surface-700"
          >
            <button
              @click="toggleSection('summary')"
              class="w-full flex items-center justify-between p-3 bg-green-50 dark:bg-green-900/20 rounded-lg text-left"
            >
              <div class="flex items-center gap-2">
                <i class="pi pi-flag-fill text-green-500" />
                <span class="font-medium text-green-700 dark:text-green-300">{{
                  t('instructions.sections.summary')
                }}</span>
              </div>
              <i
                :class="[
                  'pi transition-transform',
                  expandedSections.has('summary') ? 'pi-chevron-up' : 'pi-chevron-down',
                ]"
              />
            </button>
            <div
              v-if="expandedSections.has('summary')"
              class="mt-3 prose prose-sm dark:prose-invert max-w-none"
            >
              <div v-html="renderMarkdown(instructions.summary)" />
            </div>
          </div>

          <!-- Tips Section -->
          <div v-if="instructions.tips?.length" class="mt-6">
            <button
              @click="toggleSection('tips')"
              class="w-full flex items-center justify-between p-3 bg-yellow-50 dark:bg-yellow-900/20 rounded-lg text-left"
            >
              <div class="flex items-center gap-2">
                <i class="pi pi-lightbulb text-yellow-500" />
                <span class="font-medium text-yellow-700 dark:text-yellow-300">{{
                  t('instructions.sections.tips')
                }}</span>
              </div>
              <i
                :class="[
                  'pi transition-transform',
                  expandedSections.has('tips') ? 'pi-chevron-up' : 'pi-chevron-down',
                ]"
              />
            </button>
            <div v-if="expandedSections.has('tips')" class="mt-3 space-y-2">
              <div
                v-for="(tip, i) in instructions.tips"
                :key="i"
                class="flex items-start gap-2 p-2 bg-surface-50 dark:bg-surface-800 rounded text-sm"
              >
                <i class="pi pi-angle-right text-yellow-500 mt-0.5" />
                <span class="text-surface-700 dark:text-surface-300">{{ tip }}</span>
              </div>
            </div>
          </div>

          <!-- Resources Section -->
          <div v-if="instructions.resources?.length" class="mt-6">
            <button
              @click="toggleSection('resources')"
              class="w-full flex items-center justify-between p-3 bg-accent-50 dark:bg-accent-900/20 rounded-lg text-left"
            >
              <div class="flex items-center gap-2">
                <i class="pi pi-external-link text-accent-500" />
                <span class="font-medium text-accent-700 dark:text-accent-300">{{
                  t('instructions.sections.resources')
                }}</span>
              </div>
              <i
                :class="[
                  'pi transition-transform',
                  expandedSections.has('resources') ? 'pi-chevron-up' : 'pi-chevron-down',
                ]"
              />
            </button>
            <div v-if="expandedSections.has('resources')" class="mt-3 space-y-2">
              <a
                v-for="(resource, i) in instructions.resources"
                :key="i"
                :href="resource.url"
                target="_blank"
                rel="noopener noreferrer"
                class="flex items-center gap-2 p-2 bg-surface-50 dark:bg-surface-800 rounded text-sm hover:bg-surface-100 dark:hover:bg-surface-700 transition-colors"
              >
                <i class="pi pi-link text-accent-500" />
                <span class="text-primary-600 dark:text-primary-400 hover:underline">{{
                  resource.title
                }}</span>
                <i class="pi pi-external-link text-xs text-surface-400 ml-auto" />
              </a>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- No Instructions Fallback -->
    <div v-else class="flex-1 flex items-center justify-center p-4">
      <div class="text-center">
        <i class="pi pi-file text-4xl text-surface-400 mb-3" />
        <p class="text-surface-700 dark:text-surface-300 mb-2">
          {{ t('instructions.empty.title') }}
        </p>
        <p class="text-sm text-surface-500">{{ t('instructions.empty.hint') }}</p>
      </div>
    </div>

    <!-- Footer Navigation -->
    <div
      v-if="instructions && totalSteps > 0"
      class="p-4 border-t border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-800"
    >
      <div class="flex items-center justify-between">
        <Button
          @click="prevStep"
          :disabled="currentStepIndex === 0"
          icon="pi pi-arrow-left"
          :label="t('instructions.nav.previous')"
          text
        />
        <div class="text-sm text-surface-500">
          {{ t('instructions.stepCounter', { current: currentStepIndex + 1, total: totalSteps }) }}
        </div>
        <Button
          @click="nextStep"
          :disabled="currentStepIndex >= totalSteps - 1"
          icon="pi pi-arrow-right"
          iconPos="right"
          :label="t('instructions.nav.next')"
        />
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import DOMPurify from 'dompurify'

// Simple markdown to HTML renderer with XSS protection
function renderMarkdown(text: string): string {
  if (!text) return ''

  let html = text
    // Code blocks (```...```)
    .replace(/```(\w+)?\n([\s\S]*?)```/g, '<pre><code class="language-$1">$2</code></pre>')
    // Inline code (`...`)
    .replace(/`([^`]+)`/g, '<code>$1</code>')
    // Headers
    .replace(/^### (.+)$/gm, '<h3>$1</h3>')
    .replace(/^## (.+)$/gm, '<h2>$1</h2>')
    .replace(/^# (.+)$/gm, '<h1>$1</h1>')
    // Bold
    .replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
    // Italic
    .replace(/\*([^*]+)\*/g, '<em>$1</em>')
    // Links
    .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>')
    // Tables (simplified)
    .replace(/\|(.+)\|/g, match => {
      const cells = match.split('|').filter(c => c.trim())
      if (cells.every(c => c.match(/^[\s-]+$/))) {
        return '' // Skip separator row
      }
      const row = cells.map(c => `<td class="border px-2 py-1">${c.trim()}</td>`).join('')
      return `<tr>${row}</tr>`
    })
    // Wrap tables
    .replace(/(<tr>[\s\S]*?<\/tr>)+/g, '<table class="border-collapse border">$&</table>')
    // Unordered lists
    .replace(/^- (.+)$/gm, '<li>$1</li>')
    // Paragraphs (double newlines)
    .replace(/\n\n/g, '</p><p>')
    // Single newlines in non-pre blocks
    .replace(/(?<!<\/pre>)\n(?!<)/g, '<br/>')
  // Wrap in paragraph

  // Wrap loose text in paragraphs
  if (!html.startsWith('<')) {
    html = `<p>${html}</p>`
  }

  // Wrap list items
  html = html.replace(/(<li>[\s\S]*?<\/li>)+/g, '<ul>$&</ul>')

  // Sanitize HTML to prevent XSS attacks
  // Allow safe tags for markdown rendering but block scripts, event handlers, etc.
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: [
      'h1',
      'h2',
      'h3',
      'h4',
      'h5',
      'h6',
      'p',
      'br',
      'hr',
      'ul',
      'ol',
      'li',
      'table',
      'thead',
      'tbody',
      'tr',
      'th',
      'td',
      'a',
      'strong',
      'em',
      'code',
      'pre',
      'blockquote',
      'span',
      'div',
    ],
    ALLOWED_ATTR: ['href', 'target', 'rel', 'class'],
    ALLOW_DATA_ATTR: false,
  })
}

export { renderMarkdown }
</script>

<style scoped>
@reference "../../style.css";

.step-content :deep(pre) {
  @apply bg-surface-900 text-green-400 p-4 rounded-lg overflow-x-auto my-4;
}

.step-content :deep(code) {
  @apply bg-surface-100 dark:bg-surface-800 px-1.5 py-0.5 rounded text-sm font-mono;
}

.step-content :deep(pre code) {
  @apply bg-transparent p-0;
}

.step-content :deep(h2) {
  @apply text-lg font-semibold text-surface-900 dark:text-surface-100 mt-6 mb-3;
}

.step-content :deep(h3) {
  @apply text-base font-medium text-surface-800 dark:text-surface-200 mt-4 mb-2;
}

.step-content :deep(table) {
  @apply w-full my-4 text-sm;
}

.step-content :deep(th),
.step-content :deep(td) {
  @apply border border-surface-200 dark:border-surface-700 px-3 py-2;
}

.step-content :deep(th) {
  @apply bg-surface-100 dark:bg-surface-800 font-medium;
}

.step-content :deep(ul) {
  @apply list-disc list-inside my-3 space-y-1;
}

.step-content :deep(p) {
  @apply my-3 text-surface-700 dark:text-surface-300 leading-relaxed;
}

.step-content :deep(a) {
  @apply text-primary-600 dark:text-primary-400 hover:underline;
}

.step-content :deep(strong) {
  @apply font-semibold text-surface-900 dark:text-surface-100;
}
</style>
