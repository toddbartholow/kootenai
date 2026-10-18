<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { DashboardCardConfig } from '@/composables/useDashboardLayout'
import Accordion from '@volt/Accordion.vue'
import AccordionPanel from '@volt/AccordionPanel.vue'
import AccordionHeader from '@volt/AccordionHeader.vue'
import AccordionContent from '@volt/AccordionContent.vue'

const props = defineProps<{
  cards: DashboardCardConfig[]
}>()

const emit = defineEmits<{
  add: [id: string]
}>()

const { t, te } = useI18n()
const hiddenCount = computed(() => props.cards.filter(c => !c.visible).length)

function cardLabel(card: DashboardCardConfig): string {
  const key = `dashboard.widgets.${card.id}`
  return te(key) ? t(key) : card.label
}
</script>

<template>
  <Accordion :value="hiddenCount > 0 ? ['widgets'] : []" multiple>
    <AccordionPanel value="widgets">
      <AccordionHeader>
        <div class="flex items-center gap-2">
          <i class="pi pi-th-large" aria-hidden="true" />
          <span>{{ t('dashboard.availableWidgets.heading') }}</span>
          <span
            v-if="hiddenCount > 0"
            class="inline-flex items-center justify-center w-5 h-5 text-xs font-medium rounded-full bg-primary-100 text-primary-700 dark:bg-primary-900/50 dark:text-primary-300"
          >
            {{ hiddenCount }}
          </span>
        </div>
      </AccordionHeader>
      <AccordionContent>
        <div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
          <div
            v-for="card in cards"
            :key="card.id"
            class="flex items-center gap-2 p-3 rounded-lg border transition-colors"
            :class="card.visible
              ? 'border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-800/50 opacity-60'
              : 'border-surface-200 dark:border-surface-700 hover:border-primary-300 dark:hover:border-primary-600'"
          >
            <i :class="['pi', card.icon, 'text-sm', card.visible ? 'text-surface-400' : 'text-primary-500']" aria-hidden="true" />
            <span
              class="flex-1 text-sm truncate"
              :class="card.visible ? 'text-surface-400 dark:text-surface-500' : 'text-surface-700 dark:text-surface-300'"
            >
              {{ cardLabel(card) }}
            </span>
            <button
              v-if="!card.visible"
              class="p-1 rounded-md text-green-600 hover:text-green-700 dark:text-green-400 dark:hover:text-green-300 hover:bg-green-50 dark:hover:bg-green-900/20 transition-colors"
              :aria-label="t('dashboard.availableWidgets.addAria', { label: cardLabel(card) })"
              @click="emit('add', card.id)"
            >
              <i class="pi pi-plus text-sm" aria-hidden="true" />
            </button>
            <i
              v-else
              class="pi pi-check text-sm text-surface-400"
              aria-hidden="true"
            />
          </div>
        </div>
      </AccordionContent>
    </AccordionPanel>
  </Accordion>
</template>
