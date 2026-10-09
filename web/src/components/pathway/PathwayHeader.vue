<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Tag from '@volt/Tag.vue'
import type { Pathway } from '@/api'
import { getDifficultySeverity } from '@/utils/status'

const { t } = useI18n()

interface Props {
  pathway: Pathway
}

defineProps<Props>()

// getDifficultySeverity imported from @/utils/status
const getDifficultyColor = getDifficultySeverity
</script>

<template>
  <div class="pathway-header">
    <!-- Badges row -->
    <div class="flex items-center gap-2 mb-4">
      <Tag
        v-if="pathway.isFeatured"
        :value="t('pathway.header.featuredTag')"
        severity="danger"
        class="font-medium"
      />
      <Tag
        v-if="pathway.difficulty"
        :value="pathway.difficulty"
        :severity="getDifficultyColor(pathway.difficulty)"
        class="font-medium"
      />
    </div>

    <!-- Title -->
    <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100 mb-3">
      {{ pathway.name }}
    </h1>

    <!-- Description -->
    <p
      v-if="pathway.description"
      class="text-lg text-surface-600 dark:text-surface-400 mb-4"
    >
      {{ pathway.description }}
    </p>

    <!-- Tags -->
    <div v-if="pathway.tags?.length" class="flex flex-wrap gap-2">
      <Tag
        v-for="tag in pathway.tags"
        :key="tag"
        :value="tag"
        severity="secondary"
        class="text-sm"
      />
    </div>
  </div>
</template>
