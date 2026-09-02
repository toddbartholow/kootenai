import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useLabForm } from './useLabForm'
import { useAuthStore } from '@/stores/auth'

describe('useLabForm', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  describe('initialization', () => {
    it('should initialize with default values when no initial data provided', () => {
      const form = useLabForm()

      expect(form.labData.value.name).toBe('')
      expect(form.labData.value.difficulty).toBe('beginner')
      expect(form.labData.value.durationMinutes).toBe(60)
      expect(form.labData.value.platform).toBe('proxmox')
      expect(form.labData.value.version).toBe('1.0.0')
      expect(form.labData.value.passThreshold).toBe(70)
      expect(form.labData.value.visibility).toBe('private')
      expect(form.labData.value.isActive).toBe(true)
      expect(form.labData.value.tags).toEqual([])
    })

    it('should initialize with provided initial data', () => {
      const initialData = {
        name: 'Test Lab',
        description: 'A test lab',
        difficulty: 'intermediate',
        durationMinutes: 120,
        platform: 'cloudstack',
        version: '2.0.0',
        passThreshold: 80,
        visibility: 'global',
        isActive: false,
        tags: ['security', 'networking'],
      }

      const form = useLabForm(initialData)

      expect(form.labData.value.name).toBe('Test Lab')
      expect(form.labData.value.description).toBe('A test lab')
      expect(form.labData.value.difficulty).toBe('intermediate')
      expect(form.labData.value.durationMinutes).toBe(120)
      expect(form.labData.value.platform).toBe('cloudstack')
      expect(form.labData.value.version).toBe('2.0.0')
      expect(form.labData.value.passThreshold).toBe(80)
      expect(form.labData.value.visibility).toBe('global')
      expect(form.labData.value.isActive).toBe(false)
      expect(form.labData.value.tags).toEqual(['security', 'networking'])
    })

    it('should start with empty VMs, objectives, and questions arrays', () => {
      const form = useLabForm()

      expect(form.vms.value).toEqual([])
      expect(form.objectives.value).toEqual([])
      expect(form.questions.value).toEqual([])
    })

    it('should start at step 1', () => {
      const form = useLabForm()

      expect(form.currentStep.value).toBe(1)
      expect(form.totalSteps).toBe(5)
    })
  })

  describe('step validation', () => {
    describe('isStep1Valid', () => {
      it('should be false when name is empty', () => {
        const form = useLabForm()
        form.labData.value.name = ''
        form.labData.value.version = '1.0.0'
        form.labData.value.platform = 'proxmox'

        expect(form.isStep1Valid.value).toBe(false)
      })

      it('should be false when name is too short', () => {
        const form = useLabForm()
        form.labData.value.name = 'A'
        form.labData.value.version = '1.0.0'
        form.labData.value.platform = 'proxmox'

        expect(form.isStep1Valid.value).toBe(false)
      })

      it('should be false when version is empty', () => {
        const form = useLabForm()
        form.labData.value.name = 'Valid Name'
        form.labData.value.version = ''
        form.labData.value.platform = 'proxmox'

        expect(form.isStep1Valid.value).toBe(false)
      })

      it('should be false when platform is empty', () => {
        const form = useLabForm()
        form.labData.value.name = 'Valid Name'
        form.labData.value.version = '1.0.0'
        form.labData.value.platform = ''

        expect(form.isStep1Valid.value).toBe(false)
      })

      it('should be true when all fields are valid', () => {
        const form = useLabForm()
        form.labData.value.name = 'Valid Name'
        form.labData.value.version = '1.0.0'
        form.labData.value.platform = 'proxmox'

        expect(form.isStep1Valid.value).toBe(true)
      })
    })

    describe('isStep2Valid', () => {
      it('should be false when no VMs are added', () => {
        const form = useLabForm()

        expect(form.isStep2Valid.value).toBe(false)
      })

      it('should be false when VM name is too short', () => {
        const form = useLabForm()
        form.vms.value = [
          {
            id: 'vm-1',
            name: 'A',
            template: 'ubuntu-22.04',
            cpu: 2,
            memory: 2048,
            disk: 16,
            wazuhAgent: true,
            snapshots: [],
          },
        ]

        expect(form.isStep2Valid.value).toBe(false)
      })

      it('should be false when VM template is empty', () => {
        const form = useLabForm()
        form.vms.value = [
          {
            id: 'vm-1',
            name: 'Valid VM',
            template: '',
            cpu: 2,
            memory: 2048,
            disk: 16,
            wazuhAgent: true,
            snapshots: [],
          },
        ]

        expect(form.isStep2Valid.value).toBe(false)
      })

      it('should be true when VMs are properly configured', () => {
        const form = useLabForm()
        form.vms.value = [
          {
            id: 'vm-1',
            name: 'Valid VM',
            template: 'ubuntu-22.04',
            cpu: 2,
            memory: 2048,
            disk: 16,
            wazuhAgent: true,
            snapshots: [],
          },
        ]

        expect(form.isStep2Valid.value).toBe(true)
      })
    })

    describe('isStep3Valid', () => {
      it('should be false when no objectives are added', () => {
        const form = useLabForm()

        expect(form.isStep3Valid.value).toBe(false)
      })

      it('should be false when objective description is empty', () => {
        const form = useLabForm()
        form.objectives.value = [
          {
            id: 'obj-1',
            description: '',
            points: 20,
            hint: '',
            order: 1,
            dependsOn: [],
            triggers: [],
          },
        ]

        expect(form.isStep3Valid.value).toBe(false)
      })

      it('should be false when objective points is 0', () => {
        const form = useLabForm()
        form.objectives.value = [
          {
            id: 'obj-1',
            description: 'Valid description',
            points: 0,
            hint: '',
            order: 1,
            dependsOn: [],
            triggers: [],
          },
        ]

        expect(form.isStep3Valid.value).toBe(false)
      })

      it('should be true when objectives are properly configured', () => {
        const form = useLabForm()
        form.objectives.value = [
          {
            id: 'obj-1',
            description: 'Valid description',
            points: 20,
            hint: '',
            order: 1,
            dependsOn: [],
            triggers: [],
          },
        ]

        expect(form.isStep3Valid.value).toBe(true)
      })
    })

    describe('isStep4Valid', () => {
      it('should be true when no questions are added', () => {
        const form = useLabForm()

        expect(form.isStep4Valid.value).toBe(true)
      })

      it('should be false when text question description is too short', () => {
        const form = useLabForm()
        form.questions.value = [
          {
            id: 'q-1',
            type: 'text',
            description: 'A',
            points: 10,
            hint: '',
            order: 1,
            dependsOn: [],
            validation: { type: 'exact', answer: 'answer', pattern: '', caseSensitive: false },
            options: [],
            multiSelect: false,
          },
        ]

        expect(form.isStep4Valid.value).toBe(false)
      })

      it('should be false when text question has zero points', () => {
        const form = useLabForm()
        form.questions.value = [
          {
            id: 'q-1',
            type: 'text',
            description: 'Valid question',
            points: 0,
            hint: '',
            order: 1,
            dependsOn: [],
            validation: { type: 'exact', answer: 'answer', pattern: '', caseSensitive: false },
            options: [],
            multiSelect: false,
          },
        ]

        expect(form.isStep4Valid.value).toBe(false)
      })

      it('should be false when exact text question has no answer', () => {
        const form = useLabForm()
        form.questions.value = [
          {
            id: 'q-1',
            type: 'text',
            description: 'Valid question',
            points: 10,
            hint: '',
            order: 1,
            dependsOn: [],
            validation: { type: 'exact', answer: '', pattern: '', caseSensitive: false },
            options: [],
            multiSelect: false,
          },
        ]

        expect(form.isStep4Valid.value).toBe(false)
      })

      it('should be false when regex text question has no pattern', () => {
        const form = useLabForm()
        form.questions.value = [
          {
            id: 'q-1',
            type: 'text',
            description: 'Valid question',
            points: 10,
            hint: '',
            order: 1,
            dependsOn: [],
            validation: { type: 'regex', answer: '', pattern: '', caseSensitive: false },
            options: [],
            multiSelect: false,
          },
        ]

        expect(form.isStep4Valid.value).toBe(false)
      })

      it('should be true when text question with exact validation is valid', () => {
        const form = useLabForm()
        form.questions.value = [
          {
            id: 'q-1',
            type: 'text',
            description: 'Valid question',
            points: 10,
            hint: '',
            order: 1,
            dependsOn: [],
            validation: {
              type: 'exact',
              answer: 'correct answer',
              pattern: '',
              caseSensitive: false,
            },
            options: [],
            multiSelect: false,
          },
        ]

        expect(form.isStep4Valid.value).toBe(true)
      })

      it('should be true when text question with regex validation is valid', () => {
        const form = useLabForm()
        form.questions.value = [
          {
            id: 'q-1',
            type: 'text',
            description: 'Valid question',
            points: 10,
            hint: '',
            order: 1,
            dependsOn: [],
            validation: { type: 'regex', answer: '', pattern: '^\\d+$', caseSensitive: false },
            options: [],
            multiSelect: false,
          },
        ]

        expect(form.isStep4Valid.value).toBe(true)
      })

      it('should be false when multiple choice has less than 2 options', () => {
        const form = useLabForm()
        form.questions.value = [
          {
            id: 'q-1',
            type: 'multiple_choice',
            description: 'Valid question',
            points: 10,
            hint: '',
            order: 1,
            dependsOn: [],
            validation: { type: 'exact', answer: '', pattern: '', caseSensitive: false },
            options: [{ id: 'A', text: 'Option A', correct: true }],
            multiSelect: false,
          },
        ]

        expect(form.isStep4Valid.value).toBe(false)
      })

      it('should be false when multiple choice has empty option text', () => {
        const form = useLabForm()
        form.questions.value = [
          {
            id: 'q-1',
            type: 'multiple_choice',
            description: 'Valid question',
            points: 10,
            hint: '',
            order: 1,
            dependsOn: [],
            validation: { type: 'exact', answer: '', pattern: '', caseSensitive: false },
            options: [
              { id: 'A', text: 'Option A', correct: true },
              { id: 'B', text: '', correct: false },
            ],
            multiSelect: false,
          },
        ]

        expect(form.isStep4Valid.value).toBe(false)
      })

      it('should be false when multiple choice has no correct answer', () => {
        const form = useLabForm()
        form.questions.value = [
          {
            id: 'q-1',
            type: 'multiple_choice',
            description: 'Valid question',
            points: 10,
            hint: '',
            order: 1,
            dependsOn: [],
            validation: { type: 'exact', answer: '', pattern: '', caseSensitive: false },
            options: [
              { id: 'A', text: 'Option A', correct: false },
              { id: 'B', text: 'Option B', correct: false },
            ],
            multiSelect: false,
          },
        ]

        expect(form.isStep4Valid.value).toBe(false)
      })

      it('should be true when multiple choice question is valid', () => {
        const form = useLabForm()
        form.questions.value = [
          {
            id: 'q-1',
            type: 'multiple_choice',
            description: 'Valid question',
            points: 10,
            hint: '',
            order: 1,
            dependsOn: [],
            validation: { type: 'exact', answer: '', pattern: '', caseSensitive: false },
            options: [
              { id: 'A', text: 'Option A', correct: true },
              { id: 'B', text: 'Option B', correct: false },
            ],
            multiSelect: false,
          },
        ]

        expect(form.isStep4Valid.value).toBe(true)
      })
    })

    describe('canProceed', () => {
      it('should reflect current step validation', () => {
        const form = useLabForm()
        form.currentStep.value = 1

        // Step 1 requires name, version, platform
        expect(form.canProceed.value).toBe(false)

        form.labData.value.name = 'Valid Name'
        expect(form.canProceed.value).toBe(true)
      })

      it('should return true for step 5', () => {
        const form = useLabForm()
        form.currentStep.value = 5

        expect(form.canProceed.value).toBe(true)
      })

      it('should return false for invalid step number', () => {
        const form = useLabForm()
        form.currentStep.value = 99

        expect(form.canProceed.value).toBe(false)
      })
    })
  })

  describe('navigation', () => {
    it('nextStep should advance step when valid', () => {
      const form = useLabForm()
      form.labData.value.name = 'Valid Name'
      form.currentStep.value = 1

      form.nextStep()

      expect(form.currentStep.value).toBe(2)
    })

    it('nextStep should not advance when step is invalid', () => {
      const form = useLabForm()
      form.labData.value.name = ''
      form.currentStep.value = 1

      form.nextStep()

      expect(form.currentStep.value).toBe(1)
    })

    it('nextStep should not go past totalSteps', () => {
      const form = useLabForm()
      form.currentStep.value = 5

      form.nextStep()

      expect(form.currentStep.value).toBe(5)
    })

    it('prevStep should go back', () => {
      const form = useLabForm()
      form.currentStep.value = 3

      form.prevStep()

      expect(form.currentStep.value).toBe(2)
    })

    it('prevStep should not go below 1', () => {
      const form = useLabForm()
      form.currentStep.value = 1

      form.prevStep()

      expect(form.currentStep.value).toBe(1)
    })

    it('goToStep should navigate to lower step', () => {
      const form = useLabForm()
      form.currentStep.value = 3

      form.goToStep(1)

      expect(form.currentStep.value).toBe(1)
    })

    it('goToStep should navigate to higher step when canProceed', () => {
      const form = useLabForm()
      form.labData.value.name = 'Valid Name'
      form.currentStep.value = 1

      form.goToStep(2)

      expect(form.currentStep.value).toBe(2)
    })
  })

  describe('tag management', () => {
    it('addTag should add a new tag', () => {
      const form = useLabForm()
      form.tagInput.value = 'security'

      form.addTag()

      expect(form.labData.value.tags).toContain('security')
      expect(form.tagInput.value).toBe('')
    })

    it('addTag should lowercase tags', () => {
      const form = useLabForm()
      form.tagInput.value = 'SECURITY'

      form.addTag()

      expect(form.labData.value.tags).toContain('security')
    })

    it('addTag should not add duplicate tags', () => {
      const form = useLabForm()
      form.labData.value.tags = ['security']
      form.tagInput.value = 'security'

      form.addTag()

      expect(form.labData.value.tags).toEqual(['security'])
    })

    it('addTag should not add empty tags', () => {
      const form = useLabForm()
      form.tagInput.value = '   '

      form.addTag()

      expect(form.labData.value.tags).toEqual([])
    })

    it('removeTag should remove a tag', () => {
      const form = useLabForm()
      form.labData.value.tags = ['security', 'networking']

      form.removeTag('security')

      expect(form.labData.value.tags).toEqual(['networking'])
    })
  })

  describe('VM management', () => {
    it('addVM should add a new VM with default values', () => {
      const form = useLabForm()
      form.newVMName.value = 'web-server'
      form.newVMTemplate.value = 'ubuntu-22.04'

      form.addVM()

      expect(form.vms.value).toHaveLength(1)
      expect(form.vms.value[0]!.name).toBe('web-server')
      expect(form.vms.value[0]!.template).toBe('ubuntu-22.04')
      expect(form.vms.value[0]!.cpu).toBe(2)
      expect(form.vms.value[0]!.memory).toBe(2048)
      expect(form.vms.value[0]!.disk).toBe(16)
      expect(form.vms.value[0]!.wazuhAgent).toBe(true)
      expect(form.vms.value[0]!.snapshots).toHaveLength(1)
      expect(form.vms.value[0]!.snapshots[0]!.isDefault).toBe(true)
      expect(form.newVMName.value).toBe('')
    })

    it('addVM should not add VM with short name', () => {
      const form = useLabForm()
      form.newVMName.value = 'A'

      form.addVM()

      expect(form.vms.value).toHaveLength(0)
    })

    it('removeVM should remove VM at index', () => {
      const form = useLabForm()
      form.newVMName.value = 'vm1'
      form.addVM()
      form.newVMName.value = 'vm2'
      form.addVM()

      form.removeVM(0)

      expect(form.vms.value).toHaveLength(1)
      expect(form.vms.value[0]!.name).toBe('vm2')
    })

    it('addSnapshot should add snapshot to VM', () => {
      const form = useLabForm()
      form.newVMName.value = 'vm1'
      form.addVM()

      form.addSnapshot(0)

      expect(form.vms.value[0]!.snapshots).toHaveLength(2)
      expect(form.vms.value[0]!.snapshots[1]!.name).toBe('snapshot-2')
    })

    it('addSnapshot should handle invalid index', () => {
      const form = useLabForm()

      form.addSnapshot(99)

      // Should not throw
    })

    it('removeSnapshot should remove snapshot from VM', () => {
      const form = useLabForm()
      form.newVMName.value = 'vm1'
      form.addVM()
      form.addSnapshot(0)

      form.removeSnapshot(0, 1)

      expect(form.vms.value[0]!.snapshots).toHaveLength(1)
    })

    it('setDefaultSnapshot should set the correct snapshot as default', () => {
      const form = useLabForm()
      form.newVMName.value = 'vm1'
      form.addVM()
      form.addSnapshot(0)

      form.setDefaultSnapshot(0, 1)

      expect(form.vms.value[0]!.snapshots[0]!.isDefault).toBe(false)
      expect(form.vms.value[0]!.snapshots[1]!.isDefault).toBe(true)
    })
  })

  describe('objective management', () => {
    it('addObjective should add a new objective', () => {
      const form = useLabForm()
      form.newVMName.value = 'vm1'
      form.addVM()
      form.newObjectiveDescription.value = 'Create a file'
      form.newObjectivePoints.value = 25

      form.addObjective()

      expect(form.objectives.value).toHaveLength(1)
      expect(form.objectives.value[0]!.description).toBe('Create a file')
      expect(form.objectives.value[0]!.points).toBe(25)
      expect(form.objectives.value[0]!.order).toBe(1)
      expect(form.objectives.value[0]!.triggers).toHaveLength(1)
      expect(form.newObjectiveDescription.value).toBe('')
      expect(form.newObjectivePoints.value).toBe(20)
    })

    it('addObjective should not add with short description', () => {
      const form = useLabForm()
      form.newObjectiveDescription.value = 'A'

      form.addObjective()

      expect(form.objectives.value).toHaveLength(0)
    })

    it('removeObjective should remove and reorder', () => {
      const form = useLabForm()
      form.newObjectiveDescription.value = 'Obj 1'
      form.addObjective()
      form.newObjectiveDescription.value = 'Obj 2'
      form.addObjective()
      form.newObjectiveDescription.value = 'Obj 3'
      form.addObjective()

      form.removeObjective(1)

      expect(form.objectives.value).toHaveLength(2)
      expect(form.objectives.value[0]!.order).toBe(1)
      expect(form.objectives.value[1]!.order).toBe(2)
    })

    it('moveObjectiveUp should swap with previous', () => {
      const form = useLabForm()
      form.newObjectiveDescription.value = 'Obj 1'
      form.addObjective()
      form.newObjectiveDescription.value = 'Obj 2'
      form.addObjective()

      form.moveObjectiveUp(1)

      expect(form.objectives.value[0]!.description).toBe('Obj 2')
      expect(form.objectives.value[1]!.description).toBe('Obj 1')
      expect(form.objectives.value[0]!.order).toBe(1)
      expect(form.objectives.value[1]!.order).toBe(2)
    })

    it('moveObjectiveUp should not move first item', () => {
      const form = useLabForm()
      form.newObjectiveDescription.value = 'Obj 1'
      form.addObjective()

      form.moveObjectiveUp(0)

      expect(form.objectives.value[0]!.description).toBe('Obj 1')
    })

    it('moveObjectiveDown should swap with next', () => {
      const form = useLabForm()
      form.newObjectiveDescription.value = 'Obj 1'
      form.addObjective()
      form.newObjectiveDescription.value = 'Obj 2'
      form.addObjective()

      form.moveObjectiveDown(0)

      expect(form.objectives.value[0]!.description).toBe('Obj 2')
      expect(form.objectives.value[1]!.description).toBe('Obj 1')
    })

    it('moveObjectiveDown should not move last item', () => {
      const form = useLabForm()
      form.newObjectiveDescription.value = 'Obj 1'
      form.addObjective()

      form.moveObjectiveDown(0)

      expect(form.objectives.value[0]!.description).toBe('Obj 1')
    })

    it('addTrigger should add trigger to objective', () => {
      const form = useLabForm()
      form.newObjectiveDescription.value = 'Obj 1'
      form.addObjective()

      form.addTrigger(0)

      expect(form.objectives.value[0]!.triggers).toHaveLength(2)
    })

    it('removeTrigger should remove trigger from objective', () => {
      const form = useLabForm()
      form.newObjectiveDescription.value = 'Obj 1'
      form.addObjective()
      form.addTrigger(0)

      form.removeTrigger(0, 1)

      expect(form.objectives.value[0]!.triggers).toHaveLength(1)
    })
  })

  describe('question management', () => {
    it('addQuestion should add a text question', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'What is 2+2?'
      form.newQuestionPoints.value = 15
      form.newQuestionType.value = 'text'

      form.addQuestion()

      expect(form.questions.value).toHaveLength(1)
      expect(form.questions.value[0]!.description).toBe('What is 2+2?')
      expect(form.questions.value[0]!.points).toBe(15)
      expect(form.questions.value[0]!.type).toBe('text')
      expect(form.questions.value[0]!.options).toEqual([])
      expect(form.newQuestionDescription.value).toBe('')
      expect(form.newQuestionPoints.value).toBe(10)
      expect(form.newQuestionType.value).toBe('text')
    })

    it('addQuestion should add a multiple choice question with default options', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'Which is correct?'
      form.newQuestionType.value = 'multiple_choice'

      form.addQuestion()

      expect(form.questions.value).toHaveLength(1)
      expect(form.questions.value[0]!.type).toBe('multiple_choice')
      expect(form.questions.value[0]!.options).toHaveLength(2)
      expect(form.questions.value[0]!.options[0]!.id).toBe('A')
      expect(form.questions.value[0]!.options[1]!.id).toBe('B')
    })

    it('addQuestion should not add with short description', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'A'

      form.addQuestion()

      expect(form.questions.value).toHaveLength(0)
    })

    it('removeQuestion should remove and reorder', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'Q1'
      form.addQuestion()
      form.newQuestionDescription.value = 'Q2'
      form.addQuestion()

      form.removeQuestion(0)

      expect(form.questions.value).toHaveLength(1)
      expect(form.questions.value[0]!.order).toBe(1)
    })

    it('moveQuestionUp should swap with previous', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'Q1'
      form.addQuestion()
      form.newQuestionDescription.value = 'Q2'
      form.addQuestion()

      form.moveQuestionUp(1)

      expect(form.questions.value[0]!.description).toBe('Q2')
      expect(form.questions.value[1]!.description).toBe('Q1')
    })

    it('moveQuestionDown should swap with next', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'Q1'
      form.addQuestion()
      form.newQuestionDescription.value = 'Q2'
      form.addQuestion()

      form.moveQuestionDown(0)

      expect(form.questions.value[0]!.description).toBe('Q2')
      expect(form.questions.value[1]!.description).toBe('Q1')
    })

    it('duplicateQuestion should create a copy', () => {
      vi.useFakeTimers()
      const form = useLabForm()
      form.newQuestionDescription.value = 'Original Q'
      form.newQuestionPoints.value = 25
      form.addQuestion()

      // Advance time so duplicate gets different ID
      vi.advanceTimersByTime(10)
      form.duplicateQuestion(0)

      expect(form.questions.value).toHaveLength(2)
      expect(form.questions.value[1]!.description).toBe('Original Q')
      expect(form.questions.value[1]!.points).toBe(25)
      expect(form.questions.value[1]!.id).not.toBe(form.questions.value[0]!.id)
      vi.useRealTimers()
    })

    it('handleQuestionTypeChange should convert to multiple choice', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'Question'
      form.newQuestionType.value = 'text'
      form.addQuestion()

      form.handleQuestionTypeChange(0, 'multiple_choice')

      expect(form.questions.value[0]!.type).toBe('multiple_choice')
      expect(form.questions.value[0]!.options).toHaveLength(2)
    })

    it('addOption should add an option to question', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'Question'
      form.newQuestionType.value = 'multiple_choice'
      form.addQuestion()

      form.addOption(0)

      expect(form.questions.value[0]!.options).toHaveLength(3)
      expect(form.questions.value[0]!.options[2]!.id).toBe('C')
    })

    it('removeOption should remove option but keep minimum 2', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'Question'
      form.newQuestionType.value = 'multiple_choice'
      form.addQuestion()
      form.addOption(0) // Now has 3 options

      form.removeOption(0, 2)

      expect(form.questions.value[0]!.options).toHaveLength(2)
    })

    it('removeOption should not remove below 2 options', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'Question'
      form.newQuestionType.value = 'multiple_choice'
      form.addQuestion()

      form.removeOption(0, 0)

      expect(form.questions.value[0]!.options).toHaveLength(2)
    })

    it('getAvailableDependencies should return objectives and other questions', () => {
      vi.useFakeTimers()
      const form = useLabForm()
      form.newObjectiveDescription.value = 'Objective 1'
      form.addObjective()
      vi.advanceTimersByTime(10)
      form.newQuestionDescription.value = 'Question 1'
      form.addQuestion()
      vi.advanceTimersByTime(10)
      form.newQuestionDescription.value = 'Question 2'
      form.addQuestion()

      const deps = form.getAvailableDependencies(form.questions.value[1]!.id)

      expect(deps).toHaveLength(2) // 1 objective + 1 question (excluding current)
      expect(deps[0]!.label).toContain('[Obj]')
      expect(deps[1]!.label).toContain('[Q]')
      vi.useRealTimers()
    })
  })

  describe('totals', () => {
    it('totalObjectivePoints should sum objective points', () => {
      const form = useLabForm()
      form.newObjectiveDescription.value = 'Obj 1'
      form.newObjectivePoints.value = 20
      form.addObjective()
      form.newObjectiveDescription.value = 'Obj 2'
      form.newObjectivePoints.value = 30
      form.addObjective()

      expect(form.totalObjectivePoints.value).toBe(50)
    })

    it('totalQuestionPoints should sum question points', () => {
      const form = useLabForm()
      form.newQuestionDescription.value = 'Q1'
      form.newQuestionPoints.value = 10
      form.addQuestion()
      form.newQuestionDescription.value = 'Q2'
      form.newQuestionPoints.value = 15
      form.addQuestion()

      expect(form.totalQuestionPoints.value).toBe(25)
    })

    it('totalPoints should sum all points', () => {
      const form = useLabForm()
      form.newObjectiveDescription.value = 'Obj 1'
      form.newObjectivePoints.value = 20
      form.addObjective()
      form.newQuestionDescription.value = 'Q1'
      form.newQuestionPoints.value = 10
      form.addQuestion()

      expect(form.totalPoints.value).toBe(30)
    })

    it('totalVMs should count VMs', () => {
      const form = useLabForm()
      form.newVMName.value = 'vm1'
      form.addVM()
      form.newVMName.value = 'vm2'
      form.addVM()

      expect(form.totalVMs.value).toBe(2)
    })
  })

  describe('buildLabSpec', () => {
    it('should build complete lab spec', () => {
      const form = useLabForm()
      form.labData.value.name = 'Test Lab'
      form.labData.value.description = 'A test lab'
      form.labData.value.durationMinutes = 90
      form.labData.value.difficulty = 'intermediate'
      form.labData.value.tags = ['security']
      form.labData.value.version = '1.0.0'
      form.labData.value.platform = 'proxmox'
      form.labData.value.passThreshold = 80

      form.newVMName.value = 'web-server'
      form.addVM()

      form.newObjectiveDescription.value = 'Create a file'
      form.newObjectivePoints.value = 20
      form.addObjective()
      form.objectives.value[0]!.triggers[0]!.type = 'file_exists'
      form.objectives.value[0]!.triggers[0]!.matchPath = '/tmp/test.txt'

      const spec = form.buildLabSpec() as Record<string, unknown>

      expect(spec['apiVersion']).toBe('v1')
      expect(spec['kind']).toBe('LabTemplate')
      expect((spec['metadata'] as Record<string, unknown>)['name']).toBe('Test Lab')
      expect((spec['metadata'] as Record<string, unknown>)['duration']).toBe('90m')
      expect((spec['metadata'] as Record<string, unknown>)['difficulty']).toBe('intermediate')
      expect((spec['spec'] as Record<string, unknown>)['platform']).toBe('proxmox')
      expect(
        ((spec['spec'] as Record<string, unknown>)['checkpoints'] as Record<string, unknown>)[
          'pass_threshold'
        ],
      ).toBe(80)
      expect(((spec['spec'] as Record<string, unknown>)['vms'] as unknown[]).length).toBe(1)
      expect(((spec['spec'] as Record<string, unknown>)['objectives'] as unknown[]).length).toBe(1)
    })

    it('should include questions when present', () => {
      const form = useLabForm()
      form.labData.value.name = 'Test Lab'
      form.newVMName.value = 'vm1'
      form.addVM()
      form.newObjectiveDescription.value = 'Obj'
      form.addObjective()
      form.newQuestionDescription.value = 'Question'
      form.newQuestionPoints.value = 10
      form.addQuestion()
      form.questions.value[0]!.validation.type = 'exact'
      form.questions.value[0]!.validation.answer = 'answer'

      const spec = form.buildLabSpec() as Record<string, unknown>
      const specQuestions = (spec['spec'] as Record<string, unknown>)['questions'] as unknown[]

      expect(specQuestions).toHaveLength(1)
    })

    it('should not include questions when empty', () => {
      const form = useLabForm()
      form.labData.value.name = 'Test Lab'
      form.newVMName.value = 'vm1'
      form.addVM()
      form.newObjectiveDescription.value = 'Obj'
      form.addObjective()

      const spec = form.buildLabSpec() as Record<string, unknown>

      expect((spec['spec'] as Record<string, unknown>)['questions']).toBeUndefined()
    })
  })

  describe('resetForm', () => {
    it('should reset all form state', () => {
      const form = useLabForm()
      form.labData.value.name = 'Test Lab'
      form.newVMName.value = 'vm1'
      form.addVM()
      form.newObjectiveDescription.value = 'Obj'
      form.addObjective()
      form.newQuestionDescription.value = 'Q'
      form.addQuestion()
      form.currentStep.value = 3

      form.resetForm()

      expect(form.labData.value.name).toBe('')
      expect(form.vms.value).toEqual([])
      expect(form.objectives.value).toEqual([])
      expect(form.questions.value).toEqual([])
      expect(form.currentStep.value).toBe(1)
    })
  })

  describe('visibilityOptions', () => {
    it('should include global option for admin users', () => {
      const authStore = useAuthStore()
      authStore.user = { id: '1', email: 'admin@example.com', name: 'Admin', roles: ['admin'] }

      const form = useLabForm()

      expect(form.visibilityOptions.value).toContainEqual({
        label: 'Global (Everyone)',
        value: 'global',
      })
    })

    it('should not include global option for non-admin users', () => {
      const authStore = useAuthStore()
      authStore.user = { id: '1', email: 'user@example.com', name: 'User', roles: ['student'] }

      const form = useLabForm()

      expect(form.visibilityOptions.value).not.toContainEqual({
        label: 'Global (Everyone)',
        value: 'global',
      })
      expect(form.visibilityOptions.value).toContainEqual({ label: 'Private', value: 'private' })
    })
  })
})
