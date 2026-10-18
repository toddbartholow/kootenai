<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Dialog from '@volt/Dialog.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import { computed } from 'vue'

export interface UserFormData {
  username: string
  email: string
  displayName: string
  role: { label: string; value: string }
}

const props = defineProps<{
  visible: boolean
  mode: 'create' | 'edit'
  formData: UserFormData
  saving: boolean
}>()

const emit = defineEmits<{
  (e: 'update:visible', value: boolean): void
  (e: 'update:formData', value: UserFormData): void
  (e: 'submit'): void
}>()

const { t } = useI18n()

const isCreate = computed(() => props.mode === 'create')

const header = computed(() => (isCreate.value ? t('users.create.header') : t('users.edit.header')))

const submitLabel = computed(() =>
  isCreate.value ? t('users.create.submit') : t('users.edit.submit'),
)

const cancelLabel = computed(() =>
  isCreate.value ? t('users.create.cancel') : t('users.edit.cancel'),
)

const formRoleOptions = computed(() => [
  { label: t('profile.roles.student'), value: 'student' },
  { label: t('profile.roles.instructor'), value: 'instructor' },
  { label: t('profile.roles.admin'), value: 'admin' },
])

function updateField<K extends keyof UserFormData>(key: K, value: UserFormData[K]) {
  emit('update:formData', { ...props.formData, [key]: value })
}
</script>

<template>
  <Dialog
    :visible="visible"
    @update:visible="emit('update:visible', $event)"
    :header="header"
    :style="{ width: '450px' }"
    :modal="true"
  >
    <div class="space-y-4">
      <div>
        <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">
          {{ isCreate ? t('users.create.usernameLabel') : t('users.edit.usernameLabel') }}
          <span v-if="isCreate" class="text-red-500">*</span>
        </label>
        <InputText
          :modelValue="formData.username"
          @update:modelValue="updateField('username', $event)"
          :placeholder="isCreate ? t('users.create.usernamePlaceholder') : undefined"
          class="w-full"
          :disabled="!isCreate || saving"
        />
        <p v-if="!isCreate" class="text-xs text-surface-500 mt-1">
          {{ t('users.edit.usernameDisabledNote') }}
        </p>
      </div>
      <div>
        <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">
          {{ isCreate ? t('users.create.emailLabel') : t('users.edit.emailLabel') }}
        </label>
        <InputText
          :modelValue="formData.email"
          @update:modelValue="updateField('email', $event)"
          :placeholder="
            isCreate ? t('users.create.emailPlaceholder') : t('users.edit.emailPlaceholder')
          "
          class="w-full"
          :disabled="saving"
        />
      </div>
      <div>
        <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">
          {{ isCreate ? t('users.create.displayNameLabel') : t('users.edit.displayNameLabel') }}
        </label>
        <InputText
          :modelValue="formData.displayName"
          @update:modelValue="updateField('displayName', $event)"
          :placeholder="
            isCreate
              ? t('users.create.displayNamePlaceholder')
              : t('users.edit.displayNamePlaceholder')
          "
          class="w-full"
          :disabled="saving"
        />
      </div>
      <div>
        <label class="block text-sm font-medium text-surface-700 dark:text-surface-300 mb-1">
          {{ isCreate ? t('users.create.roleLabel') : t('users.edit.roleLabel') }}
        </label>
        <Select
          :modelValue="formData.role"
          @update:modelValue="updateField('role', $event)"
          :options="formRoleOptions"
          optionLabel="label"
          :aria-label="isCreate ? t('users.create.roleAria') : t('users.edit.roleAria')"
          class="w-full"
          :disabled="saving"
        />
      </div>
    </div>
    <template #footer>
      <Button
        @click="emit('update:visible', false)"
        :label="cancelLabel"
        severity="secondary"
        :disabled="saving"
      />
      <Button @click="emit('submit')" :label="submitLabel" icon="pi pi-check" :loading="saving" />
    </template>
  </Dialog>
</template>
