<script setup lang="ts">
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'

const router = useRouter()
const { t } = useI18n()
</script>

<template>
  <div class="not-found">
    <h1>{{ t('notFound.heading') }}</h1>
    <p>{{ t('notFound.body') }}</p>
    <button class="btn-primary" @click="router.push('/dashboard')">
      {{ t('notFound.backAction') }}
    </button>
  </div>
</template>

<style scoped>
.not-found {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 60vh;
  text-align: center;
}

.not-found h1 {
  font-size: 6rem;
  font-weight: 700;
  color: var(--color-text-muted, #6b7280);
  margin: 0;
}

.not-found p {
  font-size: 1.25rem;
  color: var(--color-text-secondary, #9ca3af);
  margin: 0.5rem 0 2rem;
}

.btn-primary {
  padding: 0.75rem 1.5rem;
  background-color: var(--color-primary, #3b82f6);
  color: white;
  border: none;
  border-radius: 0.5rem;
  font-size: 1rem;
  cursor: pointer;
  transition: background-color 0.2s;
}

.btn-primary:hover {
  background-color: var(--color-primary-dark, #2563eb);
}
</style>
