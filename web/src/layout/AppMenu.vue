<script setup lang="ts">
import { computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useLayout } from './composables/useLayout'
import { useAuthStore } from '../stores/auth'

interface MenuItem {
  label: string
  icon: string
  to: string
  badge?: string | number
}

interface MenuSection {
  label: string
  items: MenuItem[]
}

defineProps<{
  collapsed?: boolean
}>()

const { t } = useI18n()
const router = useRouter()
const route = useRoute()
const { closeSidebar, isMobile } = useLayout()
const authStore = useAuthStore()

const isAdmin = computed(() => authStore.isAdmin)
const isInstructor = computed(
  () => authStore.isAdmin || authStore.user?.roles?.includes('instructor'),
)

// Menu trees are computed so section/item labels react to locale changes.
const menuItems = computed<MenuSection[]>(() => [
  {
    label: t('menu.sections.home'),
    items: [{ label: t('menu.items.dashboard'), icon: 'pi pi-home', to: '/' }],
  },
  {
    label: t('menu.sections.learning'),
    items: [
      { label: t('menu.items.pathways'), icon: 'pi pi-map', to: '/pathways' },
      { label: t('menu.items.labCatalog'), icon: 'pi pi-table', to: '/labs' },
    ],
  },
  {
    label: t('menu.sections.myWork'),
    items: [
      { label: t('menu.items.myPods'), icon: 'pi pi-server', to: '/pods' },
      { label: t('menu.items.createPod'), icon: 'pi pi-plus', to: '/pods/create' },
      { label: t('menu.items.sessions'), icon: 'pi pi-clock', to: '/sessions' },
    ],
  },
  {
    label: t('menu.sections.progress'),
    items: [
      { label: t('menu.items.achievements'), icon: 'pi pi-star', to: '/achievements' },
      { label: t('menu.items.leaderboard'), icon: 'pi pi-trophy', to: '/leaderboard' },
    ],
  },
  {
    label: t('menu.sections.schedule'),
    items: [{ label: t('menu.items.reservations'), icon: 'pi pi-calendar', to: '/reservations' }],
  },
])

const instructorMenuItems = computed<MenuSection[]>(() => [
  {
    label: t('menu.sections.instructor'),
    items: [
      { label: t('menu.items.classDashboard'), icon: 'pi pi-chart-pie', to: '/instructor' },
      { label: t('menu.items.createPathway'), icon: 'pi pi-plus-circle', to: '/pathways/create' },
      { label: t('menu.items.createLab'), icon: 'pi pi-plus-circle', to: '/labs/create' },
    ],
  },
])

const adminMenuItems = computed<MenuSection[]>(() => [
  {
    label: t('menu.sections.administration'),
    items: [
      { label: t('menu.items.userManagement'), icon: 'pi pi-users', to: '/admin/users' },
      { label: t('menu.items.labTemplates'), icon: 'pi pi-book', to: '/admin/templates' },
      { label: t('menu.items.analytics'), icon: 'pi pi-chart-bar', to: '/admin/analytics' },
      { label: t('menu.items.eventMonitoring'), icon: 'pi pi-chart-line', to: '/admin/events' },
      { label: t('menu.items.systemHealth'), icon: 'pi pi-heart', to: '/admin/health' },
      { label: t('menu.items.proxmoxVms'), icon: 'pi pi-desktop', to: '/proxmox-vms' },
    ],
  },
])

const navigate = (to: string) => {
  router.push(to)
  if (isMobile.value) {
    closeSidebar()
  }
}

const isActive = (to: string) => {
  return route.path === to
}
</script>

<template>
  <nav class="p-4">
    <template v-for="section in menuItems" :key="section.label">
      <div
        v-if="!collapsed"
        class="text-xs font-semibold text-surface-500 uppercase tracking-wider mb-2 mt-4 first:mt-0"
      >
        {{ section.label }}
      </div>
      <ul class="space-y-1">
        <li v-for="item in section.items" :key="item.to">
          <button
            @click="navigate(item.to)"
            class="w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-colors"
            :class="[
              isActive(item.to)
                ? 'bg-primary-100 text-primary-700 dark:bg-primary-900 dark:text-primary-300'
                : 'text-surface-700 dark:text-surface-300 hover:bg-surface-100 dark:hover:bg-surface-800',
              { 'justify-center': collapsed },
            ]"
          >
            <i :class="item.icon" class="text-lg" />
            <span v-if="!collapsed" class="flex-1 text-left">{{ item.label }}</span>
            <span
              v-if="item.badge && !collapsed"
              class="px-2 py-0.5 text-xs font-medium bg-primary-100 text-primary-700 dark:bg-primary-900 dark:text-primary-300 rounded-md"
            >
              {{ item.badge }}
            </span>
          </button>
        </li>
      </ul>
    </template>

    <!-- Instructor menu section (visible to instructors and admins) -->
    <!-- v-if is pulled outside v-for to avoid Vue's precedence rule where v-if
         evaluates before v-for bindings are in scope. Same treatment below. -->
    <template v-if="isInstructor">
      <template v-for="section in instructorMenuItems" :key="section.label">
        <div
          v-if="!collapsed"
          class="text-xs font-semibold text-primary-500 dark:text-primary-400 uppercase tracking-wider mb-2 mt-4"
        >
          {{ section.label }}
        </div>
        <ul class="space-y-1">
          <li v-for="item in section.items" :key="item.to">
            <button
              @click="navigate(item.to)"
              class="w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-colors"
              :class="[
                isActive(item.to)
                  ? 'bg-primary-100 text-primary-700 dark:bg-primary-900 dark:text-primary-300'
                  : 'text-surface-700 dark:text-surface-300 hover:bg-surface-100 dark:hover:bg-surface-800',
                { 'justify-center': collapsed },
              ]"
            >
              <i :class="item.icon" class="text-lg" />
              <span v-if="!collapsed" class="flex-1 text-left">{{ item.label }}</span>
            </button>
          </li>
        </ul>
      </template>
    </template>

    <!-- Admin menu section (only visible to admins) -->
    <template v-if="isAdmin">
      <template v-for="section in adminMenuItems" :key="section.label">
        <div
          v-if="!collapsed"
          class="text-xs font-semibold text-red-500 dark:text-red-400 uppercase tracking-wider mb-2 mt-4"
        >
          {{ section.label }}
        </div>
        <ul class="space-y-1">
          <li v-for="item in section.items" :key="item.to">
            <button
              @click="navigate(item.to)"
              class="w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-colors"
              :class="[
                isActive(item.to)
                  ? 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300'
                  : 'text-surface-700 dark:text-surface-300 hover:bg-surface-100 dark:hover:bg-surface-800',
                { 'justify-center': collapsed },
              ]"
            >
              <i :class="item.icon" class="text-lg" />
              <span v-if="!collapsed" class="flex-1 text-left">{{ item.label }}</span>
            </button>
          </li>
        </ul>
      </template>
    </template>
  </nav>
</template>
