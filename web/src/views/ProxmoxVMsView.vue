<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { proxmoxApi, type ConsoleTicket } from '@/api'
import { loggers } from '@/utils/logger'
import VncConsole from '@/components/console/VncConsole.vue'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Message from '@volt/Message.vue'
import Menu from '@volt/Menu.vue'
import Toast from '@volt/Toast.vue'
import ProgressSpinner from '@volt/ProgressSpinner.vue'
import { useToast } from 'primevue/usetoast'

const { t } = useI18n()
const toast = useToast()

// VMs should be fetched from API - empty until configured
const knownVMs: Array<{ vmid: number; name: string; node: string }> = []

const error = ref<string | null>(null)

// Console modal state
const showConsoleModal = ref(false)
const consoleVM = ref<{ vmid: number; name: string; node: string } | null>(null)
const consoleTicket = ref<ConsoleTicket | null>(null)
const consoleLoading = ref(false)
const consoleError = ref<string | null>(null)
const consoleType = ref<'vnc' | 'spice'>('vnc')

// Menu refs for each VM
const menuRefs = ref<Record<number, InstanceType<typeof Menu> | null>>({})

function setMenuRef(el: unknown, vmid: number) {
  menuRefs.value[vmid] = el as InstanceType<typeof Menu> | null
}

function getMenuItems(vm: { vmid: number; name: string; node: string }) {
  return [
    {
      label: t('proxmoxVms.menuNoVnc'),
      icon: 'pi pi-desktop',
      command: () => openConsole(vm, 'vnc')
    },
    {
      label: t('proxmoxVms.menuDownloadSpice'),
      icon: 'pi pi-download',
      command: () => downloadSpiceFile(vm)
    }
  ]
}

async function openConsole(vm: { vmid: number; name: string; node: string }, type: 'vnc' | 'spice' = 'vnc') {
  consoleVM.value = vm
  consoleType.value = type
  consoleLoading.value = true
  consoleError.value = null
  showConsoleModal.value = true

  try {
    // Get console ticket using direct VMID endpoint
    consoleTicket.value = await proxmoxApi.getDirectConsole(vm.vmid, type, vm.node)
    loggers.console.debug(`Got ${type.toUpperCase()} console ticket`, {}, consoleTicket.value)
  } catch (err) {
    console.error('Failed to get console ticket:', err)
    consoleError.value = err instanceof Error ? err.message : t('proxmoxVms.connectionFailedFallback')
    toast.add({
      severity: 'error',
      summary: t('proxmoxVms.connectionFailedTitle'),
      detail: consoleError.value,
      life: 5000
    })
  } finally {
    consoleLoading.value = false
  }
}

function downloadSpiceFile(vm: { vmid: number; name: string; node: string }) {
  // Download .vv file from API
  const url = `/api/v1/proxmox/vms/${vm.vmid}/spice.vv?node=${vm.node}`
  window.open(url, '_blank')
}

function closeConsole() {
  showConsoleModal.value = false
  consoleVM.value = null
  consoleTicket.value = null
  consoleError.value = null
}

function toggleMenu(event: Event, vmid: number) {
  menuRefs.value[vmid]?.toggle(event)
}
</script>

<template>
  <div class="space-y-8">
    <Toast />

    <div class="flex justify-between items-center">
      <div>
        <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ t('proxmoxVms.title') }}</h1>
        <p class="text-surface-500 mt-1">{{ t('proxmoxVms.subtitle') }}</p>
      </div>
    </div>

    <Message v-if="error" severity="error" :closable="true" @close="error = null">
      {{ error }}
    </Message>

    <Card>
      <template #title>{{ t('proxmoxVms.runningHeading') }}</template>
      <template #content>
        <div class="space-y-4">
          <div
            v-for="vm in knownVMs"
            :key="vm.vmid"
            class="border border-surface-200 dark:border-surface-700 rounded-lg p-4 hover:bg-surface-50 dark:hover:bg-surface-800 transition-colors"
          >
            <div class="flex items-center justify-between">
              <div>
                <h3 class="font-medium text-surface-900 dark:text-surface-100">{{ vm.name }}</h3>
                <p class="text-sm text-surface-500">{{ t('proxmoxVms.vmIdentity', { vmid: vm.vmid, node: vm.node }) }}</p>
              </div>
              <div class="flex gap-2">
                <Button
                  @click="toggleMenu($event, vm.vmid)"
                  icon="pi pi-desktop"
                  :label="t('proxmoxVms.consoleButton')"
                  severity="secondary"
                />
                <Menu :ref="(el: unknown) => setMenuRef(el, vm.vmid)" :model="getMenuItems(vm)" :popup="true" />
              </div>
            </div>
          </div>
        </div>
      </template>
    </Card>

    <!-- VNC Console (floating window - no modal wrapper needed) -->
    <VncConsole
      v-if="showConsoleModal && consoleTicket && consoleVM && consoleTicket.type === 'vnc'"
      :vmid="consoleVM.vmid"
      :ticket="consoleTicket"
      @close="closeConsole"
    />

    <!-- Loading Toast -->
    <div v-if="showConsoleModal && consoleLoading" class="fixed bottom-4 right-4 z-50">
      <Card class="shadow-lg">
        <template #content>
          <div class="flex items-center gap-3">
            <ProgressSpinner style="width: 20px; height: 20px" />
            <span class="text-surface-700 dark:text-surface-300">
              {{ t('proxmoxVms.loadingConsole', { type: consoleType.toUpperCase(), name: consoleVM?.name }) }}
            </span>
          </div>
        </template>
      </Card>
    </div>

    <!-- Error Toast with Retry -->
    <div v-if="showConsoleModal && consoleError" class="fixed bottom-4 right-4 z-50 max-w-md">
      <Card class="shadow-lg border-red-200 dark:border-red-800">
        <template #content>
          <div class="flex items-start gap-3">
            <i class="pi pi-exclamation-triangle text-xl text-red-500" />
            <div class="flex-1">
              <p class="font-medium text-surface-900 dark:text-surface-100">{{ t('proxmoxVms.connectionFailedTitle') }}</p>
              <p class="text-sm text-surface-600 dark:text-surface-400">{{ consoleError }}</p>
            </div>
            <Button
              @click="closeConsole"
              icon="pi pi-times"
              text
              rounded
              severity="secondary"
              size="small"
            />
          </div>
          <Button
            v-if="consoleVM"
            @click="openConsole(consoleVM, consoleType)"
            :label="t('proxmoxVms.retry')"
            severity="danger"
            size="small"
            class="w-full mt-3"
          />
        </template>
      </Card>
    </div>
  </div>
</template>
