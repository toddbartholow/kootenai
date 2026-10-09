<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useOrganizationStore } from '@/stores/organization'
import { organizationsApi } from '@/api/domains/organizations'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Tag from '@volt/Tag.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const orgStore = useOrganizationStore()

const saving = ref(false)

const orgId = computed(() => route.params['orgId'] as string)
const organization = computed(() => orgStore.currentOrganization)

// Form state
const name = ref('')
const contactEmail = ref('')

function getEditionSeverity(edition: string): 'primary' | 'secondary' | 'success' | 'info' | 'warn' | 'danger' | 'contrast' | undefined {
  switch (edition) {
    case 'enterprise': return 'primary'
    case 'professional': return 'success'
    default: return 'secondary'
  }
}

async function saveSettings() {
  saving.value = true
  try {
    await organizationsApi.update(orgId.value, { name: name.value, contactEmail: contactEmail.value })
    await orgStore.selectOrganization(orgId.value)
  } catch (e) {
    console.error('Failed to save settings:', e)
  } finally {
    saving.value = false
  }
}

function goBack() {
  router.push(`/organizations/${orgId.value}`)
}

onMounted(async () => {
  if (orgId.value && orgId.value !== orgStore.currentOrganization?.id) {
    await orgStore.selectOrganization(orgId.value)
  }

  // Initialize form with current values
  if (organization.value) {
    name.value = organization.value.name
    contactEmail.value = organization.value.contactEmail || ''
  }
})
</script>

<template>
  <div class="p-6">
    <!-- Header -->
    <div class="flex items-center gap-4 mb-6">
      <Button
        icon="pi pi-arrow-left"
        severity="secondary"
        text
        rounded
        @click="goBack"
      />
      <div>
        <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-0">
          {{ t('organization.settings.title') }}
        </h1>
        <p class="text-surface-500 dark:text-surface-400 mt-1">
          {{ t('organization.settings.subtitle', { name: organization?.name ?? '' }) }}
        </p>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-3 gap-6">
      <!-- Main Settings -->
      <div class="lg:col-span-2 space-y-6">
        <Card>
          <template #title>{{ t('organization.settings.generalHeading') }}</template>
          <template #content>
            <div class="space-y-4">
              <div>
                <label class="block text-sm font-medium mb-2">{{ t('organization.settings.nameLabel') }}</label>
                <InputText
                  v-model="name"
                  class="w-full"
                  :placeholder="t('organization.settings.namePlaceholder')"
                />
              </div>
              <div>
                <label class="block text-sm font-medium mb-2">{{ t('organization.settings.slugLabel') }}</label>
                <InputText
                  :modelValue="organization?.slug"
                  class="w-full"
                  disabled
                />
                <p class="text-xs text-surface-500 mt-1">
                  {{ t('organization.settings.slugHelp') }}
                </p>
              </div>
              <div>
                <label class="block text-sm font-medium mb-2">{{ t('organization.settings.contactEmailLabel') }}</label>
                <InputText
                  v-model="contactEmail"
                  type="email"
                  class="w-full"
                  :placeholder="t('organization.settings.contactEmailPlaceholder')"
                />
              </div>
            </div>
          </template>
          <template #footer>
            <div class="flex justify-end gap-2">
              <Button
                :label="t('organization.settings.cancel')"
                severity="secondary"
                @click="goBack"
              />
              <Button
                :label="t('organization.settings.submit')"
                icon="pi pi-check"
                :loading="saving"
                @click="saveSettings"
              />
            </div>
          </template>
        </Card>
      </div>

      <!-- Sidebar -->
      <div class="space-y-6">
        <Card>
          <template #title>{{ t('organization.settings.planHeading') }}</template>
          <template #content>
            <div class="space-y-4">
              <div class="flex items-center justify-between">
                <span class="text-surface-600 dark:text-surface-300">{{ t('organization.settings.editionLabel') }}</span>
                <Tag
                  :severity="getEditionSeverity(organization?.edition || '')"
                  :value="organization?.edition || t('organization.settings.editionFallback')"
                />
              </div>
              <div class="flex items-center justify-between">
                <span class="text-surface-600 dark:text-surface-300">{{ t('organization.settings.maxUsersLabel') }}</span>
                <span class="font-medium">{{ organization?.maxUsers || t('organization.settings.maxUsersUnlimited') }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-surface-600 dark:text-surface-300">{{ t('organization.settings.maxPodsLabel') }}</span>
                <span class="font-medium">{{ organization?.maxConcurrentPods || t('organization.settings.maxPodsUnlimited') }}</span>
              </div>
            </div>
          </template>
          <template #footer>
            <Button
              :label="t('organization.settings.upgradeAction')"
              icon="pi pi-arrow-up"
              class="w-full"
              severity="success"
              outlined
            />
          </template>
        </Card>

        <Card v-if="orgStore.isOwner">
          <template #title>{{ t('organization.settings.dangerHeading') }}</template>
          <template #content>
            <p class="text-sm text-surface-500 mb-4">
              {{ t('organization.settings.dangerBody') }}
            </p>
            <Button
              :label="t('organization.settings.deleteAction')"
              icon="pi pi-trash"
              severity="danger"
              outlined
              class="w-full"
            />
          </template>
        </Card>
      </div>
    </div>
  </div>
</template>
