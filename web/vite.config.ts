import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    host: '127.0.0.1',
    port: 9011,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:9010',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
