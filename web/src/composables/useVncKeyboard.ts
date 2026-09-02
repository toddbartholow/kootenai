import { ref } from 'vue'
import type RFB from '@novnc/novnc/lib/rfb.js'

// X11 keysyms used by noVNC
const XK_Tab = 0xff09
const XK_Escape = 0xff1b
const XK_Shift_L = 0xffe1
const XK_Control_L = 0xffe3
const XK_Alt_L = 0xffe9
const XK_Super_L = 0xffeb

/**
 * Composable for sending keyboard shortcuts to a noVNC RFB connection.
 *
 * Manages sticky modifier key state (Ctrl, Alt, Shift) and provides helpers
 * for common key combinations. Accepts a reactive RFB reference so the same
 * composable can be used whether the component holds a single connection
 * (VncConsole) or a per-VM cache (InlineVncConsole).
 */
export function useVncKeyboard(getRfb: () => RFB | null | undefined) {
  const ctrlActive = ref(false)
  const altActive = ref(false)
  const shiftActive = ref(false)

  /** Send a single key press (down + up) or a targeted down/up event. */
  function sendKey(keysym: number, down?: boolean) {
    const rfb = getRfb()
    if (!rfb) return

    if (down === undefined) {
      rfb.sendKey(keysym, null, true)
      rfb.sendKey(keysym, null, false)
    } else {
      rfb.sendKey(keysym, null, down)
    }
  }

  function toggleCtrl() {
    ctrlActive.value = !ctrlActive.value
    const rfb = getRfb()
    if (rfb) rfb.sendKey(XK_Control_L, null, ctrlActive.value)
  }

  function toggleAlt() {
    altActive.value = !altActive.value
    const rfb = getRfb()
    if (rfb) rfb.sendKey(XK_Alt_L, null, altActive.value)
  }

  function toggleShift() {
    shiftActive.value = !shiftActive.value
    const rfb = getRfb()
    if (rfb) rfb.sendKey(XK_Shift_L, null, shiftActive.value)
  }

  /** Release all held modifier keys. */
  function releaseModifiers() {
    const rfb = getRfb()
    if (ctrlActive.value) {
      ctrlActive.value = false
      if (rfb) rfb.sendKey(XK_Control_L, null, false)
    }
    if (altActive.value) {
      altActive.value = false
      if (rfb) rfb.sendKey(XK_Alt_L, null, false)
    }
    if (shiftActive.value) {
      shiftActive.value = false
      if (rfb) rfb.sendKey(XK_Shift_L, null, false)
    }
  }

  function sendCtrlAltDel() {
    const rfb = getRfb()
    if (rfb) rfb.sendCtrlAltDel()
  }

  /**
   * Send Tab with any active modifier keys, then release the modifiers.
   * This matches the InlineVncConsole pattern where modifiers are "one-shot"
   * sticky keys that auto-release after the next key press.
   */
  function sendTab() {
    const rfb = getRfb()
    if (!rfb) return

    if (ctrlActive.value) rfb.sendKey(XK_Control_L, null, true)
    if (altActive.value) rfb.sendKey(XK_Alt_L, null, true)
    rfb.sendKey(XK_Tab, null, true)
    rfb.sendKey(XK_Tab, null, false)
    if (altActive.value) rfb.sendKey(XK_Alt_L, null, false)
    if (ctrlActive.value) rfb.sendKey(XK_Control_L, null, false)

    ctrlActive.value = false
    altActive.value = false
  }

  function sendEscape() {
    sendKey(XK_Escape)
  }

  function sendSuper() {
    sendKey(XK_Super_L)
  }

  return {
    ctrlActive,
    altActive,
    shiftActive,
    sendKey,
    toggleCtrl,
    toggleAlt,
    toggleShift,
    releaseModifiers,
    sendCtrlAltDel,
    sendTab,
    sendEscape,
    sendSuper,
  }
}

export type UseVncKeyboardReturn = ReturnType<typeof useVncKeyboard>
