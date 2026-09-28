import { existsSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'
import { listDocs, readDoc } from '../src/main/docs'
import { MUTATING_TASKS, listTasks } from '../src/main/tasks'
import { detectBobRepoRoot } from '../src/main/binary'

const here = dirname(fileURLToPath(import.meta.url))
const repoRoot = resolve(here, '..', '..')
const hasCheckout = existsSync(join(repoRoot, 'Taskfile.yml')) && existsSync(join(repoRoot, 'cmd', 'bob'))

describe.skipIf(!hasCheckout)('docs against the Bob checkout', () => {
  it('finds the reference pages, guides, and root Markdown', () => {
    const docs = listDocs(repoRoot)
    const ids = docs.map((doc) => doc.id)
    expect(ids).toContain('docs/reference/cli')
    expect(ids).toContain('docs/ownership-and-safety')
    expect(ids).toContain('README')
    expect(docs.every((doc) => doc.title.length > 0)).toBe(true)
    expect(docs.every((doc) => doc.group.length > 0)).toBe(true)
    expect(docs.some((doc) => doc.group === 'Reference')).toBe(true)
  })

  it('never lists build output or dependencies', () => {
    const docs = listDocs(repoRoot)
    expect(docs.some((doc) => doc.relativePath.includes('node_modules'))).toBe(false)
    expect(docs.some((doc) => doc.relativePath.includes('.vitepress'))).toBe(false)
  })

  it('reads a page by id, with or without the docs/ prefix', () => {
    const withPrefix = readDoc(repoRoot, 'docs/reference/cli')
    const withoutPrefix = readDoc(repoRoot, 'reference/cli')
    expect(withPrefix?.markdown).toContain('bob')
    expect(withoutPrefix?.markdown).toBe(withPrefix?.markdown)
  })

  it('refuses traversal and unknown ids', () => {
    expect(readDoc(repoRoot, '../../etc/passwd')).toBeNull()
    expect(readDoc(repoRoot, '/absolute')).toBeNull()
    expect(readDoc(repoRoot, 'does/not/exist')).toBeNull()
    expect(readDoc('', 'README')).toBeNull()
  })
})

describe.skipIf(!hasCheckout)('tasks against the Bob Taskfile', () => {
  it('lists the documented gates with descriptions', () => {
    const tasks = listTasks(repoRoot)
    const names = tasks.map((task) => task.name)
    for (const expected of ['build', 'test', 'race', 'lint', 'specs', 'docs-build', 'ship']) {
      expect(names, `task ${expected} is missing`).toContain(expected)
    }
    expect(tasks.every((task) => task.desc.length > 0)).toBe(true)
    expect(names).not.toContain('default')
  })

  it('flags the tasks that rewrite files or install outside the workspace', () => {
    const tasks = listTasks(repoRoot)
    for (const name of MUTATING_TASKS) {
      const task = tasks.find((entry) => entry.name === name)
      if (task) expect(task.mutates).toBe(true)
    }
    expect(tasks.find((task) => task.name === 'test')?.mutates).toBe(false)
  })

  it('returns nothing without a Taskfile', () => {
    expect(listTasks('/tmp')).toEqual([])
  })
})

describe('detectBobRepoRoot', () => {
  it.skipIf(!hasCheckout)('finds the checkout from a bin/bob path', () => {
    expect(detectBobRepoRoot(join(repoRoot, 'bin', 'bob'))).toBe(repoRoot)
  })

  it.skipIf(!hasCheckout)('falls back to a workspace that looks like the source tree', () => {
    expect(detectBobRepoRoot(null, [repoRoot])).toBe(repoRoot)
  })

  it('returns null when nothing matches', () => {
    expect(detectBobRepoRoot('/tmp/nope/bob', ['/tmp'])).toBeNull()
  })
})
