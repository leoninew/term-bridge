// @vitest-environment happy-dom
import { describe, it, expect } from 'vitest'
import WorkspaceFileDrawer from './WorkspaceFileDrawer.vue'

describe('import drawer', () => {
  it('loads', () => {
    expect(WorkspaceFileDrawer).toBeTruthy()
  })
})
