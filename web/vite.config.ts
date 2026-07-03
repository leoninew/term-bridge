import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    host: 'localhost',
    port: 9030,
    proxy: {
      '/agent-api': {
        target: 'http://127.0.0.1:9031',
        changeOrigin: true,
        ws: true,
        rewrite: (path) => path.replace(/^\/agent-api/, ''),
      },
      '/cloud-api': {
        target: 'http://127.0.0.1:9032',
        changeOrigin: true,
        ws: true,
        rewrite: (path) => path.replace(/^\/cloud-api/, ''),
      },
    },
  },
})
