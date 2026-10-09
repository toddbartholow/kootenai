<script setup lang="ts">
import { computed } from 'vue'
import Card from '@volt/Card.vue'
import DashboardWidgetHeader from './DashboardWidgetHeader.vue'
import type { UseDashboardDragReturn } from '@/composables'
import type { useDashboardLayout } from '@/composables/useDashboardLayout'

/**
 * Slotted wrapper for a single dashboard widget. Collapses the ~15-line
 * outer div + Card + header boilerplate that used to be duplicated seven
 * times in DashboardView.vue.
 *
 * Consumers provide:
 *   - Identity props (id, label, icon).
 *   - `layout` and `drag` composable instances — these are explicit props
 *     rather than provide/inject to keep the component independently
 *     testable and to make the data flow visible.
 *   - A default slot with the widget body. The slot only renders when the
 *     widget is not minimized.
 *
 * The component itself is stateless; all visibility, order, drag class,
 * and minimize state comes from the passed-in composable instances.
 */
type DashboardLayout = ReturnType<typeof useDashboardLayout>

const props = defineProps<{
  id: string
  label: string
  icon: string
  hasSettings?: boolean
  layout: DashboardLayout
  drag: UseDashboardDragReturn
}>()

const emit = defineEmits<{
  (e: 'configure', event: Event): void
}>()

// The outer-div grid span is driven by the widget's configured size.
const colSpanClass = computed(() => {
  switch (props.layout.getSize(props.id)) {
    case 'full':
      return 'lg:col-span-3'
    case 'large':
      return 'lg:col-span-2'
    case 'small':
      return 'lg:col-span-1'
    default:
      return 'lg:col-span-1'
  }
})

// Drag + keyboard bindings (draggable, tabindex, aria-label, class, event
// handlers) are all produced per-widget by the composable. We spread them
// onto the outer div via v-bind.
const dragBindings = computed(() => props.drag.widgetBindings(props.id, props.label))

// Header action handlers — mostly thin delegates to the layout store.
function onMinimize(): void {
  props.layout.setMinimized(props.id, !props.layout.isMinimized(props.id))
}
function onRemove(): void {
  props.layout.removeWidget(props.id)
}
function onConfigure(event: Event): void {
  emit('configure', event)
}
</script>

<template>
  <div
    v-if="layout.isVisible(id)"
    :style="{ order: layout.getOrder(id) }"
    :class="[colSpanClass, dragBindings.class]"
    :draggable="dragBindings.draggable"
    :tabindex="dragBindings.tabindex"
    :aria-label="dragBindings['aria-label']"
    @dragstart="dragBindings.onDragstart"
    @dragover="dragBindings.onDragover"
    @dragleave="dragBindings.onDragleave"
    @drop="dragBindings.onDrop"
    @dragend="dragBindings.onDragend"
    @keydown="dragBindings.onKeydown"
  >
    <Card>
      <template #title>
        <DashboardWidgetHeader
          :id="id"
          :label="label"
          :icon="icon"
          :locked="layout.locked.value"
          :minimized="layout.isMinimized(id)"
          :has-settings="hasSettings ?? false"
          @minimize="onMinimize"
          @remove="onRemove"
          @configure="onConfigure"
        >
          <!-- Forward the `actions` slot so widgets can inject header
               actions (e.g. a "View all" link) without having to unpack
               the DashboardWidgetHeader layer themselves. -->
          <template v-if="$slots['actions']" #actions>
            <slot name="actions" />
          </template>
        </DashboardWidgetHeader>
      </template>
      <template #content>
        <div v-show="!layout.isMinimized(id)">
          <slot />
        </div>
      </template>
    </Card>
  </div>
</template>
