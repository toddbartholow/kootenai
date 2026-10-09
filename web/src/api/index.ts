/**
 * API Module - Primary Export Point
 *
 * This barrel file provides a clean, organized way to import API functionality.
 * All domain APIs, types, and utilities are exported from this file.
 *
 * Usage:
 *   import { labsApi, Lab, Pod, sessionsApi } from '@/api'
 *   import { api, setOrganizationContext } from '@/api'
 */

// =============================================================================
// Infrastructure
// =============================================================================
export { api, setOrganizationContext } from './config'
export { USE_MOCK_DATA, simulateDelay, setMockDataEnabled, isMockDataEnabled } from './shared/mock'
export {
  DEFAULT_PAGINATION,
  calculatePage,
  calculateOffset,
  pageParams,
  hasNextPage,
  hasPrevPage,
  paginationToQueryParams,
} from './shared/pagination'
export type { PaginationMeta, PaginatedResponse, PaginationParams } from './shared/pagination'

// =============================================================================
// Domain APIs
// =============================================================================

// Labs
export { labsApi, labTemplatesApi, mockLabs, getMockLabs } from './domains/labs'
export type {
  Lab,
  LabWithSpec,
  LabsListResponse,
  CreateLabRequest,
  CreateLabResponse,
  LabInstructions,
  InstructionStep,
  InstructionResource,
} from './domains/labs'

// Pods
export { podsApi, mockPods } from './domains/pods'
export type { Pod, PodVM, PodStatus, PodsListResponse } from './domains/pods'

// Snapshots
export { snapshotsApi, mockSnapshots } from './domains/snapshots'
export type { Snapshot, SnapshotsListResponse } from './domains/snapshots'

// Sessions
export { sessionsApi, mockSessions } from './domains/sessions'
export type {
  Session,
  SessionsListResponse,
  CreateSessionRequest,
  CreateSessionResponse,
  SubmissionCheckpoint,
  ModuleCompletionInfo,
  SubmissionResponse,
  CheckpointHintResponse,
} from './domains/sessions'

// Questions
export { questionsApi } from './domains/questions'
export type {
  QuestionType,
  QuestionResponseStatus,
  QuestionOption,
  QuestionProgress,
  QuestionsListResponse,
  SubmitAnswerRequest,
  SubmitAnswerResponse,
  GetHintResponse,
} from './domains/questions'

// Assessment
export { assessmentApi, mockAssessmentResult } from './domains/assessment'
export type {
  AssessmentStatus,
  VerifyType,
  VerifyConfig,
  AssessmentCheckConfig,
  AssessmentInterfaceConfig,
  AssessmentDeviceConfig,
  AssessmentComponentConfig,
  AssessmentTemplate,
  CheckResult,
  InterfaceResult,
  DeviceResult,
  ComponentResult,
  AssessmentResult,
  AssessmentStatusResponse,
  CheckUpdatePayload,
  DeviceUpdatePayload,
  ComponentUpdatePayload,
  AssessmentCompletePayload,
  AssessmentUpdate,
} from './domains/assessment'

// Console
export { consoleApi } from './domains/console'
export type { ConsoleTicket } from './domains/console'

// Reservations
export { reservationsApi, mockReservations } from './domains/reservations'
export type {
  ReservationStatus,
  Reservation,
  CreateReservationRequest,
  AvailabilitySlot,
  AvailabilityResponse,
} from './domains/reservations'

// Proxmox
export { proxmoxApi } from './domains/proxmox'
export type { ProxmoxVM } from './domains/proxmox'

// Achievements
export { achievementsApi } from './domains/achievements'
export type {
  AchievementType,
  AchievementTier,
  AchievementCriteria,
  Achievement,
  UserAchievement,
  AchievementProgress,
  AchievementWithProgress,
  UserAchievementSummary,
  AchievementsListResponse,
} from './domains/achievements'

