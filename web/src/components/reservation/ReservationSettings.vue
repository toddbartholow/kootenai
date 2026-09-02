<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { Lab } from '@/api'

const { t } = useI18n()

const props = defineProps<{
  labs: Lab[]
  selectedLab: Lab | null
}>()

const emit = defineEmits<{
  (e: 'update:selectedLab', lab: Lab): void
  (e: 'update:duration', duration: number): void
  (e: 'update:resources', resources: { cpu: number; memory: number; storage: number }): void
}>()

const selectedLabId = ref(props.selectedLab?.id || '')
const duration = ref(1)
const cpuCores = ref(4)
const memoryGB = ref(8)
const storageGB = ref(60)

// Duration options (re-computed so labels track locale changes)
const DURATION_VALUES = [1, 2, 3, 4, 6, 8] as const
const durationOptions = computed(() =>
  DURATION_VALUES.map(value => ({
    value,
    label: t(
      value === 1
        ? 'reservations.confirm.durationHourSingular'
        : 'reservations.confirm.durationHourPlural',
      { count: value },
    ),
  })),
)

// Resource limits
const maxCpu = 8
const maxMemory = 32
const maxStorage = 200

// Current lab details
const currentLab = computed(() => {
  return props.labs.find(l => l.id === selectedLabId.value) || null
})

// Lab features (mock)
const labFeatures = computed(() => {
  if (!currentLab.value) return []

  const features: Record<string, string[]> = {
    'lab-firewall-101': ['pfSense 2.7', 'Kali Linux', 'Windows Server', 'Network Tools'],
    'lab-ids-snort': ['Security Onion', 'Snort IDS', 'Attack VM', 'PCAP Analysis'],
    'lab-network-fundamentals': ['Ubuntu Desktop', 'Network Simulator', 'Wireshark', 'CLI Tools'],
    'lab-vpn-config': ['OpenVPN Server', 'WireGuard', 'Certificate Authority', 'Client VMs'],
    'lab-siem-basics': ['Wazuh Manager', 'Elastic Stack', 'Log Generators', 'Dashboard'],
    'lab-cloud-aws': ['LocalStack', 'AWS CLI', 'Terraform', 'CloudFormation'],
    'lab-incident-response': ['Forensics VM', 'Evidence Files', 'Timeline Tools', 'Report Templates'],
    'lab-windows-hardening': ['Windows Server 2022', 'GPO Templates', 'CIS Benchmarks', 'Audit Tools'],
  }

  return features[currentLab.value.id] || ['Virtual Machine', 'Network Access', 'Lab Materials']
})

// Watch for lab selection changes
watch(selectedLabId, (newId) => {
  const lab = props.labs.find(l => l.id === newId)
  if (lab) {
    emit('update:selectedLab', lab)
  }
})

watch(duration, (newDuration) => {
  emit('update:duration', newDuration)
})

watch([cpuCores, memoryGB, storageGB], () => {
  emit('update:resources', {
    cpu: cpuCores.value,
    memory: memoryGB.value,
    storage: storageGB.value,
  })
})

// Initialize with first lab if none selected
const firstLab = props.labs[0]
if (!selectedLabId.value && firstLab) {
  selectedLabId.value = firstLab.id
}
</script>

