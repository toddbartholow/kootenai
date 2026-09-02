<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Tag from '@volt/Tag.vue'

const { t } = useI18n()

interface Props {
  prerequisites: string[]
}

defineProps<Props>()
</script>

<template>
  <div v-if="prerequisites.length > 0" class="pathway-prerequisites">
    <h3 class="text-sm font-semibold text-surface-900 dark:text-surface-100 mb-2">
      {{ t('pathway.prerequisites.heading') }}
    </h3>
    <div class="flex flex-wrap gap-2">
      <Tag
        v-for="prereq in prerequisites"
        :key="prereq"
        severity="success"
        class="text-sm"
      >
        <template #default>
          <div class="flex items-center gap-1.5">
            <i class="pi pi-check text-xs" />
            <span>{{ prereq }}</span>
          </div>
        </template>
      </Tag>
    </div>
  </div>
</template>
