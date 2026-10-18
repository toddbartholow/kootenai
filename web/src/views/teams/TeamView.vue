<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useOrganizationStore } from '@/stores/organization'
import { useFocusRestore } from '@/composables'
import { teamsApi, type TeamWithMembership, type TeamMembership } from '@/api'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import DataTable from '@volt/DataTable.vue'
import Column from 'primevue/column'
import Dialog from '@volt/Dialog.vue'
import InputText from '@volt/InputText.vue'
import Textarea from '@volt/Textarea.vue'

interface TeamMemberWithUser {
  user: { id: string; name: string; email: string }
  membership: TeamMembership
}

import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const orgStore = useOrganizationStore()
const { formatDate } = useFormatters()

const team = ref<TeamWithMembership | null>(null)
const members = ref<TeamMemberWithUser[]>([])
const loading = ref(true)
const membersLoading = ref(false)

// Edit dialog
const editDialogVisible = ref(false)
useFocusRestore(editDialogVisible)
const editName = ref('')
const editDescription = ref('')
const saving = ref(false)

// Delete dialog
const deleteDialogVisible = ref(false)
useFocusRestore(deleteDialogVisible)
const deleting = ref(false)

const teamId = computed(() => route.params['teamId'] as string)

const canManage = computed(() => {
  // Can manage if org admin or team lead
  if (orgStore.isAdmin) return true
  return team.value?.membership?.role === 'lead'
})

function getRoleSeverity(role: string): 'primary' | 'secondary' | 'success' | 'info' | 'warn' | 'danger' | 'contrast' | undefined {
  switch (role) {
    case 'lead': return 'primary'
    default: return 'secondary'
  }
}

async function fetchTeam() {
  loading.value = true
  try {
    team.value = await teamsApi.get(teamId.value)
  } catch (e) {
    console.error('Failed to fetch team:', e)
  } finally {
    loading.value = false
  }
}

async function fetchMembers() {
  membersLoading.value = true
  try {
    members.value = await teamsApi.getMembers(teamId.value)
  } catch (e) {
    console.error('Failed to fetch members:', e)
  } finally {
    membersLoading.value = false
  }
}

function openEditDialog() {
  if (team.value) {
    editName.value = team.value.team.name
    editDescription.value = team.value.team.description || ''
    editDialogVisible.value = true
  }
}

async function saveTeam() {
  if (!team.value) return

  saving.value = true
  try {
    const descValue = editDescription.value.trim()
    await orgStore.updateTeam(teamId.value, {
      name: editName.value.trim(),
      ...(descValue ? { description: descValue } : {}),
    })
    await fetchTeam()
    editDialogVisible.value = false
  } catch (e) {
    console.error('Failed to save team:', e)
  } finally {
    saving.value = false
  }
}

async function deleteTeam() {
  deleting.value = true
  try {
    await orgStore.deleteTeam(teamId.value)
    router.push(`/organizations/${team.value?.team.organizationId}`)
  } catch (e) {
    console.error('Failed to delete team:', e)
  } finally {
    deleting.value = false
  }
}

function goBack() {
  if (team.value) {
    router.push(`/organizations/${team.value.team.organizationId}`)
  } else {
    router.back()
  }
}

onMounted(async () => {
  await fetchTeam()
  await fetchMembers()
})
</script>

