import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'happy-dom',
    exclude: [
      '**/node_modules/**',
      '**/dist/**',
      'src/features/files/**',
      'src/store/fileWorkbench.test.ts',
      'src/components/workspace/Git*.test.ts',
      'src/components/workspace/WorkspaceFile*.test.ts',
      'src/components/workspace/WorkspaceGit*.test.ts',
      'src/components/workspace/WorkspaceTextEditor.test.ts',
      'src/components/workspace/WorkspaceEditorTabs.test.ts',
    ],
  },
})
