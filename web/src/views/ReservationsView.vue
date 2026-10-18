<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Lab, Reservation, CreateReservationRequest } from '@/api'
import { reservationsApi, labsApi } from '@/api'
import { formatDate } from '@/utils/format'
import { getDifficultySeverity } from '@/utils/status'
import { useFocusRestore } from '@/composables'

import { useFormatters } from '@/composables/useFormatters'

const { t } = useI18n()
const { formatWeekdayDate, formatTime: fmtTime } = useFormatters()
import ReservationCalendar from '@/components/reservation/ReservationCalendar.vue'
import ReservationSettings from '@/components/reservation/ReservationSettings.vue'
import type { CalendarDay, TimeSlot } from '@/components/reservation/ReservationCalendar.vue'
// Note: This view has custom getStatusSeverity that uses getReservationDisplayStatus
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import Dialog from '@volt/Dialog.vue'
import Tabs from '@volt/Tabs.vue'
import TabList from '@volt/TabList.vue'
import Tab from '@volt/Tab.vue'
import TabPanels from '@volt/TabPanels.vue'
import TabPanel from '@volt/TabPanel.vue'

// Tab management
const activeTab = ref('0')

// Reservation state
const selectedLab = ref<Lab | null>(null)
const selectedSlot = ref<{ day: CalendarDay; slot: TimeSlot } | null>(null)
const duration = ref(1)
const resources = ref({ cpu: 4, memory: 8, storage: 60 })

// Loading states
const loading = ref(false)
const reservationsLoading = ref(false)

// Confirmation modal
const showConfirmModal = ref(false)
useFocusRestore(showConfirmModal)

// Labs and reservations data
const labs = ref<Lab[]>([])
const myReservations = ref<Reservation[]>([])

// Load labs on mount
onMounted(async () => {
  loading.value = true
  try {
    const response = await labsApi.list()
    labs.value = response.labs
    // Initialize with first lab if available
    if (labs.value[0] && !selectedLab.value) {
      selectedLab.value = labs.value[0]
    }
  } catch (error) {
    console.error('Failed to load labs:', error)
  } finally {
    loading.value = false
  }

  // Load reservations
  await loadReservations()
})

async function loadReservations() {
  reservationsLoading.value = true
  try {
    myReservations.value = await reservationsApi.list()
  } catch (error) {
    console.error('Failed to load reservations:', error)
  } finally {
    reservationsLoading.value = false
  }
}

function handleSlotSelect(day: CalendarDay, slot: TimeSlot) {
  selectedSlot.value = { day, slot }
}

function handleLabUpdate(lab: Lab) {
  selectedLab.value = lab
}

function handleDurationUpdate(newDuration: number) {
  duration.value = newDuration
}

function handleResourcesUpdate(newResources: { cpu: number; memory: number; storage: number }) {
  resources.value = newResources
}

function openConfirmModal() {
  if (selectedSlot.value) {
    showConfirmModal.value = true
  }
}

function closeConfirmModal() {
  showConfirmModal.value = false
}

async function confirmReservation() {
  if (!selectedLab.value || !selectedSlot.value) return

  const { day, slot } = selectedSlot.value
  // Build start time from selected day and slot
  const startDate = new Date(day.year, day.month, day.date)
  // Parse slot time (e.g., "2:00 PM")
  const timeMatch = slot.time.match(/^(\d+):(\d+)\s*(AM|PM)$/i)
  if (timeMatch) {
    let hours = parseInt(timeMatch[1] ?? '12', 10)
    const minutes = parseInt(timeMatch[2] ?? '0', 10)
    const period = (timeMatch[3] ?? 'PM').toUpperCase()
    if (period === 'PM' && hours !== 12) hours += 12
    if (period === 'AM' && hours === 12) hours = 0
    startDate.setHours(hours, minutes, 0, 0)
  }

  const req: CreateReservationRequest = {
    labTemplateId: selectedLab.value.id,
    startTime: startDate.toISOString(),
    durationMinutes: duration.value * 60, // Convert hours to minutes
    resources: resources.value,
  }

  try {
    loading.value = true
    await reservationsApi.create(req)
    showConfirmModal.value = false
    selectedSlot.value = null
    // Reload reservations to show the new one
    await loadReservations()
  } catch (error) {
    console.error('Failed to create reservation:', error)
  } finally {
    loading.value = false
  }
}

const selectedDateFormatted = computed(() => {
  if (!selectedSlot.value) return ''
  const { day } = selectedSlot.value
  return formatWeekdayDate(new Date(day.year, day.month, day.date))
})

