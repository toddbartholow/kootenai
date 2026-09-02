import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  // Mirror the compile-time constants defined in vite.config.ts so tests can
  // reference them without hitting a ReferenceError. Tests default to sane
  // values; tests that want specific behavior set it via the normal
  // test-time mocking plumbing in src/test-setup.ts.
  define: {
    __USE_MOCK_DATA__: JSON.stringify(false),
    __API_BASE_URL__: JSON.stringify(''),
  },
  test: {
    environment: 'happy-dom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    include: ['src/**/*.{test,spec}.ts'],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'json', 'html'],
      include: ['src/**/*.{ts,vue}'],
      exclude: ['src/**/*.{test,spec}.ts', 'src/main.ts'],
    },
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
      '@volt': fileURLToPath(new URL('./src/components/volt', import.meta.url)),
    },
  },
})
