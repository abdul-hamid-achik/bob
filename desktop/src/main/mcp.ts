import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process'
import { existsSync, statSync } from 'node:fs'
import type { McpCallResult, McpStartOptions } from '../shared/ipc'
import type { McpServerInfo, McpTool } from '../shared/types'

const REQUEST_TIMEOUT_MS = 20_000
const START_TIMEOUT_MS = 15_000

interface PendingRequest {
  resolve: (value: unknown) => void
  reject: (reason: Error) => void
  timer: NodeJS.Timeout
  sentAt: number
  method: string
  params: unknown
}

interface JsonRpcResponse {
  jsonrpc: string
  id?: number
  result?: unknown
  error?: { code: number; message: string }
}

export interface McpClientHooks {
  onLog?: (line: string, stream: 'stdout' | 'stderr') => void
  onStateChange?: (info: McpServerInfo | null) => void
}

/**
 * Minimal newline-delimited JSON-RPC client for `bob mcp serve`.
 *
 * stdout is protocol-only; stderr is diagnostics. The server's workspace
 * allowlist is fixed at startup, exactly as Bob documents it.
 */
export class McpClient {
  private child: ChildProcessWithoutNullStreams | null = null
  private buffer = ''
  private nextId = 1
  private readonly pending = new Map<number, PendingRequest>()
  private info: McpServerInfo | null = null
  private options: McpStartOptions | null = null

  constructor(
    private binaryPath: string,
    private readonly hooks: McpClientHooks = {}
  ) {}

  status(): McpServerInfo | null {
    return this.info
  }

  setBinaryPath(path: string): void {
    this.binaryPath = path
  }

  tools(): McpTool[] {
    return this.info?.tools ?? []
  }

  private emitState(): void {
    this.hooks.onStateChange?.(this.info ? { ...this.info, tools: [...this.info.tools] } : null)
  }

  private send(payload: Record<string, unknown>): void {
    if (!this.child || this.child.stdin.destroyed) throw new Error('MCP server is not running')
    this.child.stdin.write(`${JSON.stringify(payload)}\n`)
  }

