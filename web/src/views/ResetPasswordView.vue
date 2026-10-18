<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { passwordApi } from '@/api'

const { t } = useI18n()
const route = useRoute()

const token = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const isLoading = ref(false)
const error = ref('')
const success = ref(false)
const showPassword = ref(false)

// Password strength validation
const passwordStrength = computed(() => {
  const password = newPassword.value
  if (!password) return { score: 0, label: '', color: '' }

  let score = 0
  if (password.length >= 8) score++
  if (password.length >= 12) score++
  if (/[a-z]/.test(password)) score++
  if (/[A-Z]/.test(password)) score++
  if (/[0-9]/.test(password)) score++
  if (/[^a-zA-Z0-9]/.test(password)) score++

  if (score <= 2) return { score, label: t('resetPassword.strength.weak'), color: 'bg-red-500' }
  if (score <= 4) return { score, label: t('resetPassword.strength.fair'), color: 'bg-yellow-500' }
  return { score, label: t('resetPassword.strength.strong'), color: 'bg-green-500' }
})

const passwordRequirements = computed(() => {
  const password = newPassword.value
  return {
    length: password.length >= 8,
    uppercase: /[A-Z]/.test(password),
    lowercase: /[a-z]/.test(password),
    number: /[0-9]/.test(password),
  }
})

const canSubmit = computed(() => {
  return (
    token.value &&
    newPassword.value &&
    confirmPassword.value &&
    newPassword.value === confirmPassword.value &&
    passwordRequirements.value.length &&
    passwordRequirements.value.uppercase &&
    passwordRequirements.value.lowercase &&
    passwordRequirements.value.number
  )
})

onMounted(() => {
  // Get token from URL query parameter
  const urlToken = route.query['token'] as string | undefined
  if (urlToken) {
    token.value = urlToken
  }
})