const selectedTimeRange = computed(() => {
  if (!selectedSlot.value) return ''
  const startTime = selectedSlot.value.slot.time
  // Parse start time and add duration
  const parts = startTime.split(' ')
  const time = parts[0] ?? '12:00'
  const period = parts[1] ?? 'PM'
  const timeParts = time.split(':').map(Number)
  const hours = timeParts[0] ?? 12
  let hour24 = period === 'PM' && hours !== 12 ? hours + 12 : hours
  if (period === 'AM' && hours === 12) hour24 = 0

  const endHour24 = hour24 + duration.value
  const endPeriod = endHour24 >= 12 ? 'PM' : 'AM'
  const endHour12 = endHour24 > 12 ? endHour24 - 12 : endHour24 === 0 ? 12 : endHour24

  return `${startTime} - ${endHour12}:00 ${endPeriod}`
})

// formatDate imported from @/utils/format

function formatTime(isoString: string): string {
  return fmtTime(isoString)
}

function formatTimeRange(startTime: string, endTime: string): string {
  return `${formatTime(startTime)} - ${formatTime(endTime)}`
}

function getReservationDisplayStatus(status: string): 'upcoming' | 'active' | 'completed' {
  if (status === 'pending' || status === 'confirmed') return 'upcoming'
  if (status === 'active') return 'active'
  return 'completed'
}

function getStatusSeverity(status: string): 'success' | 'info' | 'secondary' {
  const displayStatus = getReservationDisplayStatus(status)
  if (displayStatus === 'upcoming') return 'success'
  if (displayStatus === 'active') return 'info'
  return 'secondary'
}

function getStatusLabel(status: string): string {
  const displayStatus = getReservationDisplayStatus(status)
  if (displayStatus === 'upcoming') return t('reservations.statusUpcoming')
  if (displayStatus === 'active') return t('reservations.statusActive')
  return t('reservations.statusCompleted')
}

async function cancelReservation(id: string) {
  try {
    await reservationsApi.cancel(id)
    await loadReservations()
  } catch (error) {
    console.error('Failed to cancel reservation:', error)
  }
}

// getDifficultySeverity imported from @/utils/status
</script>

