<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  usersApi,
  passwordApi,
  type User,
  type CreateUserRequest,
  type UpdateUserRequest,
} from '@/api'
import { formatDate } from '@/utils/format'
import { createErrorState } from '@/types/errors'
import { useAuthStore } from '../stores/auth'
import { useFocusRestore } from '@/composables'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Tag from '@volt/Tag.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import ConfirmDialog from '@volt/ConfirmDialog.vue'
import { useConfirm } from 'primevue/useconfirm'
import { useToast } from 'primevue/usetoast'
import UserFormDialog, { type UserFormData } from '@/components/user/UserFormDialog.vue'
import PasswordResetDialog from '@/components/user/PasswordResetDialog.vue'

const { t, te } = useI18n()
const authStore = useAuthStore()
const confirm = useConfirm()
const toast = useToast()

const ROLE_LABEL_KEYS: Record<string, string> = {
  admin: 'profile.roles.admin',
  instructor: 'profile.roles.instructor',
  student: 'profile.roles.student',
}

function getRoleLabel(role: string): string {
  const key = ROLE_LABEL_KEYS[role]
  return key && te(key) ? t(key) : role
}

// Check if current user is admin
const isAdmin = computed(() => authStore.isAdmin)

// Data
const users = ref<User[]>([])
const loading = ref(true)
const error = ref<string | null>(null)
const total = ref(0)

// Filter state
const searchQuery = ref('')
const filterRole = ref<{ label: string; value: string | null }>({
  label: t('users.roleOptions.all'),
  value: null,
})
const filterActive = ref<{ label: string; value: boolean | null }>({
  label: t('users.statusOptions.all'),
  value: null,
})

// Filter options are computed so labels refresh when the active locale changes.
const roleOptions = computed(() => [
  { label: t('users.roleOptions.all'), value: null },
  { label: t('users.roleOptions.students'), value: 'student' },
  { label: t('users.roleOptions.instructors'), value: 'instructor' },
  { label: t('users.roleOptions.admins'), value: 'admin' },
])

const statusOptions = computed(() => [
  { label: t('users.statusOptions.all'), value: null },
  { label: t('users.statusOptions.active'), value: true },
  { label: t('users.statusOptions.inactive'), value: false },
])

// Dialog state
const showPasswordDialog = ref(false)

// Keyboard-focus restoration on modal close (Phase 5 a11y follow-up).
useFocusRestore(showPasswordDialog)
const editingUser = ref<User | null>(null)
const passwordResetUser = ref<User | null>(null)
const saving = ref(false)

// Password reset form state
const passwordResetForm = ref({
  generatePassword: true,
  manualPassword: '',
  mustChangePassword: true,
})
const generatedPassword = ref<string | null>(null)

// Form state for create/edit
const formData = ref<UserFormData>({
  username: '',
  email: '',
  displayName: '',
  role: { label: t('profile.roles.student'), value: 'student' },
})

// Track which dialog mode is active
const formMode = ref<'create' | 'edit'>('create')

onMounted(async () => {
  await loadUsers()
})

async function loadUsers() {
  try {
    loading.value = true
    error.value = null
    const params: { search?: string; role?: string; active?: boolean } = {}
    if (searchQuery.value) {
      params.search = searchQuery.value
    }
    if (filterRole.value?.value) {
      params.role = filterRole.value.value
    }
    if (filterActive.value?.value !== undefined && filterActive.value?.value !== null) {
      params.active = filterActive.value.value
    }
    const response = await usersApi.list(params)
    users.value = response.users
    total.value = response.total
  } catch (err) {
    console.error('Failed to load users:', err)
    error.value = t('users.loadFailed')
  } finally {
    loading.value = false
  }
}

// Shared dialog visibility for create/edit (now a single component)
const showFormDialog = ref(false)
useFocusRestore(showFormDialog)

function openCreateDialog() {
  formMode.value = 'create'
  formData.value = {
    username: '',
    email: '',
    displayName: '',
    role: { label: t('profile.roles.student'), value: 'student' },
  }
  showFormDialog.value = true
}

