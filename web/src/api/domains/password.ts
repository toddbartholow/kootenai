/**
 * Password API
 * Password reset and change functionality
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export interface ChangePasswordRequest {
  currentPassword: string
  newPassword: string
}

export interface AdminResetPasswordRequest {
  password?: string
  generatePassword?: boolean
}

export interface AdminSetPasswordRequest {
  password: string
  mustChangePassword?: boolean
}

export interface PasswordResetResponse {
  message: string
  mustChangePassword: boolean
  temporaryPassword?: string
}

// ============================================================================
// API
// ============================================================================
export const passwordApi = {
  // Request a password reset (unauthenticated)
  requestReset: async (email: string): Promise<{ message: string }> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      return { message: 'If an account with that email exists, a password reset link has been sent.' }
    }
    const response = await api.post<{ message: string }>('/auth/password/reset-request', { email })
    return response.data
  },

  // Confirm password reset with token (unauthenticated)
  confirmReset: async (token: string, newPassword: string): Promise<{ message: string }> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      if (!token || !newPassword) {
        throw new Error('Token and password are required')
      }
      return { message: 'password has been reset successfully' }
    }
    const response = await api.post<{ message: string }>('/auth/password/reset-confirm', { token, newPassword })
    return response.data
  },

  change: async (data: ChangePasswordRequest): Promise<{ message: string }> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      // Simulate password change in mock mode
      if (data.currentPassword === '' || data.newPassword === '') {
        throw new Error('Password is required')
      }
      return { message: 'password updated successfully' }
    }
    const response = await api.put<{ message: string }>('/password/change', data)
    return response.data
  },

  adminReset: async (userId: string, data: AdminResetPasswordRequest): Promise<PasswordResetResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      const response: PasswordResetResponse = {
        message: 'password reset successfully',
        mustChangePassword: true,
      }
      if (data.generatePassword) {
        response.temporaryPassword = 'TempPass123!'
      }
      return response
    }
    const response = await api.put<PasswordResetResponse>(`/users/${userId}/password/reset`, data)
    return response.data
  },

  adminSet: async (userId: string, data: AdminSetPasswordRequest): Promise<{ message: string; mustChangePassword: boolean }> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      return {
        message: 'password set successfully',
        mustChangePassword: data.mustChangePassword ?? false,
      }
    }
    const response = await api.put<{ message: string; mustChangePassword: boolean }>(`/users/${userId}/password`, data)
    return response.data
  },
}
