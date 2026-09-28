import { BrowserWindow, dialog, ipcMain, shell } from 'electron'
import { existsSync, renameSync, statSync, writeFileSync } from 'node:fs'
import { basename, isAbsolute, join, resolve, sep } from 'node:path'
import { displayArgv } from '../shared/format'
import { featureById } from '../shared/features'
import { IPC } from '../shared/ipc'
import type {
  ActivityEntry,
  BinaryInfo,
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
} from '../shared/ipc'
import type { AppSettings, McpServerInfo, RunRequest, RunResult, WorkspaceInfo } from '../shared/types'
import { listDocs, readDoc } from './docs'
import { readFileBounded, listPaths, safeJoin } from './fsx'
import { gitDiff, gitLog, gitStatus } from './git'
import { refreshBinary, type ConsoleState } from './state'
import { MUTATING_TASKS, isLongRunningTask, listTasks } from './tasks'
import { launchInTerminal, whichTool } from './terminal'
import { inspectWorkspace, normalizeWorkspaceInput, readWorkspaceFiles, touchRecent } from './workspaces'
import { spawnStream } from './runner'
import { writeJsonAtomic } from './jsonFile'

const TASK_TIMEOUT_MS = 15 * 60_000

export interface IpcDeps {
  state: ConsoleState
  getWindow: () => BrowserWindow | null
}

function send(window: BrowserWindow | null, event: StreamEvent): void {
  if (window && !window.isDestroyed()) window.webContents.send(IPC.streamEvent, event)
}

function isInsideRoot(path: string, root: string | null): boolean {
  if (!root) return false
  const target = resolve(path)
  const base = resolve(root)
  return target === base || target.startsWith(base + sep)
}

/** Paths the console may open or read: known workspaces, the Bob checkout, userData. */
function authorizedRoots(state: ConsoleState): string[] {
  const settings = state.settings.get()
  return [
    ...settings.recentWorkspaces,
    settings.defaultWorkspace,
    state.repoRoot ?? '',
    state.userDataDir
  ].filter(Boolean).map((entry) => resolve(entry))
}

function isAuthorizedPath(state: ConsoleState, path: string): boolean {
  return authorizedRoots(state).some((root) => isInsideRoot(path, root))
}

function requireWorkspace(state: ConsoleState, path: unknown): string {
  if (typeof path !== 'string' || !path.trim()) throw new Error('a workspace path is required')
  const normalized = normalizeWorkspaceInput(path)
  if (!normalized) throw new Error(`not an existing directory: ${path}`)
  return normalized
}