function openEditDialog(user: User) {
  formMode.value = 'edit'
  editingUser.value = user
  formData.value = {
    username: user.username,
    email: user.email || '',
    displayName: user.displayName || '',
    role: { label: t(`profile.roles.${user.role}`), value: user.role },
  }
  showFormDialog.value = true
}

function handleFormSubmit() {
  if (formMode.value === 'create') {
    createUser()
  } else {
    updateUser()
  }
}

async function createUser() {
  if (!formData.value.username) {
    toast.add({
      severity: 'error',
      summary: t('users.toast.validationErrorSummary'),
      detail: t('users.toast.validationUsernameRequired'),
      life: 3000,
    })
    return
  }

  try {
    saving.value = true
    const request: CreateUserRequest = {
      username: formData.value.username,
      email: formData.value.email || undefined,
      displayName: formData.value.displayName || undefined,
      role: formData.value.role.value as 'student' | 'instructor' | 'admin',
    }
    await usersApi.create(request)
    showFormDialog.value = false
    toast.add({
      severity: 'success',
      summary: t('users.toast.createSuccessSummary'),
      detail: t('users.toast.createSuccessDetail', { username: formData.value.username }),
      life: 3000,
    })
    await loadUsers()
  } catch (e: unknown) {
    console.error('Failed to create user:', e)
    const message = createErrorState(e, t('users.toast.createFailedFallback')).message
    toast.add({
      severity: 'error',
      summary: t('users.toast.errorSummary'),
      detail: message,
      life: 5000,
    })
  } finally {
    saving.value = false
  }
}

async function updateUser() {
  if (!editingUser.value) return

  try {
    saving.value = true
    const request: UpdateUserRequest = {
      email: formData.value.email || undefined,
      displayName: formData.value.displayName || undefined,
      role: formData.value.role.value as 'student' | 'instructor' | 'admin',
    }
    await usersApi.update(editingUser.value.id, request)
    showFormDialog.value = false
    toast.add({
      severity: 'success',
      summary: t('users.toast.updateSuccessSummary'),
      detail: t('users.toast.updateSuccessDetail', { username: editingUser.value.username }),
      life: 3000,
    })
    await loadUsers()
  } catch (e: unknown) {
    console.error('Failed to update user:', e)
    const message = createErrorState(e, t('users.toast.updateFailedFallback')).message
    toast.add({
      severity: 'error',
      summary: t('users.toast.errorSummary'),
      detail: message,
      life: 5000,
    })
  } finally {
    saving.value = false
  }
}

async function toggleUserStatus(user: User) {
  const newStatus = !user.isActive

  confirm.require({
    message: newStatus
      ? t('users.confirm.activateMessage', { username: user.username })
      : t('users.confirm.deactivateMessage', { username: user.username }),
    header: newStatus ? t('users.confirm.activateHeader') : t('users.confirm.deactivateHeader'),
    icon: newStatus ? 'pi pi-check-circle' : 'pi pi-ban',
    accept: async () => {
      try {
        await usersApi.updateStatus(user.id, newStatus)
        toast.add({
          severity: 'success',
          summary: t('users.toast.statusUpdateSuccessSummary'),
          detail: newStatus
            ? t('users.toast.statusActivatedDetail', { username: user.username })
            : t('users.toast.statusDeactivatedDetail', { username: user.username }),
          life: 3000,
        })
        await loadUsers()
      } catch (e: unknown) {
        console.error('Failed to update user status:', e)
        const message = createErrorState(e, t('users.toast.statusFailedFallback')).message
        toast.add({
          severity: 'error',
          summary: t('users.toast.errorSummary'),
          detail: message,
          life: 5000,
        })
      }
    },
  })
}

