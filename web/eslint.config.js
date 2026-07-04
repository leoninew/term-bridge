import js from '@eslint/js'
import eslintConfigPrettier from 'eslint-config-prettier'
import tseslint from 'typescript-eslint'
import vue from 'eslint-plugin-vue'

export default tseslint.config(
  {
    ignores: ['dist', 'node_modules', 'src/gen/proto/**'],
  },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  ...vue.configs['flat/recommended'],
  {
    files: ['**/*.vue'],
    languageOptions: {
      parserOptions: {
        parser: tseslint.parser,
      },
    },
  },
  {
    files: ['**/*.{ts,vue,mjs}'],
    languageOptions: {
      globals: {
        console: 'readonly',
        process: 'readonly',
        window: 'readonly',
        document: 'readonly',
        WebSocket: 'readonly',
        TextEncoder: 'readonly',
        Uint8Array: 'readonly',
        URL: 'readonly',
        URLSearchParams: 'readonly',
        Blob: 'readonly',
        ArrayBuffer: 'readonly',
        HTMLElement: 'readonly',
        MessageEvent: 'readonly',
        ResizeObserver: 'readonly',
      },
    },
  },
  {
    files: ['**/*.vue'],
    rules: {
      'vue/block-order': ['error', { order: ['template', 'script', 'style'] }],
      'vue/html-indent': ['error', 2, { baseIndent: 1 }],
    },
  },
  eslintConfigPrettier,
)
