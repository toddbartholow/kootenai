<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { passwordApi } from '@/api'

const { t } = useI18n()

const email = ref('')
const isLoading = ref(false)
const error = ref('')
const success = ref(false)

async function handleSubmit() {
  if (!email.value) {
    error.value = t('forgotPassword.emailRequired')
    return
  }

  isLoading.value = true
  error.value = ''

  try {
    await passwordApi.requestReset(email.value)
    success.value = true
  } catch {
    // Always show success message to prevent email enumeration
    // The API also returns success even for non-existent emails
    success.value = true
  } finally {
    isLoading.value = false
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-gray-900 px-4">
    <div class="max-w-md w-full">
      <!-- Logo/Brand -->
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold text-white">{{ t('app.brand') }}</h1>
        <p class="text-gray-400 mt-2">{{ t('forgotPassword.subtitle') }}</p>
      </div>

      <!-- Success Message -->
      <div v-if="success" class="bg-gray-800 rounded-lg shadow-xl p-8">
        <div class="text-center">
          <div class="mx-auto flex items-center justify-center h-12 w-12 rounded-full bg-green-100 mb-4">
            <svg class="h-6 w-6 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
            </svg>
          </div>
          <h2 class="text-xl font-semibold text-white mb-2">{{ t('forgotPassword.success.heading') }}</h2>
          <p class="text-gray-400 mb-6">
            {{ t('forgotPassword.success.body', { email }) }}
          </p>
          <p class="text-sm text-gray-500 mb-6">
            {{ t('forgotPassword.success.spamHint') }}
          </p>
          <div class="space-y-3">
            <button
              @click="success = false; email = ''"
              class="w-full py-2 px-4 border border-gray-600 rounded-lg text-gray-300 hover:bg-gray-700 transition-colors"
            >
              {{ t('forgotPassword.success.tryAnother') }}
            </button>
            <router-link
              to="/login"
              class="block w-full py-2 px-4 bg-blue-600 hover:bg-blue-700 text-white rounded-lg text-center transition-colors"
            >
              {{ t('forgotPassword.success.backToLogin') }}
            </router-link>
          </div>
        </div>
      </div>

      <!-- Request Form -->
      <div v-else class="bg-gray-800 rounded-lg shadow-xl p-8">
        <form @submit.prevent="handleSubmit" class="space-y-6">
          <div>
            <p class="text-gray-400 text-sm mb-4">
              {{ t('forgotPassword.form.intro') }}
            </p>
          </div>

          <!-- Error Message -->
          <div v-if="error" class="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-lg text-sm">
            {{ error }}
          </div>

          <!-- Email Field -->
          <div>
            <label for="email" class="block text-sm font-medium text-gray-300 mb-2">
              {{ t('forgotPassword.form.emailLabel') }}
            </label>
            <input
              id="email"
              v-model="email"
              type="email"
              required
              autocomplete="email"
              class="w-full px-4 py-3 bg-gray-700 border border-gray-600 rounded-lg text-white placeholder-gray-400 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors"
              :placeholder="t('forgotPassword.form.emailPlaceholder')"
              :disabled="isLoading"
            />
          </div>

          <!-- Submit Button -->
          <button
            type="submit"
            :disabled="isLoading"
            class="w-full py-3 px-4 bg-blue-600 hover:bg-blue-700 disabled:bg-blue-600/50 disabled:cursor-not-allowed text-white font-medium rounded-lg transition-colors flex items-center justify-center"
          >
            <svg
              v-if="isLoading"
              class="animate-spin -ml-1 mr-3 h-5 w-5 text-white"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            {{ isLoading ? t('forgotPassword.form.sending') : t('forgotPassword.form.submit') }}
          </button>

          <!-- Back to Login -->
          <div class="text-center">
            <router-link
              to="/login"
              class="text-sm text-blue-400 hover:text-blue-300 transition-colors"
            >
              {{ t('forgotPassword.form.backToLogin') }}
            </router-link>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>
