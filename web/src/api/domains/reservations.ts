/**
 * Reservations API
 * Lab time slot reservations
 */
import { api } from '../config'
import { simulateDelay, USE_MOCK_DATA } from '../shared/mock'
import { getMockLabs } from './labs'

// ============================================================================
// Types
// ============================================================================
export type ReservationStatus = 'pending' | 'confirmed' | 'active' | 'completed' | 'cancelled' | 'expired'

export interface Reservation {
  id: string
  userId: string
  labTemplateId: string
  labTemplateName?: string
  podId?: string
  status: ReservationStatus
  startTime: string // ISO 8601 timestamp
  endTime: string // ISO 8601 timestamp
  durationMinutes: number
  resources: {
    cpu: number
    memory: number
    storage: number
  }
  notes?: string
  createdAt: string
  cancelledAt?: string
}

export interface CreateReservationRequest {
  labTemplateId: string
  startTime: string // ISO 8601 format
  durationMinutes: number
  resources?: {
    cpu?: number
    memory?: number
    storage?: number
  }
  notes?: string
}

export interface AvailabilitySlot {
  startTime: string // HH:MM format
  endTime: string
  available: boolean
}

export interface AvailabilityResponse {
  date: string // YYYY-MM-DD
  slots: AvailabilitySlot[]
}

// ============================================================================
// Mock Data
// ============================================================================
// Helper to create ISO date at specific hour
const createDateTime = (daysFromNow: number, hour: number): string => {
  const date = new Date()
  date.setDate(date.getDate() + daysFromNow)
  date.setHours(hour, 0, 0, 0)
  return date.toISOString()
}

export const mockReservations: Reservation[] = [
  {
    id: 'res-001',
    userId: 'demo@example.com',
    labTemplateId: 'lab-firewall-101',
    labTemplateName: 'Firewall Configuration 101',
    status: 'confirmed',
    startTime: createDateTime(3, 14), // 3 days from now at 2:00 PM
    endTime: createDateTime(3, 16), // 4:00 PM
    durationMinutes: 120,
    resources: { cpu: 4, memory: 8, storage: 60 },
    createdAt: createDateTime(-2, 10),
  },
  {
    id: 'res-002',
    userId: 'demo@example.com',
    labTemplateId: 'lab-ids-snort',
    labTemplateName: 'Intrusion Detection with Snort',
    status: 'confirmed',
    startTime: createDateTime(7, 10), // 7 days from now at 10:00 AM
    endTime: createDateTime(7, 13), // 1:00 PM
    durationMinutes: 180,
    resources: { cpu: 4, memory: 16, storage: 100 },
    createdAt: createDateTime(-1, 15),
  },
  {
    id: 'res-003',
    userId: 'demo@example.com',
    labTemplateId: 'lab-network-fundamentals',
    labTemplateName: 'Network Fundamentals Review',
    status: 'completed',
    startTime: createDateTime(-2, 9), // 2 days ago at 9:00 AM
    endTime: createDateTime(-2, 11), // 11:00 AM
    durationMinutes: 120,
    resources: { cpu: 2, memory: 4, storage: 40 },
    createdAt: createDateTime(-5, 14),
  },
  {
    id: 'res-004',
    userId: 'demo@example.com',
    labTemplateId: 'lab-vpn-config',
    labTemplateName: 'VPN Configuration',
    status: 'completed',
    startTime: createDateTime(-5, 13), // 5 days ago at 1:00 PM
    endTime: createDateTime(-5, 16), // 4:00 PM
    durationMinutes: 180,
    resources: { cpu: 4, memory: 8, storage: 80 },
    createdAt: createDateTime(-10, 9),
  },
  {
    id: 'res-005',
    userId: 'demo@example.com',
    labTemplateId: 'lab-siem-basics',
    labTemplateName: 'Security Monitoring & SIEM',
    status: 'completed',
    startTime: createDateTime(-10, 8), // 10 days ago at 8:00 AM
    endTime: createDateTime(-10, 12), // 12:00 PM
    durationMinutes: 240,
    resources: { cpu: 6, memory: 16, storage: 120 },
    createdAt: createDateTime(-15, 11),
  },
]

// ============================================================================
// API
// ============================================================================
export const reservationsApi = {
  list: async (status?: ReservationStatus): Promise<Reservation[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      if (status) {
        return mockReservations.filter(r => r.status === status)
      }
      return mockReservations
    }
    const params = status ? { status } : {}
    const response = await api.get<Reservation[]>('/reservations', { params })
    return response.data
  },

  get: async (id: string): Promise<Reservation> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      const reservation = mockReservations.find(r => r.id === id)
      if (!reservation) throw new Error(`Reservation not found: ${id}`)
      return reservation
    }
    const response = await api.get<Reservation>(`/reservations/${id}`)
    return response.data
  },

  create: async (req: CreateReservationRequest): Promise<Reservation> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(500)
      const labs = await getMockLabs()
      const lab = labs.find(l => l.id === req.labTemplateId)
      const startTime = new Date(req.startTime)
      const endTime = new Date(startTime.getTime() + req.durationMinutes * 60 * 1000)
      const newReservation: Reservation = {
        id: `res-${Date.now()}`,
        userId: 'demo@example.com',
        labTemplateId: req.labTemplateId,
        ...(lab?.name && { labTemplateName: lab.name }),
        status: 'confirmed',
        startTime: startTime.toISOString(),
        endTime: endTime.toISOString(),
        durationMinutes: req.durationMinutes,
        resources: {
          cpu: req.resources?.cpu || 4,
          memory: req.resources?.memory || 8,
          storage: req.resources?.storage || 60,
        },
        ...(req.notes && { notes: req.notes }),
        createdAt: new Date().toISOString(),
      }
      mockReservations.push(newReservation)
      return newReservation
    }
    const response = await api.post<Reservation>('/reservations', req)
    return response.data
  },

  cancel: async (id: string, reason?: string): Promise<void> => {
    if (USE_MOCK_DATA) {
      await simulateDelay(300)
      const reservation = mockReservations.find(r => r.id === id)
      if (reservation) {
        reservation.status = 'cancelled'
        reservation.cancelledAt = new Date().toISOString()
      }
      return
    }
    await api.delete(`/reservations/${id}`, { data: { reason } })
  },

  getAvailability: async (startDate?: string, endDate?: string): Promise<AvailabilityResponse[]> => {
    if (USE_MOCK_DATA) {
      await simulateDelay()
      // Generate mock availability for a week
      const availability: AvailabilityResponse[] = []
      const start = startDate ? new Date(startDate) : new Date()
      const end = endDate ? new Date(endDate) : new Date(start.getTime() + 7 * 24 * 60 * 60 * 1000)

      for (let d = new Date(start); d <= end; d.setDate(d.getDate() + 1)) {
        const slots: AvailabilitySlot[] = []
        for (let hour = 8; hour < 20; hour++) {
          slots.push({
            startTime: `${hour.toString().padStart(2, '0')}:00`,
            endTime: `${(hour + 1).toString().padStart(2, '0')}:00`,
            available: Math.random() > 0.2, // 80% available
          })
        }
        availability.push({
          date: d.toISOString().split('T')[0]!,
          slots,
        })
      }
      return availability
    }
    const params: Record<string, string> = {}
    if (startDate) params['startDate'] = startDate
    if (endDate) params['endDate'] = endDate
    const response = await api.get<AvailabilityResponse[]>('/reservations/availability', { params })
    return response.data
  },
}
