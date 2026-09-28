import { chmodSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { BobRunner, spawnCapture } from '../src/main/runner'
import type { ActivityEntry } from '../src/shared/ipc'

const ENVELOPE = JSON.stringify({
  schema_version: 1,
  ok: true,
  command: 'version',
  data: { name: 'bob', version: 'v9.9.9', commit: 'abc1234', date: '2026-01-01T00:00:00Z' },
  warnings: [],
  next_actions: []
})

let dir = ''
let fakeBob = ''

function writeFakeBob(script: string): void {
  writeFileSync(fakeBob, script, 'utf8')
  chmodSync(fakeBob, 0o755)
}

beforeAll(() => {
  dir = mkdtempSync(join(tmpdir(), 'bob-console-runner-'))
  fakeBob = join(dir, 'bob')
})

afterAll(() => {
  rmSync(dir, { recursive: true, force: true })
})

describe('spawnCapture', () => {
  it('captures stdout, stderr, and the exit code', async () => {
    writeFakeBob('#!/bin/sh\necho out\necho err 1>&2\nexit 3\n')
    const outcome = await spawnCapture({ command: fakeBob, argv: [], cwd: dir })
    expect(outcome.stdout.trim()).toBe('out')
    expect(outcome.stderr.trim()).toBe('err')
    expect(outcome.exitCode).toBe(3)
    expect(outcome.timedOut).toBe(false)
  })

  it('enforces a timeout', async () => {
    writeFakeBob('#!/bin/sh\nsleep 5\n')
    const outcome = await spawnCapture({ command: fakeBob, argv: [], cwd: dir, timeoutMs: 300 })
    expect(outcome.timedOut).toBe(true)
    expect(outcome.durationMs).toBeLessThan(4000)
  })

  it('stops on abort', async () => {
    writeFakeBob('#!/bin/sh\nsleep 5\n')
    const controller = new AbortController()
    const promise = spawnCapture({ command: fakeBob, argv: [], cwd: dir, signal: controller.signal })
    setTimeout(() => controller.abort(), 150)
    const outcome = await promise
    expect(outcome.cancelled).toBe(true)
  })
})

describe('BobRunner guards', () => {
  const activity: ActivityEntry[] = []

  function makeRunner(binaryPath = fakeBob): BobRunner {
    return new BobRunner({ binaryPath, onActivity: (entry) => activity.push(entry) })
  }

  it('refuses an unknown feature id without spawning', () => {
    const refusal = makeRunner().guard({ featureId: 'nope', argv: ['plan'], cwd: dir })
    expect(refusal?.reason).toMatch(/unknown feature/)
  })

  it('refuses when no binary is resolved', () => {
    const refusal = makeRunner('').guard({ featureId: 'plan', argv: ['plan'], cwd: dir })
    expect(refusal?.reason).toMatch(/no bob binary/)
  })

  it('refuses an unconfirmed apply', () => {
    const refusal = makeRunner().guard({ featureId: 'apply', argv: ['apply', dir, '--json'], cwd: dir })
    expect(refusal?.reason).toMatch(/not confirmed/)
  })

  it('allows a confirmed apply', () => {
    expect(makeRunner().guard({ featureId: 'apply', argv: ['apply', dir, '--json'], cwd: dir, confirmed: true })).toBeNull()
  })

  it('allows previews that carry no write flag', () => {
    expect(makeRunner().guard({ featureId: 'init', argv: ['init', dir, '--json'], cwd: dir })).toBeNull()
    expect(makeRunner().guard({ featureId: 'remove', argv: ['remove', dir, '--dry-run', '--json'], cwd: dir })).toBeNull()
  })

  it('refuses the specialist probe without explicit authority', () => {
    const refusal = makeRunner().guard({
      featureId: 'inspect-probe',
      argv: ['inspect', dir, '--probe-integrations', '--json'],
      cwd: dir
    })
    expect(refusal?.reason).toMatch(/explicit subprocess authority/)
  })
})

describe('BobRunner runs', () => {
  it('parses the JSON envelope and records the activity entry', async () => {
    const dir2 = mkdtempSync(join(tmpdir(), 'bob-console-run-'))
    const binary = join(dir2, 'bob')
    writeFileSync(binary, `#!/bin/sh\nprintf '%s' '${ENVELOPE}'\n`, 'utf8')
    chmodSync(binary, 0o755)

    const entries: ActivityEntry[] = []
    const runner = new BobRunner({ binaryPath: binary, onActivity: (entry) => entries.push(entry) })
    const result = await runner.run({ featureId: 'version', argv: ['version', '--json'], cwd: dir2 })

    expect(result.exitCode).toBe(0)
    expect(result.envelope?.ok).toBe(true)
    expect(result.envelope?.command).toBe('version')
    expect(result.displayCommand).toBe('bob version --json')
    expect(entries).toHaveLength(1)
    expect(entries[0]?.featureId).toBe('version')
    expect(entries[0]?.ok).toBe(true)
    expect(entries[0]?.bytesOut).toBeGreaterThan(0)
    rmSync(dir2, { recursive: true, force: true })
  })

  it('returns a refusal result instead of spawning when unconfirmed', async () => {
    const runner = new BobRunner({ binaryPath: fakeBob })
    const result = await runner.run({ featureId: 'apply', argv: ['apply', dir, '--json'], cwd: dir })
    expect(result.exitCode).toBe(-1)
    expect(result.envelope).toBeNull()
    expect(result.parseError).toMatch(/not confirmed/)
  })

  it('reports a failure envelope and its error code in the ledger', async () => {
    const dir3 = mkdtempSync(join(tmpdir(), 'bob-console-fail-'))
    const binary = join(dir3, 'bob')
    writeFileSync(
      binary,
      `#!/bin/sh\nprintf '%s' '{"schema_version":1,"ok":false,"command":"apply","data":{"error":{"code":"conflicts","message":"apply: plan contains conflicts"}},"warnings":[],"next_actions":[]}'\nexit 2\n`,
      'utf8'
    )
    chmodSync(binary, 0o755)

    const entries: ActivityEntry[] = []
    const runner = new BobRunner({ binaryPath: binary, onActivity: (entry) => entries.push(entry) })
    const result = await runner.run({ featureId: 'apply', argv: ['apply', dir3, '--json'], cwd: dir3, confirmed: true })

    expect(result.exitCode).toBe(2)
    expect(result.envelope?.ok).toBe(false)
    expect(entries[0]?.errorCode).toBe('conflicts')
    expect(entries[0]?.ok).toBe(false)
    rmSync(dir3, { recursive: true, force: true })
  })
})