export function registerIpc({ state, getWindow }: IpcDeps): void {
  const { settings, activity, runner, mcp } = state

  runner.setStreamSink((event) => send(getWindow(), event))

  /* ------------------------------------------------------------ app info */
  ipcMain.handle(IPC.appInfo, () => ({
    appVersion: process.env.npm_package_version ?? '0.1.0',
    electron: process.versions.electron ?? '',
    node: process.versions.node ?? '',
    chrome: process.versions.chrome ?? '',
    platform: process.platform,
    arch: process.arch,
    userData: state.userDataDir
  }))

  ipcMain.handle(IPC.repoRoot, () => ({ path: state.repoRoot }))

  ipcMain.handle(IPC.resolveBinary, async (): Promise<BinaryInfo> => refreshBinary(state))

  ipcMain.handle(IPC.getSettings, (): AppSettings => settings.get())

  ipcMain.handle(IPC.setSettings, async (_event, patch: Partial<AppSettings>): Promise<AppSettings> => {
    const next = settings.set(patch ?? {})
    if (patch && ('bobBinaryPath' in patch || 'defaultWorkspace' in patch || 'recentWorkspaces' in patch)) {
      await refreshBinary(state)
      if (!state.taskBinary) state.taskBinary = await whichTool('task')
    }
    return next
  })

  /* ----------------------------------------------------------- workspaces */
  ipcMain.handle(IPC.listWorkspaces, (): WorkspaceInfo[] => {
    const current = settings.get()
    const paths = [current.defaultWorkspace, ...current.recentWorkspaces].filter(Boolean)
    const seen = new Set<string>()
    const out: WorkspaceInfo[] = []
    for (const path of paths) {
      const normalized = normalizeWorkspaceInput(path)
      if (!normalized || seen.has(normalized)) continue
      seen.add(normalized)
      out.push(inspectWorkspace(normalized))
    }
    return out
  })

  ipcMain.handle(IPC.addWorkspace, (_event, path: string): WorkspaceInfo => {
    const normalized = requireWorkspace(state, path)
    const current = settings.get()
    settings.set({ recentWorkspaces: touchRecent(current.recentWorkspaces, normalized) })
    if (!settings.get().defaultWorkspace) settings.set({ defaultWorkspace: normalized })
    void refreshBinary(state)
    return inspectWorkspace(normalized)
  })

  ipcMain.handle(IPC.forgetWorkspace, (_event, path: string): WorkspaceInfo[] => {
    const normalized = requireWorkspace(state, path)
    const current = settings.get()
    settings.set({
      recentWorkspaces: current.recentWorkspaces.filter((entry) => entry !== normalized),
      defaultWorkspace: current.defaultWorkspace === normalized ? '' : current.defaultWorkspace
    })
    return current.recentWorkspaces.filter((entry) => entry !== normalized).map((entry) => inspectWorkspace(entry))
  })

  ipcMain.handle(IPC.chooseDirectory, async (): Promise<string | null> => {
    const window = getWindow()
    const options = {
      title: 'Choose a Bob workspace',
      properties: ['openDirectory', 'createDirectory'] as ('openDirectory' | 'createDirectory')[]
    }
    const result = window ? await dialog.showOpenDialog(window, options) : await dialog.showOpenDialog(options)
    if (result.canceled || result.filePaths.length === 0) return null
    const chosen = result.filePaths[0] ?? null
    if (!chosen) return null
    const current = settings.get()
    settings.set({ recentWorkspaces: touchRecent(current.recentWorkspaces, chosen) })
    if (!settings.get().defaultWorkspace) settings.set({ defaultWorkspace: chosen })
    void refreshBinary(state)
    return chosen
  })

  ipcMain.handle(IPC.readWorkspace, (_event, path: string): WorkspaceFiles =>
    readWorkspaceFiles(requireWorkspace(state, path))
  )

  ipcMain.handle(IPC.writeManifest, (_event, path: string, content: string): { written: boolean; error: string | null } => {
    const workspace = requireWorkspace(state, path)
    if (typeof content !== 'string' || !content.trim()) return { written: false, error: 'refusing to write an empty bob.yaml' }
    const target = join(workspace, 'bob.yaml')
    if (!isAuthorizedPath(state, target)) return { written: false, error: 'workspace is not authorized' }
    const temp = `${target}.console-tmp`
    try {
      writeFileSync(temp, content, 'utf8')
      renameSync(temp, target)
      return { written: true, error: null }
    } catch (error) {
      return { written: false, error: error instanceof Error ? error.message : String(error) }
    }
  })

  /* ------------------------------------------------------------ bob runs */
  ipcMain.handle(IPC.runBob, async (_event, request: RunRequest): Promise<RunResult> => {
    if (!request || typeof request !== 'object') throw new Error('a run request is required')
    const cwd = requireWorkspace(state, request.cwd)
    const feature = featureById(request.featureId)
    if (!feature) throw new Error(`unknown feature id: ${request.featureId}`)
    if (!Array.isArray(request.argv)) throw new Error('argv must be an array')
    for (const token of request.argv) {
      if (typeof token !== 'string') throw new Error('argv tokens must be strings')
    }
    return runner.run({ ...request, cwd })
  })

  ipcMain.handle(IPC.cancelRun, (_event, requestId: string) => ({ cancelled: runner.cancel(String(requestId ?? '')) }))

  ipcMain.handle(IPC.startStream, (_event, request: RunRequest): { streamId: string; error: string | null } => {
    const cwd = requireWorkspace(state, request.cwd)
    return runner.startStream({ ...request, cwd })
  })

  ipcMain.handle(IPC.stopStream, (_event, streamId: string) => ({ stopped: runner.stopStream(String(streamId ?? '')) }))

  /* -------------------------------------------------------------- files */
  ipcMain.handle(IPC.listPaths, (_event, workspace: string): PathEntry[] => {
    const root = requireWorkspace(state, workspace)
    const files = readWorkspaceFiles(root)
    return listPaths(root, files.lockFiles)
  })

  ipcMain.handle(IPC.readFile, (_event, workspace: string, relativePath: string): FileContent => {
    const root = requireWorkspace(state, workspace)
    const content = readFileBounded(root, String(relativePath ?? ''))
    if (!content) throw new Error(`cannot read ${relativePath}`)
    return content
  })

  ipcMain.handle(
    IPC.openExternal,
    async (_event, target: string, mode: 'editor' | 'finder' | 'browser'): Promise<{ opened: boolean; error: string | null }> => {
      const value = String(target ?? '')
      if (mode === 'browser') {
        let url: URL
        try {
          url = new URL(value)
        } catch {
          return { opened: false, error: 'not a valid URL' }
        }
        if (url.protocol !== 'https:') return { opened: false, error: 'only https links may be opened' }
        await shell.openExternal(url.toString())
        return { opened: true, error: null }
      }
      const absolute = isAbsolute(value) ? resolve(value) : null
      if (!absolute) return { opened: false, error: 'an absolute path is required' }
      if (!isAuthorizedPath(state, absolute)) return { opened: false, error: 'path is outside an authorized workspace' }
      if (!existsSync(absolute)) return { opened: false, error: 'path does not exist' }
      if (mode === 'finder') {
        if (!statSync(absolute).isFile() && !statSync(absolute).isDirectory()) return { opened: false, error: 'unsupported file type' }
        shell.showItemInFolder(absolute)
        return { opened: true, error: null }
      }
      const error = await shell.openPath(absolute)
      return { opened: !error, error: error || null }
    }
  )

  /* ---------------------------------------------------------------- git */
  ipcMain.handle(IPC.gitStatus, async (_event, cwd: string): Promise<GitStatus> => gitStatus(requireWorkspace(state, cwd)))
  ipcMain.handle(IPC.gitLog, async (_event, cwd: string, limit: number): Promise<GitCommit[]> =>
    gitLog(requireWorkspace(state, cwd), Number(limit) || 20)
  )
  ipcMain.handle(IPC.gitDiff, async (_event, cwd: string, relativePath?: string) => {
    const root = requireWorkspace(state, cwd)
    const relative = relativePath ? String(relativePath) : undefined
    if (relative && !safeJoin(root, relative)) throw new Error('path escapes the workspace')
    return gitDiff(root, relative)
  })

  /* --------------------------------------------------------------- tasks */
  ipcMain.handle(IPC.listTasks, (_event, cwd: string): TaskEntry[] => listTasks(requireWorkspace(state, cwd)))
  ipcMain.handle(IPC.taskBinary, () => ({ path: state.taskBinary }))

  ipcMain.handle(
    IPC.runTask,
    (_event, name: string, cwd: string, confirmed?: boolean): { streamId: string; error: string | null } => {
      const root = requireWorkspace(state, cwd)
      const taskName = String(name ?? '')
      const available = listTasks(root)
      if (!available.some((task) => task.name === taskName)) {
        return { streamId: '', error: `unknown task: ${taskName}` }
      }
      if (!state.taskBinary) return { streamId: '', error: 'task is not installed or not on PATH' }
      if (MUTATING_TASKS.has(taskName) && !confirmed) {
        return { streamId: '', error: `task ${taskName} rewrites files and was not confirmed` }
      }
      const requestId = `task-${Date.now()}`
      const startedAt = new Date().toISOString()
      const window = getWindow()
      const handle = spawnStream(
        { command: state.taskBinary, argv: [taskName], cwd: root, timeoutMs: isLongRunningTask(taskName) ? 0 : TASK_TIMEOUT_MS },
        (event) => send(window, { streamId: handle.streamId, requestId, type: event.type, text: event.text })
      )
      void handle.done.then((outcome) => {
        send(window, { streamId: handle.streamId, requestId, type: 'exit' })
        activity.record({
          id: requestId,
          at: startedAt,
          kind: 'task',
          featureId: 'tasks',
          displayCommand: `task ${taskName}`,
          argv: [taskName],
          cwd: root,
          exitCode: outcome.exitCode,
          durationMs: outcome.durationMs,
          ok: outcome.exitCode === 0,
          planDigest: null,
          errorCode: null,
          bytesOut: Buffer.byteLength(outcome.stdout),
          bytesErr: Buffer.byteLength(outcome.stderr),
          binaryPath: state.taskBinary ?? '',
          timedOut: outcome.timedOut,
          cancelled: outcome.cancelled
        })
      })
      return { streamId: handle.streamId, error: null }
    }
  )

  /* ---------------------------------------------------------------- docs */
  ipcMain.handle(IPC.listDocs, (): DocEntry[] => listDocs(state.repoRoot ?? ''))
  ipcMain.handle(IPC.readDoc, (_event, id: string): DocContent | null => readDoc(state.repoRoot ?? '', String(id ?? '')))

  /* ----------------------------------------------------------------- mcp */
  ipcMain.handle(IPC.mcpStatus, (): McpServerInfo | null => mcp.status())
  ipcMain.handle(IPC.mcpStart, async (_event, options: McpStartOptions): Promise<McpServerInfo> => {
    const workspace = requireWorkspace(state, options?.workspace)
    const allowWorkspaces = (options?.allowWorkspaces ?? []).map((entry) => requireWorkspace(state, entry))
    if (options?.allowAnyWorkspace && !state.settings.get().allowIntegrationProbes) {
      throw new Error('--allow-any-workspace expands read authority; enable it in Settings first')
    }
    return mcp.start({ workspace, allowWorkspaces, allowAnyWorkspace: Boolean(options?.allowAnyWorkspace) })
  })
  ipcMain.handle(IPC.mcpStop, () => {
    mcp.stop()
    return { stopped: true }
  })
  ipcMain.handle(IPC.mcpCallTool, (_event, tool: string, args: Record<string, unknown>): Promise<McpCallResult> =>
    mcp.callTool(String(tool ?? ''), args && typeof args === 'object' ? args : {})
  )
  ipcMain.handle(IPC.mcpTools, () => mcp.tools())
  ipcMain.handle(IPC.mcpLog, (): McpLogLine[] => [...state.mcpLog])

  /* ------------------------------------------------------------ terminal */
  ipcMain.handle(IPC.launchTerminal, async (_event, command: string) => {
    const value = String(command ?? '').trim()
    const binary = state.binary?.path ?? ''
    if (!binary || !value.startsWith(binary)) {
      return { launched: false, command: value, error: 'only the resolved bob binary may be launched in a terminal' }
    }
    return launchInTerminal(value)
  })

  /* ------------------------------------------------------------ activity */
  ipcMain.handle(IPC.activity, (): ActivityEntry[] => activity.list())
  ipcMain.handle(IPC.clearActivity, () => ({ cleared: activity.clear() }))
  ipcMain.handle(IPC.exportActivity, async (): Promise<{ path: string | null; error: string | null }> => {
    const window = getWindow()
    const options = {
      title: 'Export activity ledger',
      defaultPath: `bob-console-activity-${Date.now()}.json`
    }
    const result = window ? await dialog.showSaveDialog(window, options) : await dialog.showSaveDialog(options)
    if (result.canceled || !result.filePath) return { path: null, error: null }
    try {
      writeJsonAtomic(result.filePath, activity.list())
      return { path: result.filePath, error: null }
    } catch (error) {
      return { path: null, error: error instanceof Error ? error.message : String(error) }
    }
  })
}

export function describeArgv(argv: string[]): string {
  return displayArgv(argv)
}
