declare module '@novnc/novnc/lib/rfb.js' {
  export interface RFBOptions {
    shared?: boolean
    credentials?: {
      username?: string
      password?: string
      target?: string
    }
    repeaterID?: string
    wsProtocols?: string[]
  }

  export interface RFBConnectEvent extends CustomEvent {
    detail: Record<string, unknown>
  }

  export interface RFBDisconnectEvent extends CustomEvent {
    detail: {
      clean: boolean
    }
  }

  export interface RFBCredentialsRequiredEvent extends CustomEvent {
    detail: {
      types: string[]
    }
  }

  export interface RFBSecurityFailureEvent extends CustomEvent {
    detail: {
      status: number
      reason: string
    }
  }

  export interface RFBClipboardEvent extends CustomEvent {
    detail: {
      text: string
    }
  }

  export interface RFBBellEvent extends CustomEvent {
    detail: Record<string, unknown>
  }

  export interface RFBDesktopNameEvent extends CustomEvent {
    detail: {
      name: string
    }
  }

  export default class RFB {
    constructor(target: HTMLElement, url: string, options?: RFBOptions)

    // Event handling
    addEventListener(
      type: 'connect',
      listener: (event: RFBConnectEvent) => void
    ): void
    addEventListener(
      type: 'disconnect',
      listener: (event: RFBDisconnectEvent) => void
    ): void
    addEventListener(
      type: 'credentialsrequired',
      listener: (event: RFBCredentialsRequiredEvent) => void
    ): void
    addEventListener(
      type: 'securityfailure',
      listener: (event: RFBSecurityFailureEvent) => void
    ): void
    addEventListener(
      type: 'clipboard',
      listener: (event: RFBClipboardEvent) => void
    ): void
    addEventListener(type: 'bell', listener: (event: RFBBellEvent) => void): void
    addEventListener(
      type: 'desktopname',
      listener: (event: RFBDesktopNameEvent) => void
    ): void
    addEventListener(type: string, listener: (event: CustomEvent) => void): void

    removeEventListener(
      type: string,
      listener: (event: CustomEvent) => void
    ): void

    // Methods
    disconnect(): void
    sendCredentials(credentials: {
      username?: string
      password?: string
      target?: string
    }): void
    // noVNC accepts `code: null` to skip DOM-code derivation and emit based
    // on keysym alone. Not documented well upstream, but used in practice.
    sendKey(keysym: number, code: string | null, down?: boolean): void
    sendCtrlAltDel(): void
    focus(): void
    blur(): void
    machineShutdown(): void
    machineReboot(): void
    machineReset(): void
    clipboardPasteFrom(text: string): void

    // Properties
    scaleViewport: boolean
    resizeSession: boolean
    showDotCursor: boolean
    background: string
    qualityLevel: number
    compressionLevel: number
    viewOnly: boolean
    clipViewport: boolean
    dragViewport: boolean
  }
}
