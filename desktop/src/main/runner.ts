import { spawn, type ChildProcessWithoutNullStreams } from 'node:child_process'
import { randomUUID } from 'node:crypto'
import { envelopeError, parseEnvelope } from '../shared/envelope'
import { displayArgv } from '../shared/format'
import { featureById } from '../shared/features'
import { invocationMutates, invocationNeedsAuthority } from '../shared/safety'
import type { ActivityEntry, StreamEvent } from '../shared/ipc'
import type { Envelope, RunRequest, RunResult } from '../shared/types'

const MAX_CAPTURE_BYTES = 8 * 1024 * 1024
const DEFAULT_TIMEOUT_MS = 120_000

export interface ProcOutcome {
  exitCode: number
  stdout: string
  stderr: string
  timedOut: boolean
  cancelled: boolean
  durationMs: number
  truncated: boolean
}

export interface SpawnOptions {
  command: string
  argv: string[]
  cwd: string
  timeoutMs?: number
  env?: NodeJS.ProcessEnv
  signal?: AbortSignal
}

/**
 * Kills the child's whole process group.
 *
 * Bob, task, and git all spawn children of their own; signaling only the direct
 * child leaves grandchildren holding the stdout pipe open, so a timeout or a
 * cancel would appear to hang until they finished on their own.
 */
function killTree(child: { pid?: number; kill: (signal?: NodeJS.Signals) => boolean }, signal: NodeJS.Signals): void {
  const pid = child.pid
  if (pid) {
    try {
      process.kill(-pid, signal)
      return
    } catch {
      /* group already gone or not a leader; fall through */
    }
  }
  try {
    child.kill(signal)
  } catch {
    /* already exited */
  }
}

/** Buffers one child process to completion with a timeout and abort support. */
export function spawnCapture(options: SpawnOptions): Promise<ProcOutcome> {
  const startedAt = Date.now()
  return new Promise((promiseResolve, promiseReject) => {
    let child: ChildProcessWithoutNullStreams
    try {
      child = spawn(options.command, options.argv, {
        cwd: options.cwd,
        env: options.env ?? process.env,
        windowsHide: true,
        detached: true
      }) as ChildProcessWithoutNullStreams
    } catch (error) {
      promiseReject(error instanceof Error ? error : new Error(String(error)))
      return
    }

    let stdout = ''
    let stderr = ''
    let truncated = false
    let timedOut = false
    let cancelled = false
    let settled = false

    const timer = options.timeoutMs
      ? setTimeout(() => {
          timedOut = true
          killTree(child, 'SIGTERM')
          setTimeout(() => killTree(child, 'SIGKILL'), 2000).unref()
        }, options.timeoutMs)
      : null

    const onAbort = (): void => {
      cancelled = true
      killTree(child, 'SIGTERM')
      setTimeout(() => killTree(child, 'SIGKILL'), 2000).unref()
    }
    options.signal?.addEventListener('abort', onAbort, { once: true })

    const append = (target: 'stdout' | 'stderr', chunk: Buffer): void => {
      if (truncated) return
      const text = chunk.toString('utf8')
      if (target === 'stdout') {
        if (stdout.length + text.length > MAX_CAPTURE_BYTES) {
          stdout += text.slice(0, MAX_CAPTURE_BYTES - stdout.length)
          truncated = true
          return
        }
        stdout += text
      } else {
        if (stderr.length + text.length > MAX_CAPTURE_BYTES) {
          stderr += text.slice(0, MAX_CAPTURE_BYTES - stderr.length)
          truncated = true
          return
        }
        stderr += text
      }
    }

    child.stdout.on('data', (chunk: Buffer) => append('stdout', chunk))
    child.stderr.on('data', (chunk: Buffer) => append('stderr', chunk))

    const finish = (code: number | null): void => {
      if (settled) return
      settled = true
      if (timer) clearTimeout(timer)
      options.signal?.removeEventListener('abort', onAbort)
      promiseResolve({
        exitCode: code ?? (cancelled ? -1 : 1),
        stdout,
        stderr,
        timedOut,
        cancelled,
        durationMs: Date.now() - startedAt,
        truncated
      })
    }

    child.on('error', (error) => {
      if (settled) return
      settled = true
      if (timer) clearTimeout(timer)
      options.signal?.removeEventListener('abort', onAbort)
      promiseReject(error)
    })
    child.on('close', (code) => finish(code))
  })
}

