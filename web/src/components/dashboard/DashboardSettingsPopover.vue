<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useDashboardLayout } from '@/composables/useDashboardLayout'
import Popover from '@volt/Popover.vue'
import InputNumber from '@volt/InputNumber.vue'

const { t } = useI18n()
const layout = useDashboardLayout()

const popover = ref<InstanceType<typeof Popover> | null>(null)
const activeWidgetId = ref<string | null>(null)

const WIDGETS_WITH_ITEM_COUNT = ['enrollments', 'achievements', 'sessions', 'recommendations']

const SIZE_OPTIONS = computed(() => [
  { value: 'small' as const, label: t('dashboard.widgetSettings.widthOptions.small'), icon: 'pi-stop' },
  { value: 'large' as const, label: t('dashboard.widgetSettings.widthOptions.large'), icon: 'pi-th-large' },
  { value: 'full' as const, label: t('dashboard.widgetSettings.widthOptions.full'), icon: 'pi-arrows-h' },
])

// Resolve the itemCount setting for the currently-active widget. Kept
// as a computed rather than an inline template expression so the
// numeric default is applied without a runtime-compile TS cast.
const activeItemCount = computed<number>(() => {
  if (!activeWidgetId.value) return 3
  const raw = layout.getSettings(activeWidgetId.value)['itemCount']
  return typeof raw === 'number' ? raw : 3
})

function open(widgetId: string, event: Event): void {
  activeWidgetId.value = widgetId
  popover.value?.toggle(event)
}

function setActiveItemCount(count: number): void {
  if (activeWidgetId.value) {
    layout.updateSettings(activeWidgetId.value, { itemCount: count })
  }
}

defineExpose({ open })
</script>

<template>
  <Popover ref="popover">
    <div v-if="activeWidgetId" class="p-4 w-72 space-y-4">
      <h3 class="font-semibold text-surface-900 dark:text-surface-100">{{ t('dashboard.widgetSettings.heading') }}</h3>

      <div>
        <label class="text-sm text-surface-700 dark:text-surface-300 mb-2 block">{{ t('dashboard.widgetSettings.widthLabel') }}</label>
        <div class="flex gap-1">
          <button
            v-for="opt in SIZE_OPTIONS"
            :key="opt.value"
            class="flex-1 flex items-center justify-center gap-1.5 px-3 py-2 rounded-lg border text-sm font-medium transition-colors"
            :class="layout.getSize(activeWidgetId) === opt.value
              ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/30 text-primary-700 dark:text-primary-300'
              : 'border-surface-200 dark:border-surface-700 text-surface-600 dark:text-surface-400 hover:border-surface-300 dark:hover:border-surface-600'"
            :aria-label="t('dashboard.widgetSettings.widthAria', { size: opt.label })"
            :aria-pressed="layout.getSize(activeWidgetId) === opt.value"
            @click="activeWidgetId && layout.setSize(activeWidgetId, opt.value)"
          >
            <i :class="['pi', opt.icon, 'text-xs']" aria-hidden="true" />
            {{ opt.label }}
          </button>
        </div>
      </div>

      <div v-if="WIDGETS_WITH_ITEM_COUNT.includes(activeWidgetId)">
        <div class="flex items-center justify-between">
          <label :for="`settings-item-count-${activeWidgetId}`" class="text-sm text-surface-700 dark:text-surface-300">
            {{ t('dashboard.widgetSettings.itemsLabel') }}
          </label>
          <InputNumber
            :id="`settings-item-count-${activeWidgetId}`"
            :modelValue="activeItemCount"
            :min="1"
            :max="10"
            showButtons
            buttonLayout="horizontal"
            :inputStyle="{ width: '3rem', textAlign: 'center' }"
            decrementButtonClass="p-button-secondary"
            incrementButtonClass="p-button-secondary"
            incrementButtonIcon="pi pi-plus"
            decrementButtonIcon="pi pi-minus"
            @update:modelValue="setActiveItemCount"
          />
        </div>
      </div>
    </div>
  </Popover>
</template>
