// @vitest-environment happy-dom
import { describe, expect, it } from 'vitest'
import WorkspaceFileWorkbench from './WorkspaceFileWorkbench.vue'

describe('workspace workbench module', () => {
  it('exports the page workbench component', () => {
    expect(WorkspaceFileWorkbench).toBeTruthy()
  })
})