async function handleSubmit() {
  if (!canSubmit.value) {
    error.value = t('resetPassword.validation.fillFields')
    return
  }

  if (newPassword.value !== confirmPassword.value) {
    error.value = t('resetPassword.validation.mismatchField')
    return
  }

  isLoading.value = true
  error.value = ''

  try {
    await passwordApi.confirmReset(token.value, newPassword.value)
    success.value = true
  } catch (err: unknown) {
    if (err && typeof err === 'object' && 'response' in err) {
      const axiosError = err as { response?: { data?: { error?: string } } }
      error.value = axiosError.response?.data?.error || t('resetPassword.errors.expired')
    } else {
      error.value = t('resetPassword.errors.generic')
    }
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <main class="min-h-screen flex items-center justify-center bg-gray-900 px-4" aria-labelledby="page-title">
    <div class="max-w-md w-full">
      <!-- Logo/Brand -->
      <div class="text-center mb-8">
        <h1 id="page-title" class="text-3xl font-bold text-white">{{ t('app.brand') }}</h1>
        <p class="text-gray-400 mt-2">{{ t('resetPassword.subtitle') }}</p>
      </div>

      <!-- Success Message -->
      <div v-if="success" class="bg-gray-800 rounded-lg shadow-xl p-8" role="status" aria-live="polite">
        <div class="text-center">
          <div class="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-green-100 mb-4" aria-hidden="true">
            <svg class="h-6 w-6 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
          </div>
          <h2 class="text-xl font-semibold text-white mb-2">{{ t('resetPassword.success.heading') }}</h2>
          <p class="text-gray-400 mb-6">
            {{ t('resetPassword.success.body') }}
          </p>
          <router-link
            to="/login"
            class="block w-full py-3 px-4 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg text-center transition-colors"
          >
            {{ t('resetPassword.success.goToLogin') }}
          </router-link>
        </div>
      </div>

      <!-- No Token Message -->
      <div v-else-if="!token" class="bg-gray-800 rounded-lg shadow-xl p-8" role="alert" aria-live="assertive">
        <div class="text-center">
          <div class="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-red-100 mb-4" aria-hidden="true">
            <svg class="h-6 w-6 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <h2 class="text-xl font-semibold text-white mb-2">{{ t('resetPassword.invalid.heading') }}</h2>
          <p class="text-gray-400 mb-6">
            {{ t('resetPassword.invalid.body') }}
          </p>
          <router-link
            to="/forgot-password"
            class="block w-full py-3 px-4 bg-blue-600 hover:bg-blue-700 text-white font-medium rounded-lg text-center transition-colors"
          >
            {{ t('resetPassword.invalid.requestNew') }}
          </router-link>
        </div>
      </div>

      <!-- Reset Form -->
      <div v-else class="bg-gray-800 rounded-lg shadow-xl p-8">
        <h2 class="sr-only">{{ t('resetPassword.srForm') }}</h2>
        <form @submit.prevent="handleSubmit" class="space-y-6" :aria-label="t('resetPassword.formAria')">
          <!-- Error Message -->
          <div
            v-if="error"
            class="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-lg text-sm"
            role="alert"
            aria-live="assertive"
          >
            {{ error }}
          </div>

          <!-- New Password Field -->
          <div>
            <label for="newPassword" class="block text-sm font-medium text-gray-300 mb-2">
              {{ t('resetPassword.newPasswordLabel') }}
              <span class="text-red-400" aria-hidden="true">{{ t('resetPassword.requiredMarker') }}</span>
              <span class="sr-only">{{ t('resetPassword.requiredHint') }}</span>
            </label>
            <div class="relative">
              <input
                id="newPassword"
                v-model="newPassword"
                :type="showPassword ? 'text' : 'password'"
                required
                aria-required="true"
                autocomplete="new-password"
                aria-describedby="password-strength password-requirements"
                class="w-full px-4 py-3 bg-gray-700 border border-gray-600 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors pr-12"
                :placeholder="t('resetPassword.newPasswordPlaceholder')"
                :disabled="isLoading"
              />
              <button
                type="button"
                @click="showPassword = !showPassword"
                class="absolute inset-y-0 right-0 flex items-center px-3 text-gray-400 hover:text-gray-300"
                :aria-label="showPassword ? t('resetPassword.hidePasswordAria') : t('resetPassword.showPasswordAria')"
                :aria-pressed="showPassword"
              >
                <svg v-if="showPassword" class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21" />
                </svg>
                <svg v-else class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
                </svg>
              </button>
            </div>

            <!-- Password Strength Indicator -->
            <div v-if="newPassword" class="mt-2" id="password-strength">
              <div class="flex items-center gap-2">
                <div
                  class="flex-1 h-1.5 bg-gray-600 rounded-full overflow-hidden"
                  role="progressbar"
                  :aria-valuenow="passwordStrength.score"
                  aria-valuemin="0"
                  aria-valuemax="6"
                  :aria-label="t('resetPassword.strength.aria', { label: passwordStrength.label || t('resetPassword.strength.ariaUnevaluated') })"
                >
                  <div
                    class="h-full transition-all duration-300"
                    :class="passwordStrength.color"
                    :style="{ width: `${(passwordStrength.score / 6) * 100}%` }"
                  ></div>
                </div>
                <span class="text-xs" aria-hidden="true" :class="{
                  'text-red-400': passwordStrength.score <= 2,
                  'text-yellow-400': passwordStrength.score > 2 && passwordStrength.score <= 4,
                  'text-green-400': passwordStrength.score > 4,
                }">{{ passwordStrength.label }}</span>
              </div>
              <!-- Live region for screen reader announcements -->
              <div class="sr-only" aria-live="polite" aria-atomic="true">
                {{ t('resetPassword.strength.srAnnounce', { label: passwordStrength.label }) }}
              </div>
            </div>

            <!-- Password Requirements -->
            <ul id="password-requirements" class="mt-3 space-y-1 text-xs" :aria-label="t('resetPassword.requirements.aria')">
              <li class="flex items-center gap-2" :class="passwordRequirements.length ? 'text-green-400' : 'text-gray-500'">
                <svg class="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                  <path v-if="passwordRequirements.length" fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  <path v-else fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
                </svg>
                <span>{{ t('passwordRequirements.minLength') }}</span>
                <span class="sr-only">{{ passwordRequirements.length ? t('resetPassword.requirements.met') : t('resetPassword.requirements.notMet') }}</span>
              </li>
              <li class="flex items-center gap-2" :class="passwordRequirements.uppercase ? 'text-green-400' : 'text-gray-500'">
                <svg class="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                  <path v-if="passwordRequirements.uppercase" fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  <path v-else fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
                </svg>
                <span>{{ t('passwordRequirements.hasUppercase') }}</span>
                <span class="sr-only">{{ passwordRequirements.uppercase ? t('resetPassword.requirements.met') : t('resetPassword.requirements.notMet') }}</span>
              </li>
              <li class="flex items-center gap-2" :class="passwordRequirements.lowercase ? 'text-green-400' : 'text-gray-500'">
                <svg class="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                  <path v-if="passwordRequirements.lowercase" fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  <path v-else fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
                </svg>
                <span>{{ t('passwordRequirements.hasLowercase') }}</span>
                <span class="sr-only">{{ passwordRequirements.lowercase ? t('resetPassword.requirements.met') : t('resetPassword.requirements.notMet') }}</span>
              </li>
              <li class="flex items-center gap-2" :class="passwordRequirements.number ? 'text-green-400' : 'text-gray-500'">
                <svg class="h-3.5 w-3.5" fill="currentColor" viewBox="0 0 20 20" aria-hidden="true">
                  <path v-if="passwordRequirements.number" fill-rule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clip-rule="evenodd" />
                  <path v-else fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
                </svg>
                <span>{{ t('resetPassword.requirements.number') }}</span>
                <span class="sr-only">{{ passwordRequirements.number ? t('resetPassword.requirements.met') : t('resetPassword.requirements.notMet') }}</span>
              </li>
            </ul>
          </div>

          <!-- Confirm Password Field -->
          <div>
            <label for="confirmPassword" class="block text-sm font-medium text-gray-300 mb-2">
              {{ t('resetPassword.confirmLabel') }}
              <span class="text-red-400" aria-hidden="true">{{ t('resetPassword.requiredMarker') }}</span>
              <span class="sr-only">{{ t('resetPassword.requiredHint') }}</span>
            </label>
            <input
              id="confirmPassword"
              v-model="confirmPassword"
              :type="showPassword ? 'text' : 'password'"
              required
              aria-required="true"
              autocomplete="new-password"
              :aria-invalid="confirmPassword && confirmPassword !== newPassword ? 'true' : undefined"
              :aria-describedby="confirmPassword && confirmPassword !== newPassword ? 'confirm-password-error' : undefined"
              class="w-full px-4 py-3 bg-gray-700 border border-gray-600 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors"
              :class="{
                'border-red-500': confirmPassword && confirmPassword !== newPassword,
                'border-green-500': confirmPassword && confirmPassword === newPassword,
              }"
              :placeholder="t('resetPassword.confirmPlaceholder')"
              :disabled="isLoading"
            />
            <p
              v-if="confirmPassword && confirmPassword !== newPassword"
              id="confirm-password-error"
              class="mt-1 text-xs text-red-400"
              role="alert"
            >
              {{ t('resetPassword.mismatchError') }}
            </p>
          </div>

          <!-- Submit Button -->
          <button
            type="submit"
            :disabled="isLoading || !canSubmit"
            :aria-disabled="isLoading || !canSubmit"
            :aria-busy="isLoading"
            :aria-label="isLoading ? t('resetPassword.resettingAria') : t('resetPassword.submitAria')"
            class="w-full py-3 px-4 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-600/50 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-colors flex items-center justify-center"
          >
            <svg
              v-if="isLoading"
              class="animate-spin -ml-1 mr-3 h-5 w-5 text-white"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              aria-hidden="true"
              role="status"
            >
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <span aria-hidden="true">{{ isLoading ? t('resetPassword.resetting') : t('resetPassword.submit') }}</span>
            <span v-if="isLoading" class="sr-only">{{ t('resetPassword.submitSrAnnounce') }}</span>
          </button>

          <!-- Back to Login -->
          <div class="text-center">
            <router-link
              to="/login"
              class="text-sm text-blue-400 hover:text-blue-300 transition-colors"
            >
              {{ t('resetPassword.backToLogin') }}
            </router-link>
          </div>
        </form>
      </div>
    </div>
  </main>
</template>
