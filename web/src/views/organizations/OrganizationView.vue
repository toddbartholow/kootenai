<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useOrganizationStore } from '@/stores/organization'
import { organizationsApi, type OrganizationMembership } from '@/api'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import DataTable from '@volt/DataTable.vue'
import Column from 'primevue/column'
import Tabs from '@volt/Tabs.vue'
import TabList from '@volt/TabList.vue'
import Tab from '@volt/Tab.vue'
import TabPanels from '@volt/TabPanels.vue'
import TabPanel from '@volt/TabPanel.vue'

interface MemberWithUser {
  user: { id: string; name: string; email: string }
  membership: OrganizationMembership
}

import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatDate } = useFormatters()
const route = useRoute()
const router = useRouter()
const orgStore = useOrganizationStore()

const activeTab = ref('0')
const members = ref<MemberWithUser[]>([])
const membersLoading = ref(false)

const orgId = computed(() => route.params['orgId'] as string)

const organization = computed(() => orgStore.currentOrganization)
const teams = computed(() => orgStore.teams)

function getRoleSeverity(role: string): 'primary' | 'secondary' | 'success' | 'info' | 'warn' | 'danger' | 'contrast' | undefined {
  switch (role) {
    case 'owner': return 'danger'
    case 'admin': return 'warn'
    case 'instructor': return 'info'
    default: return 'secondary'
  }
}

function getEditionSeverity(edition: string): 'primary' | 'secondary' | 'success' | 'info' | 'warn' | 'danger' | 'contrast' | undefined {
  switch (edition) {
    case 'enterprise': return 'primary'
    case 'professional': return 'success'
    default: return 'secondary'
  }
}

async function fetchMembers() {
  if (!orgId.value) return
  membersLoading.value = true
  try {
    const response = await organizationsApi.getMembers(orgId.value)
    members.value = response.members
  } catch (e) {
    console.error('Failed to fetch members:', e)
  } finally {
    membersLoading.value = false
  }
}

function navigateToTeam(teamId: string) {
  router.push(`/teams/${teamId}`)
}

function navigateToSettings() {
  router.push(`/organizations/${orgId.value}/settings`)
}

onMounted(async () => {
  if (orgId.value && orgId.value !== orgStore.currentOrganization?.id) {
    await orgStore.selectOrganization(orgId.value)
  }
  await fetchMembers()
})
</script>

