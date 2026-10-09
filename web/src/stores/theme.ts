import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'

/**
 * Get initial theme from system preference
 */
function getSystemTheme(): 'light' | 'dark' {
  if (typeof window === 'undefined') return 'light'
  if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
    return 'dark'
  }
  return 'light'
}

/**
 * Apply theme to document root
 */
function applyTheme(newTheme: 'light' | 'dark') {
  if (typeof document === 'undefined') return
  if (newTheme === 'dark') {
    document.documentElement.classList.add('dark')
  } else {
    document.documentElement.classList.remove('dark')
  }
}

export const useThemeStore = defineStore('theme', () => {
  // State - will be hydrated from localStorage by persist plugin
  const theme = ref<'light' | 'dark'>(getSystemTheme())
  const isDark = computed(() => theme.value === 'dark')

  // Watch for changes to apply theme to DOM
  watch(theme, (newTheme) => {
    applyTheme(newTheme)
  }, { immediate: true })

  function toggleTheme() {
    theme.value = theme.value === 'dark' ? 'light' : 'dark'
  }

  function setTheme(newTheme: 'light' | 'dark') {
    theme.value = newTheme
  }

  return {
    theme,
    isDark,
    toggleTheme,
    setTheme,
  }
}, {
  persist: {
    key: 'theme',
    paths: ['theme'],
    // Apply theme immediately after hydration
    afterRestore: (ctx) => {
      applyTheme(ctx.store['theme'])
    },
  },
})
