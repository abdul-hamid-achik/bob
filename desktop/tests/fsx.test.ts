import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { afterAll, beforeAll, describe, expect, it } from 'vitest'
import { listPaths, readFileBounded, safeJoin } from '../src/main/fsx'

let root = ''

beforeAll(() => {
  root = mkdtempSync(join(tmpdir(), 'bob-console-fsx-'))
  mkdirSync(join(root, 'internal/cli'), { recursive: true })
  mkdirSync(join(root, 'node_modules/pkg'), { recursive: true })
  mkdirSync(join(root, '.git'), { recursive: true })
  writeFileSync(join(root, 'bob.yaml'), 'schema_version: 1\nrecipe: ts-app\n', 'utf8')
  writeFileSync(join(root, 'bob.lock'), 'schema_version: 1\n', 'utf8')
  writeFileSync(join(root, 'internal/cli/root.go'), 'package cli\n', 'utf8')
  writeFileSync(join(root, 'node_modules/pkg/index.js'), 'export default 1\n', 'utf8')
  writeFileSync(join(root, '.git/HEAD'), 'ref: refs/heads/main\n', 'utf8')
  writeFileSync(join(root, 'binary.bin'), Buffer.from([0, 1, 2, 3]), 'utf8')
})

afterAll(() => {
  rmSync(root, { recursive: true, force: true })
})

describe('safeJoin', () => {
  it('resolves paths inside the workspace', () => {
    expect(safeJoin(root, 'bob.yaml')).toBe(join(root, 'bob.yaml'))
    expect(safeJoin(root, 'internal/cli/root.go')).toBe(join(root, 'internal/cli/root.go'))
  })

  it('refuses traversal, absolute paths, and NUL bytes', () => {
    expect(safeJoin(root, '../secret')).toBeNull()
    expect(safeJoin(root, 'internal/../../secret')).toBeNull()
    expect(safeJoin(root, '/etc/passwd')).toBeNull()
    expect(safeJoin(root, 'a\0b')).toBeNull()
    expect(safeJoin(root, '')).toBeNull()
  })
})

describe('listPaths', () => {
  it('skips heavy directories and marks lock-owned paths', () => {
    const entries = listPaths(root, [{ path: 'bob.lock', sha256: 'a'.repeat(64) }])
    const paths = entries.map((entry) => entry.path)
    expect(paths).toContain('bob.yaml')
    expect(paths).toContain('internal/cli/root.go')
    expect(paths.some((path) => path.startsWith('node_modules'))).toBe(false)
    expect(paths.some((path) => path.startsWith('.git/'))).toBe(false)

    const lock = entries.find((entry) => entry.path === 'bob.lock')
    expect(lock?.kind).toBe('managed')
    expect(lock?.lockedSha256).toBe('a'.repeat(64))
    expect(entries.find((entry) => entry.path === 'bob.yaml')?.kind).toBe('unmanaged')
  })

  it('reports lock entries that are missing from the working tree', () => {
    const entries = listPaths(root, [{ path: 'gone/file.yml', sha256: 'b'.repeat(64) }])
    const missing = entries.find((entry) => entry.path === 'gone/file.yml')
    expect(missing?.kind).toBe('lock-only')
    expect(missing?.exists).toBe(false)
  })

  it('returns nothing for a missing workspace', () => {
    expect(listPaths(join(root, 'nope'), [])).toEqual([])
  })
})

describe('readFileBounded', () => {
  it('reads text inside the workspace', () => {
    const file = readFileBounded(root, 'bob.yaml')
    expect(file?.content).toContain('recipe: ts-app')
    expect(file?.binary).toBe(false)
    expect(file?.truncated).toBe(false)
  })

  it('flags binary content instead of rendering it', () => {
    const file = readFileBounded(root, 'binary.bin')
    expect(file?.binary).toBe(true)
    expect(file?.content).toBe('')
  })

  it('truncates past the byte cap', () => {
    const file = readFileBounded(root, 'bob.yaml', 4)
    expect(file?.truncated).toBe(true)
    expect(file?.content.length).toBe(4)
  })

  it('refuses traversal and missing files', () => {
    expect(readFileBounded(root, '../package.json')).toBeNull()
    expect(readFileBounded(root, 'nope.txt')).toBeNull()
  })
})
