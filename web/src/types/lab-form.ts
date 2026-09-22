/**
 * Shared type definitions for lab creation and editing forms
 */

export interface SnapshotConfig {
  name: string
  description: string
  isDefault: boolean
}

export interface VMConfig {
  id: string
  name: string
  template: string
  cpu: number
  memory: number
  disk: number
  wazuhAgent: boolean
  snapshots: SnapshotConfig[]
}

export interface TriggerConfig {
  type: string
  target: string
  matchPath: string
  matchContains: string
  matchPattern: string
}

export interface ObjectiveConfig {
  id: string
  description: string
  points: number
  hint: string
  order: number
  dependsOn: string[]
  triggers: TriggerConfig[]
}

export interface QuestionOptionConfig {
  id: string
  text: string
  correct: boolean
}

export interface QuestionValidationConfig {
  type: 'exact' | 'regex'
  answer: string
  pattern: string
  caseSensitive: boolean
}

export interface QuestionConfig {
  id: string
  type: 'text' | 'multiple_choice'
  description: string
  points: number
  hint: string
  order: number
  dependsOn: string[]
  validation: QuestionValidationConfig
  options: QuestionOptionConfig[]
  multiSelect: boolean
}

export interface LabFormData {
  name: string
  description: string
  difficulty: string
  durationMinutes: number
  platform: string
  version: string
  passThreshold: number
  visibility: string
  isActive: boolean
  tags: string[]
}

export interface SelectOption {
  label: string
  value: string
  description?: string
}

// Default values for new lab form
export function createDefaultLabData(): LabFormData {
  return {
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
}

export function createDefaultVM(name: string, template: string): VMConfig {
  return {
    id: `vm-${Date.now()}`,
    name,
    template,
    cpu: 2,
    memory: 2048,
    disk: 16,
    wazuhAgent: true,
    snapshots: [
      { name: 'initial', description: 'Initial state', isDefault: true }
    ],
  }
}

export function createDefaultObjective(description: string, points: number, targetVM: string): ObjectiveConfig {
  return {
    id: `obj-${Date.now()}`,
    description,
    points,
    hint: '',
    order: 1,
    dependsOn: [],
    triggers: [
      { type: 'file_exists', target: targetVM, matchPath: '', matchContains: '', matchPattern: '' }
    ],
  }
}

export function createDefaultQuestion(
  description: string,
  points: number,
  type: 'text' | 'multiple_choice'
): QuestionConfig {
  return {
    id: `q-${Date.now()}`,
    type,
    description,
    points,
    hint: '',
    order: 1,
    dependsOn: [],
    validation: {
      type: 'exact',
      answer: '',
      pattern: '',
      caseSensitive: false,
    },
    options: type === 'multiple_choice'
      ? [
          { id: 'A', text: '', correct: false },
          { id: 'B', text: '', correct: false },
        ]
      : [],
    multiSelect: false,
  }
}
