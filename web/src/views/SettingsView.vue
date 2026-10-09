<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { useThemeStore } from '@/stores/theme'
import { useOrganizationStore } from '@/stores/organization'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import ToggleSwitch from '@volt/ToggleSwitch.vue'
import Message from '@volt/Message.vue'

const { t } = useI18n()

const NOTIFICATION_SETTINGS_KEY = 'kootenai-notification-settings'

interface NotificationSettings {
  emailNotifications: boolean
  achievementNotifications: boolean
  progressReminders: boolean
}

function loadNotificationSettings(): NotificationSettings {
  const saved = localStorage.getItem(NOTIFICATION_SETTINGS_KEY)
  if (saved) {
    try {
      return JSON.parse(saved)
    } catch {
      // Ignore invalid JSON
    }
  }
  return {
    emailNotifications: true,
    achievementNotifications: true,
    progressReminders: false,
  }
}

function saveNotificationSettings(settings: NotificationSettings): void {
  localStorage.setItem(NOTIFICATION_SETTINGS_KEY, JSON.stringify(settings))
}

const authStore = useAuthStore()
const themeStore = useThemeStore()
const orgStore = useOrganizationStore()

const saving = ref(false)
const success = ref(false)
const error = ref<string | null>(null)

// User settings
const displayName = ref('')
const email = ref('')
const currentTheme = computed(() => (themeStore.isDark ? 'dark' : 'light'))

// Notification preferences (persisted to localStorage)
const savedSettings = loadNotificationSettings()
const emailNotifications = ref(savedSettings.emailNotifications)
const achievementNotifications = ref(savedSettings.achievementNotifications)
const progressReminders = ref(savedSettings.progressReminders)

// Theme options. Computed so labels re-render when the active locale changes.
const themeOptions = computed(() => [
  { label: t('settings.themeOptions.light'), value: 'light' },
  { label: t('settings.themeOptions.dark'), value: 'dark' },
  { label: t('settings.themeOptions.system'), value: 'system' },
])

const selectedTheme = ref(currentTheme.value)

onMounted(() => {
  if (authStore.user) {
    displayName.value = authStore.user.name || ''
    email.value = authStore.user.email || ''
  }
})

function handleThemeChange(newTheme: string) {
  if (newTheme === 'dark' || newTheme === 'light') {
    themeStore.setTheme(newTheme)
  }
  // System preference would need more implementation
}

// Single statement so the template handlers stay one expression. Prettier
// reflows a multi-statement inline handler onto separate lines and drops the
// separating semicolon, which Vue's template expression parser rejects.
function selectTheme(newTheme: string) {
  selectedTheme.value = newTheme
  handleThemeChange(newTheme)
}

