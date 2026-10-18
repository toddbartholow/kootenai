<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useLayout } from './composables/useLayout'
import AppTopbar from './AppTopbar.vue'
import AppSidebar from './AppSidebar.vue'

const { t } = useI18n()
const { sidebarVisible, isMobile, checkMobile, closeSidebar, contentClass } = useLayout()

onMounted(() => {
  checkMobile()
  window.addEventListener('resize', checkMobile)
})

onUnmounted(() => {
  window.removeEventListener('resize', checkMobile)
})
</script>

<template>
  <div class="min-h-screen bg-surface-50 dark:bg-surface-950">
    <!-- Skip link for keyboard navigation -->
    <a
      href="#main-content"
      class="skip-link"
    >
      {{ t('app.skipToContent') }}
    </a>

    <AppTopbar />

    <AppSidebar />

    <!-- Overlay for mobile -->
    <button
      v-if="sidebarVisible && isMobile"
      type="button"
      class="fixed inset-0 z-40 bg-black/50 cursor-default"
      :aria-label="t('app.closeSidebarAria')"
      @click="closeSidebar"
      @keydown.escape="closeSidebar"
    />

    <!--
      When the mobile sidebar is open, mark the main content region as
      `inert` so screen readers and tab navigation can't reach through the
      overlay into occluded content. `inert` is a first-class HTML boolean
      attribute widely supported in modern browsers; Vue serializes it as
      a real attribute when truthy.
    -->
    <main
      id="main-content"
      class="pt-16 transition-all duration-300"
      :class="contentClass"
      :aria-label="t('app.mainContentAria')"
      :inert="sidebarVisible && isMobile ? true : undefined"
    >
      <div class="p-4 lg:p-6">
        <slot />
      </div>
    </main>
  </div>
</template>
