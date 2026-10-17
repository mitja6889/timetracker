import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Dev: /api se posreduje na Go backend (privzeto :8080).
export default defineConfig({
  plugins: [vue()],
  server: {
    port: 5173,
    host: true,
    allowedHosts: true,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
