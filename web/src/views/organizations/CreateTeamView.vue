<script setup lang="ts">
import { ref, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useOrganizationStore } from '@/stores/organization'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Textarea from '@volt/Textarea.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const orgStore = useOrganizationStore()

const name = ref('')
const description = ref('')
const creating = ref(false)
const error = ref<string | null>(null)

const orgId = computed(() => route.params['orgId'] as string)

const isValid = computed(() => {
  return name.value.trim().length >= 2
})

async function createTeam() {
  if (!isValid.value) return

  creating.value = true
  error.value = null
  try {
    const descValue = description.value.trim()
    const team = await orgStore.createTeam({
      name: name.value.trim(),
      ...(descValue ? { description: descValue } : {}),
    })
    router.push(`/teams/${team.team.id}`)
  } catch (e) {
    error.value = e instanceof Error ? e.message : t('team.create.loadFailedFallback')
  } finally {
    creating.value = false
  }
}

function goBack() {
  router.push(`/organizations/${orgId.value}`)
}
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
          {{ t('team.create.title') }}
        </h1>
        <p class="text-surface-500 dark:text-surface-400 mt-1">
          {{ t('team.create.subtitle', { org: orgStore.currentOrganization?.name ?? '' }) }}
        </p>
      </div>
    </div>

    <div class="max-w-2xl">
      <Card>
        <template #content>
          <div v-if="error" class="mb-4 p-3 bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-300 rounded-md">
            {{ error }}
          </div>

          <div class="space-y-4">
            <div>
              <label class="block text-sm font-medium mb-2">
                {{ t('team.create.nameLabel') }} <span class="text-red-500">*</span>
              </label>
              <InputText
                v-model="name"
                class="w-full"
                :placeholder="t('team.create.namePlaceholder')"
                :class="{ 'p-invalid': name.length > 0 && name.length < 2 }"
              />
              <p class="text-xs text-surface-500 mt-1">
                {{ t('team.create.nameHint') }}
              </p>
            </div>

            <div>
              <label class="block text-sm font-medium mb-2">{{ t('team.create.descriptionLabel') }}</label>
              <Textarea
                v-model="description"
                class="w-full"
                rows="3"
                :placeholder="t('team.create.descriptionPlaceholder')"
                :aria-label="t('team.create.descriptionAria')"
              />
            </div>
          </div>
        </template>
        <template #footer>
          <div class="flex justify-end gap-2">
            <Button
              :label="t('team.create.cancel')"
              severity="secondary"
              @click="goBack"
            />
            <Button
              :label="t('team.create.submit')"
              icon="pi pi-plus"
              :loading="creating"
              :disabled="!isValid"
              @click="createTeam"
            />
          </div>
        </template>
      </Card>
    </div>
  </div>
</template>
