export { useWebSocket } from './useWebSocket'
export type {
  SessionEventPayload,
  CheckpointUpdatePayload,
  GradeUpdatePayload,
  PodStatusPayload,
  VMStatusPayload,
  WebSocketMessage,
  OutboundMessage,
  UseWebSocketOptions,
} from './useWebSocket'

export { useVMConsole } from './useVMConsole'
export type { ConsoleState, UseVMConsoleOptions } from './useVMConsole'

export { useSessionActions } from './useSessionActions'
export type { SessionActionOptions, UseSessionActionsReturn } from './useSessionActions'

export { useSessionWebSocket } from './useSessionWebSocket'
export type { SessionWebSocketOptions, UseSessionWebSocketReturn } from './useSessionWebSocket'

export { useNotifications } from './useNotifications'
export type {
  NotificationType,
  NotificationOptions,
  UseNotificationsReturn,
} from './useNotifications'

export { useVirtualList, useLazyList } from './useVirtualList'
export type { VirtualListOptions, VirtualListReturn } from './useVirtualList'

export { useDashboardLayout } from './useDashboardLayout'
export type { DashboardCardConfig } from './useDashboardLayout'

export { useFocusRestore } from './useFocusRestore'

export { usePodRealtime } from './usePodRealtime'
export type { UsePodRealtimeReturn } from './usePodRealtime'

export { useConsoleLauncher } from './useConsoleLauncher'
export type { UseConsoleLauncherReturn, ConsoleVMRef } from './useConsoleLauncher'

export { useDashboardDrag } from './useDashboardDrag'
export type { UseDashboardDragReturn, WidgetDragBindings } from './useDashboardDrag'

export { useVncConnection, buildWsProxyUrl } from './useVncConnection'
export type {
  ConnectionStatus,
  VncConnectionOptions,
  UseVncConnectionReturn,
} from './useVncConnection'

export { useVncKeyboard } from './useVncKeyboard'
export type { UseVncKeyboardReturn } from './useVncKeyboard'
