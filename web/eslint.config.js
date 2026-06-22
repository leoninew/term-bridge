import js from '@eslint/js'
import eslintConfigPrettier from 'eslint-config-prettier'
import tseslint from 'typescript-eslint'
import vue from 'eslint-plugin-vue'

export default tseslint.config(
  {
    ignores: ['dist', 'node_modules'],
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
    files: ['**/*.{ts,vue}'],
    languageOptions: {
      globals: {
        console: 'readonly',
        window: 'readonly',
        document: 'readonly',
        WebSocket: 'readonly',
        TextEncoder: 'readonly',
        Uint8Array: 'readonly',
        URL: 'readonly',
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
