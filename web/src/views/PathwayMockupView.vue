<script setup lang="ts">
import { ref, computed } from 'vue'
import { logger } from '@/utils/logger'
import Card from '@volt/Card.vue'
import Button from '@volt/Button.vue'
import Tag from '@volt/Tag.vue'
import ProgressBar from '@volt/ProgressBar.vue'
import PathwayRoadmap from '../components/pathway/PathwayRoadmap.vue'
import type { PathwayModule, ModuleLab } from '@/api'
import { useFormatters } from '@/composables/useFormatters'

const { formatNumber } = useFormatters()

// Mock pathway data
const pathway = {
  id: 'mock-pathway-001',
  name: 'Linux Fundamentals',
  slug: 'linux-fundamentals',
  description: 'A comprehensive pathway to mastering Linux system administration. Starting from basic navigation and shell usage, progress through file management, text processing, user administration, and advanced topics like storage management and networking.',
  shortDescription: 'Master Linux from the command line up',
  difficulty: 'beginner',
  estimatedHours: 9,
  isFeatured: true,
  tags: ['linux', 'sysadmin', 'fundamentals', 'certification-prep'],
  prerequisites: ['Basic computer literacy', 'Familiarity with command-line concepts helpful but not required']
}

// Helper to create a mock module with all required fields
function createModule(
  id: string,
  name: string,
  slug: string,
  description: string,
  displayOrder: number,
  labs: Omit<ModuleLab, 'id' | 'moduleId'>[]
): PathwayModule {
  const now = new Date().toISOString()
  return {
    id,
    pathwayId: 'mock-pathway-001',
    name,
    slug,
    description,
    displayOrder,
    unlockType: 'sequential',
    isActive: true,
    createdAt: now,
    labCount: labs.length,
    totalPoints: labs.reduce((sum, l) => sum + (l.labMaxPoints || 0), 0),
    labs: labs.map((lab, idx) => ({
      ...lab,
      id: `${id}-lab-${idx + 1}`,
      moduleId: id,
    }))
  }
}

