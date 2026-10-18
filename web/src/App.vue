<script setup lang="ts">
import { computed } from 'vue'
import { RouterView, useRoute } from 'vue-router'
import { useThemeStore } from './stores/theme'
import { useAuthStore } from './stores/auth'
import AppLayout from './layout/AppLayout.vue'
import ErrorBoundary from './components/common/ErrorBoundary.vue'

const route = useRoute()
const authStore = useAuthStore()

// Initialize theme store so its internal watcher applies the dark class
useThemeStore()

// Show layout only when authenticated and not on login page
const showLayout = computed(() => authStore.isAuthenticated && route.name !== 'login')
</script>

<template>
  <ErrorBoundary>
    <AppLayout v-if="showLayout">
      <RouterView />
    </AppLayout>
    <RouterView v-else />
  </ErrorBoundary>
</template>
