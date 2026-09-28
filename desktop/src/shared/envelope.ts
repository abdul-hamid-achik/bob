import type { BobError, Envelope } from './types'
import { EXIT_CODE_MEANING } from './types'

export interface ParsedEnvelope {
  envelope: Envelope<Record<string, unknown>> | null
  parseError: string | null
}

/** Parses one JSON envelope from stdout, tolerating a trailing newline. */
export function parseEnvelope(stdout: string): ParsedEnvelope {
  const text = stdout.trim()
  if (!text) return { envelope: null, parseError: 'empty stdout' }
  try {
    const value = JSON.parse(text) as Envelope<Record<string, unknown>>
    if (typeof value !== 'object' || value === null || !('ok' in value)) {
      return { envelope: null, parseError: 'stdout is not a Bob envelope' }
    }
    return { envelope: value, parseError: null }
  } catch (error) {
    return { envelope: null, parseError: error instanceof Error ? error.message : String(error) }
  }
}

/** Parses newline-delimited JSON-RPC frames, skipping unparsable lines. */
export function parseJsonLines(stdout: string): unknown[] {
  const out: unknown[] = []
  for (const line of stdout.split('\n')) {
    const trimmed = line.trim()
    if (!trimmed) continue
    try {
      out.push(JSON.parse(trimmed))
    } catch {
      /* non-protocol line; diagnostics belong on stderr */
    }
  }
  return out
}

/**
 * Failure payloads appear either as data.error (command refusal) or as
 * data.result alongside data.error (remove reports both). ok=false with no
 * error object still counts as a failure, e.g. check reporting drift.
 */
export function envelopeError(envelope: Envelope<unknown> | null): BobError | null {
  if (!envelope || envelope.ok) return null
  const data = envelope.data as Record<string, unknown> | undefined
  const direct = data?.error as BobError | undefined
  if (direct && typeof direct.message === 'string') return direct
  const nested = (data?.result as Record<string, unknown> | undefined)?.error as BobError | undefined
  if (nested && typeof nested.message === 'string') return nested
  return { code: 'command_failed', message: `${envelope.command} failed` }
}

/** The payload the panel should render, unwrapping data.result when present. */
export function envelopePayload<T>(envelope: Envelope<unknown> | null): T | null {
  if (!envelope) return null
  const data = envelope.data as Record<string, unknown> | undefined
  if (!data) return null
  if (data.result && typeof data.result === 'object') return data.result as T
  return data as T
}

export function exitCodeMeaning(code: number): string {
  return EXIT_CODE_MEANING[code] ?? 'undocumented exit code'
}

export function exitTone(code: number): 'positive' | 'caution' | 'danger' | 'neutral' {
  if (code === 0) return 'positive'
  if (code === 3 || code === 5) return 'caution'
  if (code === 2 || code === 1) return 'danger'
  return 'neutral'
}

export interface NextActionCommand {
  /** The guidance line exactly as Bob emitted it. */
  text: string
  /** argv without the leading `bob`, or null when the guidance is prose. */
  argv: string[] | null
  /** True when the guidance contains a placeholder such as <module>. */
  needsInput: boolean
  /** True when the resolved argv would mutate a workspace. */
  mutates: boolean
}

const MUTATING_LEADERS = new Set(['apply', 'upgrade', 'remove', 'init', 'new', 'config'])
const STOP_WORDS = new Set(['to', 'then', 'and', 'before', 'after', 'for', 'with', 'the', 'a', 'an'])

function tokenizeCommand(rest: string): string[] {
  const tokens: string[] = []
  for (const raw of rest.split(/\s+/)) {
    const token = raw.replace(/[.,;]$/, '')
    if (!token) continue
    if (STOP_WORDS.has(token.toLowerCase()) && !token.startsWith('-')) break
    tokens.push(token)
  }
  return tokens
}

/**
 * Extracts runnable commands from an envelope's next_actions guidance.
 * Bob writes guidance as prose; the `run: bob …` and `rerun bob …` forms are
 * the machine-usable ones, so only those become executable.
 */
export function nextActionCommands(nextActions: string[] | undefined): NextActionCommand[] {
  const out: NextActionCommand[] = []
  for (const text of nextActions ?? []) {
    const runMatch = text.match(/^(?:run|rerun|next):\s*(bob\s+.+)$/i)
    const rerunMatch = runMatch ? null : text.match(/\brerun\s+(bob\s+\S+(?:\s+--\S+)*)/i)
    const body = runMatch?.[1] ?? rerunMatch?.[1]
    if (!body) {
      out.push({ text, argv: null, needsInput: false, mutates: false })
      continue
    }
    const tokens = tokenizeCommand(body.replace(/^bob\s+/i, ''))
    if (tokens.length === 0) {
      out.push({ text, argv: null, needsInput: false, mutates: false })
      continue
    }
    const needsInput = tokens.some((token) => token.includes('<') || token.includes('…'))
    const mutates = MUTATING_LEADERS.has(tokens[0] ?? '')
    out.push({ text, argv: tokens, needsInput, mutates })
  }
  return out
}

/** Splits a `--set key=value` style repeatable flag into typed pairs. */
export function parseKeyValuePairs(values: string[]): Record<string, string> {
  const out: Record<string, string> = {}
  for (const value of values) {
    const index = value.indexOf('=')
    if (index <= 0) continue
    out[value.slice(0, index)] = value.slice(index + 1)
  }
  return out
}