export interface StreamHandle {
  streamId: string
  requestId: string
  stop(): void
  done: Promise<ProcOutcome>
}

/** Spawns a long-running process and forwards chunks to a listener. */
export function spawnStream(
  options: SpawnOptions,
  onEvent: (event: { type: 'stdout' | 'stderr'; text: string }) => void
): StreamHandle {
  const streamId = randomUUID()
  const startedAt = Date.now()
  const child = spawn(options.command, options.argv, {
    cwd: options.cwd,
    env: options.env ?? process.env,
    windowsHide: true,
    detached: true
  }) as ChildProcessWithoutNullStreams

  let stdout = ''
  let stderr = ''
  let cancelled = false
  let timedOut = false

  const done = new Promise<ProcOutcome>((promiseResolve) => {
    const finish = (code: number | null): void => {
      promiseResolve({
        exitCode: code ?? (cancelled ? -1 : 1),
        stdout,
        stderr,
        timedOut,
        cancelled,
        durationMs: Date.now() - startedAt,
        truncated: false
      })
    }
    child.on('close', (code) => finish(code))
    child.on('error', () => finish(null))
  })

  const timer = options.timeoutMs
    ? setTimeout(() => {
        timedOut = true
        killTree(child, 'SIGTERM')
      }, options.timeoutMs)
    : null

  child.stdout.setEncoding('utf8')
  child.stderr.setEncoding('utf8')
  child.stdout.on('data', (text: string) => {
    stdout += text
    onEvent({ type: 'stdout', text })
  })
  child.stderr.on('data', (text: string) => {
    stderr += text
    onEvent({ type: 'stderr', text })
  })

  return {
    streamId,
    requestId: streamId,
    stop() {
      cancelled = true
      if (timer) clearTimeout(timer)
      killTree(child, 'SIGTERM')
      setTimeout(() => killTree(child, 'SIGKILL'), 2000).unref()
    },
    done
  }
}

function planDigestOf(envelope: Envelope<unknown> | null): string | null {
  if (!envelope) return null
  const data = envelope.data as Record<string, unknown> | undefined
  const payload = (data?.result as Record<string, unknown> | undefined) ?? data
  if (!payload) return null
  for (const key of ['plan_digest', 'applied_plan_digest', 'context_digest'] as const) {
    const value = payload[key]
    if (typeof value === 'string') return value
  }
  const nested = payload.plan as Record<string, unknown> | undefined
  if (nested && typeof nested.plan_digest === 'string') return nested.plan_digest
  return null
}

export interface BobRunnerOptions {
  binaryPath: string
  onActivity?: (entry: ActivityEntry) => void
  onStreamEvent?: (event: StreamEvent) => void
}

export interface RefusedRun {
  refused: true
  reason: string
}

/**
 * Runs Bob for the console.
 *
 * The binary always comes from the resolver, never from the renderer, and
 * mutating or authority-gated features are refused unless the caller carries an
 * explicit confirmation from the operator.
 */
export class BobRunner {
  private readonly inflight = new Map<string, AbortController>()
  private readonly streams = new Map<string, StreamHandle>()

  constructor(private readonly options: BobRunnerOptions) {}

  setBinaryPath(path: string): void {
    this.options.binaryPath = path
  }

  /** Routes stream chunks to the renderer; set once during IPC registration. */
  setStreamSink(sink: (event: StreamEvent) => void): void {
    this.options.onStreamEvent = sink
  }

  /** Pre-flight refusal check shared by run and startStream. */
  guard(request: RunRequest): RefusedRun | null {
    const feature = featureById(request.featureId)
    if (!feature) return { refused: true, reason: `unknown feature id: ${request.featureId}` }
    if (!this.options.binaryPath) return { refused: true, reason: 'no bob binary resolved; set one in Settings' }
    if (invocationMutates(feature, request.argv) && !request.confirmed) {
      return { refused: true, reason: `${feature.command} mutates the workspace and was not confirmed` }
    }
    if (invocationNeedsAuthority(feature, request.argv) && !request.confirmed) {
      return { refused: true, reason: `${feature.command} needs explicit subprocess authority` }
    }
    return null
  }

  private refusedResult(request: RunRequest, reason: string): RunResult {
    return {
      requestId: randomUUID(),
      featureId: request.featureId,
      argv: request.argv,
      displayCommand: `bob ${displayArgv(request.argv)}`,
      cwd: request.cwd,
      exitCode: -1,
      durationMs: 0,
      stdout: '',
      stderr: reason,
      envelope: null,
      parseError: reason,
      binaryPath: this.options.binaryPath,
      startedAt: new Date().toISOString(),
      timedOut: false,
      cancelled: false
    }
  }

