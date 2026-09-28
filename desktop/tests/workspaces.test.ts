import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { inspectWorkspace, normalizeWorkspaceInput, readWorkspaceFiles, touchRecent } from '../src/main/workspaces'
import { createSettingsStore } from '../src/main/settings'
import { createActivityLog } from '../src/main/activity'
import type { ActivityEntry } from '../src/shared/ipc'
import { DEFAULT_SETTINGS } from '../src/shared/types'

const LOCK = `schema_version: 1
recipe:
    id: ts-app
    version: 2
files:
    - path: .editorconfig
      sha256: b1524faf1235bc57d67601cf1f1bf116f4c50510aaaa
    - path: .github/workflows/ci.yml
      sha256: 93750323417395ff03a26ed0e7e9ed17e3aa9579bbbb
`

const MANIFEST = `schema_version: 1
recipe: ts-app
product:
  name: demo-ts
runtime:
  language: typescript
  kind: app
`

let root = ''

beforeAll(() => {
  root = mkdtempSync(join(tmpdir(), 'bob-console-ws-'))
  writeFileSync(join(root, 'bob.lock'), LOCK, 'utf8')
  writeFileSync(join(root, 'bob.yaml'), MANIFEST, 'utf8')
})

afterAll(() => {
  rmSync(root, { recursive: true, force: true })
})

describe('readWorkspaceFiles', () => {
  it('parses the lock recipe and every entry', () => {
    const files = readWorkspaceFiles(root)
    expect(files.lockRecipe).toEqual({ id: 'ts-app', version: 2 })
    expect(files.lockFiles).toHaveLength(2)
    expect(files.lockFiles[0]?.path).toBe('.editorconfig')
    expect(files.manifest).toContain('recipe: ts-app')
    expect(files.lockError).toBeNull()
    expect(files.manifestError).toBeNull()
  })

  it('falls back to the manifest recipe when there is no lock', () => {
    const dir = mkdtempSync(join(tmpdir(), 'bob-console-nolock-'))
    writeFileSync(join(dir, 'bob.yaml'), MANIFEST, 'utf8')
    const files = readWorkspaceFiles(dir)
    expect(files.lockPath).toBeNull()
    expect(files.lockRecipe).toEqual({ id: 'ts-app', version: 0 })
    rmSync(dir, { recursive: true, force: true })
  })

  it('reports a parse error instead of throwing', () => {
    const dir = mkdtempSync(join(tmpdir(), 'bob-console-bad-'))
    writeFileSync(join(dir, 'bob.lock'), 'recipe: [unbalanced\n', 'utf8')
    const files = readWorkspaceFiles(dir)
    expect(files.lockError).toBeTruthy()
    expect(files.lockFiles).toEqual([])
    rmSync(dir, { recursive: true, force: true })
  })
})

describe('inspectWorkspace', () => {
  it('describes the workspace for the switcher', () => {
    const info = inspectWorkspace(root)
    expect(info.hasManifest).toBe(true)
    expect(info.hasLock).toBe(true)
    expect(info.managedFiles).toBe(2)
    expect(info.recipe?.id).toBe('ts-app')
    expect(info.name).toBeTruthy()
  })
})

describe('normalizeWorkspaceInput', () => {
  it('accepts directories and rejects everything else', () => {
    expect(normalizeWorkspaceInput(root)).toBe(root)
    expect(normalizeWorkspaceInput(join(root, 'missing'))).toBeNull()
    expect(normalizeWorkspaceInput('   ')).toBeNull()
  })
})

describe('touchRecent', () => {
  it('moves a path to the front without duplicating it', () => {
    expect(touchRecent(['/a', '/b'], '/b')).toEqual(['/b', '/a'])
    expect(touchRecent(['/a'], '/c')).toEqual(['/c', '/a'])
  })

  it('caps the list', () => {
    const many = Array.from({ length: 30 }, (_unused, index) => `/ws/${index}`)
    expect(touchRecent(many, '/new', 5)).toHaveLength(5)
  })
})

describe('settings store', () => {
  it('round-trips through disk and sanitizes unknown input', () => {
    const dir = mkdtempSync(join(tmpdir(), 'bob-console-settings-'))
    const store = createSettingsStore(dir)
    expect(store.get()).toEqual(DEFAULT_SETTINGS)

    store.set({ theme: 'light', requireDigestBoundApply: false, recentWorkspaces: ['/a', '/b'] })
    const reloaded = createSettingsStore(dir)
    expect(reloaded.get().theme).toBe('light')
    expect(reloaded.get().requireDigestBoundApply).toBe(false)
    expect(reloaded.get().recentWorkspaces).toEqual(['/a', '/b'])

    writeFileSync(store.file, '{"theme":"neon","recentWorkspaces":[1,2,"/ok"],"unknown":true}', 'utf8')
    const sanitized = createSettingsStore(dir).get()
    expect(sanitized.theme).toBe('dark')
    expect(sanitized.recentWorkspaces).toEqual(['/ok'])
    expect(sanitized.demoMode).toBe(false)

    writeFileSync(store.file, 'not json at all', 'utf8')
    expect(createSettingsStore(dir).get()).toEqual(DEFAULT_SETTINGS)
    rmSync(dir, { recursive: true, force: true })
  })

  it('keeps safety defaults on', () => {
    expect(DEFAULT_SETTINGS.requireDigestBoundApply).toBe(true)
    expect(DEFAULT_SETTINGS.requireMutationConfirmation).toBe(true)
    expect(DEFAULT_SETTINGS.allowIntegrationProbes).toBe(false)
  })
})

describe('activity log', () => {
  function entry(id: string): ActivityEntry {
    return {
      id,
      at: new Date().toISOString(),
      kind: 'bob',
      featureId: 'plan',
      displayCommand: 'bob plan .',
      argv: ['plan', '.'],
      cwd: '/tmp',
      exitCode: 0,
      durationMs: 5,
      ok: true,
      planDigest: null,
      errorCode: null,
      bytesOut: 10,
      bytesErr: 0,
      binaryPath: '/usr/local/bin/bob',
      timedOut: false,
      cancelled: false
    }
  }

  it('records newest first, persists, clears, and exports', () => {
    const dir = mkdtempSync(join(tmpdir(), 'bob-console-activity-'))
    const log = createActivityLog(dir)
    log.record(entry('a'))
    log.record(entry('b'))
    expect(log.list().map((item) => item.id)).toEqual(['b', 'a'])

    const reloaded = createActivityLog(dir)
    expect(reloaded.list()).toHaveLength(2)

    const target = join(dir, 'export.json')
    expect(log.exportTo(target)).toBe(target)

    expect(log.clear()).toBe(2)
    expect(log.list()).toEqual([])
    expect(createActivityLog(dir).list()).toEqual([])
    rmSync(dir, { recursive: true, force: true })
  })

  it('caps the ledger and ignores malformed stored entries', () => {
    const dir = mkdtempSync(join(tmpdir(), 'bob-console-activity-cap-'))
    const log = createActivityLog(dir)
    for (let index = 0; index < 520; index += 1) log.record(entry(`id-${index}`))
    expect(log.list()).toHaveLength(500)
    expect(log.list()[0]?.id).toBe('id-519')

    writeFileSync(log.file, '[{"nonsense":true},null]', 'utf8')
    expect(createActivityLog(dir).list()).toEqual([])
    rmSync(dir, { recursive: true, force: true })
  })
})