async function deleteUser(user: User) {
  confirm.require({
    message: t('users.confirm.deleteMessage', { username: user.username }),
    header: t('users.confirm.deleteHeader'),
    icon: 'pi pi-exclamation-triangle',
    acceptClass: 'p-button-danger',
    accept: async () => {
      try {
        await usersApi.delete(user.id)
        toast.add({
          severity: 'success',
          summary: t('users.toast.deleteSuccessSummary'),
          detail: t('users.toast.deleteSuccessDetail', { username: user.username }),
          life: 3000,
        })
        await loadUsers()
      } catch (e: unknown) {
        console.error('Failed to delete user:', e)
        const message = createErrorState(e, t('users.toast.deleteFailedFallback')).message
        toast.add({
          severity: 'error',
          summary: t('users.toast.errorSummary'),
          detail: message,
          life: 5000,
        })
      }
    },
  })
}

function openPasswordDialog(user: User) {
  passwordResetUser.value = user
  passwordResetForm.value = {
    generatePassword: true,
    manualPassword: '',
    mustChangePassword: true,
  }
  generatedPassword.value = null
  showPasswordDialog.value = true
}

async function resetPassword() {
  if (!passwordResetUser.value) return

  // Validate if not generating
  if (!passwordResetForm.value.generatePassword && !passwordResetForm.value.manualPassword) {
    toast.add({
      severity: 'error',
      summary: t('users.toast.validationErrorSummary'),
      detail: t('users.toast.validationPasswordRequired'),
      life: 3000,
    })
    return
  }

  try {
    saving.value = true
    const request: { generatePassword?: boolean; password?: string } = {
      generatePassword: passwordResetForm.value.generatePassword,
    }
    if (!passwordResetForm.value.generatePassword) {
      request.password = passwordResetForm.value.manualPassword
    }
    const response = await passwordApi.adminReset(passwordResetUser.value.id, request)

    // If generated, show the temporary password
    if (response.temporaryPassword) {
      generatedPassword.value = response.temporaryPassword
      toast.add({
        severity: 'success',
        summary: t('users.toast.passwordResetSuccessSummary'),
        detail: t('users.toast.passwordResetGeneratedDetail'),
        life: 5000,
      })
    } else {
      showPasswordDialog.value = false
      toast.add({
        severity: 'success',
        summary: t('users.toast.passwordResetSuccessSummary'),
        detail: t('users.toast.passwordResetManualDetail', {
          username: passwordResetUser.value.username,
        }),
        life: 3000,
      })
    }
  } catch (e: unknown) {
    console.error('Failed to reset password:', e)
    const message = createErrorState(e, t('users.toast.passwordFailedFallback')).message
    toast.add({
      severity: 'error',
      summary: t('users.toast.errorSummary'),
      detail: message,
      life: 5000,
    })
  } finally {
    saving.value = false
  }
}

function copyPassword() {
  if (generatedPassword.value) {
    navigator.clipboard.writeText(generatedPassword.value)
    toast.add({
      severity: 'info',
      summary: t('users.toast.copiedSummary'),
      detail: t('users.toast.copiedDetail'),
      life: 2000,
    })
  }
}

function closePasswordDialog() {
  showPasswordDialog.value = false
  generatedPassword.value = null
  passwordResetUser.value = null
}

function getRoleSeverity(
  role: string,
): 'success' | 'warn' | 'danger' | 'secondary' | 'info' | 'contrast' | undefined {
  switch (role) {
    case 'admin':
      return 'danger'
    case 'instructor':
      return 'warn'
    case 'student':
      return 'info'
    default:
      return 'secondary'
  }
}

// formatDate imported from @/utils/format

function clearFilters() {
  searchQuery.value = ''
  filterRole.value = { label: t('users.roleOptions.all'), value: null }
  filterActive.value = { label: t('users.statusOptions.all'), value: null }
  loadUsers()
}
</script>

