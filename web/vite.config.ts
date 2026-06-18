import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

const backend = process.env.TERMBRIDGE_WEB_BACKEND ?? 'http://127.0.0.1:9010'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    host: '127.0.0.1',
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
