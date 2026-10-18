<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useLayout } from './composables/useLayout'
import { useThemeStore } from '@/stores/theme'
import { useAuthStore } from '@/stores/auth'
import { useOrganizationStore } from '@/stores/organization'
import Button from '@volt/Button.vue'
import Menu from '@volt/Menu.vue'
import OrganizationSwitcher from '@/components/organization/OrganizationSwitcher.vue'
import { LanguageSwitcher } from '@/components/common'

const { t } = useI18n()
const router = useRouter()
const { toggleSidebar } = useLayout()
const themeStore = useThemeStore()
const authStore = useAuthStore()
const orgStore = useOrganizationStore()

const userMenu = ref()
// Menu items are computed so labels re-render when the active locale changes.
const userMenuItems = computed(() => [
  {
    label: t('topbar.userMenu.profile'),
    icon: 'pi pi-user',
    command: () => router.push('/profile')
  },
  {
    label: t('topbar.userMenu.settings'),
    icon: 'pi pi-cog',
    command: () => router.push('/settings')
  },
  {
    label: t('topbar.userMenu.organization'),
    icon: 'pi pi-building',
    command: () => {
      if (orgStore.currentOrganization) {
        router.push(`/organizations/${orgStore.currentOrganization.id}`)
      }
    },
    visible: () => !!orgStore.currentOrganization
  },
  { separator: true },
  {
    label: t('topbar.userMenu.logout'),
    icon: 'pi pi-sign-out',
    command: () => {
      orgStore.clearOrganization()
      authStore.logout()
      router.push('/login')
    }
  }
])

const toggleUserMenu = (event: Event) => {
  userMenu.value.toggle(event)
}
</script>

<template>
  <header class="fixed top-0 left-0 right-0 z-50 h-16 bg-surface-0 dark:bg-surface-900 border-b border-surface-200 dark:border-surface-700">
    <div class="flex items-center justify-between h-full px-4">
      <!-- Left: Menu toggle + Logo -->
      <div class="flex items-center gap-4">
        <Button
          icon="pi pi-bars"
          text
          rounded
          :aria-label="t('topbar.toggleSidebarAria')"
          @click="toggleSidebar"
        />
        <router-link to="/" class="flex items-center gap-2">
          <span class="text-xl font-bold text-primary-600 dark:text-primary-400">
            {{ t('app.brand') }}
          </span>
        </router-link>
      </div>

      <!-- Center: Organization Switcher -->
      <div class="hidden md:flex items-center">
        <OrganizationSwitcher v-if="authStore.isAuthenticated" />
      </div>

      <!-- Right: Language switcher + Theme toggle + User menu -->
      <div class="flex items-center gap-2">
        <LanguageSwitcher v-if="authStore.isAuthenticated" />
        <Button
          :icon="themeStore.isDark ? 'pi pi-sun' : 'pi pi-moon'"
          text
          rounded
          :aria-label="themeStore.isDark ? t('topbar.lightModeAria') : t('topbar.darkModeAria')"
          @click="themeStore.toggleTheme()"
          v-tooltip.bottom="themeStore.isDark ? t('topbar.lightModeTooltip') : t('topbar.darkModeTooltip')"
        />
        <Button
          icon="pi pi-user"
          text
          rounded
          :aria-label="t('topbar.openUserMenuAria')"
          @click="toggleUserMenu"
        />
        <Menu ref="userMenu" :model="userMenuItems" popup />
      </div>
    </div>
  </header>
</template>