<template>
  <div class="space-y-6">
    <div>
      <h1 class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ t('reservations.title') }}</h1>
      <p class="text-surface-600 dark:text-surface-400 mt-1">{{ t('reservations.subtitle') }}</p>
    </div>

    <Tabs v-model:value="activeTab">
      <TabList>
        <Tab value="0">{{ t('reservations.tabs.reserve') }}</Tab>
        <Tab value="1">{{ t('reservations.tabs.my') }}</Tab>
        <Tab value="2">{{ t('reservations.tabs.templates') }}</Tab>
      </TabList>
      <TabPanels>
        <!-- Reserve a VM Tab -->
        <TabPanel value="0">
          <div class="grid grid-cols-1 lg:grid-cols-4 gap-8 pt-6">
            <!-- Sidebar -->
            <div class="lg:col-span-1">
              <ReservationSettings
                :labs="labs"
                :selected-lab="selectedLab"
                @update:selected-lab="handleLabUpdate"
                @update:duration="handleDurationUpdate"
                @update:resources="handleResourcesUpdate"
              />
            </div>

            <!-- Calendar -->
            <div class="lg:col-span-3">
              <ReservationCalendar
                :selected-slot="selectedSlot"
                @select="handleSlotSelect"
              />

              <!-- Reservation Actions -->
              <div class="mt-6 flex justify-end">
                <Button
                  @click="openConfirmModal"
                  :disabled="!selectedSlot"
                  icon="pi pi-calendar-plus"
                  :label="t('reservations.reserveAction')"
                />
              </div>
            </div>
          </div>
        </TabPanel>

        <!-- My Reservations Tab -->
        <TabPanel value="1">
          <Card class="mt-6">
            <template #title>{{ t('reservations.my.heading') }}</template>
            <template #subtitle>{{ t('reservations.my.subheading') }}</template>
            <template #content>
              <div v-if="reservationsLoading" class="flex justify-center py-12">
                <i class="pi pi-spinner pi-spin text-4xl text-surface-400" />
              </div>
              <div v-else-if="myReservations.length > 0" class="divide-y divide-surface-200 dark:divide-surface-700 -mx-4">
                <div
                  v-for="reservation in myReservations"
                  :key="reservation.id"
                  class="p-4 hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
                >
                  <div class="flex items-center justify-between">
                    <div class="flex-1">
                      <div class="flex items-center gap-3">
                        <h3 class="font-medium text-surface-900 dark:text-surface-100">{{ reservation.labTemplateName || reservation.labTemplateId }}</h3>
                        <Tag
                          :value="getStatusLabel(reservation.status)"
                          :severity="getStatusSeverity(reservation.status)"
                        />
                      </div>
                      <div class="mt-2 text-sm text-surface-500 space-y-1">
                        <p><i class="pi pi-calendar mr-2" />{{ formatDate(reservation.startTime) }} · {{ formatTimeRange(reservation.startTime, reservation.endTime) }}</p>
                        <p><i class="pi pi-server mr-2" />{{ t('reservations.my.resources', { cpu: reservation.resources.cpu, memory: reservation.resources.memory, storage: reservation.resources.storage }) }}</p>
                      </div>
                    </div>
                    <div class="flex items-center gap-2">
                      <Button
                        v-if="getReservationDisplayStatus(reservation.status) === 'upcoming'"
                        icon="pi pi-play"
                        :label="t('reservations.my.launch')"
                        size="small"
                      />
                      <Button
                        v-if="getReservationDisplayStatus(reservation.status) === 'upcoming'"
                        icon="pi pi-times"
                        :label="t('reservations.my.cancel')"
                        severity="secondary"
                        size="small"
                        @click="cancelReservation(reservation.id)"
                      />
                      <Button
                        v-if="getReservationDisplayStatus(reservation.status) === 'completed'"
                        icon="pi pi-eye"
                        :label="t('reservations.my.viewDetails')"
                        severity="secondary"
                        size="small"
                      />
                    </div>
                  </div>
                </div>
              </div>

              <div v-else class="text-center py-12">
                <i class="pi pi-calendar text-5xl text-surface-400 mb-4" />
                <h3 class="text-lg font-medium text-surface-900 dark:text-surface-100">{{ t('reservations.my.emptyTitle') }}</h3>
                <p class="text-surface-500 mt-1">{{ t('reservations.my.emptyBody') }}</p>
                <Button
                  @click="activeTab = '0'"
                  icon="pi pi-plus"
                  :label="t('reservations.my.emptyAction')"
                  class="mt-4"
                />
              </div>
            </template>
          </Card>
        </TabPanel>

        <!-- Templates Tab -->
        <TabPanel value="2">
          <Card class="mt-6">
            <template #title>{{ t('reservations.templates.heading') }}</template>
            <template #subtitle>{{ t('reservations.templates.subheading') }}</template>
            <template #content>
              <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                <Card
                  v-for="lab in labs"
                  :key="lab.id"
                  class="cursor-pointer hover:ring-2 hover:ring-primary-500 transition-all focus-within:ring-2 focus-within:ring-primary-500"
                  role="button"
                  tabindex="0"
                  :aria-label="t('reservations.templates.selectAria', { name: lab.name })"
                  @click="selectedLab = lab; activeTab = '0'"
                  @keydown.enter="selectedLab = lab; activeTab = '0'"
                  @keydown.space.prevent="selectedLab = lab; activeTab = '0'"
                >
                  <template #content>
                    <div class="flex items-start justify-between mb-3">
                      <h3 class="font-medium text-surface-900 dark:text-surface-100">{{ lab.name }}</h3>
                      <Tag
                        :value="lab.platform"
                        :severity="lab.platform === 'proxmox' ? 'contrast' : 'info'"
                      />
                    </div>
                    <p class="text-sm text-surface-500 mb-3">{{ lab.description }}</p>
                    <div class="flex items-center gap-4 text-xs text-surface-500">
                      <Tag :value="lab.difficulty" :severity="getDifficultySeverity(lab.difficulty)" />
                      <span><i class="pi pi-clock mr-1" />{{ t('reservations.templates.durationMinutes', { count: lab.durationMinutes }) }}</span>
                    </div>
                  </template>
                </Card>
              </div>
            </template>
          </Card>
        </TabPanel>
      </TabPanels>
    </Tabs>

    <!-- Confirmation Modal -->
    <Dialog
      v-model:visible="showConfirmModal"
      modal
      :header="t('reservations.confirm.header')"
      :style="{ width: '450px' }"
    >
      <div class="space-y-4">
        <div class="grid grid-cols-2 gap-4 text-sm">
          <div class="text-surface-500">{{ t('reservations.confirm.templateLabel') }}</div>
          <div class="text-surface-900 dark:text-surface-100 font-medium">{{ selectedLab?.name }}</div>

          <div class="text-surface-500">{{ t('reservations.confirm.dateLabel') }}</div>
          <div class="text-surface-900 dark:text-surface-100">{{ selectedDateFormatted }}</div>

          <div class="text-surface-500">{{ t('reservations.confirm.timeLabel') }}</div>
          <div class="text-surface-900 dark:text-surface-100">{{ selectedTimeRange }}</div>

          <div class="text-surface-500">{{ t('reservations.confirm.durationLabel') }}</div>
          <div class="text-surface-900 dark:text-surface-100">{{ duration > 1 ? t('reservations.confirm.durationHourPlural', { count: duration }) : t('reservations.confirm.durationHourSingular', { count: duration }) }}</div>

          <div class="text-surface-500">{{ t('reservations.confirm.resourcesLabel') }}</div>
          <div class="text-surface-900 dark:text-surface-100">{{ t('reservations.confirm.resourcesValue', { cpu: resources.cpu, memory: resources.memory, storage: resources.storage }) }}</div>
        </div>
      </div>

      <template #footer>
        <Button
          @click="closeConfirmModal"
          :label="t('reservations.confirm.cancel')"
          severity="secondary"
        />
        <Button
          @click="confirmReservation"
          icon="pi pi-check"
          :label="t('reservations.confirm.submit')"
        />
      </template>
    </Dialog>
  </div>
</template>
