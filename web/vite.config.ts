import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  envPrefix: 'TERMBRIDGE_',
  plugins: [vue(), tailwindcss()],
  server: {
    host: 'localhost',
    port: 9030,
    proxy: {
      '/local-api': {
        target: 'http://127.0.0.1:9031',
        changeOrigin: true,
        rewrite: (requestPath) => requestPath.replace(/^\/local-api/, '/api'),
        ws: true,
      },
      '/cloud-api': {
        target: 'http://127.0.0.1:9032',
        changeOrigin: true,
        rewrite: (requestPath) => requestPath.replace(/^\/cloud-api/, '/api'),
        ws: true,
      },
    },
  },
})
