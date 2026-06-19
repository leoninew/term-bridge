import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

const backend = process.env.TERMBRIDGE_WEB_BACKEND ?? 'http://localhost:9010'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    host: 'localhost',
    port: 9011,
    proxy: {
      '/api': {
        target: backend,
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
