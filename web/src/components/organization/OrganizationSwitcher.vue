<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useOrganizationStore } from '@/stores/organization'
import Select from '@volt/Select.vue'
import Tag from '@volt/Tag.vue'

const { t } = useI18n()
const router = useRouter()
const orgStore = useOrganizationStore()

// Build options for the select dropdown
const orgOptions = computed(() => {
  return orgStore.organizations
    .filter(o => o.organization && o.membership) // Filter out malformed data
    .map(o => ({
      label: o.organization.name,
      value: o.organization.id,
      role: o.membership.role,
      edition: o.organization.edition,
    }))
})

const selectedOrgId = computed({
  get: () => orgStore.currentOrganization?.id,
  set: async (value: string | undefined) => {
    if (value && value !== orgStore.currentOrganization?.id) {
      await orgStore.selectOrganization(value)
      // Optionally refresh current route data
    }
  },
})

// Role badge styling
function getRoleSeverity(role: string): 'primary' | 'secondary' | 'success' | 'info' | 'warn' | 'danger' | 'contrast' | undefined {
  switch (role) {
    case 'owner': return 'danger'
    case 'admin': return 'warn'
    case 'instructor': return 'info'
    default: return 'secondary'
  }
}

// Edition badge styling
function getEditionSeverity(edition: string): 'primary' | 'secondary' | 'success' | 'info' | 'warn' | 'danger' | 'contrast' | undefined {
  switch (edition) {
    case 'enterprise': return 'primary'
    case 'professional': return 'success'
    default: return 'secondary'
  }
}

function _navigateToOrgSettings() {
  if (orgStore.currentOrganization) {
    router.push(`/organizations/${orgStore.currentOrganization.id}/settings`)
  }
}

onMounted(async () => {
  if (orgStore.organizations.length === 0) {
    await orgStore.fetchOrganizations()
  }
})
</script>

<template>
  <div class="organization-switcher flex items-center gap-2" data-pseudo-skip>
    <Select
      v-if="orgOptions.length > 0"
      v-model="selectedOrgId"
      :options="orgOptions"
      optionLabel="label"
      optionValue="value"
      :placeholder="t('organization.switcher.placeholder')"
      :aria-label="t('organization.switcher.aria')"
      class="min-w-[200px]"
      :loading="orgStore.loading"
    >
      <template #value="slotProps">
        <div v-if="slotProps.value" class="flex items-center gap-2">
          <i class="pi pi-building text-surface-400"></i>
          <span>{{ orgOptions.find(o => o.value === slotProps.value)?.label }}</span>
        </div>
        <span v-else class="text-surface-400">{{ t('organization.switcher.placeholder') }}</span>
      </template>
      <template #option="slotProps">
        <div class="flex items-center justify-between w-full gap-3">
          <div class="flex items-center gap-2">
            <i class="pi pi-building text-surface-400"></i>
            <span>{{ slotProps.option.label }}</span>
          </div>
          <div class="flex items-center gap-1">
            <Tag
              :severity="getEditionSeverity(slotProps.option.edition)"
              :value="slotProps.option.edition"
              class="text-xs px-1.5 py-0.5"
            />
            <Tag
              :severity="getRoleSeverity(slotProps.option.role)"
              :value="slotProps.option.role"
              class="text-xs px-1.5 py-0.5"
            />
          </div>
        </div>
      </template>
    </Select>

    <!-- Show current edition badge next to the selector -->
    <Tag
      v-if="orgStore.currentOrganization"
      :severity="getEditionSeverity(orgStore.edition)"
      :value="orgStore.edition"
      class="hidden sm:inline-flex text-xs"
    />
  </div>
</template>