  async run(request: RunRequest): Promise<RunResult> {
    const refusal = this.guard(request)
    if (refusal) return this.refusedResult(request, refusal.reason)

    const requestId = randomUUID()
    const controller = new AbortController()
    this.inflight.set(requestId, controller)
    const startedAt = new Date().toISOString()

    try {
      const outcome = await spawnCapture({
        command: this.options.binaryPath,
        argv: request.argv,
        cwd: request.cwd,
        timeoutMs: request.timeoutMs ?? DEFAULT_TIMEOUT_MS,
        signal: controller.signal
      })
      const { envelope, parseError } = request.argv.includes('--json')
        ? parseEnvelope(outcome.stdout)
        : { envelope: null, parseError: null }
      const error = envelopeError(envelope)
      const result: RunResult = {
        requestId,
        featureId: request.featureId,
        argv: request.argv,
        displayCommand: `bob ${displayArgv(request.argv)}`,
        cwd: request.cwd,
        exitCode: outcome.exitCode,
        durationMs: outcome.durationMs,
        stdout: outcome.stdout,
        stderr: outcome.stderr,
        envelope,
        parseError,
        binaryPath: this.options.binaryPath,
        startedAt,
        timedOut: outcome.timedOut,
        cancelled: outcome.cancelled
      }
      this.options.onActivity?.({
        id: requestId,
        at: startedAt,
        kind: 'bob',
        featureId: request.featureId,
        displayCommand: result.displayCommand,
        argv: request.argv,
        cwd: request.cwd,
        exitCode: outcome.exitCode,
        durationMs: outcome.durationMs,
        ok: outcome.exitCode === 0 && (envelope ? envelope.ok : true),
        planDigest: planDigestOf(envelope),
        errorCode: error?.code ?? null,
        bytesOut: Buffer.byteLength(outcome.stdout),
        bytesErr: Buffer.byteLength(outcome.stderr),
        binaryPath: this.options.binaryPath,
        timedOut: outcome.timedOut,
        cancelled: outcome.cancelled
      })
      return result
    } finally {
      this.inflight.delete(requestId)
    }
  }

  startStream(request: RunRequest): { streamId: string; error: string | null } {
    const refusal = this.guard(request)
    if (refusal) return { streamId: '', error: refusal.reason }

    const requestId = randomUUID()
    const startedAt = new Date().toISOString()
    const handle = spawnStream(
      {
        command: this.options.binaryPath,
        argv: request.argv,
        cwd: request.cwd,
        timeoutMs: request.timeoutMs ?? 0
      },
      (event) => {
        this.options.onStreamEvent?.({ streamId: handle.streamId, requestId, type: event.type, text: event.text })
      }
    )
    this.streams.set(handle.streamId, handle)
    void handle.done.then((outcome) => {
      this.streams.delete(handle.streamId)
      this.options.onStreamEvent?.({ streamId: handle.streamId, requestId, type: 'exit' })
      this.options.onActivity?.({
        id: requestId,
        at: startedAt,
        kind: 'bob',
        featureId: request.featureId,
        displayCommand: `bob ${displayArgv(request.argv)}`,
        argv: request.argv,
        cwd: request.cwd,
        exitCode: outcome.exitCode,
        durationMs: outcome.durationMs,
        ok: outcome.exitCode === 0,
        planDigest: null,
        errorCode: null,
        bytesOut: Buffer.byteLength(outcome.stdout),
        bytesErr: Buffer.byteLength(outcome.stderr),
        binaryPath: this.options.binaryPath,
        timedOut: outcome.timedOut,
        cancelled: outcome.cancelled
      })
    })
    return { streamId: handle.streamId, error: null }
  }

  stopStream(streamId: string): boolean {
    const handle = this.streams.get(streamId)
    if (!handle) return false
    handle.stop()
    this.streams.delete(streamId)
    return true
  }

  cancel(requestId: string): boolean {
    const controller = this.inflight.get(requestId)
    if (!controller) return false
    controller.abort()
    return true
  }

  stopAll(): void {
    for (const controller of this.inflight.values()) controller.abort()
    for (const handle of this.streams.values()) handle.stop()
    this.inflight.clear()
    this.streams.clear()
  }
}
