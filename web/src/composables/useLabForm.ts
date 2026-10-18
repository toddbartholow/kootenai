/**
 * Composable for shared lab form logic between CreateLabView and EditLabView
 */
import { ref, computed } from 'vue'
import { useAuthStore } from '@/stores/auth'
import type { LabFormData, VMConfig, ObjectiveConfig, QuestionConfig } from '@/types/lab-form'
import { getDifficultyColor } from '@/utils/status'

// Re-export types for convenience
export * from '@/types/lab-form'

// Re-export form options from constants for backward compatibility
export {
  difficultyOptions,
  platformOptions,
  templateOptions,
  triggerTypeOptions,
  questionTypeOptions,
  validationTypeOptions,
} from '@/constants/formOptions'

export function useLabForm(initialData?: LabFormData) {
  const authStore = useAuthStore()

  // Form state
  const labData = ref<LabFormData>(
    initialData || {
      name: '',
      description: '',
      difficulty: 'beginner',
      durationMinutes: 60,
      platform: 'proxmox',
      version: '1.0.0',
      passThreshold: 70,
      visibility: 'private',
      isActive: true,
      tags: [],
    },
  )

  const vms = ref<VMConfig[]>([])
  const objectives = ref<ObjectiveConfig[]>([])
  const questions = ref<QuestionConfig[]>([])

  // Wizard state
  const currentStep = ref(1)
  const totalSteps = 5

  // New item inputs
  const tagInput = ref('')
  const newVMName = ref('')
  const newVMTemplate = ref('ubuntu-22.04-server')
  const newObjectiveDescription = ref('')
  const newObjectivePoints = ref(20)
  const newQuestionDescription = ref('')
  const newQuestionPoints = ref(10)
  const newQuestionType = ref<'text' | 'multiple_choice'>('text')

  // Visibility options (admin-dependent)
  const visibilityOptions = computed(() => {
    const options = [{ label: 'Private', value: 'private' }]
    if (authStore.isAdmin) {
      options.unshift({ label: 'Global (Everyone)', value: 'global' })
    }
    return options
  })

  // Validation computed properties
  const isStep1Valid = computed(() => {
    return (
      labData.value.name.trim().length >= 2 &&
      labData.value.version.trim().length > 0 &&
      labData.value.platform.length > 0
    )
  })

  const isStep2Valid = computed(() => {
    return (
      vms.value.length > 0 &&
      vms.value.every(vm => vm.name.trim().length >= 2 && vm.template.length > 0)
    )
  })

  const isStep3Valid = computed(() => {
    return (
      objectives.value.length > 0 &&
      objectives.value.every(obj => obj.description.trim().length > 0 && obj.points > 0)
    )
  })

  const isStep4Valid = computed(() => {
    if (questions.value.length === 0) return true
    return questions.value.every(q => {
      if (q.description.trim().length < 2) return false
      if (q.points <= 0) return false

      if (q.type === 'text') {
        if (q.validation.type === 'exact') {
          return q.validation.answer.trim().length > 0
        } else {
          return q.validation.pattern.trim().length > 0
        }
      } else {
        if (q.options.length < 2) return false
        if (!q.options.every(opt => opt.text.trim().length > 0)) return false
        if (!q.options.some(opt => opt.correct)) return false
      }
      return true
    })
  })

  const canProceed = computed(() => {
    switch (currentStep.value) {
      case 1:
        return isStep1Valid.value
      case 2:
        return isStep2Valid.value
      case 3:
        return isStep3Valid.value
      case 4:
        return isStep4Valid.value
      case 5:
        return true
      default:
        return false
    }
  })

  // Totals
  const totalObjectivePoints = computed(() =>
    objectives.value.reduce((sum, obj) => sum + obj.points, 0),
  )
  const totalQuestionPoints = computed(() => questions.value.reduce((sum, q) => sum + q.points, 0))
  const totalPoints = computed(() => totalObjectivePoints.value + totalQuestionPoints.value)
  const totalVMs = computed(() => vms.value.length)

  // Navigation functions
  function nextStep() {
    if (canProceed.value && currentStep.value < totalSteps) {
      currentStep.value++
    }
  }

  function prevStep() {
    if (currentStep.value > 1) {
      currentStep.value--
    }
  }

  function goToStep(step: number) {
    if (step <= currentStep.value || canProceed.value) {
      currentStep.value = step
    }
  }

  // Tag management
  function addTag() {
    const tag = tagInput.value.trim().toLowerCase()
    if (tag && !labData.value.tags.includes(tag)) {
      labData.value.tags = [...labData.value.tags, tag]
    }
    tagInput.value = ''
  }

  function removeTag(tag: string) {
    labData.value.tags = labData.value.tags.filter(t => t !== tag)
  }

  // VM management
  function addVM() {
    if (newVMName.value.trim().length < 2) return

    vms.value.push({
      id: `vm-${Date.now()}`,
      name: newVMName.value.trim(),
      template: newVMTemplate.value,
      cpu: 2,
      memory: 2048,
      disk: 16,
      wazuhAgent: true,
      snapshots: [{ name: 'initial', description: 'Initial state', isDefault: true }],
    })
    newVMName.value = ''
  }

  function removeVM(index: number) {
    vms.value.splice(index, 1)
  }

  function addSnapshot(vmIndex: number) {
    const vm = vms.value[vmIndex]
    if (!vm) return
    vm.snapshots.push({
      name: `snapshot-${vm.snapshots.length + 1}`,
      description: '',
      isDefault: false,
    })
  }

  function removeSnapshot(vmIndex: number, snapshotIndex: number) {
    const vm = vms.value[vmIndex]
    if (!vm) return
    vm.snapshots.splice(snapshotIndex, 1)
  }

  function setDefaultSnapshot(vmIndex: number, snapshotIndex: number) {
    const vm = vms.value[vmIndex]
    if (!vm) return
    vm.snapshots.forEach((s, i) => {
      s.isDefault = i === snapshotIndex
    })
  }

  // Objective management
  function addObjective() {
    if (newObjectiveDescription.value.trim().length < 2) return

    objectives.value.push({
      id: `obj-${Date.now()}`,
      description: newObjectiveDescription.value.trim(),
      points: newObjectivePoints.value,
      hint: '',
      order: objectives.value.length + 1,
      dependsOn: [],
      triggers: [
        {
          type: 'file_exists',
          target: vms.value[0]?.name || '',
          matchPath: '',
          matchContains: '',
          matchPattern: '',
        },
      ],
    })
    newObjectiveDescription.value = ''
    newObjectivePoints.value = 20
  }

  function removeObjective(index: number) {
    objectives.value.splice(index, 1)
    objectives.value.forEach((obj, i) => (obj.order = i + 1))
  }

  function moveObjectiveUp(index: number) {
    if (index > 0) {
      const current = objectives.value[index]
      const prev = objectives.value[index - 1]
      if (!current || !prev) return
      objectives.value[index] = prev
      objectives.value[index - 1] = current
      objectives.value.forEach((obj, i) => (obj.order = i + 1))
    }
  }

  function moveObjectiveDown(index: number) {
    if (index < objectives.value.length - 1) {
      const current = objectives.value[index]
      const next = objectives.value[index + 1]
      if (!current || !next) return
      objectives.value[index] = next
      objectives.value[index + 1] = current
      objectives.value.forEach((obj, i) => (obj.order = i + 1))
    }
  }

  function addTrigger(objIndex: number) {
    const obj = objectives.value[objIndex]
    if (!obj) return
    obj.triggers.push({
      type: 'file_exists',
      target: vms.value[0]?.name || '',
      matchPath: '',
      matchContains: '',
      matchPattern: '',
    })
  }

  function removeTrigger(objIndex: number, triggerIndex: number) {
    const obj = objectives.value[objIndex]
    if (!obj) return
    obj.triggers.splice(triggerIndex, 1)
  }

  // Question management
  function addQuestion() {
    if (newQuestionDescription.value.trim().length < 2) return

    const newQuestion: QuestionConfig = {
      id: `q-${Date.now()}`,
      type: newQuestionType.value,
      description: newQuestionDescription.value.trim(),
      points: newQuestionPoints.value,
      hint: '',
      order: questions.value.length + 1,
      dependsOn: [],
      validation: {
        type: 'exact',
        answer: '',
        pattern: '',
        caseSensitive: false,
      },
      options:
        newQuestionType.value === 'multiple_choice'
          ? [
              { id: 'A', text: '', correct: false },
              { id: 'B', text: '', correct: false },
            ]
          : [],
      multiSelect: false,
    }
    questions.value.push(newQuestion)
    newQuestionDescription.value = ''
    newQuestionPoints.value = 10
    newQuestionType.value = 'text'
  }

  function removeQuestion(index: number) {
    questions.value.splice(index, 1)
    questions.value.forEach((q, i) => (q.order = i + 1))
  }

  function moveQuestionUp(index: number) {
    if (index > 0) {
      const current = questions.value[index]
      const prev = questions.value[index - 1]
      if (!current || !prev) return
      questions.value[index] = prev
      questions.value[index - 1] = current
      questions.value.forEach((q, i) => (q.order = i + 1))
    }
  }

  function moveQuestionDown(index: number) {
    if (index < questions.value.length - 1) {
      const current = questions.value[index]
      const next = questions.value[index + 1]
      if (!current || !next) return
      questions.value[index] = next
      questions.value[index + 1] = current
      questions.value.forEach((q, i) => (q.order = i + 1))
    }
  }

  function duplicateQuestion(index: number) {
    const original = questions.value[index]
    if (!original) return
    const copy: QuestionConfig = {
      ...JSON.parse(JSON.stringify(original)),
      id: `q-${Date.now()}`,
      order: questions.value.length + 1,
    }
    questions.value.push(copy)
  }

  function handleQuestionTypeChange(qIndex: number, newType: 'text' | 'multiple_choice') {
    const q = questions.value[qIndex]
    if (!q) return
    q.type = newType
    if (newType === 'multiple_choice' && q.options.length < 2) {
      q.options = [
        { id: 'A', text: '', correct: false },
        { id: 'B', text: '', correct: false },
      ]
    }
  }

  function addOption(qIndex: number) {
    const q = questions.value[qIndex]
    if (!q) return
    const nextId = String.fromCharCode(65 + q.options.length)
    q.options.push({ id: nextId, text: '', correct: false })
  }

  function removeOption(qIndex: number, optIndex: number) {
    const q = questions.value[qIndex]
    if (!q) return
    if (q.options.length > 2) {
      q.options.splice(optIndex, 1)
      q.options.forEach((opt, i) => (opt.id = String.fromCharCode(65 + i)))
    }
  }

  function getAvailableDependencies(currentQuestionId: string) {
    const deps: { label: string; value: string }[] = []
    objectives.value.forEach(obj => {
      deps.push({ label: `[Obj] ${obj.description.substring(0, 40)}...`, value: obj.id })
    })
    questions.value.forEach(q => {
      if (q.id !== currentQuestionId) {
        deps.push({ label: `[Q] ${q.description.substring(0, 40)}...`, value: q.id })
      }
    })
    return deps
  }

  // Build lab spec JSON
  function buildLabSpec(): object {
    return {
      apiVersion: 'v1',
      kind: 'LabTemplate',
      metadata: {
        name: labData.value.name,
        description: labData.value.description,
        duration: `${labData.value.durationMinutes}m`,
        difficulty: labData.value.difficulty,
        tags: labData.value.tags,
        version: labData.value.version,
      },
      spec: {
        platform: labData.value.platform,
        network: {
          segments: [
            {
              name: 'lab-network',
              vlan: 100,
              subnet: '10.10.100.0/24',
              gateway: '10.10.100.1',
              dhcp: false,
            },
          ],
        },
        vms: vms.value.map((vm, idx) => ({
          name: vm.name,
          template: vm.template,
          wazuhAgent: vm.wazuhAgent,
          resources: {
            cpu: vm.cpu,
            memory: vm.memory,
            disk: vm.disk,
          },
          networks: [
            {
              segment: 'lab-network',
              ip: `10.10.100.${10 + idx}`,
            },
          ],
          snapshots: vm.snapshots.map(s => ({
            name: s.name,
            description: s.description,
            default: s.isDefault,
          })),
        })),
        checkpoints: {
          enabled: true,
          pass_threshold: labData.value.passThreshold,
          allow_retry: true,
          show_hints: true,
          realtime_update: true,
        },
        objectives: objectives.value.map(obj => ({
          id: obj.id,
          description: obj.description,
          points: obj.points,
          hint: obj.hint || undefined,
          depends_on: obj.dependsOn.length > 0 ? obj.dependsOn : undefined,
          order: obj.order,
          triggers: obj.triggers.map(t => {
            const trigger: Record<string, unknown> = {
              type: t.type,
              target: t.target,
              match: {},
            }
            if (t.type === 'file_exists' && t.matchPath) {
              trigger['match'] = { path: t.matchPath }
            } else if (t.type === 'file_content' && t.matchPath) {
              trigger['match'] = { path: t.matchPath, contains: t.matchContains }
            } else if (t.type === 'command_executed' && t.matchPattern) {
              trigger['match'] = { pattern: t.matchPattern }
            } else if (t.type === 'service_running' && t.matchPattern) {
              trigger['match'] = { name: t.matchPattern }
            }
            return trigger
          }),
        })),
        questions:
          questions.value.length > 0
            ? questions.value.map(q => {
                const question: Record<string, unknown> = {
                  id: q.id,
                  type: q.type,
                  description: q.description,
                  points: q.points,
                  order: q.order,
                }
                if (q.hint) question['hint'] = q.hint
                if (q.dependsOn.length > 0) question['depends_on'] = q.dependsOn
                if (q.type === 'text') {
                  question['validation'] = {
                    type: q.validation.type,
                    ...(q.validation.type === 'exact'
                      ? { answer: q.validation.answer, case_sensitive: q.validation.caseSensitive }
                      : { pattern: q.validation.pattern }),
                  }
                } else {
                  question['options'] = q.options.map(opt => ({
                    id: opt.id,
                    text: opt.text,
                    correct: opt.correct,
                  }))
                  question['multi_select'] = q.multiSelect
                }
                return question
              })
            : undefined,
      },
    }
  }

  // Reset form to initial state
  function resetForm() {
    labData.value = {
      name: '',
      description: '',
      difficulty: 'beginner',
      durationMinutes: 60,
      platform: 'proxmox',
      version: '1.0.0',
      passThreshold: 70,
      visibility: 'private',
      isActive: true,
      tags: [],
    }
    vms.value = []
    objectives.value = []
    questions.value = []
    currentStep.value = 1
  }

  return {
    // State
    labData,
    vms,
    objectives,
    questions,
    currentStep,
    totalSteps,

    // Input refs
    tagInput,
    newVMName,
    newVMTemplate,
    newObjectiveDescription,
    newObjectivePoints,
    newQuestionDescription,
    newQuestionPoints,
    newQuestionType,

    // Options
    visibilityOptions,

    // Validation
    isStep1Valid,
    isStep2Valid,
    isStep3Valid,
    isStep4Valid,
    canProceed,

    // Totals
    totalObjectivePoints,
    totalQuestionPoints,
    totalPoints,
    totalVMs,

    // Navigation
    nextStep,
    prevStep,
    goToStep,

    // Tag management
    addTag,
    removeTag,

    // VM management
    addVM,
    removeVM,
    addSnapshot,
    removeSnapshot,
    setDefaultSnapshot,

    // Objective management
    addObjective,
    removeObjective,
    moveObjectiveUp,
    moveObjectiveDown,
    addTrigger,
    removeTrigger,

    // Question management
    addQuestion,
    removeQuestion,
    moveQuestionUp,
    moveQuestionDown,
    duplicateQuestion,
    handleQuestionTypeChange,
    addOption,
    removeOption,
    getAvailableDependencies,

    // Spec building
    buildLabSpec,

    // Helpers
    getDifficultyColor,
    resetForm,
  }
}
