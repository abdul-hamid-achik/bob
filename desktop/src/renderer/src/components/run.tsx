import { useCallback, useEffect, useRef, useState } from 'react'
import { CircleStop, Play, RotateCw } from 'lucide-react'
import { exitCodeMeaning, nextActionCommands } from '@shared/envelope'
import { displayArgv } from '@shared/format'
import type { StreamEvent } from '@shared/ipc'
import type { Envelope, RunResult } from '@shared/types'
import { Badge, Button, Callout, Label, Mono, cx } from './ui'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'

/* ------------------------------------------------------------- run chrome -- */

export function RunButton({
  label = 'Run',
  pending,
  onClick,
  disabled,
  variant = 'primary'
}: {
  label?: string
  pending?: boolean
  onClick: () => void
  disabled?: boolean
  variant?: 'primary' | 'default' | 'danger'
}): JSX.Element {
  return (
    <Button variant={variant} loading={pending} disabled={disabled} onClick={onClick} icon={pending ? undefined : <Play className="size-3.5" aria-hidden />}>
      {label}
    </Button>
  )
}

export function RefreshButton({ pending, onClick, label = 'Refresh' }: { pending?: boolean; onClick: () => void; label?: string }): JSX.Element {
  return <Button variant="quiet" loading={pending} onClick={onClick} icon={<RotateCw className="size-3.5" aria-hidden />}>{label}</Button>
}

export function FailureNotice({
  result,
  error,
  errorCode,
  title = 'Bob refused this run'
}: {
  result: RunResult | null
  error: string | null
  errorCode: string | null
  title?: string
}): JSX.Element | null {
  if (!error) return null
  return (
    <Callout tone="danger" title={`${title}${errorCode ? ` — ${errorCode}` : ''}`}>
      <p className="break-words">{error}</p>
      {result ? (
        <p className="mt-1.5">
          <Mono className="text-clay/80">
            exit {result.exitCode} · {exitCodeMeaning(result.exitCode)}
          </Mono>
        </p>
      ) : null}
      {result?.stderr.trim() ? <pre className="argv mt-1.5 text-clay/80 whitespace-pre-wrap">{result.stderr.trim()}</pre> : null}
    </Callout>
  )
}

export function WarningList({ warnings }: { warnings: string[] | undefined }): JSX.Element | null {
  if (!warnings || warnings.length === 0) return null
  return (
    <ul className="flex flex-col gap-1">
      {warnings.map((warning) => (
        <li key={warning} className="rounded-[6px] border border-brass/40 bg-brass/10 px-2.5 py-1.5 text-[12px] text-brass">
          {warning}
        </li>
      ))}
    </ul>
  )
}

/**
 * Renders Bob's next_actions guidance. Lines that name a command become
 * runnable; mutating ones are marked and still require their own confirmation.
 */
export function NextActionsList({ envelope, onRunCommand }: { envelope: Envelope<unknown> | null; onRunCommand: (argv: string[]) => void }): JSX.Element | null {
  const actions = nextActionCommands(envelope?.next_actions)
  if (actions.length === 0) return null
  return (
    <div className="flex flex-col gap-2">
      <Label>next actions</Label>
      <ul className="flex flex-col gap-1.5">
        {actions.map((action) => (
          <li key={action.text} className="flex items-start gap-2">
            {action.argv ? (
              <Button
                size="sm"
                variant={action.mutates ? 'quiet' : 'default'}
                className="shrink-0"
                onClick={() => action.argv && onRunCommand(action.argv)}
                title={action.mutates ? 'This guidance mutates a workspace' : 'Run this command'}
              >
                {action.mutates ? 'Mutating' : 'Run'}
              </Button>
            ) : (
              <span className="mt-1 size-1.5 shrink-0 rounded-full bg-line-strong" aria-hidden />
            )}
            <div className="min-w-0 flex-1">
              <p className="text-[12px] leading-snug text-ink-muted">{action.text}</p>
              {action.argv ? <Mono className="mt-0.5 block break-all text-ink-dim">bob {displayArgv(action.argv)}</Mono> : null}
              {action.needsInput ? <Badge tone="caution">needs a value</Badge> : null}
            </div>
          </li>
        ))}
      </ul>
    </div>
  )
}

/* ---------------------------------------------------------------- streams -- */

const STREAM_CAP = 400_000

