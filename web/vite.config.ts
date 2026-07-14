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
  build: {
    rolldownOptions: {  // 注意：新版用 rolldownOptions，老版可能是 rollupOptions
      checks: {
        invalidAnnotation: false,   // 关闭 INVALID_ANNOTATION 警告
      },
    },
  },
})
