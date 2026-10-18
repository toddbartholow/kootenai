/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL: string
  readonly VITE_USE_MOCK_DATA?: string
  readonly PROD: boolean
  readonly DEV: boolean
  readonly MODE: string
  readonly SSR: boolean
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

// Injected as a literal boolean by Vite's `define` in vite.config.ts.
// Reading this in source compiles down to `true` or `false`, which lets
// the bundler statically eliminate `if (__USE_MOCK_DATA__)` branches.
declare const __USE_MOCK_DATA__: boolean

// Injected as a literal string by Vite's `define`. Dev builds get the
// empty string (Vite proxy handles routing); prod builds get whatever
// VITE_API_BASE_URL was set to at build time. No runtime lookups.
declare const __API_BASE_URL__: string

// The spice-html5 library reads `window.spice_connection` during its resize
// helpers. We store a reference to the active SpiceMainConn there so the
// vendor's `handle_resize` can find it without prop drilling.
interface Window {
  spice_connection?: unknown
}
