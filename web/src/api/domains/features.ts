/**
 * Features API
 * Feature flags for edition-based functionality
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export type Edition = 'community' | 'professional' | 'enterprise'

export interface FeatureFlag {
  id: string
  name: string
  description?: string
  editions: Edition[]
  isGlobal: boolean
  enabled?: boolean
}

// ============================================================================
// Mock Data
// ============================================================================
export const mockFeatures: FeatureFlag[] = [
  { id: 'teams', name: 'Team Management', editions: ['professional', 'enterprise'], isGlobal: false, enabled: true },
  { id: 'labs.basic', name: 'Basic Lab Access', editions: ['community', 'professional', 'enterprise'], isGlobal: true, enabled: true },
  { id: 'labs.custom', name: 'Custom Labs', editions: ['professional', 'enterprise'], isGlobal: false, enabled: true },
  { id: 'sso.saml', name: 'SAML SSO', editions: ['enterprise'], isGlobal: false, enabled: false },
]

// ============================================================================
// API
// ============================================================================
export const featuresApi = {
  list: async (): Promise<FeatureFlag[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      return mockFeatures
    }
    const response = await api.get<FeatureFlag[]>('/features')
    return response.data
  },
}