export interface StreamState {
  streamId: string | null
  text: string
  running: boolean
  error: string | null
  start: (featureId: string, argv: string[], cwd: string) => Promise<void>
  stop: () => Promise<void>
  clear: () => void
}

/** Subscribes to main-process stream events for one live command at a time. */
export function useStream(): StreamState {
  const [streamId, setStreamId] = useState<string | null>(null)
  const [text, setText] = useState('')
  const [running, setRunning] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const bufferRef = useRef('')

  useEffect(() => {
    const unsubscribe = bridge().onStreamEvent((event: StreamEvent) => {
      if (event.type === 'exit') {
        setRunning(false)
        setStreamId(null)
        return
      }
      if (event.type === 'stdout' || event.type === 'stderr') {
        bufferRef.current = (bufferRef.current + (event.text ?? '')).slice(-STREAM_CAP)
        setText(bufferRef.current)
      }
    })
    return unsubscribe
  }, [])

  const stop = useCallback(async () => {
    if (streamId) await bridge().stopStream(streamId)
    setRunning(false)
    setStreamId(null)
  }, [streamId])

  const start = useCallback(
    async (featureId: string, argv: string[], cwd: string) => {
      if (running) await stop()
      bufferRef.current = ''
      setText('')
      setError(null)
      const outcome = await bridge().startStream({ featureId, argv, cwd, stream: true })
      if (outcome.error) {
        setError(outcome.error)
        return
      }
      setStreamId(outcome.streamId)
      setRunning(true)
    },
    [running, stop]
  )

  useEffect(() => {
    return () => {
      if (streamId) void bridge().stopStream(streamId)
    }
  }, [streamId])

  return { streamId, text, running, error, start, stop, clear: () => setText('') }
}

export function StreamConsole({
  stream,
  title,
  onStop,
  height = '20rem',
  actions
}: {
  stream: StreamState
  title: string
  onStop?: () => void
  height?: string
  actions?: React.ReactNode
}): JSX.Element {
  const ref = useRef<HTMLPreElement>(null)
  useEffect(() => {
    const node = ref.current
    if (node) node.scrollTop = node.scrollHeight
  }, [stream.text])

  return (
    <div className="overflow-hidden rounded-[8px] border border-line bg-sunken">
      <div className="flex items-center gap-2 border-b border-line bg-panel px-3 py-1.5">
        <span className={cx('size-1.5 rounded-full', stream.running ? 'animate-pulse bg-jade' : 'bg-ink-dim')} aria-hidden />
        <Label>{title}</Label>
        {stream.running ? <Badge tone="positive">streaming</Badge> : null}
        <span className="ml-auto flex items-center gap-2">
          {actions}
          {stream.running ? (
            <Button size="sm" variant="quiet" icon={<CircleStop className="size-3.5" aria-hidden />} onClick={onStop ?? (() => void stream.stop())}>
              Stop
            </Button>
          ) : null}
        </span>
      </div>
      {stream.error ? (
        <p className="border-b border-clay/40 bg-clay/10 px-3 py-1.5 text-[11.5px] text-clay">{stream.error}</p>
      ) : null}
      <pre
        ref={ref}
        className="argv overflow-auto px-3 py-2 text-ink-muted"
        style={{ height }}
        aria-live="polite"
        aria-label={`${title} output`}
      >
        {stream.text || (stream.running ? 'waiting for output…' : 'no output yet')}
      </pre>
    </div>
  )
}

/* ------------------------------------------------------------- run footer -- */

export function RunMeta({ result }: { result: RunResult | null }): JSX.Element | null {
  if (!result) return null
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-[11px] text-ink-dim">
      <Mono className={result.exitCode === 0 ? 'text-jade' : 'text-clay'}>exit {result.exitCode}</Mono>
      <span>{exitCodeMeaning(result.exitCode)}</span>
      <span>{result.durationMs} ms</span>
      <Mono className="max-w-[28rem] truncate" title={result.displayCommand}>
        {result.displayCommand}
      </Mono>
    </div>
  )
}

export function useRunCommand(): (argv: string[]) => Promise<RunResult> {
  const { run, showInspector, workspace } = useStore()
  return useCallback(
    async (argv: string[]) => {
      const result = await run('next-actions', { argv, cwd: workspace })
      showInspector(`bob ${displayArgv(argv)}`, result)
      return result
    },
    [run, showInspector, workspace]
  )
}
