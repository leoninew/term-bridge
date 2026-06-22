import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig, loadEnv } from 'vite'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const backend = env.VITE_TERMBRIDGE_BACKEND

  if (!backend) {
    throw new Error('VITE_TERMBRIDGE_BACKEND must be set in web/.env')
  }

  return {
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
  }
})
