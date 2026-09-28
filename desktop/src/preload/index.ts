import { contextBridge, ipcRenderer } from 'electron'
import { IPC } from '../shared/ipc'
import type { AppCommand, BobApi, StreamEvent } from '../shared/ipc'

function subscribe<T>(channel: string, listener: (payload: T) => void): () => void {
  const handler = (_event: Electron.IpcRendererEvent, payload: T): void => listener(payload)
  ipcRenderer.on(channel, handler)
  return () => ipcRenderer.removeListener(channel, handler)
}

const api: BobApi = {
  appInfo: () => ipcRenderer.invoke(IPC.appInfo),
  resolveBinary: () => ipcRenderer.invoke(IPC.resolveBinary),
  getSettings: () => ipcRenderer.invoke(IPC.getSettings),
  setSettings: (patch) => ipcRenderer.invoke(IPC.setSettings, patch),

  listWorkspaces: () => ipcRenderer.invoke(IPC.listWorkspaces),
  addWorkspace: (path) => ipcRenderer.invoke(IPC.addWorkspace, path),
  forgetWorkspace: (path) => ipcRenderer.invoke(IPC.forgetWorkspace, path),
  chooseDirectory: () => ipcRenderer.invoke(IPC.chooseDirectory),
  readWorkspace: (path) => ipcRenderer.invoke(IPC.readWorkspace, path),
  writeManifest: (path, content) => ipcRenderer.invoke(IPC.writeManifest, path, content),

  run: (request) => ipcRenderer.invoke(IPC.runBob, request),
  cancel: (requestId) => ipcRenderer.invoke(IPC.cancelRun, requestId),
  startStream: (request) => ipcRenderer.invoke(IPC.startStream, request),
  stopStream: (streamId) => ipcRenderer.invoke(IPC.stopStream, streamId),
  onStreamEvent: (listener) => subscribe<StreamEvent>(IPC.streamEvent, listener),

  listPaths: (workspace) => ipcRenderer.invoke(IPC.listPaths, workspace),
  readFile: (workspace, relativePath) => ipcRenderer.invoke(IPC.readFile, workspace, relativePath),
  openExternal: (target, mode) => ipcRenderer.invoke(IPC.openExternal, target, mode),

  gitStatus: (cwd) => ipcRenderer.invoke(IPC.gitStatus, cwd),
  gitLog: (cwd, limit) => ipcRenderer.invoke(IPC.gitLog, cwd, limit),
  gitDiff: (cwd, relativePath) => ipcRenderer.invoke(IPC.gitDiff, cwd, relativePath),

  listTasks: (cwd) => ipcRenderer.invoke(IPC.listTasks, cwd),
  runTask: (name, cwd, confirmed) => ipcRenderer.invoke(IPC.runTask, name, cwd, confirmed),
  taskBinary: () => ipcRenderer.invoke(IPC.taskBinary),
  launchTerminal: (command) => ipcRenderer.invoke(IPC.launchTerminal, command),

  listDocs: () => ipcRenderer.invoke(IPC.listDocs),
  readDoc: (id) => ipcRenderer.invoke(IPC.readDoc, id),

  mcpStatus: () => ipcRenderer.invoke(IPC.mcpStatus),
  mcpStart: (options) => ipcRenderer.invoke(IPC.mcpStart, options),
  mcpStop: () => ipcRenderer.invoke(IPC.mcpStop),
  mcpCallTool: (tool, args) => ipcRenderer.invoke(IPC.mcpCallTool, tool, args),
  mcpTools: () => ipcRenderer.invoke(IPC.mcpTools),
  mcpLog: () => ipcRenderer.invoke(IPC.mcpLog),
  repoRoot: () => ipcRenderer.invoke(IPC.repoRoot),

  activity: () => ipcRenderer.invoke(IPC.activity),
  clearActivity: () => ipcRenderer.invoke(IPC.clearActivity),
  exportActivity: () => ipcRenderer.invoke(IPC.exportActivity),

  onCommand: (listener) => subscribe<AppCommand>(IPC.appCommand, listener)
}

contextBridge.exposeInMainWorld('bob', api)
