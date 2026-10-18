<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { VMConfig } from '@/types/lab-form'
import { templateOptions } from '@/constants/formOptions'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import InputText from '@volt/InputText.vue'
import Select from '@volt/Select.vue'
import InputNumber from '@volt/InputNumber.vue'
import Checkbox from '@volt/Checkbox.vue'
import Message from '@volt/Message.vue'

defineProps<{
  vms: VMConfig[]
  newVMName: string
  newVMTemplate: string
}>()

const emit = defineEmits<{
  'update:newVMName': [value: string]
  'update:newVMTemplate': [value: string]
  addVM: []
  removeVM: [index: number]
  addSnapshot: [vmIndex: number]
  removeSnapshot: [vmIndex: number, snapIndex: number]
  setDefaultSnapshot: [vmIndex: number, snapIndex: number]
}>()

const { t } = useI18n()
</script>

<template>
  <Card>
    <template #content>
      <div class="space-y-6">
        <h2 class="text-xl font-semibold text-surface-900 dark:text-surface-100">
          {{ t('createLab.vmsStep.heading') }}
        </h2>
        <p class="text-surface-600 dark:text-surface-400">{{ t('createLab.vmsStep.intro') }}</p>

        <div class="bg-surface-50 dark:bg-surface-800 rounded-lg p-4 space-y-4">
          <h3 class="font-medium text-surface-900 dark:text-surface-100">
            {{ t('createLab.vmsStep.addHeading') }}
          </h3>
          <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div class="space-y-2">
              <label
                for="new-vm-name"
                class="block text-sm text-surface-600 dark:text-surface-400"
                >{{ t('createLab.vmsStep.nameLabel') }}</label
              >
              <InputText
                id="new-vm-name"
                :modelValue="newVMName"
                @update:modelValue="emit('update:newVMName', $event as string)"
                :placeholder="t('createLab.vmsStep.namePlaceholder')"
                class="w-full"
              />
            </div>
            <div class="space-y-2">
              <label
                for="new-vm-template"
                class="block text-sm text-surface-600 dark:text-surface-400"
                >{{ t('createLab.vmsStep.templateLabel') }}</label
              >
              <Select
                id="new-vm-template"
                :modelValue="newVMTemplate"
                @update:modelValue="emit('update:newVMTemplate', $event as string)"
                :options="templateOptions"
                optionLabel="label"
                optionValue="value"
                class="w-full"
              />
            </div>
          </div>
          <Button
            :label="t('createLab.vmsStep.addAction')"
            icon="pi pi-plus"
            @click="emit('addVM')"
            :disabled="newVMName.trim().length < 2"
          />
        </div>

        <div v-if="vms.length > 0" class="space-y-4">
          <div
            v-for="(vm, vmIndex) in vms"
            :key="vm.id"
            class="border border-surface-200 dark:border-surface-700 rounded-lg p-4 space-y-4"
          >
            <div class="flex items-center justify-between">
              <div>
                <h4 class="font-medium text-surface-900 dark:text-surface-100">{{ vm.name }}</h4>
                <p class="text-sm text-surface-500">{{ vm.template }}</p>
              </div>
              <Button
                icon="pi pi-trash"
                severity="danger"
                text
                size="small"
                @click="emit('removeVM', vmIndex)"
                :aria-label="t('createLab.vmsStep.removeAria', { name: vm.name })"
              />
            </div>

            <div class="grid grid-cols-3 gap-4">
              <div class="space-y-1">
                <label :for="`vm-cpu-${vmIndex}`" class="block text-xs text-surface-500">{{
                  t('createLab.vmsStep.cpuLabel')
                }}</label>
                <InputNumber
                  :id="`vm-cpu-${vmIndex}`"
                  v-model="vm.cpu"
                  :min="1"
                  :max="16"
                  class="w-full"
                />
              </div>
              <div class="space-y-1">
                <label :for="`vm-memory-${vmIndex}`" class="block text-xs text-surface-500">{{
                  t('createLab.vmsStep.memoryLabel')
                }}</label>
                <InputNumber
                  :id="`vm-memory-${vmIndex}`"
                  v-model="vm.memory"
                  :min="512"
                  :max="65536"
                  :step="512"
                  class="w-full"
                />
              </div>
              <div class="space-y-1">
                <label :for="`vm-disk-${vmIndex}`" class="block text-xs text-surface-500">{{
                  t('createLab.vmsStep.diskLabel')
                }}</label>
                <InputNumber
                  :id="`vm-disk-${vmIndex}`"
                  v-model="vm.disk"
                  :min="1"
                  :max="500"
                  class="w-full"
                />
              </div>
            </div>

            <div class="flex items-center gap-2">
              <Checkbox v-model="vm.wazuhAgent" :binary="true" :inputId="`wazuh-${vmIndex}`" />
              <label
                :for="`wazuh-${vmIndex}`"
                class="text-sm text-surface-600 dark:text-surface-400"
                >{{ t('createLab.vmsStep.wazuhLabel') }}</label
              >
            </div>

            <div class="space-y-2">
              <div class="flex items-center justify-between">
                <label class="text-sm font-medium text-surface-700 dark:text-surface-300">{{
                  t('createLab.vmsStep.snapshotsLabel')
                }}</label>
                <Button
                  :label="t('createLab.vmsStep.addSnapshot')"
                  icon="pi pi-plus"
                  size="small"
                  severity="secondary"
                  text
                  @click="emit('addSnapshot', vmIndex)"
                />
              </div>
              <div class="space-y-2">
                <div
                  v-for="(snapshot, snapIndex) in vm.snapshots"
                  :key="snapIndex"
                  class="flex items-center gap-2 bg-surface-100 dark:bg-surface-700 rounded p-2"
                >
                  <InputText
                    v-model="snapshot.name"
                    :placeholder="t('createLab.vmsStep.snapshotNamePlaceholder')"
                    class="flex-1"
                    :aria-label="t('createLab.vmsStep.snapshotNameAria', { index: snapIndex + 1 })"
                  />
                  <InputText
                    v-model="snapshot.description"
                    :placeholder="t('createLab.vmsStep.snapshotDescriptionPlaceholder')"
                    class="flex-1"
                    :aria-label="
                      t('createLab.vmsStep.snapshotDescriptionAria', { index: snapIndex + 1 })
                    "
                  />
                  <Button
                    :icon="snapshot.isDefault ? 'pi pi-star-fill' : 'pi pi-star'"
                    :severity="snapshot.isDefault ? 'warn' : 'secondary'"
                    text
                    size="small"
                    @click="emit('setDefaultSnapshot', vmIndex, snapIndex)"
                    :aria-label="
                      snapshot.isDefault
                        ? t('createLab.vmsStep.snapshotDefaultAria')
                        : t('createLab.vmsStep.snapshotSetDefaultAria')
                    "
                  />
                  <Button
                    icon="pi pi-times"
                    severity="danger"
                    text
                    size="small"
                    @click="emit('removeSnapshot', vmIndex, snapIndex)"
                    :disabled="vm.snapshots.length <= 1"
                    :aria-label="t('createLab.vmsStep.snapshotRemoveAria')"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>

        <Message v-else severity="info">{{ t('createLab.vmsStep.empty') }}</Message>
      </div>
    </template>
  </Card>
</template>
