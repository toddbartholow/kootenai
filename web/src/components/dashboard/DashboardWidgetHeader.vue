<script setup lang="ts">
defineProps<{
  id: string
  label: string
  icon: string
  locked: boolean
  minimized: boolean
  hasSettings?: boolean
}>()

defineEmits<{
  minimize: []
  remove: []
  configure: [event: Event]
}>()
</script>

<template>
  <div class="flex items-center justify-between w-full">
    <div class="flex items-center gap-2 min-w-0">
      <i
        v-if="!locked"
        class="pi pi-bars text-surface-400 cursor-grab active:cursor-grabbing"
        aria-hidden="true"
      />
      <i :class="['pi', icon, 'text-primary-500']" aria-hidden="true" />
      <span class="truncate">{{ label }}</span>
    </div>
    <div class="flex items-center gap-1 shrink-0">
      <slot name="actions" />
      <template v-if="!locked">
        <button
          v-if="hasSettings"
          class="p-1.5 rounded-md text-surface-400 hover:text-surface-700 dark:hover:text-surface-200 hover:bg-surface-100 dark:hover:bg-surface-700 transition-colors"
          :aria-label="`Configure ${label}`"
          @click="$emit('configure', $event)"
        >
          <i class="pi pi-cog text-sm" aria-hidden="true" />
        </button>
        <button
          class="p-1.5 rounded-md text-surface-400 hover:text-surface-700 dark:hover:text-surface-200 hover:bg-surface-100 dark:hover:bg-surface-700 transition-colors"
          :aria-label="minimized ? `Expand ${label}` : `Minimize ${label}`"
          @click="$emit('minimize')"
        >
          <i :class="['pi', minimized ? 'pi-plus' : 'pi-minus', 'text-sm']" aria-hidden="true" />
        </button>
        <button
          class="p-1.5 rounded-md text-surface-400 hover:text-red-500 dark:hover:text-red-400 hover:bg-red-50 dark:hover:bg-red-900/20 transition-colors"
          :aria-label="`Remove ${label}`"
          @click="$emit('remove')"
        >
          <i class="pi pi-times text-sm" aria-hidden="true" />
        </button>
      </template>
    </div>
  </div>
</template>