  private request(method: string, params: unknown, timeoutMs = REQUEST_TIMEOUT_MS): Promise<unknown> {
    const id = this.nextId++
    return new Promise((promiseResolve, promiseReject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id)
        promiseReject(new Error(`${method} timed out after ${timeoutMs} ms`))
      }, timeoutMs)
      this.pending.set(id, { resolve: promiseResolve, reject: promiseReject, timer, sentAt: Date.now(), method, params })
      try {
        this.send({ jsonrpc: '2.0', id, method, params })
      } catch (error) {
        clearTimeout(timer)
        this.pending.delete(id)
        promiseReject(error instanceof Error ? error : new Error(String(error)))
      }
    })
  }

  private handleLine(line: string): void {
    const trimmed = line.trim()
    if (!trimmed) return
    let message: JsonRpcResponse
    try {
      message = JSON.parse(trimmed) as JsonRpcResponse
    } catch {
      this.hooks.onLog?.(trimmed, 'stdout')
      return
    }
    if (typeof message.id !== 'number') return
    const pending = this.pending.get(message.id)
    if (!pending) return
    this.pending.delete(message.id)
    clearTimeout(pending.timer)
    if (message.error) pending.reject(new Error(message.error.message))
    else pending.resolve(message.result)
  }

  async start(options: McpStartOptions): Promise<McpServerInfo> {
    if (this.child) this.stop()
    if (!this.binaryPath || !existsSync(this.binaryPath)) {
      throw new Error('no bob binary resolved; set one in Settings')
    }
    if (!existsSync(options.workspace) || !statSync(options.workspace).isDirectory()) {
      throw new Error(`workspace does not exist: ${options.workspace}`)
    }

    this.options = options
    const argv = ['mcp', 'serve', '--workspace', options.workspace]
    for (const extra of options.allowWorkspaces ?? []) argv.push('--allow-workspace', extra)
    if (options.allowAnyWorkspace) argv.push('--allow-any-workspace')

    const child = spawn(this.binaryPath, argv, { cwd: options.workspace, windowsHide: true }) as ChildProcessWithoutNullStreams
    this.child = child
    this.buffer = ''

    child.stdout.setEncoding('utf8')
    child.stderr.setEncoding('utf8')
    child.stdout.on('data', (chunk: string) => {
      this.buffer += chunk
      let index = this.buffer.indexOf('\n')
      while (index !== -1) {
        const line = this.buffer.slice(0, index)
        this.buffer = this.buffer.slice(index + 1)
        this.handleLine(line)
        index = this.buffer.indexOf('\n')
      }
    })
    child.stderr.on('data', (chunk: string) => {
      for (const line of chunk.split('\n')) if (line.trim()) this.hooks.onLog?.(line, 'stderr')
    })

    const exited = new Promise<never>((_resolve, promiseReject) => {
      child.on('exit', (code) => promiseReject(new Error(`bob mcp serve exited with code ${code}`)))
    })

    try {
      const initialize = (await Promise.race([
        this.request(
          'initialize',
          {
            protocolVersion: '2024-11-05',
            capabilities: {},
            clientInfo: { name: 'bob-console', version: '0.1.0' }
          },
          START_TIMEOUT_MS
        ),
        exited
      ])) as { instructions?: string }

      this.send({ jsonrpc: '2.0', method: 'notifications/initialized' })

      const listed = (await Promise.race([this.request('tools/list', {}, START_TIMEOUT_MS), exited])) as {
        tools?: McpTool[]
      }

      this.info = {
        started: true,
        workspace: options.workspace,
        allowAnyWorkspace: Boolean(options.allowAnyWorkspace),
        allowWorkspaces: options.allowWorkspaces ?? [],
        pid: child.pid,
        instructions: initialize?.instructions ?? '',
        tools: listed?.tools ?? [],
        error: undefined
      }
      child.on('exit', (code) => {
        this.info = this.info ? { ...this.info, started: false, error: `server exited with code ${code}` } : null
        this.emitState()
      })
      this.emitState()
      return { ...this.info, tools: [...this.info.tools] }
    } catch (error) {
      this.stop()
      const message = error instanceof Error ? error.message : String(error)
      this.info = {
        started: false,
        workspace: options.workspace,
        allowAnyWorkspace: Boolean(options.allowAnyWorkspace),
        allowWorkspaces: options.allowWorkspaces ?? [],
        instructions: '',
        tools: [],
        error: message
      }
      this.emitState()
      throw new Error(message)
    }
  }

  async callTool(tool: string, args: Record<string, unknown>): Promise<McpCallResult> {
    if (!this.child || !this.info?.started) {
      return {
        ok: false,
        tool,
        durationMs: 0,
        content: [],
        structured: null,
        isError: true,
        error: 'MCP server is not running',
        request: { method: 'tools/call', params: { name: tool, arguments: args } },
        rawResponse: null
      }
    }
    const startedAt = Date.now()
    const params = { name: tool, arguments: args }
    try {
      const result = (await this.request('tools/call', params)) as {
        content?: { type: string; text?: string }[]
        structuredContent?: unknown
        isError?: boolean
      }
      return {
        ok: !result?.isError,
        tool,
        durationMs: Date.now() - startedAt,
        content: result?.content ?? [],
        structured: result?.structuredContent ?? null,
        isError: Boolean(result?.isError),
        error: null,
        request: { method: 'tools/call', params },
        rawResponse: JSON.stringify(result ?? null, null, 2)
      }
    } catch (error) {
      return {
        ok: false,
        tool,
        durationMs: Date.now() - startedAt,
        content: [],
        structured: null,
        isError: true,
        error: error instanceof Error ? error.message : String(error),
        request: { method: 'tools/call', params },
        rawResponse: null
      }
    }
  }

  stop(): void {
    for (const pending of this.pending.values()) {
      clearTimeout(pending.timer)
      pending.reject(new Error('MCP server stopped'))
    }
    this.pending.clear()
    const child = this.child
    this.child = null
    if (child) {
      try {
        child.stdin.end()
      } catch {
        /* already closed */
      }
      child.kill('SIGTERM')
      setTimeout(() => child.kill('SIGKILL'), 2000).unref()
    }
    const workspace = this.options?.workspace ?? ''
    this.info = this.info ? { ...this.info, started: false, pid: undefined } : null
    if (workspace) this.emitState()
  }

  get argv(): string[] {
    const options = this.options
    if (!options) return []
    const argv = [this.binaryPath, 'mcp', 'serve', '--workspace', options.workspace]
    for (const extra of options.allowWorkspaces ?? []) argv.push('--allow-workspace', extra)
    if (options.allowAnyWorkspace) argv.push('--allow-any-workspace')
    return argv
  }
}
