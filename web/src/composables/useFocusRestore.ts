import { ref, watch, onBeforeUnmount, type Ref } from 'vue'

/**
 * Composable that restores keyboard focus to the element that was active
 * when a modal/dialog/sidebar opened, after it closes. This is the missing
 * half of an accessible modal experience — PrimeVue (and most component
 * libraries) auto-focus the first focusable element on open but don't
 * restore it on close, so a screen-reader user winds up on `<body>` and
 * has to navigate back to where they were.
 *
 * Usage:
 *
 *   const show = ref(false)
 *   useFocusRestore(show)
 *   // ...open the modal via show.value = true; close via show.value = false
 *
 * The composable snapshots `document.activeElement` on every true→false
 * transition of the source ref, then calls `.focus()` on it in the next
 * microtask (so the modal has a chance to tear down its own focus trap
 * first).
 *
 * Also handles unmount: if the component owning the composable goes away
 * while the modal is open, the snapshot is discarded — we can't restore
 * focus to a detached element.
 */
export function useFocusRestore(isOpen: Ref<boolean>): {
  /** The element we'll restore focus to, or null if nothing was focused. */
  savedFocus: Ref<HTMLElement | null>
  /** Manually trigger restoration. Useful when the close flow is more
   *  complex than a simple `isOpen.value = false`. */
  restore: () => void
} {
  const savedFocus = ref<HTMLElement | null>(null)

  function capture() {
    const el = document.activeElement
    // HTMLElement guard filters out the SVGElement and generic Element
    // cases where `.focus()` may not exist or may be a no-op.
    savedFocus.value = el instanceof HTMLElement ? el : null
  }

  function restore() {
    const target = savedFocus.value
    savedFocus.value = null
    if (!target) return
    // Defer so any closing modal's own teardown (focus traps, transitions)
    // runs first; a synchronous .focus() can get clobbered by the library.
    queueMicrotask(() => {
      // If the element was removed from the DOM while the modal was open,
      // isConnected will be false — don't try to focus a detached node.
      if (target.isConnected) {
        target.focus()
      }
    })
  }

  watch(
    isOpen,
    (nextOpen, prevOpen) => {
      if (nextOpen && !prevOpen) {
        capture()
      } else if (!nextOpen && prevOpen) {
        restore()
      }
    },
  )

  onBeforeUnmount(() => {
    // Don't try to restore focus to a trigger owned by an unmounting tree.
    savedFocus.value = null
  })

  return { savedFocus, restore }
}
