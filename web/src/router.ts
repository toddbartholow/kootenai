import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { useOrganizationStore } from './stores/organization'
import type { OrgRole } from '@/api'

// Extend Vue Router's RouteMeta interface for type-safe meta properties
declare module 'vue-router' {
  interface RouteMeta {
    requiresAuth?: boolean
    requiresRole?: OrgRole
    requiresFeature?: string | string[]
    requiresAnyFeature?: string[]
  }
}

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/LoginView.vue'),
    meta: { requiresAuth: false },
  },
  {
    path: '/forgot-password',
    name: 'forgot-password',
    component: () => import('@/views/ForgotPasswordView.vue'),
    meta: { requiresAuth: false },
  },
  {
    path: '/reset-password',
    name: 'reset-password',
    component: () => import('@/views/ResetPasswordView.vue'),
    meta: { requiresAuth: false },
  },
  {
    path: '/change-password',
    name: 'change-password',
    component: () => import('@/views/ChangePasswordView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/profile',
    name: 'profile',
    component: () => import('@/views/ProfileView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('@/views/SettingsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/',
    name: 'dashboard',
    component: () => import('@/views/DashboardView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/labs',
    name: 'labs',
    component: () => import('@/views/LabsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/labs/create',
    name: 'create-lab',
    component: () => import('@/views/CreateLabView.vue'),
    meta: { requiresAuth: true, requiresRole: 'instructor' },
  },
  {
    path: '/labs/:labId',
    name: 'lab-detail',
    component: () => import('@/views/LabDetailView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/labs/:labId/edit',
    name: 'edit-lab',
    component: () => import('@/views/EditLabView.vue'),
    meta: { requiresAuth: true, requiresRole: 'instructor' },
  },
  {
    path: '/pods',
    name: 'pods',
    component: () => import('@/views/PodsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/pods/create',
    name: 'create-pod',
    component: () => import('@/views/CreatePodView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/pods/:podId',
    name: 'pod-detail',
    component: () => import('@/views/PodDetailView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/sessions',
    name: 'sessions',
    component: () => import('@/views/SessionsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/session/:sessionId',
    name: 'session',
    component: () => import('@/views/SessionView.vue'),
    meta: { requiresAuth: true },
  },
  // Progress tracking routes
  {
    path: '/progress',
    name: 'progress',
    component: () => import('@/views/ProgressDashboardView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/progress/course/:courseId',
    name: 'course-progress',
    component: () => import('@/views/CourseProgressView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/progress/lab/:labId',
    name: 'lab-progress',
    component: () => import('@/views/LabProgressView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/achievements',
    name: 'achievements',
    component: () => import('@/views/AchievementsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/leaderboard',
    name: 'leaderboard',
    component: () => import('@/views/LeaderboardView.vue'),
    meta: { requiresAuth: true },
  },
  // Certificate routes
  {
    path: '/certificates',
    name: 'certificates',
    component: () => import('@/views/CertificatesView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/verify/:code',
    name: 'verify-certificate',
    component: () => import('@/views/VerifyCertificateView.vue'),
    meta: { requiresAuth: false },
  },
  // Pathways routes
  {
    path: '/pathways',
    name: 'pathways',
    component: () => import('@/views/PathwaysListView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/pathways/create',
    name: 'create-pathway',
    component: () => import('@/views/CreatePathwayView.vue'),
    meta: { requiresAuth: true, requiresRole: 'instructor' },
  },
  {
    path: '/pathways/:slug',
    name: 'pathway-detail',
    component: () => import('@/views/PathwayDetailView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/pathways/:slug/edit',
    name: 'edit-pathway',
    component: () => import('@/views/EditPathwayView.vue'),
    meta: { requiresAuth: true, requiresRole: 'instructor' },
  },
  {
    path: '/enrollments/:id',
    name: 'enrollment',
    component: () => import('@/views/PathwayEnrollmentView.vue'),
    meta: { requiresAuth: true },
  },
  ...(import.meta.env.DEV ? [{
    path: '/pathway-mockup',
    name: 'pathway-mockup',
    component: () => import('@/views/PathwayMockupView.vue'),
    meta: { requiresAuth: true },
  }] : []),
  {
    path: '/pathway/:slug',
    name: 'pathway-interactive',
    component: () => import('@/views/PathwayInteractiveView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/reservations',
    name: 'reservations',
    component: () => import('@/views/ReservationsView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/proxmox-vms',
    name: 'proxmox-vms',
    component: () => import('@/views/ProxmoxVMsView.vue'),
    meta: { requiresAuth: true },
  },
  // Organization routes
  {
    path: '/organizations/:orgId',
    name: 'organization',
    component: () => import('@/views/organizations/OrganizationView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/organizations/:orgId/settings',
    name: 'organization-settings',
    component: () => import('@/views/organizations/OrganizationSettingsView.vue'),
    meta: { requiresAuth: true, requiresRole: 'admin' },
  },
  {
    path: '/organizations/:orgId/audit',
    name: 'organization-audit',
    component: () => import('@/views/organizations/AuditLogView.vue'),
    meta: { requiresAuth: true, requiresRole: 'admin' },
  },
  {
    path: '/organizations/:orgId/teams/new',
    name: 'create-team',
    component: () => import('@/views/organizations/CreateTeamView.vue'),
    meta: { requiresAuth: true, requiresRole: 'admin', requiresFeature: 'teams' },
  },
  // Team routes
  {
    path: '/teams/:teamId',
    name: 'team',
    component: () => import('@/views/teams/TeamView.vue'),
    meta: { requiresAuth: true, requiresFeature: 'teams' },
  },
  // Classroom simulation (enterprise)
  {
    path: '/simulation/classroom',
    name: 'classroom-simulation',
    component: () => import('@/views/ClassroomSimulationView.vue'),
    meta: { requiresAuth: true, requiresFeature: 'ai_classroom' },
  },
  // Admin routes
  {
    path: '/admin/users',
    name: 'admin-users',
    component: () => import('@/views/UsersView.vue'),
    meta: { requiresAuth: true, requiresRole: 'admin' },
  },
  {
    path: '/admin/events',
    name: 'events-monitoring',
    component: () => import('@/views/EventsMonitoringView.vue'),
    meta: { requiresAuth: true, requiresRole: 'admin' },
  },
  {
    path: '/admin/health',
    name: 'system-health',
    component: () => import('@/views/SystemHealthView.vue'),
    meta: { requiresAuth: true, requiresRole: 'admin' },
  },
  {
    path: '/instructor',
    name: 'instructor-dashboard',
    component: () => import('@/views/InstructorDashboardView.vue'),
    meta: { requiresAuth: true, requiresRole: 'instructor' },
  },
  {
    path: '/admin/analytics',
    name: 'analytics',
    component: () => import('@/views/AnalyticsView.vue'),
    meta: { requiresAuth: true, requiresRole: 'admin' },
  },
  {
    path: '/admin/templates',
    name: 'lab-templates',
    component: () => import('@/views/LabTemplateManagementView.vue'),
    meta: { requiresAuth: true, requiresRole: 'admin' },
  },
  {
    path: '/admin/labs/:labId/history',
    name: 'lab-version-history',
    component: () => import('@/views/LabVersionHistoryView.vue'),
    meta: { requiresAuth: true, requiresRole: 'admin' },
  },
  // Catch-all 404 route (must be last)
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('@/views/NotFoundView.vue'),
    meta: { requiresAuth: false },
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

// Whitelist of allowed redirect paths to prevent open redirect vulnerabilities
const ALLOWED_REDIRECT_PATHS = [
  '/dashboard',
  '/labs',
  '/pods',
  '/sessions',
  '/reservations',
  '/progress',
  '/achievements',
  '/leaderboard',
  '/certificates',
  '/pathways',
  '/enrollments',
  '/organizations',
  '/teams',
  '/simulation',
  '/admin',
  '/change-password',
  '/profile',
  '/settings',
]

/**
 * Validates and sanitizes redirect path to prevent open redirect attacks
 */
export function getSafeRedirectPath(path: string): string {
  // Check if the path starts with any of the allowed paths
  const isAllowed = ALLOWED_REDIRECT_PATHS.some(allowedPath =>
    path === allowedPath || path.startsWith(`${allowedPath}/`)
  )

  // Return the path if allowed, otherwise default to dashboard
  return isAllowed ? path : '/dashboard'
}

// Backoff flag to prevent repeated org fetch attempts on failure
let orgFetchAttempted = false

// Navigation guard for authentication, role, and feature checks
router.beforeEach(async (to, _from, next) => {
  const authStore = useAuthStore()
  const orgStore = useOrganizationStore()
  const requiresAuth = to.meta.requiresAuth !== false

  // Check authentication
  if (requiresAuth && !authStore.isAuthenticated) {
    const safeRedirect = getSafeRedirectPath(to.fullPath)
    next({ name: 'login', query: { redirect: safeRedirect } })
    return
  }

  // Redirect to dashboard if already authenticated and trying to access login
  if (to.name === 'login' && authStore.isAuthenticated) {
    next({ name: 'dashboard' })
    return
  }

  // Reset org fetch backoff when visiting login (i.e., after logout)
  if (to.name === 'login') {
    orgFetchAttempted = false
  }

  // Force password change if required (except when already on change-password page)
  if (authStore.isAuthenticated && authStore.mustChangePassword && to.name !== 'change-password') {
    next({ name: 'change-password' })
    return
  }

  // For authenticated routes, ensure org store is loaded (with backoff to avoid repeated failures)
  if (requiresAuth && authStore.isAuthenticated && orgStore.organizations.length === 0 && !orgFetchAttempted) {
    orgFetchAttempted = true
    try {
      await orgStore.fetchOrganizations()
    } catch (e) {
      console.error('Failed to fetch organizations:', e)
    }
  }

  // Check role requirements
  if (to.meta.requiresRole) {
    const requiredRole = to.meta.requiresRole
    if (!orgStore.hasRole(requiredRole)) {
      console.warn(`Access denied: requires ${requiredRole} role`)
      next({ name: 'dashboard', query: { denied: 'role', required: requiredRole } })
      return
    }
  }

  // Check feature requirements
  if (to.meta.requiresFeature) {
    const requiredFeatures = Array.isArray(to.meta.requiresFeature)
      ? to.meta.requiresFeature
      : [to.meta.requiresFeature]

    const hasAllFeatures = requiredFeatures.every(f => orgStore.hasFeature(f))
    if (!hasAllFeatures) {
      console.warn(`Access denied: requires features ${requiredFeatures.join(', ')}`)
      next({ name: 'dashboard', query: { denied: 'feature', required: requiredFeatures.join(',') } })
      return
    }
  }

  // Check any-feature requirements (OR logic)
  if (to.meta.requiresAnyFeature) {
    const hasAnyFeature = to.meta.requiresAnyFeature.some(f => orgStore.hasFeature(f))
    if (!hasAnyFeature) {
      console.warn(`Access denied: requires any of features ${to.meta.requiresAnyFeature.join(', ')}`)
      next({ name: 'dashboard', query: { denied: 'feature', required: to.meta.requiresAnyFeature.join(',') } })
      return
    }
  }

  next()
})

export default router
