import { resolve } from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

/**
 * Renderer-only browser preview.
 *
 * Without the Electron preload bridge the console falls back to demo mode, so
 * every panel can be reviewed in a plain browser against real captured Bob
 * envelopes. The desktop app itself is built by electron.vite.config.ts.
 */
export default defineConfig({
  root: resolve(import.meta.dirname, 'src/renderer'),
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@shared': resolve(import.meta.dirname, 'src/shared'),
      '@renderer': resolve(import.meta.dirname, 'src/renderer/src')
    }
  },
  server: {
    port: 5199,
    strictPort: true
  },
  build: {
    outDir: resolve(import.meta.dirname, 'dist-web'),
    emptyOutDir: true
  }
})
