import type { Lab, CreateModuleRequest, CreatePathwayRequest } from '@/api'

/**
 * Shared types for the Create Pathway wizard step components.
 * Kept colocated with the step components rather than under /api because
 * these types describe the wizard's in-memory editing shape, not the
 * server contract (which lives in @/api).
 */

export type WizardPathwayData = Partial<CreatePathwayRequest> & { name: string }

export interface WizardLab {
  labTemplateId: string
  lab: Lab
  isRequired: boolean
  passThresholdOverride?: number
}

export interface WizardModule extends CreateModuleRequest {
  id: string
  labs: WizardLab[]
}

export interface UnlockTypeOption {
  label: string
  value: 'sequential' | 'all_previous' | 'manual' | 'always'
  description: string
}
