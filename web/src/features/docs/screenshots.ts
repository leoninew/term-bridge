import type { DocLanguage, DocScreenshotId } from './catalog'

interface ScreenshotCopy {
  title: string
  planned: string
  caption: string
}

const screenshotCopy: Record<DocLanguage, Record<DocScreenshotId, ScreenshotCopy>> = {
  'zh-CN': {
    'cli-ready': {
      title: '确认 CLI 已可用',
      planned: '在设备终端中显示 Cloud CLI 或 Code XCLI 已启动并可交互。',
      caption: '先在设备终端确认外部 CLI 本身可用。',
    },
    'release-assets': {
      title: '从 Release 选择制品',
      planned: '显示最新稳定版本、Assets 区域和三个 x64 ZIP。',
      caption: '从 GitHub Releases 下载与你的系统匹配的制品。',
    },
    'package-folder': {
      title: '解压后的制品目录',
      planned: '显示 launcher、二进制、web 与 configs 等必要文件。',
      caption: '解压后保留完整目录，再从其中启动 TermBridge。',
    },
    'agent-started': {
      title: '启动本地 Agent',
      planned: '显示启动脚本运行成功与本地访问地址。',
      caption: '保持启动窗口运行，再在浏览器中打开本地工作台。',
    },
    'workspace-entry': {
      title: '进入 Workspace',
      planned: '显示 Sessions 工作台及项目目录入口。',
      caption: '选择项目目录，让 Session 从正确的项目上下文运行。',
    },
    'create-session': {
      title: '创建 CLI Session',
      planned: '显示新建会话的目录、名称和命令来源字段。',
      caption: '使用直接命令或已启用 Shortcut 创建新的独立 Session。',
    },
    'browser-terminal': {
      title: '浏览器中的 CLI Session',
      planned: '显示运行中的 Session、所属 Workspace 和终端区域。',
      caption: '创建后在浏览器终端中继续与 CLI 交互。',
    },
    'shortcut-management': {
      title: '管理常用命令',
      planned: '显示 Shortcut 列表、标签、状态和编辑入口。',
      caption: 'Shortcut 保存的是一条可复用命令，不是正在运行的终端。',
    },
    'shortcut-command-source': {
      title: '从 Shortcut 创建会话',
      planned: '显示新建会话中选择 Shortcut 命令来源的控件。',
      caption: '先选择项目目录，再从已启用 Shortcut 中选择命令。',
    },
  },
  'en-US': {
    'cli-ready': {
      title: 'Confirm that the CLI is ready',
      planned: 'Show Cloud CLI or Code XCLI started and interactive in a device terminal.',
      caption: 'Confirm that the external CLI works in the device terminal first.',
    },
    'release-assets': {
      title: 'Choose a Release package',
      planned: 'Show the latest stable version, the Assets area, and the three x64 ZIPs.',
      caption: 'Download the package for your system from GitHub Releases.',
    },
    'package-folder': {
      title: 'Extracted package directory',
      planned: 'Show the launcher, binary, web, and configs files needed to start.',
      caption: 'Keep the extracted directory intact, then start TermBridge from it.',
    },
    'agent-started': {
      title: 'Start the local Agent',
      planned: 'Show a successful launcher and local access address.',
      caption: 'Keep the launcher window open, then open the local workbench in a browser.',
    },
    'workspace-entry': {
      title: 'Enter a Workspace',
      planned: 'Show the Sessions workbench and project directory entry point.',
      caption: 'Choose a project directory so the Session runs in the right context.',
    },
    'create-session': {
      title: 'Create a CLI Session',
      planned: 'Show the new Session directory, name, and command-source fields.',
      caption: 'Use a direct command or enabled Shortcut to create an independent Session.',
    },
    'browser-terminal': {
      title: 'CLI Session in the browser',
      planned: 'Show a running Session, its Workspace, and the terminal region.',
      caption: 'Continue interacting with the CLI in the browser terminal after creation.',
    },
    'shortcut-management': {
      title: 'Manage frequent commands',
      planned: 'Show the Shortcut list, tags, state, and editing entry point.',
      caption: 'A Shortcut saves one reusable command, not a running terminal.',
    },
    'shortcut-command-source': {
      title: 'Create a Session from a Shortcut',
      planned: 'Show the Shortcut command-source picker while creating a Session.',
      caption: 'Choose the project directory first, then choose an enabled Shortcut command.',
    },
  },
}

export function getScreenshotCopy(language: DocLanguage, id: DocScreenshotId): ScreenshotCopy {
  return screenshotCopy[language][id]
}
