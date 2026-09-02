declare module 'spice-html5' {
  export interface SpiceMainConnOptions {
    uri?: string | undefined
    host?: string | undefined
    port?: string | number | undefined
    password?: string | undefined
    ticket?: string | undefined
    token?: string | undefined
    proxy?: string | undefined
    screen_id?: string | undefined
    message_id?: string | undefined
    onerror?: ((error: Error) => void) | undefined
    onsuccess?: ((msg?: string) => void) | undefined
    onagent?: ((agent: unknown) => void) | undefined
  }

  export class SpiceMainConn {
    constructor(options: SpiceMainConnOptions)

    // Event handlers
    onerror: ((error: Error) => void) | null
    onsuccess: (() => void) | null
    onagent: ((agent: unknown) => void) | null

    // Methods
    stop(): void
    send_keys(keys: number[]): void
    send_key(key: number): void
    send_mouse_position(x: number, y: number): void
    send_mouse_down(button: number): void
    send_mouse_up(button: number): void
    sendCtrlAltDel(): void
  }

  export interface DisplayOptions {
    parent: HTMLElement
    connection: SpiceMainConn
    height?: number
    width?: number
  }

  export class SpiceDisplay {
    constructor(options: DisplayOptions)

    // Methods
    destroy(): void
    resize(): void

    // Properties
    width: number
    height: number
  }

  export class SpiceInputs {
    constructor(connection: SpiceMainConn)

    // Methods
    attach(element: HTMLElement): void
    detach(): void
  }

  export interface SpiceResizeHelperOptions {
    connection: SpiceMainConn
  }

  export class SpiceResizeHelper {
    constructor(options: SpiceResizeHelperOptions)
  }

  // Utility functions
  export function handle_resize(): void
  export function resize_helper(sc: SpiceMainConn): void
  export function sendCtrlAltDel(sc: SpiceMainConn): void
}

// Support for different module paths
declare module '@spice-project/spice-html5/src/main.js' {
  export * from 'spice-html5'
}
