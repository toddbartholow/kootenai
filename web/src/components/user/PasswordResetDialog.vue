<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Dialog from '@volt/Dialog.vue'
import Button from '@volt/Button.vue'
import Checkbox from '@volt/Checkbox.vue'
import Password from '@volt/Password.vue'
import Message from '@volt/Message.vue'

export interface PasswordResetForm {
  generatePassword: boolean
  manualPassword: string
  mustChangePassword: boolean
}

const props = defineProps<{
  visible: boolean
  username: string
  saving: boolean
  form: PasswordResetForm
  generatedPassword: string | null
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'update:form', value: PasswordResetForm): void
  (e: 'submit'): void
  (e: 'copy-password'): void
  (e: 'close'): void
}>()

const { t } = useI18n()

function updateField<K extends keyof PasswordResetForm>(key: K, value: PasswordResetForm[K]) {
  emit('update:form', { ...props.form, [key]: value })
}
</script>

<template>
  <Dialog
    :visible="visible"
    @update:visible="emit('update:visible', $event)"
    :header="t('users.passwordReset.header', { username })"
    :style="{ width: '450px' }"
    :modal="true"
    :closable="!saving"
    @hide="emit('close')"
  >
    <div class="space-y-4">
      <!-- Show generated password if available -->
      <div v-if="generatedPassword" class="space-y-4">
        <Message severity="success" :closable="false">
          {{ t('users.passwordReset.successBanner') }}
        </Message>

        <div class="p-4 bg-surface-100 dark:bg-surface-800 rounded-lg">
          <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-2">
            {{ t('users.passwordReset.temporaryPasswordLabel') }}
          </label>
          <div class="flex items-center gap-2">
            <code
              class="flex-1 p-3 bg-surface-50 dark:bg-surface-900 border border-surface-200 dark:border-surface-700 rounded font-mono text-lg"
            >
              {{ generatedPassword }}
            </code>
            <Button
              @click="emit('copy-password')"
              icon="pi pi-copy"
              severity="secondary"
              v-tooltip.top="t('users.passwordReset.copyTooltip')"
            />
          </div>
        </div>

        <Message severity="info" :closable="false">
          <div class="text-sm">
            <i class="pi pi-info-circle mr-2" />
            {{ t('users.passwordReset.changeOnLoginInfo') }}
          </div>
        </Message>
      </div>

      <!-- Password reset options -->
      <div v-else class="space-y-4">
        <div class="flex items-center gap-3">
          <Checkbox
            :modelValue="form.generatePassword"
            @update:modelValue="updateField('generatePassword', $event as boolean)"
            inputId="generatePassword"
            :binary="true"
            :disabled="saving"
          />
          <label for="generatePassword" class="text-surface-700 dark:text-surface-300">
            {{ t('users.passwordReset.generateCheckboxLabel') }}
          </label>
        </div>

        <div v-if="!form.generatePassword" class="space-y-2">
          <label class="block text-sm font-medium text-surface-700 dark:text-surface-300">
            {{ t('users.passwordReset.manualPasswordLabel') }}
          </label>
          <Password
            :modelValue="form.manualPassword"
            @update:modelValue="updateField('manualPassword', $event)"
            :placeholder="t('users.passwordReset.manualPasswordPlaceholder')"
            :feedback="true"
            toggleMask
            class="w-full"
            inputClass="w-full"
            :disabled="saving"
          />
          <p class="text-xs text-surface-500">
            {{ t('users.passwordReset.manualPasswordHint') }}
          </p>
        </div>

        <Message severity="warn" :closable="false">
          <div class="text-sm">
            <i class="pi pi-exclamation-triangle mr-2" />
            {{ t('users.passwordReset.mustChangeWarning') }}
          </div>
        </Message>
      </div>
    </div>

    <template #footer>
      <Button
        @click="emit('close')"
        :label="generatedPassword ? t('users.passwordReset.done') : t('users.passwordReset.cancel')"
        severity="secondary"
        :disabled="saving"
      />
      <Button
        v-if="!generatedPassword"
        @click="emit('submit')"
        :label="t('users.passwordReset.submit')"
        icon="pi pi-key"
        :loading="saving"
      />
    </template>
  </Dialog>
</template>