// Mock modules with labs
const modules = ref<PathwayModule[]>([
  createModule('mod-001', 'Linux Foundations', 'linux-foundations',
    'Learn to navigate the Linux filesystem and master basic text editing with vim and nano.',
    1, [
      { labTemplateId: 'tpl-001', displayOrder: 1, isRequired: true, labName: 'Navigating the Linux Filesystem', labDescription: 'Learn pwd, ls, cd, and basic navigation', labDurationMinutes: 45, labDifficulty: 'beginner', labMaxPoints: 100 }
    ]),
  createModule('mod-002', 'Shell Essentials', 'shell-essentials',
    'Master environment variables, PATH, command execution, and sudo privileges.',
    2, [
      { labTemplateId: 'tpl-002', displayOrder: 1, isRequired: true, labName: 'Shell Environment & Variables', labDescription: 'Configure PATH, env vars, and understand sudo', labDurationMinutes: 45, labDifficulty: 'beginner', labMaxPoints: 100 }
    ]),
  createModule('mod-003', 'File Mastery', 'file-mastery',
    'Deep dive into file permissions, ownership, symbolic links, and archive management.',
    3, [
      { labTemplateId: 'tpl-003a', displayOrder: 1, isRequired: true, labName: 'File Permissions & Ownership', labDescription: 'Master chmod, chown, and permission concepts', labDurationMinutes: 30, labDifficulty: 'intermediate', labMaxPoints: 60 },
      { labTemplateId: 'tpl-003b', displayOrder: 2, isRequired: true, labName: 'Links & Archives', labDescription: 'Create symbolic links and manage tar/gzip archives', labDurationMinutes: 30, labDifficulty: 'intermediate', labMaxPoints: 60 }
    ]),
  createModule('mod-004', 'Text Processing', 'text-processing',
    'Harness the power of grep, awk, sed, and pipes for text manipulation.',
    4, [
      { labTemplateId: 'tpl-004', displayOrder: 1, isRequired: true, labName: 'Text Processing with grep, awk & sed', labDescription: 'Filter, transform, and analyze text data', labDurationMinutes: 60, labDifficulty: 'intermediate', labMaxPoints: 120 }
    ]),
  createModule('mod-005', 'Process & User Management', 'process-user-management',
    'Control processes, manage users and groups, and understand system resources.',
    5, [
      { labTemplateId: 'tpl-005a', displayOrder: 1, isRequired: true, labName: 'Process Management', labDescription: 'Monitor and control processes with ps, top, kill', labDurationMinutes: 40, labDifficulty: 'intermediate', labMaxPoints: 75 },
      { labTemplateId: 'tpl-005b', displayOrder: 2, isRequired: true, labName: 'User & Group Administration', labDescription: 'Create users, manage groups, set permissions', labDurationMinutes: 35, labDifficulty: 'intermediate', labMaxPoints: 75 }
    ]),
  createModule('mod-006', 'Services & System', 'services-system',
    'Master systemd service management and log analysis with journalctl.',
    6, [
      { labTemplateId: 'tpl-006', displayOrder: 1, isRequired: true, labName: 'Systemd & Service Management', labDescription: 'Control services, analyze logs, configure startup', labDurationMinutes: 60, labDifficulty: 'intermediate', labMaxPoints: 130 }
    ]),
  createModule('mod-007', 'Storage & Packages', 'storage-packages',
    'Manage disk partitions, filesystems, LVM, and package managers.',
    7, [
      { labTemplateId: 'tpl-007a', displayOrder: 1, isRequired: true, labName: 'Disk & Filesystem Management', labDescription: 'Partition disks, create filesystems, mount volumes', labDurationMinutes: 50, labDifficulty: 'advanced', labMaxPoints: 80 },
      { labTemplateId: 'tpl-007b', displayOrder: 2, isRequired: true, labName: 'Package Management', labDescription: 'Install, update, and manage packages with apt/dnf', labDurationMinutes: 40, labDifficulty: 'intermediate', labMaxPoints: 70 }
    ]),
  createModule('mod-008', 'Networking', 'networking',
    'Configure network interfaces, troubleshoot connectivity, and secure with firewalls.',
    8, [
      { labTemplateId: 'tpl-008a', displayOrder: 1, isRequired: true, labName: 'Network Configuration', labDescription: 'Configure IP addresses, DNS, and routing', labDurationMinutes: 45, labDifficulty: 'advanced', labMaxPoints: 80 },
      { labTemplateId: 'tpl-008b', displayOrder: 2, isRequired: true, labName: 'Firewall & Security', labDescription: 'Configure iptables/firewalld and secure services', labDurationMinutes: 45, labDifficulty: 'advanced', labMaxPoints: 80 }
    ])
])

// Mock progress data - simulating a user partway through the pathway
const moduleProgress = ref(new Map<string, { status: string; percentage: number }>([
  ['mod-001', { status: 'completed', percentage: 100 }],
  ['mod-002', { status: 'completed', percentage: 100 }],
  ['mod-003', { status: 'in_progress', percentage: 50 }],
  ['mod-004', { status: 'unlocked', percentage: 0 }],
  ['mod-005', { status: 'locked', percentage: 0 }],
  ['mod-006', { status: 'locked', percentage: 0 }],
  ['mod-007', { status: 'locked', percentage: 0 }],
  ['mod-008', { status: 'locked', percentage: 0 }]
]))

// Toggle progress simulation
const showProgress = ref(true)

// Calculate totals
const totalLabs = computed(() => modules.value.reduce((sum, m) => sum + (m.labs?.length || 0), 0))
const totalPoints = computed(() => modules.value.reduce((sum, m) => sum + (m.totalPoints || 0), 0))
const completedModules = computed(() => {
  let count = 0
  moduleProgress.value.forEach((p) => { if (p.status === 'completed') count++ })
  return count
})
const overallProgress = computed(() => Math.round((completedModules.value / modules.value.length) * 100))