<template>
  <div class="p-6">
    <!-- Loading State -->
    <div v-if="loading" class="flex items-center justify-center py-12">
      <i class="pi pi-spin pi-spinner text-4xl text-primary-500"></i>
    </div>

    <template v-else-if="team">
      <!-- Header -->
      <div class="flex items-center justify-between mb-6">
        <div class="flex items-center gap-4">
          <Button
            icon="pi pi-arrow-left"
            severity="secondary"
            text
            rounded
            @click="goBack"
          />
          <div>
            <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-0 flex items-center gap-2">
              <i class="pi pi-users text-primary-500"></i>
              {{ team.team.name }}
            </h1>
            <p class="text-surface-500 dark:text-surface-400 mt-1">
              {{ team.team.description || t('team.view.noDescription') }}
            </p>
          </div>
        </div>
        <div class="flex gap-2">
          <Button
            v-if="canManage"
            :label="t('team.view.editAction')"
            icon="pi pi-pencil"
            severity="secondary"
            @click="openEditDialog"
          />
          <Button
            v-if="orgStore.isAdmin"
            icon="pi pi-trash"
            severity="danger"
            outlined
            @click="deleteDialogVisible = true"
          />
        </div>
      </div>

      <!-- Stats -->
      <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-6">
        <Card>
          <template #content>
            <div class="flex items-center gap-3">
              <div class="w-12 h-12 rounded-full bg-primary-100 dark:bg-primary-900 flex items-center justify-center">
                <i class="pi pi-users text-primary-500 text-xl"></i>
              </div>
              <div>
                <div class="text-2xl font-bold">{{ team.memberCount }}</div>
                <div class="text-surface-500">{{ t('team.view.statMembers') }}</div>
              </div>
            </div>
          </template>
        </Card>
        <Card>
          <template #content>
            <div class="flex items-center gap-3">
              <div class="w-12 h-12 rounded-full bg-green-100 dark:bg-green-900 flex items-center justify-center">
                <i class="pi pi-check-circle text-green-500 text-xl"></i>
              </div>
              <div>
                <div class="text-2xl font-bold">{{ team.team.isActive ? t('team.view.statusActive') : t('team.view.statusInactive') }}</div>
                <div class="text-surface-500">{{ t('team.view.statStatus') }}</div>
              </div>
            </div>
          </template>
        </Card>
        <Card>
          <template #content>
            <div class="flex items-center gap-3">
              <div class="w-12 h-12 rounded-full bg-blue-100 dark:bg-blue-900 flex items-center justify-center">
                <i class="pi pi-calendar text-blue-500 text-xl"></i>
              </div>
              <div>
                <div class="text-2xl font-bold">{{ formatDate(team.team.createdAt) }}</div>
                <div class="text-surface-500">{{ t('team.view.statCreated') }}</div>
              </div>
            </div>
          </template>
        </Card>
      </div>

      <!-- Members Table -->
      <Card>
        <template #title>
          <div class="flex items-center justify-between">
            <span>{{ t('team.view.membersHeading') }}</span>
            <Button
              v-if="canManage"
              :label="t('team.view.addMember')"
              icon="pi pi-user-plus"
              size="small"
            />
          </div>
        </template>
        <template #content>
          <DataTable
            :value="members"
            :loading="membersLoading"
            stripedRows
            class="p-datatable-sm"
          >
            <template #empty>
              <div class="text-center py-8 text-surface-500">
                {{ t('team.view.emptyMembers') }}
              </div>
            </template>
            <Column field="user.name" :header="t('team.view.columnName')" sortable />
            <Column field="user.email" :header="t('team.view.columnEmail')" sortable />
            <Column field="membership.role" :header="t('team.view.columnRole')" sortable>
              <template #body="{ data }">
                <Tag
                  :severity="getRoleSeverity(data.membership.role)"
                  :value="data.membership.role"
                />
              </template>
            </Column>
            <Column field="membership.createdAt" :header="t('team.view.columnJoined')" sortable>
              <template #body="{ data }">
                {{ formatDate(data.membership.createdAt) }}
              </template>
            </Column>
            <Column v-if="canManage" :header="t('team.view.columnActions')" style="width: 100px">
              <template #body="{ data }">
                <Button
                  icon="pi pi-trash"
                  severity="danger"
                  text
                  size="small"
                  v-tooltip="t('team.view.removeTooltip')"
                />
              </template>
            </Column>
          </DataTable>
        </template>
      </Card>
    </template>

    <!-- Edit Dialog -->
    <Dialog
      v-model:visible="editDialogVisible"
      :header="t('team.view.editDialog.header')"
      :modal="true"
      :style="{ width: '450px' }"
    >
      <div class="space-y-4">
        <div>
          <label class="block text-sm font-medium mb-2">{{ t('team.view.editDialog.nameLabel') }}</label>
          <InputText v-model="editName" class="w-full" />
        </div>
        <div>
          <label class="block text-sm font-medium mb-2">{{ t('team.view.editDialog.descriptionLabel') }}</label>
          <Textarea v-model="editDescription" class="w-full" rows="3" :aria-label="t('team.view.editDialog.descriptionAria')" />
        </div>
      </div>
      <template #footer>
        <Button
          :label="t('team.view.editDialog.cancel')"
          severity="secondary"
          @click="editDialogVisible = false"
        />
        <Button
          :label="t('team.view.editDialog.submit')"
          icon="pi pi-check"
          :loading="saving"
          @click="saveTeam"
        />
      </template>
    </Dialog>

    <!-- Delete Dialog -->
    <Dialog
      v-model:visible="deleteDialogVisible"
      :header="t('team.view.deleteDialog.header')"
      :modal="true"
      :style="{ width: '400px' }"
    >
      <div class="flex items-start gap-4">
        <i class="pi pi-exclamation-triangle text-4xl text-red-500"></i>
        <div>
          <p class="font-medium">{{ t('team.view.deleteDialog.question') }}</p>
          <p class="text-surface-500 text-sm mt-2">
            {{ t('team.view.deleteDialog.warning') }}
          </p>
        </div>
      </div>
      <template #footer>
        <Button
          :label="t('team.view.deleteDialog.cancel')"
          severity="secondary"
          @click="deleteDialogVisible = false"
        />
        <Button
          :label="t('team.view.deleteDialog.submit')"
          severity="danger"
          icon="pi pi-trash"
          :loading="deleting"
          @click="deleteTeam"
        />
      </template>
    </Dialog>
  </div>
</template>
