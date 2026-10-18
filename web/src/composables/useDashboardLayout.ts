import { storeToRefs } from 'pinia'
import { useDashboardLayoutStore } from '@/stores/dashboardLayout'

export type { DashboardCardConfig } from '@/stores/dashboardLayout'

/**
 * Composable façade over the `dashboardLayout` Pinia store.
 *
 * Why the wrapper: the store owns state and actions (so tests get
 * automatic isolation via `setActivePinia(createPinia())` and DevTools
 * get first-class store integration), but call sites still use the
 * familiar `useDashboardLayout()` shape they had before the
 * composable-to-store migration. `storeToRefs` preserves reactivity
 * on destructured state refs.
 */
export function useDashboardLayout() {
  const store = useDashboardLayoutStore()
  const { cards, locked } = storeToRefs(store)
  return {
    cards,
    locked,
    isVisible: store.isVisible,
    getOrder: store.getOrder,
    setCardVisible: store.setCardVisible,
    moveCard: store.moveCard,
    moveCardTo: store.moveCardTo,
    resetLayout: store.resetLayout,
    toggleLocked: store.toggleLocked,
    isMinimized: store.isMinimized,
    setMinimized: store.setMinimized,
    getSize: store.getSize,
    setSize: store.setSize,
    getSettings: store.getSettings,
    updateSettings: store.updateSettings,
    addWidget: store.addWidget,
    removeWidget: store.removeWidget,
  }
}
