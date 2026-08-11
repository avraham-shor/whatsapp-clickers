import path from 'node:path'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
      '/webhooks': 'http://localhost:8080',
      '/ws': {
        target: 'http://localhost:8080',
        ws: true,
      },
    },
  },
  // One config file, not two: the `@` alias and the react plugin above are
  // exactly what the test run needs, and duplicating them in a separate
  // vitest.config.ts is how they drift. `globals` stays off (the default) —
  // every test imports describe/it/expect/vi from 'vitest' explicitly,
  // matching this codebase's explicit-imports style.
  test: {
    environment: 'jsdom',
    include: ['src/**/*.test.{ts,tsx}'],
  },
})