// Computed progress for component - use empty map when not showing progress
const displayProgress = computed(() => {
  return showProgress.value ? moduleProgress.value : new Map()
})

// Handlers
function handleModuleClick(module: PathwayModule) {
  logger.debug('Module clicked:', { module: 'PathwayMockup' }, module.name)
  // In real app, this would expand module or navigate to module detail
}

function handleLabClick(lab: ModuleLab, module: PathwayModule) {
  logger.debug('Lab clicked:', { module: 'PathwayMockup' }, lab.labName, 'in module:', module.name)
  // In real app, this would start the lab session
}

function toggleProgress() {
  showProgress.value = !showProgress.value
}

function resetProgress() {
  moduleProgress.value = new Map([
    ['mod-001', { status: 'unlocked', percentage: 0 }],
    ['mod-002', { status: 'locked', percentage: 0 }],
    ['mod-003', { status: 'locked', percentage: 0 }],
    ['mod-004', { status: 'locked', percentage: 0 }],
    ['mod-005', { status: 'locked', percentage: 0 }],
    ['mod-006', { status: 'locked', percentage: 0 }],
    ['mod-007', { status: 'locked', percentage: 0 }],
    ['mod-008', { status: 'locked', percentage: 0 }]
  ])
}

function completeAll() {
  moduleProgress.value = new Map([
    ['mod-001', { status: 'completed', percentage: 100 }],
    ['mod-002', { status: 'completed', percentage: 100 }],
    ['mod-003', { status: 'completed', percentage: 100 }],
    ['mod-004', { status: 'completed', percentage: 100 }],
    ['mod-005', { status: 'completed', percentage: 100 }],
    ['mod-006', { status: 'completed', percentage: 100 }],
    ['mod-007', { status: 'completed', percentage: 100 }],
    ['mod-008', { status: 'completed', percentage: 100 }]
  ])
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header with mockup badge -->
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-3">
        <h1 class="text-2xl font-bold text-surface-900 dark:text-surface-100">Pathway Mockup</h1>
        <Tag value="UI Preview" severity="info" />
      </div>
      <div class="flex gap-2">
        <Button
          @click="toggleProgress"
          :label="showProgress ? 'Hide Progress' : 'Show Progress'"
          :icon="showProgress ? 'pi pi-eye-slash' : 'pi pi-eye'"
          severity="secondary"
          size="small"
        />
        <Button @click="resetProgress" label="Reset" icon="pi pi-refresh" severity="secondary" size="small" />
        <Button @click="completeAll" label="Complete All" icon="pi pi-check-circle" severity="success" size="small" />
      </div>
    </div>

    <!-- Pathway Header -->
    <div class="flex flex-col lg:flex-row gap-6">
      <div class="flex-1 space-y-4">
        <div class="flex items-center gap-2 mb-2">
          <Tag v-if="pathway.isFeatured" value="Featured" severity="warn" />
          <Tag :value="pathway.difficulty" severity="success" />
        </div>
        <h2 class="text-3xl font-bold text-surface-900 dark:text-surface-100">{{ pathway.name }}</h2>
        <p class="text-lg text-surface-600 dark:text-surface-400">{{ pathway.description }}</p>
        <div class="flex flex-wrap gap-2">
          <Tag v-for="tag in pathway.tags" :key="tag" :value="tag" severity="secondary" />
        </div>
      </div>

      <!-- Stats Card -->
      <Card class="lg:w-80 flex-shrink-0">
        <template #content>
          <div class="space-y-4">
            <!-- Progress (when enrolled) -->
            <div v-if="showProgress" class="space-y-2">
              <div class="flex justify-between text-sm">
                <span class="text-surface-600 dark:text-surface-400">Overall Progress</span>
                <span class="font-semibold text-surface-900 dark:text-surface-100">{{ overallProgress }}%</span>
              </div>
              <ProgressBar :value="overallProgress" :showValue="false" class="h-2" />
              <p class="text-xs text-surface-500">{{ completedModules }} of {{ modules.length }} modules completed</p>
            </div>

            <div class="grid grid-cols-2 gap-4 text-center">
              <div>
                <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ modules.length }}</p>
                <p class="text-sm text-surface-500">Modules</p>
              </div>
              <div>
                <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ totalLabs }}</p>
                <p class="text-sm text-surface-500">Labs</p>
              </div>
              <div>
                <p class="text-2xl font-bold text-surface-900 dark:text-surface-100">{{ pathway.estimatedHours }}h</p>
                <p class="text-sm text-surface-500">Estimated</p>
              </div>
              <div>
                <p class="text-2xl font-bold text-amber-600 dark:text-amber-400">{{ formatNumber(totalPoints) }}</p>
                <p class="text-sm text-surface-500">Total Points</p>
              </div>
            </div>

            <div class="border-t border-surface-200 dark:border-surface-700 pt-4">
              <Button
                :label="showProgress ? 'Continue Learning' : 'Enroll Now'"
                :icon="showProgress ? 'pi pi-play' : 'pi pi-plus'"
                class="w-full"
                size="large"
              />
            </div>
          </div>
        </template>
      </Card>
    </div>

    <!-- Prerequisites -->
    <Card v-if="pathway.prerequisites?.length">
      <template #title>
        <div class="flex items-center gap-2">
          <i class="pi pi-exclamation-triangle text-amber-500" />
          Prerequisites
        </div>
      </template>
      <template #content>
        <ul class="list-disc list-inside space-y-1 text-surface-600 dark:text-surface-400">
          <li v-for="(prereq, index) in pathway.prerequisites" :key="index">{{ prereq }}</li>
        </ul>
      </template>
    </Card>

    <!-- Roadmap Visualization -->
    <div>
      <h3 class="text-xl font-semibold text-surface-900 dark:text-surface-100 mb-4">Learning Roadmap</h3>
      <Card class="overflow-hidden">
        <template #content>
          <PathwayRoadmap
            :modules="modules"
            :module-progress="displayProgress"
            :on-module-click="handleModuleClick"
            :on-lab-click="handleLabClick"
          />
        </template>
      </Card>
    </div>

    <!-- Legend -->
    <Card>
      <template #title>Status Legend</template>
      <template #content>
        <div class="flex flex-wrap gap-6">
          <div class="flex items-center gap-2">
            <div class="w-8 h-8 rounded-full bg-white dark:bg-surface-800 flex items-center justify-center shadow-md text-green-600 dark:text-green-400">
              <i class="pi pi-check-circle" />
            </div>
            <span class="text-sm text-surface-600 dark:text-surface-400">Completed</span>
          </div>
          <div class="flex items-center gap-2">
            <div class="w-8 h-8 rounded-full bg-white dark:bg-surface-800 flex items-center justify-center shadow-md text-blue-600 dark:text-blue-400">
              <i class="pi pi-spin pi-spinner" />
            </div>
            <span class="text-sm text-surface-600 dark:text-surface-400">In Progress</span>
          </div>
          <div class="flex items-center gap-2">
            <div class="w-8 h-8 rounded-full bg-white dark:bg-surface-800 flex items-center justify-center shadow-md text-amber-600 dark:text-amber-400">
              <i class="pi pi-lock-open" />
            </div>
            <span class="text-sm text-surface-600 dark:text-surface-400">Unlocked</span>
          </div>
          <div class="flex items-center gap-2">
            <div class="w-8 h-8 rounded-full bg-white dark:bg-surface-800 flex items-center justify-center shadow-md text-surface-400 dark:text-surface-500">
              <i class="pi pi-lock" />
            </div>
            <span class="text-sm text-surface-600 dark:text-surface-400">Locked</span>
          </div>
        </div>
      </template>
    </Card>
  </div>
</template>
