import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    host: 'localhost',
    port: 9031,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:9030',
        changeOrigin: true,
        ws: true,
      },
    },
  },
})
