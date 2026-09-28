import type { BinaryInfo } from '../shared/ipc'
import { createActivityLog, type ActivityLog } from './activity'
import { detectBobRepoRoot, resolveBobBinary } from './binary'
import { McpClient } from './mcp'
import { BobRunner } from './runner'
import { createSettingsStore, type SettingsStore } from './settings'
import { whichTool } from './terminal'

export interface ConsoleState {
  userDataDir: string
  appDir: string
  settings: SettingsStore
  activity: ActivityLog
  runner: BobRunner
  mcp: McpClient
  binary: BinaryInfo | null
  repoRoot: string | null
  taskBinary: string | null
  mcpLog: { at: string; stream: 'stdout' | 'stderr'; line: string }[]
}

const MCP_LOG_LIMIT = 400

export async function createState(userDataDir: string, appDir: string): Promise<ConsoleState> {
  const settings = createSettingsStore(userDataDir)
  const activity = createActivityLog(userDataDir)
  const mcpLog: ConsoleState['mcpLog'] = []

  const state: ConsoleState = {
    userDataDir,
    appDir,
    settings,
    activity,
    binary: null,
    repoRoot: null,
    taskBinary: null,
    mcpLog,
    runner: null as unknown as BobRunner,
    mcp: null as unknown as McpClient
  }

  state.runner = new BobRunner({
    binaryPath: '',
    onActivity: (entry) => activity.record(entry),
    onStreamEvent: () => undefined
  })
  state.mcp = new McpClient('', {
    onLog: (line, stream) => {
      mcpLog.push({ at: new Date().toISOString(), stream, line })
      if (mcpLog.length > MCP_LOG_LIMIT) mcpLog.splice(0, mcpLog.length - MCP_LOG_LIMIT)
    }
  })

  await refreshBinary(state)
  state.taskBinary = await whichTool('task')
  return state
}

/** Re-resolves the Bob binary and the source checkout that owns it. */
export async function refreshBinary(state: ConsoleState): Promise<BinaryInfo> {
  const settings = state.settings.get()
  const info = await resolveBobBinary({
    configuredPath: settings.bobBinaryPath || undefined,
    workspaces: [settings.defaultWorkspace, ...settings.recentWorkspaces].filter(Boolean),
    appDir: state.appDir
  })
  state.binary = info
  state.repoRoot = detectBobRepoRoot(info.path || null, [settings.defaultWorkspace, ...settings.recentWorkspaces].filter(Boolean))
  state.runner.setBinaryPath(info.path)
  state.mcp.setBinaryPath(info.path)
  return info
}
