import tailwindcss from '@tailwindcss/vite'
import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vite'

export default defineConfig({
  envPrefix: 'TERMBRIDGE_',
  plugins: [
    vue(),
    tailwindcss(),
    {
      name: 'load-vscode-css-as-string',
      enforce: 'pre',
      async resolveId(source, importer, options) {
        const resolved = await this.resolve(source, importer, options)
        if (
          resolved &&
          resolved.id.match(
            /node_modules[/\\](@codingame[/\\]monaco-vscode|vscode|monaco-editor).*\.css$/,
          )
        ) {
          return {
            ...resolved,
            id: `${resolved.id}?inline`,
          }
        }
        return undefined
      },
    },
  ],
  worker: {
    format: 'es',
  },
  optimizeDeps: {
    include: [
      'vscode/localExtensionHost',
      '@codingame/monaco-vscode-api',
      '@codingame/monaco-vscode-api/extensions',
      '@codingame/monaco-vscode-api/monaco',
    ],
    esbuildOptions: {
      target: 'esnext',
    },
  },
  build: {
    target: 'esnext',
    rolldownOptions: {
      checks: {
        invalidAnnotation: false,
      },
    },
  },
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
