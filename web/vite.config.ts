import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'
import { resolve } from 'path'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiBaseUrl = env.VITE_API_BASE_URL || 'http://localhost:8080'
  // Compile-time boolean. When false, Vite/Rollup can statically eliminate
  // `if (__USE_MOCK_DATA__)` branches so mock code never ships to production.
  const useMockData = env.VITE_USE_MOCK_DATA === 'true'
  // In dev, Vite proxies /api to the backend (see the `server.proxy` block
  // below), so the base URL is the empty string and callers just use
  // /api/v1/... paths. In prod, the build injects the configured backend
  // origin as a string literal — no runtime env-var lookup needed.
  const apiBaseForClient = mode === 'production' ? (env.VITE_API_BASE_URL || '') : ''

  return {
    plugins: [vue(), tailwindcss()],
    define: {
      __USE_MOCK_DATA__: JSON.stringify(useMockData),
      __API_BASE_URL__: JSON.stringify(apiBaseForClient),
    },
    resolve: {
      alias: {
        '@': resolve(__dirname, 'src'),
        '@volt': resolve(__dirname, 'src/components/volt'),
      },
    },
    server: {
      port: 3000,
      proxy: {
        '/api': {
          target: apiBaseUrl,
          changeOrigin: true,
          ws: true, // Enable WebSocket for VNC proxy endpoints
        },
      },
    },
  }
})
