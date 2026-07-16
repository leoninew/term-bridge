import js from '@eslint/js'
import eslintConfigPrettier from 'eslint-config-prettier'
import tseslint from 'typescript-eslint'
import vue from 'eslint-plugin-vue'
import importX from 'eslint-plugin-import-x'
import { createTypeScriptImportResolver } from 'eslint-import-resolver-typescript'

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
        HTMLButtonElement: 'readonly',
        EventTarget: 'readonly',
        navigator: 'readonly',
        MessageEvent: 'readonly',
        ResizeObserver: 'readonly',
      },
    },
  },
  {
    plugins: {
      'import-x': importX,
    },
    settings: {
      'import-x/resolver-next': [
        createTypeScriptImportResolver({
          alwaysTryTypes: true,
          project: './tsconfig.json',
        }),
      ],
    },
  },
  {
    files: ['**/*.{ts,vue,mjs}'],
    rules: {
      'import-x/no-unresolved': ['error', { caseSensitive: true, caseSensitiveStrict: false }],
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
