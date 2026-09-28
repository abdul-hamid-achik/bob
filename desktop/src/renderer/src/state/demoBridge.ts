import { parse as parseYaml } from 'yaml'
import type {
  ActivityEntry,
  AppCommand,
  AppInfo,
  BinaryInfo,
  BobApi,
  DocContent,
  DocEntry,
  FileContent,
  GitCommit,
  GitStatus,
  McpCallResult,
  McpLogLine,
  McpStartOptions,
  PathEntry,
  StreamEvent,
  TaskEntry,
  WorkspaceFiles
} from '@shared/ipc'
import type { AppSettings, McpServerInfo, McpTool, RunRequest, RunResult, WorkspaceInfo } from '@shared/types'
import { DEFAULT_SETTINGS } from '@shared/types'
import { displayArgv } from '@shared/format'

interface DemoData {
  DEMO_WORKSPACE: string
  DEMO_MANIFEST: string
  DEMO_LOCK: string
  DEMO_SAMPLES: Record<string, unknown>
}

let cache: DemoData | null = null

async function data(): Promise<DemoData> {
  if (!cache) cache = (await import('./demoData')) as unknown as DemoData
  return cache
}

function unavailable(featureId: string, argv: string[]): RunResult {
  return {
    requestId: `demo-${featureId}-${Date.now()}`,
    featureId,
    argv,
    displayCommand: `bob ${displayArgv(argv)}`,
    cwd: '/demo',
    exitCode: 4,
    durationMs: 0,
    stdout: '',
    stderr: '',
    envelope: {
      schema_version: 1,
      ok: false,
      command: featureId,
      data: {
        error: {
          code: 'input_invalid',
          message: `demo mode has no captured envelope for ${featureId}; disable demo mode to run the real command`
        }
      },
      warnings: [],
      next_actions: ['turn off demo mode in Settings']
    },
    parseError: null,
    binaryPath: '/demo/bob',
    startedAt: new Date().toISOString(),
    timedOut: false,
    cancelled: false
  }
}

function demoResult(featureId: string, argv: string[], envelope: unknown, cwd: string): RunResult {
  return {
    requestId: `demo-${featureId}-${Date.now()}`,
    featureId,
    argv,
    displayCommand: `bob ${displayArgv(argv)}`,
    cwd,
    exitCode: (envelope as { ok?: boolean })?.ok ? 0 : 2,
    durationMs: 12,
    stdout: JSON.stringify(envelope, null, 2),
    stderr: '',
    envelope: envelope as RunResult['envelope'],
    parseError: null,
    binaryPath: '/demo/bob',
    startedAt: new Date().toISOString(),
    timedOut: false,
    cancelled: false
  }
}

const streamListeners = new Set<(event: StreamEvent) => void>()
const commandListeners = new Set<(command: AppCommand) => void>()
let demoSettings: AppSettings = { ...DEFAULT_SETTINGS, demoMode: true }
const activityLog: ActivityEntry[] = []

/**
 * Replays real captured Bob envelopes so every panel is reviewable without a
 * bob binary. Anything without a capture says so instead of inventing data.
 */
