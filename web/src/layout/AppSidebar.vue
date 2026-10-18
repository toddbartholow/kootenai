<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useLayout } from './composables/useLayout'
import AppMenu from './AppMenu.vue'

const { t } = useI18n()
const { sidebarVisible, sidebarCollapsed, isMobile, sidebarClass, closeSidebar } = useLayout()

// When the sidebar opens on mobile, focus the aside so screen readers
// announce it and keyboard users can arrow-navigate without having to
// tab into the offcanvas first.
const asideRef = ref<HTMLElement | null>(null)
watch(
  () => sidebarVisible.value && isMobile.value,
  async (mobileOpen) => {
    if (!mobileOpen) return
    await nextTick()
    asideRef.value?.focus()
  },
)

// Escape closes the mobile sidebar (matching the overlay button in AppLayout).
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && isMobile.value) {
    closeSidebar()
  }
}
</script>

<template>
  <aside
    ref="asideRef"
    v-show="sidebarVisible"
    tabindex="-1"
    role="navigation"
    :aria-label="t('sidebar.navAria')"
    class="fixed top-16 left-0 bottom-0 z-50 bg-surface-0 dark:bg-surface-900 border-r border-surface-200 dark:border-surface-700 transition-all duration-300 overflow-y-auto focus:outline-none"
    :class="[
      sidebarClass,
      isMobile ? 'w-64' : ''
    ]"
    @keydown="onKeydown"
  >
    <AppMenu :collapsed="sidebarCollapsed && !isMobile" />
  </aside>
</template>