<template>
  <div class="bg-white dark:bg-surface-900 rounded-lg shadow p-6">
    <h2 class="text-lg font-medium text-gray-900 dark:text-white mb-6">{{ t('reservations.settings.heading') }}</h2>

    <!-- Lab Template Selection -->
    <div class="mb-6">
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
        {{ t('reservations.settings.labTemplateLabel') }}
      </label>
      <select
        v-model="selectedLabId"
        :aria-label="t('reservations.settings.labTemplateAria')"
        class="w-full rounded-md border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-700 text-gray-900 dark:text-white shadow-sm focus:border-blue-500 focus:ring focus:ring-blue-500 focus:ring-opacity-50 py-2 px-3 border"
      >
        <option v-for="lab in labs" :key="lab.id" :value="lab.id">
          {{ lab.name }}
        </option>
      </select>
      <p v-if="currentLab" class="mt-1 text-xs text-gray-500 dark:text-gray-400">
        {{ t('reservations.settings.labMeta', { difficulty: currentLab.difficulty, minutes: currentLab.durationMinutes }) }}
      </p>
    </div>

    <!-- Duration Selection -->
    <div class="mb-6">
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
        {{ t('reservations.settings.durationLabel') }}
      </label>
      <select
        v-model="duration"
        :aria-label="t('reservations.settings.durationAria')"
        class="w-full rounded-md border-gray-300 dark:border-surface-600 bg-white dark:bg-surface-700 text-gray-900 dark:text-white shadow-sm focus:border-blue-500 focus:ring focus:ring-blue-500 focus:ring-opacity-50 py-2 px-3 border"
      >
        <option v-for="opt in durationOptions" :key="opt.value" :value="opt.value">
          {{ opt.label }}
        </option>
      </select>
    </div>

    <!-- Resources -->
    <div class="mb-6">
      <label class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2">
        {{ t('reservations.settings.resourcesLabel') }}
      </label>
      <div class="space-y-4">
        <!-- CPU -->
        <div>
          <div class="flex justify-between mb-1">
            <label class="text-xs text-gray-500 dark:text-gray-400">{{ t('reservations.settings.cpuLabel') }}</label>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('reservations.settings.cpuValue', { count: cpuCores }) }}</span>
          </div>
          <input
            type="range"
            v-model.number="cpuCores"
            :min="1"
            :max="maxCpu"
            :aria-label="t('reservations.settings.cpuAria')"
            class="w-full h-2 bg-surface-200 dark:bg-surface-700 rounded-lg appearance-none cursor-pointer accent-blue-600"
          />
        </div>

        <!-- Memory -->
        <div>
          <div class="flex justify-between mb-1">
            <label class="text-xs text-gray-500 dark:text-gray-400">{{ t('reservations.settings.memoryLabel') }}</label>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('reservations.settings.memoryValue', { count: memoryGB }) }}</span>
          </div>
          <input
            type="range"
            v-model.number="memoryGB"
            :min="2"
            :max="maxMemory"
            :step="2"
            :aria-label="t('reservations.settings.memoryAria')"
            class="w-full h-2 bg-surface-200 dark:bg-surface-700 rounded-lg appearance-none cursor-pointer accent-blue-600"
          />
        </div>

        <!-- Storage -->
        <div>
          <div class="flex justify-between mb-1">
            <label class="text-xs text-gray-500 dark:text-gray-400">{{ t('reservations.settings.storageLabel') }}</label>
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('reservations.settings.storageValue', { count: storageGB }) }}</span>
          </div>
          <input
            type="range"
            v-model.number="storageGB"
            :min="20"
            :max="maxStorage"
            :step="10"
            :aria-label="t('reservations.settings.storageAria')"
            class="w-full h-2 bg-surface-200 dark:bg-surface-700 rounded-lg appearance-none cursor-pointer accent-blue-600"
          />
        </div>
      </div>
    </div>

    <!-- Template Details -->
    <div class="border-t border-gray-200 dark:border-surface-700 pt-4">
      <h3 class="font-medium text-sm text-gray-900 dark:text-white mb-2">{{ t('reservations.settings.labIncludes') }}</h3>
      <ul class="text-sm text-gray-600 dark:text-gray-400 space-y-2">
        <li v-for="feature in labFeatures" :key="feature" class="flex items-start">
          <svg class="h-5 w-5 text-green-500 mr-2 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7" />
          </svg>
          {{ feature }}
        </li>
      </ul>
    </div>

    <!-- Platform badge -->
    <div v-if="currentLab" class="mt-4 pt-4 border-t border-gray-200 dark:border-surface-700">
      <div class="flex items-center justify-between text-sm">
        <span class="text-gray-500 dark:text-gray-400">{{ t('reservations.settings.platformLabel') }}</span>
        <span
          class="px-2 py-1 rounded text-xs font-medium"
          :class="{
            'bg-purple-100 dark:bg-purple-900/30 text-purple-700 dark:text-purple-300': currentLab.platform === 'proxmox',
            'bg-cyan-100 dark:bg-cyan-900/30 text-cyan-700 dark:text-cyan-300': currentLab.platform === 'cloudstack',
          }"
        >
          {{ currentLab.platform }}
        </span>
      </div>
    </div>
  </div>
</template>
