import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Dev: /api se posreduje na Go backend (privzeto :8080).
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    proxy: {
      '/api': process.env.VITE_API_TARGET ?? 'http://localhost:8080',
    },
  },
})