// Organizations
export { organizationsApi } from './domains/organizations'
export type {
  Edition,
  OrgRole,
  OrgType,
  Organization,
  OrganizationMembership,
  OrganizationWithMembership,
} from './domains/organizations'

// Teams
export { teamsApi } from './domains/teams'
export type { Team, TeamMembership, TeamWithMembership } from './domains/teams'

// Features
export { featuresApi } from './domains/features'
export type { FeatureFlag } from './domains/features'

// Audit
export { auditApi } from './domains/audit'
export type { AuditEntry, AuditLogFilter, AuditLogListResponse } from './domains/audit'

// Pathways
export { pathwaysApi, mockPathways } from './domains/pathways'
export type {
  PathwayStatus,
  ModuleProgressStatus,
  UnlockType,
  LabVisibility,
  Pathway,
  PathwayModule,
  ModuleLab,
  PathwayStats,
  CreatePathwayRequest,
  UpdatePathwayRequest,
  CreateModuleRequest,
  UpdateModuleRequest,
  AddLabToModuleRequest,
} from './domains/pathways'

// Enrollments
export { enrollmentsApi } from './domains/enrollments'
export type {
  EnrollmentStatus,
  UnlockRequirement,
  UnlockRequirements,
  PathwayEnrollment,
  ModuleProgress,
  LabProgress,
  Certificate,
} from './domains/enrollments'

// Users
export { usersApi } from './domains/users'
export type { User, UserListResponse, CreateUserRequest, UpdateUserRequest } from './domains/users'

// Password
export { passwordApi } from './domains/password'
export type {
  ChangePasswordRequest,
  AdminResetPasswordRequest,
  AdminSetPasswordRequest,
  PasswordResetResponse,
} from './domains/password'

// Dashboard
export { dashboardApi } from './domains/dashboard'
export type {
  DashboardUser,
  DashboardPathway,
  DashboardSession,
  DashboardAchievement,
  DashboardAchievements,
  DashboardStats,
  DashboardRecommendation,
  DashboardResponse,
  ActivityItem,
  ActivityResponse,
  LeaderboardEntry,
  LeaderboardResponse,
  DailyTimeStats,
  LabTimeStats,
  WeeklyTimeStats,
  TimeStats,
  TimeAnalyticsResponse,
} from './domains/dashboard'

// Events (Monitoring)
export { eventsApi } from './domains/events'
export type {
  MonitoringEvent,
  EventsListResponse,
  EventStatsResponse,
  EventsFilterParams,
} from './domains/events'

// Recommendations
export { recommendationsApi } from './domains/recommendations'
export type {
  RecommendationType,
  LabRecommendation,
  PathwayRecommendation,
  RecommendationsResponse,
} from './domains/recommendations'

// Topology
export { topologyApi } from './domains/topology'
export type { NetworkSegment, TopologyVM, TopologyData } from './domains/topology'

// Instructor
export { instructorApi } from './domains/instructor'
export type {
  StudentSummary,
  ClassOverview,
  LabCompletionStats,
  FailurePoint,
  ActivityHeatmapData,
  StudentProgress,
  InstructorDashboardResponse,
} from './domains/instructor'

// Classroom Simulation
export { classroomApi } from './domains/classroom'
export type {
  PersonalityType,
  SimulationStatus,
  ActivityType,
  Traits,
  TechSkills,
  BehavioralConfig,
  AIStudent,
  SimulationConfig,
  ClassroomSimulation,
  StudentActivity,
  CreateSimulationRequest,
} from './domains/classroom'

// OAuth2/OIDC
export {
  oauth2Api,
  hasOAuth2Token,
  extractOAuth2Token,
  extractOAuth2Error,
  clearOAuth2Hash,
  getProviderIcon,
  getProviderColorClass,
} from './domains/oauth2'
export type { OAuth2Provider, OAuth2ProviderType, OAuth2ProvidersResponse } from './domains/oauth2'
