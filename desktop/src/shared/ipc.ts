import type { AppSettings, McpServerInfo, McpTool, RunRequest, RunResult, WorkspaceInfo } from './types'

/** Every IPC channel the console exposes. Preload and main share this list. */
export const IPC = {
  runBob: 'bob:run',
  cancelRun: 'bob:cancel',
  startStream: 'bob:start-stream',
  stopStream: 'bob:stop-stream',
  streamEvent: 'bob:stream-event',
  resolveBinary: 'bob:resolve-binary',
  getSettings: 'app:get-settings',
  setSettings: 'app:set-settings',
  appInfo: 'app:info',
  listWorkspaces: 'workspace:list',
  addWorkspace: 'workspace:add',
  forgetWorkspace: 'workspace:forget',
  chooseDirectory: 'workspace:choose-directory',
  readWorkspace: 'workspace:read',
  writeManifest: 'workspace:write-manifest',
  listPaths: 'fs:list-paths',
  readFile: 'fs:read-file',
  openExternal: 'fs:open-external',
  gitStatus: 'git:status',
  gitLog: 'git:log',
  gitDiff: 'git:diff',
  listTasks: 'task:list',
  runTask: 'task:run',
  listDocs: 'docs:list',
  readDoc: 'docs:read',
  mcpStatus: 'mcp:status',
  mcpStart: 'mcp:start',
  mcpStop: 'mcp:stop',
  mcpCallTool: 'mcp:call-tool',
  mcpTools: 'mcp:tools',
  mcpLog: 'mcp:log',
  repoRoot: 'app:repo-root',
  launchTerminal: 'app:launch-terminal',
  taskBinary: 'task:binary',
  activity: 'activity:list',
  clearActivity: 'activity:clear',
  exportActivity: 'activity:export',
  appCommand: 'app:command'
} as const

export type IpcChannel = (typeof IPC)[keyof typeof IPC]

/** Commands the native menu and global shortcuts push into the renderer. */
export type AppCommand =
  | 'open-workspace'
  | 'command-palette'
  | 'run-plan'
  | 'run-check'
  | 'refresh'
  | 'toggle-theme'
  | 'focus-console'

export interface BinaryInfo {
  path: string
  source: 'settings' | 'repository' | 'path' | 'none'
  exists: boolean
  executable: boolean
  version: string | null
  commit: string | null
  date: string | null
  candidates: { path: string; source: string; exists: boolean }[]
  error: string | null
}

export interface LockEntryView {
  path: string
  sha256: string
}

export interface WorkspaceFiles {
  path: string
  manifest: string | null
  manifestPath: string | null
  lock: string | null
  lockPath: string | null
  lockRecipe: { id: string; version: number } | null
  lockFiles: LockEntryView[]
  manifestError: string | null
  lockError: string | null
}

export interface PathEntry {
  path: string
  kind: 'managed' | 'seed' | 'unmanaged' | 'lock-only' | 'directory'
  bytes: number | null
  mtime: string | null
  lockedSha256: string | null
  exists: boolean
}

export interface FileContent {
  path: string
  content: string
  bytes: number
  truncated: boolean
  binary: boolean
}

export interface GitStatus {
  available: boolean
  branch: string | null
  dirty: boolean
  entries: { path: string; index: string; worktree: string }[]
  error: string | null
}

export interface GitCommit {
  hash: string
  shortHash: string
  subject: string
  author: string
  date: string
}

export interface TaskEntry {
  name: string
  desc: string
  mutates: boolean
}

export interface DocEntry {
  id: string
  title: string
  relativePath: string
  group: string
}

export interface DocContent {
  id: string
  title: string
  relativePath: string
  markdown: string
}

export interface McpCallResult {
  ok: boolean
  tool: string
  durationMs: number
  content: { type: string; text?: string }[]
  structured: unknown
  isError: boolean
  error: string | null
  request: { method: string; params: unknown }
  rawResponse: string | null
}

export interface McpLogLine {
  at: string
  stream: 'stdout' | 'stderr'
  line: string
}

export interface McpStartOptions {
  workspace: string
  allowWorkspaces?: string[]
  allowAnyWorkspace?: boolean
}

export interface ActivityEntry {
  id: string
  at: string
  kind: 'bob' | 'task' | 'git' | 'mcp'
  featureId: string
  displayCommand: string
  argv: string[]
  cwd: string
  exitCode: number | null
  durationMs: number
  ok: boolean
  planDigest: string | null
  errorCode: string | null
  bytesOut: number
  bytesErr: number
  binaryPath: string
  timedOut: boolean
  cancelled: boolean
}

export interface StreamEvent {
  streamId: string
  requestId: string
  type: 'stdout' | 'stderr' | 'exit'
  text?: string
  result?: RunResult
}

export interface AppInfo {
  appVersion: string
  electron: string
  node: string
  chrome: string
  platform: string
  arch: string
  userData: string
}

/** The complete bridge the renderer may use. Exposed as window.bob. */
export interface BobApi {
  appInfo(): Promise<AppInfo>
  resolveBinary(): Promise<BinaryInfo>
  getSettings(): Promise<AppSettings>
  setSettings(patch: Partial<AppSettings>): Promise<AppSettings>

  listWorkspaces(): Promise<WorkspaceInfo[]>
  addWorkspace(path: string): Promise<WorkspaceInfo>
  forgetWorkspace(path: string): Promise<WorkspaceInfo[]>
  chooseDirectory(): Promise<string | null>
  readWorkspace(path: string): Promise<WorkspaceFiles>
  writeManifest(path: string, content: string): Promise<{ written: boolean; error: string | null }>

  run(request: RunRequest): Promise<RunResult>
  cancel(requestId: string): Promise<{ cancelled: boolean }>
  startStream(request: RunRequest): Promise<{ streamId: string; error: string | null }>
  stopStream(streamId: string): Promise<{ stopped: boolean }>
  onStreamEvent(listener: (event: StreamEvent) => void): () => void

  listPaths(workspace: string): Promise<PathEntry[]>
  readFile(workspace: string, relativePath: string): Promise<FileContent>
  openExternal(target: string, mode: 'editor' | 'finder' | 'browser'): Promise<{ opened: boolean; error: string | null }>

  gitStatus(cwd: string): Promise<GitStatus>
  gitLog(cwd: string, limit: number): Promise<GitCommit[]>
  gitDiff(cwd: string, relativePath?: string): Promise<{ diff: string; error: string | null }>

  listTasks(cwd: string): Promise<TaskEntry[]>
  runTask(name: string, cwd: string, confirmed?: boolean): Promise<{ streamId: string; error: string | null }>
  taskBinary(): Promise<{ path: string | null }>
  launchTerminal(command: string): Promise<{ launched: boolean; command: string; error: string | null }>

  listDocs(): Promise<DocEntry[]>
  readDoc(id: string): Promise<DocContent | null>

  mcpStatus(): Promise<McpServerInfo | null>
  mcpStart(options: McpStartOptions): Promise<McpServerInfo>
  mcpStop(): Promise<{ stopped: boolean }>
  mcpCallTool(tool: string, args: Record<string, unknown>): Promise<McpCallResult>
  mcpTools(): Promise<McpTool[]>
  mcpLog(): Promise<McpLogLine[]>
  repoRoot(): Promise<{ path: string | null }>

  activity(): Promise<ActivityEntry[]>
  clearActivity(): Promise<{ cleared: number }>
  exportActivity(): Promise<{ path: string | null; error: string | null }>

  onCommand(listener: (command: AppCommand) => void): () => void
}

declare global {
  interface Window {
    bob: BobApi
  }
}
