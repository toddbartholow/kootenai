<script setup lang="ts">
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useDashboardLayout } from '@/composables/useDashboardLayout'
import Button from '@volt/Button.vue'

defineProps<{
  userName: string
}>()

const { t } = useI18n()
const { locked, toggleLocked } = useDashboardLayout()
</script>

<template>
  <div class="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
    <div>
      <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">
        {{ t('dashboard.toolbar.welcome', { name: userName }) }}
      </h1>
      <p class="text-surface-600 dark:text-surface-400 mt-1">
        {{ t('dashboard.toolbar.subtitle') }}
      </p>
    </div>
    <div class="flex gap-2">
      <Button
        :icon="locked ? 'pi pi-lock' : 'pi pi-lock-open'"
        :severity="locked ? 'secondary' : 'warn'"
        :text="locked"
        rounded
        :aria-label="locked ? t('dashboard.toolbar.unlockAria') : t('dashboard.toolbar.lockAria')"
        @click="toggleLocked"
      />
      <RouterLink to="/pathways">
        <Button :label="t('dashboard.toolbar.browsePathways')" icon="pi pi-compass" severity="secondary" />
      </RouterLink>
      <RouterLink to="/labs">
        <Button :label="t('dashboard.toolbar.startLab')" icon="pi pi-play" />
      </RouterLink>
    </div>
  </div>
</template>