<template>
  <div class="space-y-6">
    <ConfirmDialog />

    <!-- Access denied message for non-admins -->
    <Message v-if="!isAdmin" severity="error" :closable="false">
      <i class="pi pi-lock mr-2" />
      {{ t('users.accessDenied') }}
    </Message>

    <template v-else>
      <!-- Header -->
      <div class="flex items-center justify-between">
        <div>
          <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">
            {{ t('users.title') }}
          </h1>
          <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('users.subtitle') }}</p>
        </div>
        <div class="flex gap-2">
          <Button
            @click="loadUsers"
            :loading="loading"
            :disabled="loading"
            icon="pi pi-refresh"
            severity="secondary"
          />
          <Button @click="openCreateDialog" icon="pi pi-plus" :label="t('users.addUser')" />
        </div>
      </div>

      <!-- Filters -->
      <Card>
        <template #content>
          <h2 class="text-sm font-medium text-surface-700 dark:text-surface-300 mb-3">
            {{ t('users.filters.heading') }}
          </h2>
          <div class="flex flex-wrap gap-4 items-end">
            <div class="flex-1 min-w-[200px]">
              <label
                for="user-search"
                class="block text-sm text-surface-600 dark:text-surface-400 mb-1"
                >{{ t('users.filters.searchLabel') }}</label
              >
              <InputText
                id="user-search"
                v-model="searchQuery"
                :placeholder="t('users.filters.searchPlaceholder')"
                class="w-full"
                @keyup.enter="loadUsers"
              />
            </div>
            <div class="w-[150px]">
              <label class="block text-sm text-surface-600 dark:text-surface-400 mb-1">{{
                t('users.filters.roleLabel')
              }}</label>
              <!-- eslint-disable-next-line vuejs-accessibility/no-onchange -- @change on <Select> fires on user selection; @blur would re-fire on every focus-out without a selection change. -->
              <Select
                v-model="filterRole"
                :options="roleOptions"
                optionLabel="label"
                :aria-label="t('users.filters.roleAria')"
                class="w-full"
                @change="loadUsers"
              />
            </div>
            <div class="w-[150px]">
              <label class="block text-sm text-surface-600 dark:text-surface-400 mb-1">{{
                t('users.filters.statusLabel')
              }}</label>
              <!-- eslint-disable-next-line vuejs-accessibility/no-onchange -- @change on <Select> fires on user selection; @blur would re-fire on every focus-out without a selection change. -->
              <Select
                v-model="filterActive"
                :options="statusOptions"
                optionLabel="label"
                :aria-label="t('users.filters.statusAria')"
                class="w-full"
                @change="loadUsers"
              />
            </div>
            <Button
              @click="clearFilters"
              icon="pi pi-filter-slash"
              severity="secondary"
              text
              :label="t('users.filters.clear')"
            />
          </div>
        </template>
      </Card>

      <!-- Error Message -->
      <Message v-if="error" severity="error" :closable="false">
        {{ error }}
      </Message>

      <!-- Loading State -->
      <div v-if="loading" class="flex justify-center py-12">
        <ProgressSpinner />
      </div>

      <!-- Users Table -->
      <Card v-else-if="users.length > 0">
        <template #content>
          <div class="flex justify-between items-center mb-4">
            <h2 class="text-lg font-semibold text-surface-900 dark:text-surface-100">
              {{ t('users.table.heading', { count: total }) }}
            </h2>
          </div>

          <div class="overflow-x-auto">
            <table class="w-full">
              <thead>
                <tr class="border-b border-surface-200 dark:border-surface-700">
                  <th
                    class="text-left py-3 px-4 font-medium text-surface-600 dark:text-surface-400"
                  >
                    {{ t('users.table.user') }}
                  </th>
                  <th
                    class="text-left py-3 px-4 font-medium text-surface-600 dark:text-surface-400"
                  >
                    {{ t('users.table.email') }}
                  </th>
                  <th
                    class="text-left py-3 px-4 font-medium text-surface-600 dark:text-surface-400"
                  >
                    {{ t('users.table.role') }}
                  </th>
                  <th
                    class="text-left py-3 px-4 font-medium text-surface-600 dark:text-surface-400"
                  >
                    {{ t('users.table.status') }}
                  </th>
                  <th
                    class="text-left py-3 px-4 font-medium text-surface-600 dark:text-surface-400"
                  >
                    {{ t('users.table.created') }}
                  </th>
                  <th
                    class="text-right py-3 px-4 font-medium text-surface-600 dark:text-surface-400"
                  >
                    {{ t('users.table.actions') }}
                  </th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="user in users"
                  :key="user.id"
                  class="border-b border-surface-100 dark:border-surface-800 hover:bg-surface-50 dark:hover:bg-surface-800/50"
                >
                  <td class="py-3 px-4">
                    <div class="flex items-center gap-3">
                      <div
                        class="w-10 h-10 rounded-full bg-primary/10 flex items-center justify-center"
                      >
                        <i class="pi pi-user text-primary" />
                      </div>
                      <div>
                        <div class="font-medium text-surface-900 dark:text-surface-100">
                          {{ user.displayName || user.username }}
                        </div>
                        <div class="text-sm text-surface-500">
                          {{ t('users.table.usernameHandle', { username: user.username }) }}
                        </div>
                      </div>
                    </div>
                  </td>
                  <td class="py-3 px-4 text-surface-600 dark:text-surface-400">
                    {{ user.email || t('users.table.emailPlaceholder') }}
                  </td>
                  <td class="py-3 px-4">
                    <Tag :severity="getRoleSeverity(user.role)" :value="getRoleLabel(user.role)" />
                  </td>
                  <td class="py-3 px-4">
                    <Tag
                      :severity="user.isActive ? 'success' : 'secondary'"
                      :value="
                        user.isActive
                          ? t('users.table.statusActive')
                          : t('users.table.statusInactive')
                      "
                    />
                  </td>
                  <td class="py-3 px-4 text-surface-600 dark:text-surface-400">
                    {{ formatDate(user.createdAt) }}
                  </td>
                  <td class="py-3 px-4">
                    <div class="flex justify-end gap-2">
                      <Button
                        @click="openEditDialog(user)"
                        icon="pi pi-pencil"
                        severity="secondary"
                        text
                        rounded
                        v-tooltip.top="t('users.table.editTooltip')"
                      />
                      <Button
                        @click="openPasswordDialog(user)"
                        icon="pi pi-key"
                        severity="info"
                        text
                        rounded
                        v-tooltip.top="t('users.table.resetPasswordTooltip')"
                      />
                      <Button
                        @click="toggleUserStatus(user)"
                        :icon="user.isActive ? 'pi pi-ban' : 'pi pi-check'"
                        :severity="user.isActive ? 'warn' : 'success'"
                        text
                        rounded
                        v-tooltip.top="
                          user.isActive
                            ? t('users.table.deactivateTooltip')
                            : t('users.table.activateTooltip')
                        "
                      />
                      <Button
                        @click="deleteUser(user)"
                        icon="pi pi-trash"
                        severity="danger"
                        text
                        rounded
                        v-tooltip.top="t('users.table.deleteTooltip')"
                      />
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </template>
      </Card>

      <!-- Empty State -->
      <Card v-else>
        <template #content>
          <div class="text-center py-12">
            <i class="pi pi-users text-4xl text-surface-400 mb-4" />
            <h3 class="text-lg font-medium text-surface-700 dark:text-surface-300 mb-2">
              {{ t('users.empty.title') }}
            </h3>
            <p class="text-surface-500 mb-4">
              {{
                searchQuery || filterRole.value || filterActive.value !== null
                  ? t('users.empty.adjustFilters')
                  : t('users.empty.getStarted')
              }}
            </p>
            <Button
              v-if="!searchQuery && !filterRole.value && filterActive.value === null"
              @click="openCreateDialog"
              icon="pi pi-plus"
              :label="t('users.addUser')"
            />
          </div>
        </template>
      </Card>
    </template>

    <!-- Create/Edit User Dialog -->
    <UserFormDialog
      v-model:visible="showFormDialog"
      v-model:formData="formData"
      :mode="formMode"
      :saving="saving"
      @submit="handleFormSubmit"
    />

    <!-- Password Reset Dialog -->
    <PasswordResetDialog
      v-model:visible="showPasswordDialog"
      v-model:form="passwordResetForm"
      :username="passwordResetUser?.username ?? ''"
      :saving="saving"
      :generatedPassword="generatedPassword"
      @submit="resetPassword"
      @copy-password="copyPassword"
      @close="closePasswordDialog"
    />
  </div>
</template>
