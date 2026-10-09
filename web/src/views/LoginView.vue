<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { getSafeRedirectPath } from '@/router'
import { useAuthStore } from '../stores/auth'
import { getProviderIcon, getProviderColorClass } from '@/api'
import Card from '@volt/Card.vue'
import InputText from '@volt/InputText.vue'
import Password from '@volt/Password.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { t } = useI18n()

const email = ref('')
const password = ref('')
const error = ref<string | null>(null)
const loading = ref(false)

// Check if there are OAuth2 providers available
const hasOAuth2Providers = computed(() => authStore.oauth2Providers.length > 0)

// Handle OAuth2 callback on mount
onMounted(async () => {
  // Check if we're returning from an OAuth2 provider
  const success = await authStore.handleOAuth2Callback()
  if (success) {
    // OAuth2 login succeeded, redirect
    const redirect = getSafeRedirectPath((route.query['redirect'] as string) || '/')
    router.push(redirect)
    return
  }

  // Check for OAuth2 error in store
  if (authStore.error) {
    error.value = authStore.error
  }

  // Fetch available OAuth2 providers
  await authStore.fetchOAuth2Providers()
})

async function handleLogin() {
  if (!email.value) {
    error.value = t('login.email.missingError')
    return
  }

  loading.value = true
  error.value = null

  try {
    await authStore.login(email.value, password.value)

    // Check if user must change password
    if (authStore.mustChangePassword) {
      router.push('/change-password')
    } else {
      // Redirect to original destination or dashboard
      const redirect = getSafeRedirectPath((route.query['redirect'] as string) || '/')
      router.push(redirect)
    }
  } catch (e: unknown) {
    const err = e as { response?: { data?: { error?: string } }; message?: string }
    error.value = err.response?.data?.error || err.message || t('login.errors.fallback')
  } finally {
    loading.value = false
  }
}

function handleOAuth2Login(providerId: string) {
  const redirect = getSafeRedirectPath((route.query['redirect'] as string) || '/')
  authStore.loginWithOAuth2(providerId, redirect)
}

// Demo login helper
async function handleDemoLogin() {
  email.value = 'demo@example.com'
  password.value = ''
  await handleLogin()
}
</script>

<template>
  <main class="min-h-screen flex items-center justify-center bg-surface-50 dark:bg-surface-950 py-12 px-4 sm:px-6 lg:px-8">
    <div class="max-w-md w-full">
      <div class="text-center mb-8">
        <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ t('login.title') }}</h1>
        <p class="mt-2 text-surface-600 dark:text-surface-400">{{ t('login.subtitle') }}</p>
      </div>

      <Card>
        <template #content>
          <h2 class="sr-only">{{ t('login.formLabel') }}</h2>

          <!-- Error message -->
          <div v-if="error" role="alert" aria-live="assertive" aria-atomic="true" class="mb-6">
            <Message severity="error" :closable="false">
              {{ error }}
            </Message>
          </div>

          <!-- OAuth2 Provider Buttons -->
          <div v-if="hasOAuth2Providers" class="space-y-3 mb-6">
            <p class="text-sm text-center text-surface-500 dark:text-surface-400 mb-4">{{ t('login.oauth.promptHeading') }}</p>
            <button
              v-for="provider in authStore.oauth2Providers"
              :key="provider.id"
              type="button"
              @click="handleOAuth2Login(provider.id)"
              :class="[
                'w-full flex items-center justify-center gap-3 px-4 py-3 rounded-lg font-medium transition-colors',
                getProviderColorClass(provider.type)
              ]"
              :aria-label="t('login.oauth.ariaSignInWith', { provider: provider.name })"
            >
              <i :class="['pi', getProviderIcon(provider.type), 'text-lg']" aria-hidden="true"></i>
              <span>{{ t('login.oauth.continueWith', { provider: provider.name }) }}</span>
            </button>

            <div class="relative my-6">
              <div class="absolute inset-0 flex items-center">
                <div class="w-full border-t border-surface-200 dark:border-surface-700"></div>
              </div>
              <div class="relative flex justify-center text-sm">
                <span class="px-2 bg-white dark:bg-surface-800 text-surface-500">{{ t('login.oauth.divider') }}</span>
              </div>
            </div>
          </div>

          <form class="space-y-6" @submit.prevent="handleLogin" :aria-label="t('login.ariaForm')">
            <div class="space-y-4">
              <div class="flex flex-col gap-2">
                <label for="email" class="text-sm font-medium text-surface-700 dark:text-surface-300">
                  {{ t('login.email.label') }}
                  <span class="text-red-500" aria-hidden="true">*</span>
                  <span class="sr-only">{{ t('login.email.requiredHint') }}</span>
                </label>
                <InputText
                  id="email"
                  v-model="email"
                  type="email"
                  :placeholder="t('login.email.placeholder')"
                  class="w-full"
                  :aria-invalid="error && !email ? 'true' : undefined"
                  :aria-describedby="error && !email ? 'email-error' : undefined"
                  aria-required="true"
                  autocomplete="email"
                />
                <span v-if="error && !email" id="email-error" class="text-sm text-red-500">{{ t('login.email.missingError') }}</span>
              </div>

              <div class="flex flex-col gap-2">
                <div class="flex items-center justify-between">
                  <label for="password" class="text-sm font-medium text-surface-700 dark:text-surface-300">
                    {{ t('login.password.label') }}
                  </label>
                  <router-link
                    to="/forgot-password"
                    class="text-sm text-primary-500 hover:text-primary-600 dark:text-primary-400 dark:hover:text-primary-300"
                  >
                    {{ t('login.password.forgotLink') }}
                  </router-link>
                </div>
                <Password
                  id="password"
                  v-model="password"
                  :placeholder="t('login.password.placeholder')"
                  :feedback="false"
                  toggleMask
                  class="w-full"
                  inputClass="w-full"
                  autocomplete="current-password"
                />
              </div>
            </div>

            <div class="space-y-4">
              <Button
                type="submit"
                :loading="loading"
                :disabled="loading"
                :label="loading ? t('login.submit.pending') : t('login.submit.idle')"
                :aria-label="loading ? t('login.submit.ariaPending') : t('login.submit.ariaIdle')"
                :aria-busy="loading"
                class="w-full"
              />

              <div class="text-center">
                <Button
                  type="button"
                  @click="handleDemoLogin"
                  :disabled="loading"
                  :label="t('login.demo.label')"
                  :aria-label="t('login.demo.aria')"
                  link
                />
              </div>
            </div>
          </form>
        </template>
      </Card>

      <p class="mt-6 text-center text-sm text-surface-500">
        {{ t('login.footerHelp') }}
      </p>
    </div>
  </main>
</template>
