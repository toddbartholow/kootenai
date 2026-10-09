/**
 * OAuth2/OIDC API
 * External identity provider authentication
 */
import { api } from '../config'

// ============================================================================
// Types
// ============================================================================

export type OAuth2ProviderType = 'oidc' | 'google' | 'microsoft' | 'github'

export interface OAuth2Provider {
  id: string
  name: string
  type: OAuth2ProviderType
}

export interface OAuth2ProvidersResponse {
  providers: OAuth2Provider[]
}

// ============================================================================
// API
// ============================================================================

export const oauth2Api = {
  /**
   * Get list of available OAuth2 providers
   */
  getProviders: async (): Promise<OAuth2Provider[]> => {
    const response = await api.get<OAuth2ProvidersResponse>('/oauth2/providers')
    return response.data.providers || []
  },

  /**
   * Get the login URL for a specific provider
   * @param providerId - The provider ID (e.g., 'google', 'github')
   * @param redirectUrl - Optional URL to redirect to after login
   */
  getLoginUrl: (providerId: string, redirectUrl?: string): string => {
    const baseUrl = api.defaults.baseURL || ''
    let url = `${baseUrl}/oauth2/login/${providerId}`
    if (redirectUrl) {
      url += `?redirect=${encodeURIComponent(redirectUrl)}`
    }
    return url
  },

  /**
   * Initiate OAuth2 login by redirecting to provider
   * @param providerId - The provider ID
   * @param redirectUrl - Optional URL to redirect to after login
   */
  initiateLogin: (providerId: string, redirectUrl?: string): void => {
    const url = oauth2Api.getLoginUrl(providerId, redirectUrl)
    window.location.href = url
  }
}

// ============================================================================
// Utility Functions
// ============================================================================

/**
 * Check if the current URL contains an OAuth2 token in the hash fragment
 */
export function hasOAuth2Token(): boolean {
  return window.location.hash.includes('token=')
}

/**
 * Extract OAuth2 token from URL hash fragment
 * Returns null if no token is present
 */
export function extractOAuth2Token(): string | null {
  const hash = window.location.hash
  if (!hash) return null

  const params = new URLSearchParams(hash.substring(1))
  return params.get('token')
}

/**
 * Extract OAuth2 error from URL hash fragment
 * Returns null if no error is present
 */
export function extractOAuth2Error(): string | null {
  const hash = window.location.hash
  if (!hash) return null

  const params = new URLSearchParams(hash.substring(1))
  return params.get('error')
}

/**
 * Clear OAuth2 parameters from URL hash
 */
export function clearOAuth2Hash(): void {
  if (window.location.hash.includes('token=') || window.location.hash.includes('error=')) {
    // Replace hash with empty string to clear it
    window.history.replaceState(null, '', window.location.pathname + window.location.search)
  }
}

/**
 * Get provider icon component name based on provider type
 */
export function getProviderIcon(type: OAuth2ProviderType): string {
  switch (type) {
    case 'google':
      return 'pi-google'
    case 'microsoft':
      return 'pi-microsoft'
    case 'github':
      return 'pi-github'
    default:
      return 'pi-key'
  }
}

/**
 * Get provider button color class based on provider type
 */
export function getProviderColorClass(type: OAuth2ProviderType): string {
  switch (type) {
    case 'google':
      return 'bg-white hover:bg-gray-50 text-gray-700 border border-gray-300'
    case 'microsoft':
      return 'bg-[#2F2F2F] hover:bg-[#1F1F1F] text-white'
    case 'github':
      return 'bg-[#24292e] hover:bg-[#1b1f23] text-white'
    default:
      return 'bg-primary-500 hover:bg-primary-600 text-white'
  }
}