<template>
  <div class="p-6">
    <!-- Header -->
    <div class="flex items-center justify-between mb-6">
      <div>
        <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-0">
          {{ organization?.name || t('organization.view.fallbackTitle') }}
        </h1>
        <div class="flex items-center gap-2 mt-2">
          <Tag
            v-if="organization"
            :severity="getEditionSeverity(organization.edition)"
            :value="organization.edition"
          />
          <Tag
            v-if="orgStore.currentRole"
            :severity="getRoleSeverity(orgStore.currentRole)"
            :value="orgStore.currentRole"
          />
          <span class="text-surface-500 dark:text-surface-400">
            {{ organization?.slug }}
          </span>
        </div>
      </div>
      <div class="flex gap-2">
        <Button
          v-if="orgStore.isAdmin"
          :label="t('organization.view.settingsAction')"
          icon="pi pi-cog"
          severity="secondary"
          @click="navigateToSettings"
        />
      </div>
    </div>

    <!-- Tabs -->
    <Tabs v-model:value="activeTab">
      <TabList>
        <Tab value="0">{{ t('organization.view.tabOverview') }}</Tab>
        <Tab value="1">{{ t('organization.view.tabMembers') }}</Tab>
        <Tab value="2" v-if="orgStore.hasFeature('teams')">{{ t('organization.view.tabTeams') }}</Tab>
        <Tab value="3">{{ t('organization.view.tabFeatures') }}</Tab>
      </TabList>

      <TabPanels>
        <!-- Overview Tab -->
        <TabPanel value="0">
          <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4">
            <Card>
              <template #title>{{ t('organization.view.cardMembers') }}</template>
              <template #content>
                <div class="text-3xl font-bold text-primary-500">
                  {{ members.length }}
                </div>
                <div v-if="organization?.maxUsers" class="text-sm text-surface-500">
                  {{ t('organization.view.membersCap', { max: organization.maxUsers }) }}
                </div>
              </template>
            </Card>
            <Card v-if="orgStore.hasFeature('teams')">
              <template #title>{{ t('organization.view.cardTeams') }}</template>
              <template #content>
                <div class="text-3xl font-bold text-primary-500">
                  {{ teams.length }}
                </div>
              </template>
            </Card>
            <Card>
              <template #title>{{ t('organization.view.cardEdition') }}</template>
              <template #content>
                <Tag
                  :severity="getEditionSeverity(organization?.edition || '')"
                  :value="organization?.edition || t('organization.view.editionFallback')"
                  class="text-lg"
                />
              </template>
            </Card>
          </div>
        </TabPanel>

        <!-- Members Tab -->
        <TabPanel value="1">
          <div class="mt-4">
            <div class="flex justify-between items-center mb-4">
              <h3 class="text-lg font-semibold">{{ t('organization.view.membersHeading') }}</h3>
              <Button
                v-if="orgStore.isAdmin"
                :label="t('organization.view.inviteMember')"
                icon="pi pi-user-plus"
                size="small"
              />
            </div>
            <DataTable
              :value="members"
              :loading="membersLoading"
              stripedRows
              class="p-datatable-sm"
            >
              <Column field="user.name" :header="t('organization.view.columnName')" sortable />
              <Column field="user.email" :header="t('organization.view.columnEmail')" sortable />
              <Column field="membership.role" :header="t('organization.view.columnRole')" sortable>
                <template #body="{ data }">
                  <Tag
                    :severity="getRoleSeverity(data.membership.role)"
                    :value="data.membership.role"
                  />
                </template>
              </Column>
              <Column field="membership.acceptedAt" :header="t('organization.view.columnJoined')" sortable>
                <template #body="{ data }">
                  {{ data.membership.acceptedAt ? formatDate(data.membership.acceptedAt) : t('organization.view.joinedPending') }}
                </template>
              </Column>
            </DataTable>
          </div>
        </TabPanel>

        <!-- Teams Tab -->
        <TabPanel value="2" v-if="orgStore.hasFeature('teams')">
          <div class="mt-4">
            <div class="flex justify-between items-center mb-4">
              <h3 class="text-lg font-semibold">{{ t('organization.view.teamsHeading') }}</h3>
              <Button
                v-if="orgStore.isAdmin"
                :label="t('organization.view.createTeamAction')"
                icon="pi pi-plus"
                size="small"
                @click="router.push(`/organizations/${orgId}/teams/new`)"
              />
            </div>
            <div v-if="teams.length === 0" class="text-center py-8 text-surface-500">
              {{ t('organization.view.teamsEmpty') }}
            </div>
            <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              <Card
                v-for="teamWithMembership in teams"
                :key="teamWithMembership.team.id"
                class="cursor-pointer hover:border-primary-500 transition-colors"
                @click="navigateToTeam(teamWithMembership.team.id)"
              >
                <template #title>
                  <div class="flex items-center gap-2">
                    <i class="pi pi-users text-primary-500"></i>
                    {{ teamWithMembership.team.name }}
                  </div>
                </template>
                <template #subtitle>
                  {{ teamWithMembership.team.description || t('organization.view.teamNoDescription') }}
                </template>
                <template #content>
                  <div class="flex items-center gap-2 text-sm text-surface-500">
                    <i class="pi pi-user"></i>
                    {{ t('organization.view.teamsMemberCount', { count: teamWithMembership.memberCount }) }}
                  </div>
                </template>
              </Card>
            </div>
          </div>
        </TabPanel>

        <!-- Features Tab -->
        <TabPanel value="3">
          <div class="mt-4">
            <h3 class="text-lg font-semibold mb-4">{{ t('organization.view.featuresHeading') }}</h3>
            <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
              <Card
                v-for="[featureId, enabled] in orgStore.features"
                :key="featureId"
                :class="{ 'opacity-50': !enabled }"
              >
                <template #content>
                  <div class="flex items-center justify-between">
                    <div class="flex items-center gap-2">
                      <i :class="['pi', enabled ? 'pi-check-circle text-green-500' : 'pi-times-circle text-red-500']"></i>
                      <span class="font-medium">{{ featureId }}</span>
                    </div>
                    <Tag
                      :severity="enabled ? 'success' : 'danger'"
                      :value="enabled ? t('organization.view.featureEnabled') : t('organization.view.featureDisabled')"
                    />
                  </div>
                </template>
              </Card>
            </div>
          </div>
        </TabPanel>
      </TabPanels>
    </Tabs>
  </div>
</template>
