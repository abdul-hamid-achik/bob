import { describe, expect, it } from 'vitest'
import {
  envelopeError,
  envelopePayload,
  exitCodeMeaning,
  exitTone,
  nextActionCommands,
  parseEnvelope,
  parseJsonLines,
  parseKeyValuePairs
} from '../src/shared/envelope'
import type { Envelope } from '../src/shared/types'

describe('parseEnvelope', () => {
  it('parses a success envelope', () => {
    const { envelope, parseError } = parseEnvelope(
      JSON.stringify({ schema_version: 1, ok: true, command: 'version', data: { version: 'v0.11.0' }, warnings: [], next_actions: [] })
    )
    expect(parseError).toBeNull()
    expect(envelope?.command).toBe('version')
    expect(envelope?.ok).toBe(true)
  })

  it('reports a parse error for human output', () => {
    const { envelope, parseError } = parseEnvelope('unchanged  README.md  [in_sync] convergence\n')
    expect(envelope).toBeNull()
    expect(parseError).toBeTruthy()
  })

  it('rejects JSON that is not an envelope', () => {
    const { envelope, parseError } = parseEnvelope('{"hello":"world"}')
    expect(envelope).toBeNull()
    expect(parseError).toBe('stdout is not a Bob envelope')
  })
})

describe('envelopeError', () => {
  it('reads data.error for a refused command', () => {
    const envelope = {
      schema_version: 1,
      ok: false,
      command: 'apply',
      data: { error: { code: 'conflicts', message: 'apply: plan contains conflicts' } },
      warnings: [],
      next_actions: []
    } as Envelope<unknown>
    expect(envelopeError(envelope)).toEqual({ code: 'conflicts', message: 'apply: plan contains conflicts' })
  })

  it('reads data.result.error when both are present, as remove does', () => {
    const envelope = {
      schema_version: 1,
      ok: false,
      command: 'remove',
      data: {
        error: { code: 'conflicts', message: 'remove: some managed files were not removed' },
        result: { removed: ['a'], skipped: ['b'], conflicts: [], lock_removed: false }
      },
      warnings: [],
      next_actions: []
    } as Envelope<unknown>
    expect(envelopeError(envelope)?.code).toBe('conflicts')
    expect(envelopePayload<{ removed: string[] }>(envelope)?.removed).toEqual(['a'])
  })

  it('treats ok:false without an error object as a failure, as check does on drift', () => {
    const envelope = {
      schema_version: 1,
      ok: false,
      command: 'check',
      data: { clean: false, plan: { conflict_count: 1 } },
      warnings: [],
      next_actions: []
    } as unknown as Envelope<unknown>
    expect(envelopeError(envelope)).toEqual({ code: 'command_failed', message: 'check failed' })
  })

  it('returns null for a successful envelope', () => {
    const envelope = { schema_version: 1, ok: true, command: 'plan', data: {}, warnings: [], next_actions: [] } as Envelope<unknown>
    expect(envelopeError(envelope)).toBeNull()
  })
})

describe('exit codes', () => {
  it('documents every exit code Bob can return', () => {
    for (const code of [0, 1, 2, 3, 4, 5]) {
      expect(exitCodeMeaning(code)).toBeTruthy()
    }
    expect(exitCodeMeaning(9)).toBe('undocumented exit code')
  })

  it('separates drift and guard refusals from hard failures', () => {
    expect(exitTone(0)).toBe('positive')
    expect(exitTone(3)).toBe('caution')
    expect(exitTone(5)).toBe('caution')
    expect(exitTone(2)).toBe('danger')
  })
})

describe('nextActionCommands', () => {
  it('turns "run: bob …" guidance into argv', () => {
    const actions = nextActionCommands(['run: bob plan --json', 'review the repository diff'])
    expect(actions[0]?.argv).toEqual(['plan', '--json'])
    expect(actions[0]?.mutates).toBe(false)
    expect(actions[1]?.argv).toBeNull()
  })

  it('flags mutating guidance and placeholders', () => {
    const actions = nextActionCommands([
      'rerun bob remove --force to remove drifted managed files',
      'run: bob init --module <module> --write to create one'
    ])
    expect(actions[0]?.argv).toEqual(['remove', '--force'])
    expect(actions[0]?.mutates).toBe(true)
    expect(actions[1]?.needsInput).toBe(true)
    expect(actions[1]?.mutates).toBe(true)
  })

  it('handles undefined guidance', () => {
    expect(nextActionCommands(undefined)).toEqual([])
  })
})

describe('parseJsonLines', () => {
  it('parses newline-delimited JSON-RPC frames and skips noise', () => {
    const frames = parseJsonLines('{"jsonrpc":"2.0","id":1}\nnot json\n{"jsonrpc":"2.0","id":2}\n')
    expect(frames).toHaveLength(2)
  })
})

describe('parseKeyValuePairs', () => {
  it('splits repeatable --set pairs', () => {
    expect(parseKeyValuePairs(['path=README.md', 'action_code=managed_hash_mismatch', 'bad'])).toEqual({
      path: 'README.md',
      action_code: 'managed_hash_mismatch'
    })
  })
})
