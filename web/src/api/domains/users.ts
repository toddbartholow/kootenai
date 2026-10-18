/**
 * Users API
 * User management
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'

// ============================================================================
// Types
// ============================================================================
export interface User {
  id: string
  externalId?: string | undefined
  username: string
  email?: string | undefined
  displayName?: string | undefined
  role: 'student' | 'instructor' | 'admin'
  isActive: boolean
  createdAt: string
  updatedAt: string
  lastLoginAt?: string | undefined
  metadata?: Record<string, string> | undefined
}

export interface UserListResponse {
  users: User[]
  total: number
  limit: number
  offset: number
}

export interface CreateUserRequest {
  username: string
  email?: string | undefined
  displayName?: string | undefined
  role?: 'student' | 'instructor' | 'admin' | undefined
}

export interface UpdateUserRequest {
  email?: string | undefined
  displayName?: string | undefined
  role?: 'student' | 'instructor' | 'admin' | undefined
  isActive?: boolean | undefined
}

// ============================================================================
// Mock Data
// ============================================================================
const mockUsers: User[] = [
  {
    id: '00000000-0000-0000-0000-000000000001',
    username: 'demo',
    email: 'demo@example.com',
    displayName: 'Demo User',
    role: 'student',
    isActive: true,
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-01T00:00:00Z',
  },
  {
    id: '00000000-0000-0000-0000-000000000002',
    username: 'instructor',
    email: 'instructor@example.com',
    displayName: 'Test Instructor',
    role: 'instructor',
    isActive: true,
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-01T00:00:00Z',
  },
  {
    id: '00000000-0000-0000-0000-000000000003',
    username: 'admin',
    email: 'admin@example.com',
    displayName: 'Admin User',
    role: 'admin',
    isActive: true,
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-01T00:00:00Z',
  },
]

// ============================================================================
// API
// ============================================================================
export const usersApi = {
  list: async (params?: {
    role?: string
    active?: boolean
    search?: string
    limit?: number
    offset?: number
  }): Promise<UserListResponse> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      let users = [...mockUsers]
      if (params?.role) {
        users = users.filter(u => u.role === params.role)
      }
      if (params?.active !== undefined) {
        users = users.filter(u => u.isActive === params.active)
      }
      if (params?.search) {
        const search = params.search.toLowerCase()
        users = users.filter(
          u =>
            u.username.toLowerCase().includes(search) ||
            u.email?.toLowerCase().includes(search) ||
            u.displayName?.toLowerCase().includes(search)
        )
      }
      return {
        users,
        total: users.length,
        limit: params?.limit || 50,
        offset: params?.offset || 0,
      }
    }
    const response = await api.get<UserListResponse>('/users', { params })
    return response.data
  },

  get: async (id: string): Promise<User> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const user = mockUsers.find(u => u.id === id)
      if (!user) throw new Error(`User not found: ${id}`)
      return user
    }
    const response = await api.get<User>(`/users/${id}`)
    return response.data
  },

  create: async (data: CreateUserRequest): Promise<User> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const newUser: User = {
        id: crypto.randomUUID(),
        username: data.username,
        email: data.email,
        displayName: data.displayName || data.username,
        role: data.role || 'student',
        isActive: true,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      }
      mockUsers.push(newUser)
      return newUser
    }
    const response = await api.post<User>('/users', data)
    return response.data
  },

  update: async (id: string, data: UpdateUserRequest): Promise<User> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const index = mockUsers.findIndex(u => u.id === id)
      if (index === -1) throw new Error(`User not found: ${id}`)
      const existing = mockUsers[index]!
      const updated: User = {
        id: existing.id,
        externalId: existing.externalId,
        username: existing.username,
        email: data.email ?? existing.email,
        displayName: data.displayName ?? existing.displayName,
        role: data.role ?? existing.role,
        isActive: data.isActive ?? existing.isActive,
        createdAt: existing.createdAt,
        updatedAt: new Date().toISOString(),
        lastLoginAt: existing.lastLoginAt,
        metadata: existing.metadata,
      }
      mockUsers[index] = updated
      return updated
    }
    const response = await api.put<User>(`/users/${id}`, data)
    return response.data
  },

  delete: async (id: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const index = mockUsers.findIndex(u => u.id === id)
      if (index === -1) throw new Error(`User not found: ${id}`)
      mockUsers.splice(index, 1)
      return
    }
    await api.delete(`/users/${id}`)
  },

  updateRole: async (id: string, role: 'student' | 'instructor' | 'admin'): Promise<User> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const index = mockUsers.findIndex(u => u.id === id)
      if (index === -1) throw new Error(`User not found: ${id}`)
      const existing = mockUsers[index]!
      const updated: User = {
        id: existing.id,
        externalId: existing.externalId,
        username: existing.username,
        email: existing.email,
        displayName: existing.displayName,
        role,
        isActive: existing.isActive,
        createdAt: existing.createdAt,
        updatedAt: new Date().toISOString(),
        lastLoginAt: existing.lastLoginAt,
        metadata: existing.metadata,
      }
      mockUsers[index] = updated
      return updated
    }
    const response = await api.put<User>(`/users/${id}/role`, { role })
    return response.data
  },

  updateStatus: async (id: string, isActive: boolean): Promise<User> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const index = mockUsers.findIndex(u => u.id === id)
      if (index === -1) throw new Error(`User not found: ${id}`)
      const existing = mockUsers[index]!
      const updated: User = {
        id: existing.id,
        externalId: existing.externalId,
        username: existing.username,
        email: existing.email,
        displayName: existing.displayName,
        role: existing.role,
        isActive,
        createdAt: existing.createdAt,
        updatedAt: new Date().toISOString(),
        lastLoginAt: existing.lastLoginAt,
        metadata: existing.metadata,
      }
      mockUsers[index] = updated
      return updated
    }
    const response = await api.put<User>(`/users/${id}/status`, { isActive })
    return response.data
  },
}
