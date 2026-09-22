<script setup lang="ts">
import Chart from 'primevue/chart'

const props = defineProps<{
  type: 'bar' | 'line' | 'pie' | 'doughnut' | 'radar' | 'polarArea'
  data: object
  options?: object
  plugins?: object[]
  width?: number
  height?: number
}>()
</script>

<template>
  <Chart
    :type="props.type"
    :data="props.data"
    :options="props.options ?? {}"
    :plugins="props.plugins ?? []"
    :width="props.width"
    :height="props.height"
    class="w-full"
  />
</template>