async function saveSettings() {
  saving.value = true
  error.value = null
  success.value = false

  try {
    // Update theme (already persisted by theme store)
    handleThemeChange(selectedTheme.value)

    // Save notification settings to localStorage
    saveNotificationSettings({
      emailNotifications: emailNotifications.value,
      achievementNotifications: achievementNotifications.value,
      progressReminders: progressReminders.value,
    })

    success.value = true
    setTimeout(() => {
      success.value = false
    }, 3000)
  } catch (err) {
    error.value = t('settings.saveFailed')
    console.error('Failed to save settings:', err)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="p-6 max-w-4xl mx-auto">
    <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-0 mb-6">
      {{ t('settings.title') }}
    </h1>

    <Message v-if="success" severity="success" class="mb-4" :closable="false">
      {{ t('settings.savedMessage') }}
    </Message>
    <Message v-if="error" severity="error" class="mb-4" :closable="false">
      {{ error }}
    </Message>

    <div class="space-y-6">
      <!-- Account Settings -->
      <Card>
        <template #title>
          <div class="flex items-center gap-2">
            <i class="pi pi-user"></i>
            <span>{{ t('settings.accountHeading') }}</span>
          </div>
        </template>
        <template #content>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
                {{ t('settings.displayNameLabel') }}
              </label>
              <InputText
                v-model="displayName"
                class="w-full"
                :placeholder="t('settings.displayNamePlaceholder')"
              />
            </div>
            <div>
              <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
                {{ t('settings.emailLabel') }}
              </label>
              <InputText
                v-model="email"
                class="w-full"
                :placeholder="t('settings.emailPlaceholder')"
                disabled
              />
              <p class="text-xs text-surface-500 mt-1">{{ t('settings.emailHint') }}</p>
            </div>
          </div>

          <div class="mt-6 pt-6 border-t border-surface-200 dark:border-surface-700">
            <Button
              :label="t('settings.changePasswordAction')"
              icon="pi pi-lock"
              outlined
              @click="$router.push('/change-password')"
            />
          </div>
        </template>
      </Card>

      <!-- Appearance Settings -->
      <Card>
        <template #title>
          <div class="flex items-center gap-2">
            <i class="pi pi-palette"></i>
            <span>{{ t('settings.appearanceHeading') }}</span>
          </div>
        </template>
        <template #content>
          <div class="max-w-xs">
            <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
              {{ t('settings.themeLabel') }}
            </label>
            <!-- eslint-disable-next-line vuejs-accessibility/no-onchange -- @change on <Select> fires on user selection; @blur would fire on every focus-out. -->
            <Select
              v-model="selectedTheme"
              :options="themeOptions"
              optionLabel="label"
              optionValue="value"
              :aria-label="t('settings.themeAria')"
              class="w-full"
              @change="handleThemeChange(selectedTheme)"
            />
          </div>

          <div class="mt-6 flex gap-4">
            <div
              class="w-24 h-16 rounded-lg border-2 cursor-pointer transition-all flex items-center justify-center"
              :class="
                selectedTheme === 'light'
                  ? 'border-primary-500 bg-white'
                  : 'border-surface-300 bg-white'
              "
              role="button"
              tabindex="0"
              :aria-label="t('settings.lightAria')"
              :aria-pressed="selectedTheme === 'light'"
              @click="selectTheme('light')"
              @keydown.enter="selectTheme('light')"
              @keydown.space.prevent="selectTheme('light')"
            >
              <i class="pi pi-sun text-yellow-500"></i>
            </div>
            <div
              class="w-24 h-16 rounded-lg border-2 cursor-pointer transition-all flex items-center justify-center"
              :class="
                selectedTheme === 'dark'
                  ? 'border-primary-500 bg-surface-800'
                  : 'border-surface-600 bg-surface-800'
              "
              role="button"
              tabindex="0"
              :aria-label="t('settings.darkAria')"
              :aria-pressed="selectedTheme === 'dark'"
              @click="selectTheme('dark')"
              @keydown.enter="selectTheme('dark')"
              @keydown.space.prevent="selectTheme('dark')"
            >
              <i class="pi pi-moon text-blue-400"></i>
            </div>
          </div>
        </template>
      </Card>

      <!-- Notification Settings -->
      <Card>
        <template #title>
          <div class="flex items-center gap-2">
            <i class="pi pi-bell"></i>
            <span>{{ t('settings.notificationsHeading') }}</span>
          </div>
        </template>
        <template #content>
          <div class="space-y-4">
            <div class="flex items-center justify-between py-2">
              <div>
                <div class="font-medium text-surface-900 dark:text-surface-0">
                  {{ t('settings.notifications.emailTitle') }}
                </div>
                <div class="text-sm text-surface-500">
                  {{ t('settings.notifications.emailDescription') }}
                </div>
              </div>
              <ToggleSwitch v-model="emailNotifications" />
            </div>

            <div
              class="flex items-center justify-between py-2 border-t border-surface-200 dark:border-surface-700"
            >
              <div>
                <div class="font-medium text-surface-900 dark:text-surface-0">
                  {{ t('settings.notifications.achievementTitle') }}
                </div>
                <div class="text-sm text-surface-500">
                  {{ t('settings.notifications.achievementDescription') }}
                </div>
              </div>
              <ToggleSwitch v-model="achievementNotifications" />
            </div>

            <div
              class="flex items-center justify-between py-2 border-t border-surface-200 dark:border-surface-700"
            >
              <div>
                <div class="font-medium text-surface-900 dark:text-surface-0">
                  {{ t('settings.notifications.progressTitle') }}
                </div>
                <div class="text-sm text-surface-500">
                  {{ t('settings.notifications.progressDescription') }}
                </div>
              </div>
              <ToggleSwitch v-model="progressReminders" />
            </div>
          </div>
        </template>
      </Card>

      <!-- Organization Info (if applicable) -->
      <Card v-if="orgStore.currentOrganization">
        <template #title>
          <div class="flex items-center gap-2">
            <i class="pi pi-building"></i>
            <span>{{ t('settings.organizationHeading') }}</span>
          </div>
        </template>
        <template #content>
          <div class="flex items-center justify-between">
            <div>
              <div class="font-medium text-surface-900 dark:text-surface-0" data-pseudo-skip>
                {{ orgStore.currentOrganization.name }}
              </div>
              <div class="text-sm text-surface-500">
                {{
                  t('settings.orgRoleLabel', {
                    role: orgStore.currentRole || t('settings.orgMemberFallback'),
                  })
                }}
              </div>
            </div>
            <Button
              :label="t('settings.orgSettingsAction')"
              icon="pi pi-cog"
              text
              @click="$router.push(`/organizations/${orgStore.currentOrganization.id}/settings`)"
              v-if="orgStore.hasRole('admin')"
            />
          </div>
        </template>
      </Card>

      <!-- Save Button -->
      <div class="flex justify-end">
        <Button
          :label="t('settings.saveAction')"
          icon="pi pi-check"
          :loading="saving"
          @click="saveSettings"
        />
      </div>
    </div>
  </div>
</template>
