<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAssessmentStore } from '@/stores/assessment'
import StatusIcon from './StatusIcon.vue'

const { t } = useI18n()
const store = useAssessmentStore()

const devices = computed(() => store.result?.devices ?? [])

function getInterfaceKey(deviceName: string, interfaceName: string): string {
  return `${deviceName}:${interfaceName}`
}
</script>

<template>
  <div class="assessment-tree bg-surface-0 dark:bg-surface-800 rounded-lg shadow border border-surface-200 dark:border-surface-700">
    <div class="px-4 py-3 border-b border-surface-200 dark:border-surface-700 bg-surface-50 dark:bg-surface-900 rounded-t-lg">
      <h3 class="text-sm font-semibold text-surface-700 dark:text-surface-300">{{ t('assessment.tree.heading') }}</h3>
    </div>

    <div v-if="devices.length === 0" class="p-4 text-surface-500 dark:text-surface-400 text-sm">
      {{ t('assessment.tree.empty') }}
    </div>

    <div v-else class="divide-y divide-surface-100 dark:divide-surface-700">
      <!-- Device Level -->
      <div v-for="device in devices" :key="device.name" class="device-node">
        <!-- Device Header -->
        <button
          @click="store.toggleDevice(device.name)"
          class="w-full flex items-center px-4 py-2 hover:bg-surface-50 dark:hover:bg-surface-700 transition-colors"
        >
          <span class="mr-2 text-surface-400 dark:text-surface-500">
            <svg
              :class="['w-4 h-4 transition-transform', store.isDeviceExpanded(device.name) ? 'rotate-90' : '']"
              fill="none" stroke="currentColor" viewBox="0 0 24 24"
            >
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
            </svg>
          </span>
          <StatusIcon :status="device.status" />
          <span class="ml-2 font-medium text-surface-800 dark:text-surface-200">{{ device.name }}</span>
          <span class="ml-2 text-xs text-surface-500 dark:text-surface-400">{{ t('assessment.tree.deviceTypeParen', { type: device.type }) }}</span>
          <span class="ml-auto text-sm text-surface-600 dark:text-surface-400">
            {{ t('assessment.tree.pointsFraction', { earned: device.earnedPoints, max: device.maxPoints }) }}
          </span>
        </button>

        <!-- Device Children (expanded) -->
        <div v-if="store.isDeviceExpanded(device.name)" class="pl-6">
          <!-- Device-level checks -->
          <div
            v-for="check in (device.checks ?? [])"
            :key="check.id"
            class="flex items-center px-4 py-1.5 hover:bg-surface-50 dark:hover:bg-surface-700"
          >
            <span class="w-4 mr-2"></span>
            <StatusIcon :status="check.status" size="sm" />
            <span class="ml-2 text-sm text-surface-700 dark:text-surface-300">{{ check.description }}</span>
            <span class="ml-auto text-xs text-surface-500 dark:text-surface-400">{{ t('assessment.tree.pointsFraction', { earned: check.earnedPoints, max: check.points }) }}</span>
          </div>

          <!-- Interfaces -->
          <div v-for="iface in (device.interfaces ?? [])" :key="iface.name" class="interface-node">
            <!-- Interface Header -->
            <button
              @click="store.toggleInterface(getInterfaceKey(device.name, iface.name))"
              class="w-full flex items-center px-4 py-1.5 hover:bg-surface-50 dark:hover:bg-surface-700 transition-colors"
            >
              <span class="mr-2 text-surface-400 dark:text-surface-500">
                <svg
                  :class="['w-3 h-3 transition-transform', store.isInterfaceExpanded(device.name, iface.name) ? 'rotate-90' : '']"
                  fill="none" stroke="currentColor" viewBox="0 0 24 24"
                >
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
                </svg>
              </span>
              <StatusIcon :status="iface.status" size="sm" />
              <span class="ml-2 text-sm font-medium text-surface-700 dark:text-surface-300">{{ iface.name }}</span>
              <span class="ml-auto text-xs text-surface-500 dark:text-surface-400">
                {{ t('assessment.tree.pointsFraction', { earned: iface.earnedPoints, max: iface.maxPoints }) }}
              </span>
            </button>

            <!-- Interface Checks (expanded) -->
            <div v-if="store.isInterfaceExpanded(device.name, iface.name)" class="pl-6">
              <div
                v-for="check in iface.checks"
                :key="check.id"
                class="relative flex items-center px-4 py-1 hover:bg-surface-50 dark:hover:bg-surface-700 group"
              >
                <span class="w-4 mr-2"></span>
                <StatusIcon :status="check.status" size="sm" />
                <span class="ml-2 text-sm text-surface-600 dark:text-surface-400">{{ check.description }}</span>
                <span class="ml-auto text-xs text-surface-400 dark:text-surface-500">{{ t('assessment.tree.pointsFraction', { earned: check.earnedPoints, max: check.points }) }}</span>

                <!-- Tooltip on hover showing expected/actual (explicit undefined check) -->
                <div
                  v-if="check.status !== 'correct' && check.actual !== undefined"
                  class="hidden group-hover:block absolute right-4 top-full mt-1 p-2 bg-surface-800 dark:bg-surface-900 text-white text-xs rounded shadow-lg z-10 max-w-xs whitespace-nowrap"
                >
                  <div><strong>{{ t('assessment.tree.expectedLabel') }}</strong> {{ check.expected }}</div>
                  <div><strong>{{ t('assessment.tree.actualLabel') }}</strong> {{ check.actual || t('assessment.tree.emptyValue') }}</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.assessment-tree {
  font-size: 0.875rem;
}
</style>
