import { resolve } from 'node:path'
import { defineConfig, externalizeDepsPlugin } from 'electron-vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

const root = import.meta.dirname

export default defineConfig({
  main: {
    plugins: [externalizeDepsPlugin()],
    resolve: { alias: { '@shared': resolve(root, 'src/shared') } },
    build: {
      rollupOptions: { input: resolve(root, 'src/main/index.ts') }
    }
  },
  preload: {
    plugins: [externalizeDepsPlugin()],
    resolve: { alias: { '@shared': resolve(root, 'src/shared') } },
    build: {
      rollupOptions: { input: resolve(root, 'src/preload/index.ts') }
    }
  },
  renderer: {
    root: resolve(root, 'src/renderer'),
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@shared': resolve(root, 'src/shared'),
        '@renderer': resolve(root, 'src/renderer/src')
      }
    },
    build: {
      rollupOptions: { input: resolve(root, 'src/renderer/index.html') }
    }
  }
})
