import type { IExtensionManifest } from '@codingame/monaco-vscode-api/extensions'

export const TERM_BRIDGE_SCM_PROVIDER_ID = 'termbridge-git'

export const ScmCommandId = {
  refresh: 'termbridge.scm.refresh',
  open: 'termbridge.scm.open',
  stage: 'termbridge.scm.stage',
  add: 'termbridge.scm.add',
  unstage: 'termbridge.scm.unstage',
  discard: 'termbridge.scm.discard',
  stageAllChanges: 'termbridge.scm.stageAllChanges',
  addAll: 'termbridge.scm.addAll',
  unstageAll: 'termbridge.scm.unstageAll',
  discardAllChanges: 'termbridge.scm.discardAllChanges',
  discardAllUntracked: 'termbridge.scm.discardAllUntracked',
  commit: 'termbridge.scm.commit',
} as const

const providerWhen = `scmProvider == ${TERM_BRIDGE_SCM_PROVIDER_ID}`
const changesWhen = `${providerWhen} && scmResourceGroup == changes`
const untrackedWhen = `${providerWhen} && scmResourceGroup == untracked`
const stagedWhen = `${providerWhen} && scmResourceGroup == staged`
const changesOrUntrackedResourceWhen = `${providerWhen} && (scmResourceState == changes || scmResourceState == untracked)`
const changesOrUntrackedGroupWhen = `(${changesWhen} || ${untrackedWhen})`

/**
 * Native Workbench contributions for the TermBridge SCM provider.
 *
 * Regular menu groups supply context-menu actions. The SCM row action bar reads
 * only `inline` groups, so those entries supply native hover actions without UI.
 */
export const termBridgeWorkbenchExtensionManifest = {
  name: 'termbridge-workbench',
  publisher: 'termbridge',
  version: '1.0.0',
  engines: { vscode: '*' },
  enabledApiProposals: ['scmActionButton'],
  contributes: {
    commands: [
      { command: ScmCommandId.open, title: 'Open Changes', icon: '$(go-to-file)' },
      { command: ScmCommandId.stage, title: 'Stage Changes', icon: '$(add)' },
      { command: ScmCommandId.add, title: 'Add', icon: '$(add)' },
      { command: ScmCommandId.unstage, title: 'Unstage Changes', icon: '$(remove)' },
      { command: ScmCommandId.discard, title: 'Discard Changes', icon: '$(discard)' },
      { command: ScmCommandId.stageAllChanges, title: 'Stage All Changes', icon: '$(add)' },
      { command: ScmCommandId.addAll, title: 'Add All', icon: '$(add)' },
      { command: ScmCommandId.unstageAll, title: 'Unstage All Changes', icon: '$(remove)' },
      { command: ScmCommandId.discardAllChanges, title: 'Discard All Changes', icon: '$(discard)' },
      {
        command: ScmCommandId.discardAllUntracked,
        title: 'Discard All Untracked Files',
        icon: '$(discard)',
      },
    ],
    menus: {
      'scm/resourceState/context': [
        {
          command: ScmCommandId.open,
          when: providerWhen,
          group: 'navigation',
        },
        {
          command: ScmCommandId.stage,
          when: `${providerWhen} && scmResourceState == changes`,
          group: '1_modification',
        },
        {
          command: ScmCommandId.add,
          when: `${providerWhen} && scmResourceState == untracked`,
          group: '1_modification',
        },
        {
          command: ScmCommandId.unstage,
          when: `${providerWhen} && scmResourceState == staged`,
          group: '1_modification',
        },
        {
          command: ScmCommandId.discard,
          when: changesOrUntrackedResourceWhen,
          group: '2_destructive',
        },
        {
          command: ScmCommandId.open,
          when: providerWhen,
          group: 'inline@1',
        },
        {
          command: ScmCommandId.stage,
          when: `${providerWhen} && scmResourceState == changes`,
          group: 'inline@2',
        },
        {
          command: ScmCommandId.add,
          when: `${providerWhen} && scmResourceState == untracked`,
          group: 'inline@2',
        },
        {
          command: ScmCommandId.unstage,
          when: `${providerWhen} && scmResourceState == staged`,
          group: 'inline@2',
        },
        {
          command: ScmCommandId.discard,
          when: changesOrUntrackedResourceWhen,
          group: 'inline@3',
        },
      ],
      'scm/resourceFolder/context': [
        {
          command: ScmCommandId.open,
          when: providerWhen,
          group: 'navigation',
        },
        {
          command: ScmCommandId.stage,
          when: changesWhen,
          group: '1_modification',
        },
        {
          command: ScmCommandId.add,
          when: untrackedWhen,
          group: '1_modification',
        },
        {
          command: ScmCommandId.unstage,
          when: stagedWhen,
          group: '1_modification',
        },
        {
          command: ScmCommandId.discard,
          when: changesOrUntrackedGroupWhen,
          group: '2_destructive',
        },
        {
          command: ScmCommandId.open,
          when: providerWhen,
          group: 'inline@1',
        },
        {
          command: ScmCommandId.stage,
          when: changesWhen,
          group: 'inline@2',
        },
        {
          command: ScmCommandId.add,
          when: untrackedWhen,
          group: 'inline@2',
        },
        {
          command: ScmCommandId.unstage,
          when: stagedWhen,
          group: 'inline@2',
        },
        {
          command: ScmCommandId.discard,
          when: changesOrUntrackedGroupWhen,
          group: 'inline@3',
        },
      ],
      'scm/resourceGroup/context': [
        {
          command: ScmCommandId.stageAllChanges,
          when: changesWhen,
          group: '1_modification',
        },
        {
          command: ScmCommandId.addAll,
          when: untrackedWhen,
          group: '1_modification',
        },
        {
          command: ScmCommandId.unstageAll,
          when: stagedWhen,
          group: '1_modification',
        },
        {
          command: ScmCommandId.discardAllChanges,
          when: changesWhen,
          group: '2_destructive',
        },
        {
          command: ScmCommandId.discardAllUntracked,
          when: untrackedWhen,
          group: '2_destructive',
        },
      ],
    },
  },
} satisfies IExtensionManifest
