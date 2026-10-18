<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '../stores/auth'
import { passwordApi } from '@/api'
import Card from '@volt/Card.vue'
import Password from '@volt/Password.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const error = ref<string | null>(null)
const success = ref<string | null>(null)
const loading = ref(false)

// Password strength validation
const passwordRequirements = computed(() => {
  const pwd = newPassword.value
  return {
    minLength: pwd.length >= 8,
    hasUppercase: /[A-Z]/.test(pwd),
    hasLowercase: /[a-z]/.test(pwd),
    hasDigit: /[0-9]/.test(pwd),
  }
})

const isPasswordValid = computed(() => {
  return Object.values(passwordRequirements.value).every(v => v)
})

const passwordsMatch = computed(() => {
  return newPassword.value === confirmPassword.value
})

const canSubmit = computed(() => {
  return (
    currentPassword.value &&
    newPassword.value &&
    confirmPassword.value &&
    isPasswordValid.value &&
    passwordsMatch.value
  )
})

async function handleChangePassword() {
  if (!canSubmit.value) return

  loading.value = true
  error.value = null
  success.value = null

  try {
    await passwordApi.change({
      currentPassword: currentPassword.value,
      newPassword: newPassword.value,
    })

    success.value = t('changePassword.successMessage')

    // Clear the mustChangePassword flag
    authStore.clearMustChangePassword()

    // Clear form
    currentPassword.value = ''
    newPassword.value = ''
    confirmPassword.value = ''

    // Redirect after short delay
    setTimeout(() => {
      router.push('/')
    }, 1500)
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } }; message?: string }
    error.value = err.response?.data?.error || err.message || t('changePassword.genericError')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-surface-50 dark:bg-surface-950 py-12 px-4 sm:px-6 lg:px-8">
    <div class="max-w-md w-full">
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ t('changePassword.title') }}</h1>
        <p v-if="authStore.mustChangePassword" class="mt-2 text-amber-600 dark:text-amber-400">
          {{ t('changePassword.mustChangeNotice') }}
        </p>
        <p v-else class="mt-2 text-surface-600 dark:text-surface-400">
          {{ t('changePassword.subtitle') }}
        </p>
      </div>

      <Card>
        <template #content>
          <form class="space-y-6" @submit.prevent="handleChangePassword">
            <!-- Success message -->
            <Message v-if="success" severity="success" :closable="false">
              {{ success }}
            </Message>

            <!-- Error message -->
            <Message v-if="error" severity="error" :closable="false">
              {{ error }}
            </Message>

            <div class="space-y-4">
              <div class="flex flex-col gap-2">
                <label for="currentPassword" class="text-sm font-medium text-surface-700 dark:text-surface-300">
                  {{ t('changePassword.fields.currentLabel') }}
                </label>
                <Password
                  id="currentPassword"
                  v-model="currentPassword"
                  :placeholder="t('changePassword.fields.currentPlaceholder')"
                  :feedback="false"
                  toggleMask
                  class="w-full"
                  inputClass="w-full"
                />
              </div>

              <div class="flex flex-col gap-2">
                <label for="newPassword" class="text-sm font-medium text-surface-700 dark:text-surface-300">
                  {{ t('changePassword.fields.newLabel') }}
                </label>
                <Password
                  id="newPassword"
                  v-model="newPassword"
                  :placeholder="t('changePassword.fields.newPlaceholder')"
                  :feedback="false"
                  toggleMask
                  class="w-full"
                  inputClass="w-full"
                />

                <!-- Password requirements -->
                <div class="mt-2 text-sm space-y-1">
                  <div :class="passwordRequirements.minLength ? 'text-green-600' : 'text-surface-400'">
                    <i :class="passwordRequirements.minLength ? 'pi pi-check' : 'pi pi-times'" class="mr-2" />
                    {{ t('passwordRequirements.minLength') }}
                  </div>
                  <div :class="passwordRequirements.hasUppercase ? 'text-green-600' : 'text-surface-400'">
                    <i :class="passwordRequirements.hasUppercase ? 'pi pi-check' : 'pi pi-times'" class="mr-2" />
                    {{ t('passwordRequirements.hasUppercase') }}
                  </div>
                  <div :class="passwordRequirements.hasLowercase ? 'text-green-600' : 'text-surface-400'">
                    <i :class="passwordRequirements.hasLowercase ? 'pi pi-check' : 'pi pi-times'" class="mr-2" />
                    {{ t('passwordRequirements.hasLowercase') }}
                  </div>
                  <div :class="passwordRequirements.hasDigit ? 'text-green-600' : 'text-surface-400'">
                    <i :class="passwordRequirements.hasDigit ? 'pi pi-check' : 'pi pi-times'" class="mr-2" />
                    {{ t('passwordRequirements.hasDigit') }}
                  </div>
                </div>
              </div>

              <div class="flex flex-col gap-2">
                <label for="confirmPassword" class="text-sm font-medium text-surface-700 dark:text-surface-300">
                  {{ t('changePassword.fields.confirmLabel') }}
                </label>
                <Password
                  id="confirmPassword"
                  v-model="confirmPassword"
                  :placeholder="t('changePassword.fields.confirmPlaceholder')"
                  :feedback="false"
                  toggleMask
                  class="w-full"
                  inputClass="w-full"
                />
                <div v-if="confirmPassword && !passwordsMatch" class="text-sm text-red-600">
                  {{ t('changePassword.fields.mismatchError') }}
                </div>
              </div>
            </div>

            <div class="space-y-4">
              <Button
                type="submit"
                :loading="loading"
                :disabled="loading || !canSubmit"
                :label="t('changePassword.submit')"
                class="w-full"
              />

              <div v-if="!authStore.mustChangePassword" class="text-center">
                <Button
                  type="button"
                  @click="router.push('/')"
                  :disabled="loading"
                  :label="t('changePassword.cancel')"
                  link
                />
              </div>
            </div>
          </form>
        </template>
      </Card>
    </div>
  </div>
</template>
