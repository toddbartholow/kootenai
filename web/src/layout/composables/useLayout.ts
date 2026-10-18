import { ref, computed } from 'vue'

const sidebarVisible = ref(true)
const sidebarCollapsed = ref(false)
const isMobile = ref(false)

export function useLayout() {
  const toggleSidebar = () => {
    if (isMobile.value) {
      sidebarVisible.value = !sidebarVisible.value
    } else {
      sidebarCollapsed.value = !sidebarCollapsed.value
    }
  }

  const closeSidebar = () => {
    if (isMobile.value) {
      sidebarVisible.value = false
    }
  }

  const checkMobile = () => {
    isMobile.value = window.innerWidth < 1024
    if (isMobile.value) {
      sidebarVisible.value = false
      sidebarCollapsed.value = false
    } else {
      sidebarVisible.value = true
    }
  }

  const sidebarClass = computed(() => ({
    'w-64': !sidebarCollapsed.value,
    'w-20': sidebarCollapsed.value && !isMobile.value
  }))

  const contentClass = computed(() => ({
    'lg:ml-64': sidebarVisible.value && !sidebarCollapsed.value,
    'lg:ml-20': sidebarVisible.value && sidebarCollapsed.value
  }))

  return {
    sidebarVisible,
    sidebarCollapsed,
    isMobile,
    toggleSidebar,
    closeSidebar,
    checkMobile,
    sidebarClass,
    contentClass
  }
}
