<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useToast } from 'primevue/usetoast'
import { useConfirm } from 'primevue/useconfirm'
import { podsApi, type PodVM } from '@/api'
import Button from '@volt/Button.vue'

const { t } = useI18n()

/**
 * Start / Stop / Suspend / Resume controls for a single VM inside a pod.
 *
 * Extracted from PodDetailView.vue (~70 lines of template + 75 lines of
 * handler boilerplate). The component owns its own loading state and
 * emits a `changed` event after any successful action so the parent
 * view can refresh its pod data. Stop prompts for confirmation because
 * it's destructive; the other actions are immediate.
 */
const props = defineProps<{
  /** The VM this control set operates on. Reactive: status drives which buttons show. */
  vm: PodVM
  /** ID of the pod the VM belongs to. */
  podId: string
}>()

const emit = defineEmits<{
  /** Fired after a successful action. Parent typically reloads the pod. */
  (e: 'changed'): void
  /**
   * Fired when an action fails so the parent can surface the error in its
   * own UI. The component itself shows a toast — the emit is additive.
   */
  (e: 'error', message: string): void
}>()

const toast = useToast()
const confirm = useConfirm()

// Track loading per VM action so the specific button spins but the others
// stay interactive if the user is holding the mouse over them.
const loading = ref(false)

function isRunning(status: string): boolean {
  return status === 'running'
}
function isStopped(status: string): boolean {
  return status === 'stopped' || status === 'created'
}
function isSuspended(status: string): boolean {
  return status === 'suspended' || status === 'paused'
}

async function run(
  fn: () => Promise<unknown>,
  success: { summary: string; detail: string; severity: 'success' | 'info' | 'warn' },
  failureLabel: string,
): Promise<void> {
  loading.value = true
  try {
    await fn()
    toast.add({ ...success, life: 3000 })
    emit('changed')
  } catch (err) {
    console.error(failureLabel, err)
    emit('error', failureLabel)
  } finally {
    loading.value = false
  }
}

function startVM() {
  void run(
    () => podsApi.startVM(props.podId, props.vm.name),
    {
      severity: 'success',
      summary: t('pod.vmPower.toasts.started.summary'),
      detail: t('pod.vmPower.toasts.started.detail', { name: props.vm.name }),
    },
    t('pod.vmPower.errors.startFailed', { name: props.vm.name }),
  )
}

function confirmStopVM() {
  confirm.require({
    message: t('pod.vmPower.confirm.stop.message', { name: props.vm.name }),
    header: t('pod.vmPower.confirm.stop.header'),
    icon: 'pi pi-power-off',
    rejectProps: { label: t('pod.vmPower.confirm.stop.reject') },
    acceptProps: { label: t('pod.vmPower.confirm.stop.accept') },
    accept: () => {
      void run(
        () => podsApi.stopVM(props.podId, props.vm.name),
        {
          severity: 'info',
          summary: t('pod.vmPower.toasts.stopped.summary'),
          detail: t('pod.vmPower.toasts.stopped.detail', { name: props.vm.name }),
        },
        t('pod.vmPower.errors.stopFailed', { name: props.vm.name }),
      )
    },
  })
}

function suspendVM() {
  void run(
    () => podsApi.suspendVM(props.podId, props.vm.name),
    {
      severity: 'warn',
      summary: t('pod.vmPower.toasts.suspended.summary'),
      detail: t('pod.vmPower.toasts.suspended.detail', { name: props.vm.name }),
    },
    t('pod.vmPower.errors.suspendFailed', { name: props.vm.name }),
  )
}

function resumeVM() {
  void run(
    () => podsApi.resumeVM(props.podId, props.vm.name),
    {
      severity: 'success',
      summary: t('pod.vmPower.toasts.resumed.summary'),
      detail: t('pod.vmPower.toasts.resumed.detail', { name: props.vm.name }),
    },
    t('pod.vmPower.errors.resumeFailed', { name: props.vm.name }),
  )
}
</script>

<template>
  <div class="flex flex-wrap gap-2">
    <Button
      v-if="isStopped(vm.status)"
      icon="pi pi-play"
      :label="t('pod.vmPower.actions.start')"
      severity="success"
      size="small"
      :disabled="loading"
      :loading="loading"
      @click.stop="startVM"
    />
    <Button
      v-if="isRunning(vm.status)"
      icon="pi pi-stop"
      :label="t('pod.vmPower.actions.stop')"
      severity="danger"
      size="small"
      :disabled="loading"
      :loading="loading"
      @click.stop="confirmStopVM"
    />
    <Button
      v-if="isRunning(vm.status)"
      icon="pi pi-pause"
      :label="t('pod.vmPower.actions.suspend')"
      severity="warn"
      size="small"
      :disabled="loading"
      :loading="loading"
      @click.stop="suspendVM"
    />
    <Button
      v-if="isSuspended(vm.status)"
      icon="pi pi-play"
      :label="t('pod.vmPower.actions.resume')"
      severity="success"
      size="small"
      :disabled="loading"
      :loading="loading"
      @click.stop="resumeVM"
    />
  </div>
</template>