export const demoBridge: BobApi = {
  async appInfo(): Promise<AppInfo> {
    return {
      appVersion: '0.1.0',
      electron: 'demo',
      node: 'demo',
      chrome: 'demo',
      platform: 'demo',
      arch: 'demo',
      userData: '/demo'
    }
  },
  async resolveBinary(): Promise<BinaryInfo> {
    const loaded = await data()
    return {
      path: '/demo/bob',
      source: 'none',
      exists: false,
      executable: false,
      version: (loaded.DEMO_SAMPLES.version as { data?: { version?: string } })?.data?.version ?? 'demo',
      commit: 'demo',
      date: new Date().toISOString(),
      candidates: [],
      error: 'demo mode replays captured envelopes; no binary is executed'
    }
  },
  async getSettings(): Promise<AppSettings> {
    return { ...demoSettings }
  },
  async setSettings(patch: Partial<AppSettings>): Promise<AppSettings> {
    demoSettings = { ...demoSettings, ...patch, demoMode: true }
    return { ...demoSettings }
  },
  async listWorkspaces(): Promise<WorkspaceInfo[]> {
    const loaded = await data()
    return [
      {
        path: loaded.DEMO_WORKSPACE,
        name: loaded.DEMO_WORKSPACE.split('/').pop() ?? 'demo',
        hasManifest: true,
        hasLock: true,
        managedFiles: loaded.DEMO_LOCK.split('- path:').length - 1,
        lastOpenedAt: new Date().toISOString()
      }
    ]
  },
  async addWorkspace(path: string): Promise<WorkspaceInfo> {
    return { path, name: path.split('/').pop() ?? path, hasManifest: false, hasLock: false, managedFiles: 0, lastOpenedAt: new Date().toISOString() }
  },
  async forgetWorkspace(): Promise<WorkspaceInfo[]> {
    return this.listWorkspaces()
  },
  async chooseDirectory(): Promise<string | null> {
    return null
  },
  async readWorkspace(path: string): Promise<WorkspaceFiles> {
    const loaded = await data()
    let lockFiles: { path: string; sha256: string }[] = []
    let lockRecipe: { id: string; version: number } | null = null
    try {
      const parsed = parseYaml(loaded.DEMO_LOCK) as { recipe?: { id?: string; version?: number }; files?: { path?: string; sha256?: string }[] }
      lockRecipe = parsed?.recipe?.id ? { id: parsed.recipe.id, version: parsed.recipe.version ?? 0 } : null
      lockFiles = (parsed?.files ?? []).map((entry) => ({ path: entry.path ?? '', sha256: entry.sha256 ?? '' }))
    } catch {
      /* leave empty */
    }
    return {
      path,
      manifest: loaded.DEMO_MANIFEST,
      manifestPath: `${path}/bob.yaml`,
      lock: loaded.DEMO_LOCK,
      lockPath: `${path}/bob.lock`,
      lockRecipe,
      lockFiles,
      manifestError: null,
      lockError: null
    }
  },
  async writeManifest(): Promise<{ written: boolean; error: string | null }> {
    return { written: false, error: 'demo mode never writes files' }
  },
  async run(request: RunRequest): Promise<RunResult> {
    const loaded = await data()
    const sample = loaded.DEMO_SAMPLES[request.featureId]
    const result = sample === undefined ? unavailable(request.featureId, request.argv) : demoResult(request.featureId, request.argv, sample, request.cwd)
    activityLog.unshift({
      id: result.requestId,
      at: result.startedAt,
      kind: 'bob',
      featureId: request.featureId,
      displayCommand: result.displayCommand,
      argv: request.argv,
      cwd: request.cwd,
      exitCode: result.exitCode,
      durationMs: result.durationMs,
      ok: result.envelope?.ok ?? false,
      planDigest: null,
      errorCode: null,
      bytesOut: result.stdout.length,
      bytesErr: 0,
      binaryPath: '/demo/bob',
      timedOut: false,
      cancelled: false
    })
    return result
  },
  async cancel(): Promise<{ cancelled: boolean }> {
    return { cancelled: false }
  },
  async startStream(): Promise<{ streamId: string; error: string | null }> {
    return { streamId: '', error: 'demo mode does not stream; run the buffered command instead' }
  },
  async stopStream(): Promise<{ stopped: boolean }> {
    return { stopped: false }
  },
  onStreamEvent(listener: (event: StreamEvent) => void): () => void {
    streamListeners.add(listener)
    return () => streamListeners.delete(listener)
  },
  async listPaths(): Promise<PathEntry[]> {
    return []
  },
  async readFile(_workspace: string, relativePath: string): Promise<FileContent> {
    const loaded = await data()
    if (relativePath === 'bob.yaml') {
      return { path: relativePath, content: loaded.DEMO_MANIFEST, bytes: loaded.DEMO_MANIFEST.length, truncated: false, binary: false }
    }
    if (relativePath === 'bob.lock') {
      return { path: relativePath, content: loaded.DEMO_LOCK, bytes: loaded.DEMO_LOCK.length, truncated: false, binary: false }
    }
    return { path: relativePath, content: '', bytes: 0, truncated: false, binary: false }
  },
  async openExternal(): Promise<{ opened: boolean; error: string | null }> {
    return { opened: false, error: 'demo mode does not open external applications' }
  },
  async gitStatus(): Promise<GitStatus> {
    return { available: false, branch: null, dirty: false, entries: [], error: 'demo mode does not read git state' }
  },
  async gitLog(): Promise<GitCommit[]> {
    return []
  },
  async gitDiff(): Promise<{ diff: string; error: string | null }> {
    return { diff: '', error: 'demo mode does not read git state' }
  },
  async listTasks(): Promise<TaskEntry[]> {
    return []
  },
  async runTask(): Promise<{ streamId: string; error: string | null }> {
    return { streamId: '', error: 'demo mode does not run tasks' }
  },
  async taskBinary(): Promise<{ path: string | null }> {
    return { path: null }
  },
  async launchTerminal(command: string): Promise<{ launched: boolean; command: string; error: string | null }> {
    return { launched: false, command, error: 'demo mode does not launch terminals' }
  },
  async listDocs(): Promise<DocEntry[]> {
    return []
  },
  async readDoc(): Promise<DocContent | null> {
    return null
  },
  async mcpStatus(): Promise<McpServerInfo | null> {
    return null
  },
  async mcpStart(options: McpStartOptions): Promise<McpServerInfo> {
    return {
      started: false,
      workspace: options.workspace,
      allowAnyWorkspace: false,
      allowWorkspaces: [],
      tools: [],
      error: 'demo mode does not start an MCP server'
    }
  },
  async mcpStop(): Promise<{ stopped: boolean }> {
    return { stopped: false }
  },
  async mcpCallTool(tool: string): Promise<McpCallResult> {
    return {
      ok: false,
      tool,
      durationMs: 0,
      content: [],
      structured: null,
      isError: true,
      error: 'demo mode does not start an MCP server',
      request: { method: 'tools/call', params: { name: tool, arguments: {} } },
      rawResponse: null
    }
  },
  async mcpTools(): Promise<McpTool[]> {
    return []
  },
  async mcpLog(): Promise<McpLogLine[]> {
    return []
  },
  async repoRoot(): Promise<{ path: string | null }> {
    return { path: null }
  },
  async activity(): Promise<ActivityEntry[]> {
    return [...activityLog]
  },
  async clearActivity(): Promise<{ cleared: number }> {
    const cleared = activityLog.length
    activityLog.length = 0
    return { cleared }
  },
  async exportActivity(): Promise<{ path: string | null; error: string | null }> {
    return { path: null, error: 'demo mode does not write files' }
  },
  onCommand(listener: (command: AppCommand) => void): () => void {
    commandListeners.add(listener)
    return () => commandListeners.delete(listener)
  }
}
