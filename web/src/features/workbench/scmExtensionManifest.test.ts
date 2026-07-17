import { describe, expect, it } from 'vitest'
import { ScmCommandId, termBridgeWorkbenchExtensionManifest } from './scmExtensionManifest'

const menus = termBridgeWorkbenchExtensionManifest.contributes.menus

type MenuItem = (typeof menus)['scm/resourceState/context'][number]

function commandIds(menuId: keyof typeof menus): string[] {
  return menus[menuId].map((item) => item.command)
}

function inlineCommands(menuItems: readonly MenuItem[]): MenuItem[] {
  return menuItems.filter((item) => item.group.startsWith('inline'))
}

function contextCommands(menuItems: readonly MenuItem[]): MenuItem[] {
  return menuItems.filter((item) => !item.group.startsWith('inline'))
}

describe('termBridgeWorkbenchExtensionManifest', () => {
  it('contributes the full native SCM action surface', () => {
    expect(
      termBridgeWorkbenchExtensionManifest.contributes.commands.map((command) => command.command),
    ).toEqual(
      expect.arrayContaining([
        ScmCommandId.open,
        ScmCommandId.stage,
        ScmCommandId.add,
        ScmCommandId.unstage,
        ScmCommandId.discard,
        ScmCommandId.stageAllChanges,
        ScmCommandId.addAll,
        ScmCommandId.unstageAll,
        ScmCommandId.discardAllChanges,
        ScmCommandId.discardAllUntracked,
      ]),
    )

    expect(commandIds('scm/resourceGroup/context')).toEqual([
      ScmCommandId.stageAllChanges,
      ScmCommandId.addAll,
      ScmCommandId.unstageAll,
      ScmCommandId.discardAllChanges,
      ScmCommandId.discardAllUntracked,
    ])
  })

  it('keeps SCM resource and folder actions in right-click menus and native hover bars', () => {
    for (const menuItems of [
      menus['scm/resourceState/context'],
      menus['scm/resourceFolder/context'],
    ]) {
      expect(contextCommands(menuItems).map((item) => item.command)).toEqual([
        ScmCommandId.open,
        ScmCommandId.stage,
        ScmCommandId.add,
        ScmCommandId.unstage,
        ScmCommandId.discard,
      ])
      expect(inlineCommands(menuItems).map((item) => item.command)).toEqual([
        ScmCommandId.open,
        ScmCommandId.stage,
        ScmCommandId.add,
        ScmCommandId.unstage,
        ScmCommandId.discard,
      ])
    }
  })

  it('limits all actions to the TermBridge provider and declares themed hover icons', () => {
    for (const menu of Object.values(menus).flat()) {
      expect(menu.when).toContain('scmProvider == termbridge-git')
    }

    const commandById = new Map(
      termBridgeWorkbenchExtensionManifest.contributes.commands.map((command) => [
        command.command,
        command,
      ]),
    )
    for (const commandId of [
      ScmCommandId.open,
      ScmCommandId.stage,
      ScmCommandId.add,
      ScmCommandId.unstage,
      ScmCommandId.discard,
    ]) {
      expect(commandById.get(commandId)?.icon).toMatch(/^\$\([\w-]+\)$/)
    }
  })

  it('uses state-specific conditions and stable hover action order', () => {
    const resourceInline = inlineCommands(menus['scm/resourceState/context'])
    expect(resourceInline.map((item) => item.group)).toEqual([
      'inline@1',
      'inline@2',
      'inline@2',
      'inline@2',
      'inline@3',
    ])
    expect(resourceInline[0]?.when).toBe('scmProvider == termbridge-git')
    expect(resourceInline[1]?.when).toContain('scmResourceState == changes')
    expect(resourceInline[2]?.when).toContain('scmResourceState == untracked')
    expect(resourceInline[3]?.when).toContain('scmResourceState == staged')
    expect(resourceInline[4]?.when).toContain(
      'scmResourceState == changes || scmResourceState == untracked',
    )

    const folderInline = inlineCommands(menus['scm/resourceFolder/context'])
    expect(folderInline[1]?.when).toContain('scmResourceGroup == changes')
    expect(folderInline[2]?.when).toContain('scmResourceGroup == untracked')
    expect(folderInline[3]?.when).toContain('scmResourceGroup == staged')
  })
})
