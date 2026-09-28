import { describe, expect, it } from 'vitest'
import { diffLines, parseUnifiedDiff, summarizeDiff } from '../src/shared/diff'
import { parseWatchBuffer, parseWatchLine } from '../src/shared/parseWatch'

describe('parseUnifiedDiff', () => {
  const diff = `--- a/.github/dependabot.yml
+++ b/.github/dependabot.yml
@@ -1,4 +1,5 @@
 version: 2
 updates:
-  - package-ecosystem: gomod
+  - package-ecosystem: npm
+    directory: /
`

  it('classifies meta, hunk, context, add, and remove lines', () => {
    const lines = parseUnifiedDiff(diff)
    expect(lines.filter((line) => line.type === 'meta')).toHaveLength(2)
    expect(lines.filter((line) => line.type === 'hunk')).toHaveLength(1)
    expect(lines.filter((line) => line.type === 'context')).toHaveLength(2)
    expect(lines.filter((line) => line.type === 'remove')).toHaveLength(1)
    expect(lines.filter((line) => line.type === 'add')).toHaveLength(2)
  })

  it('numbers both sides from the hunk header', () => {
    const lines = parseUnifiedDiff(diff)
    const removed = lines.find((line) => line.type === 'remove')
    expect(removed?.oldNo).toBe(3)
    expect(removed?.newNo).toBeNull()
    expect(summarizeDiff(lines)).toEqual({ added: 2, removed: 1, context: 2 })
  })
})

describe('diffLines', () => {
  it('aligns unchanged context and isolates the change', () => {
    const lines = diffLines('a\nb\nc', 'a\nB\nc')
    expect(lines.map((line) => `${line.type}:${line.text}`)).toEqual(['context:a', 'remove:b', 'add:B', 'context:c'])
  })

  it('handles pure additions and deletions', () => {
    expect(summarizeDiff(diffLines('', 'x\ny'))).toEqual({ added: 2, removed: 1, context: 0 })
    expect(diffLines('x\ny\nz', 'y').filter((line) => line.type === 'context').map((line) => line.text)).toEqual(['y'])
  })

  it('reports identical bodies as all context', () => {
    const lines = diffLines('same\ntext', 'same\ntext')
    expect(lines.every((line) => line.type === 'context')).toBe(true)
  })
})

describe('parseWatchLine', () => {
  it('reads the header timestamp', () => {
    const parsed = parseWatchLine('bob watch: 2026-09-28 06:19:34 — watching bob.yaml (Ctrl+C to stop)')
    expect(parsed).toEqual({ type: 'header', timestamp: '2026-09-28 06:19:34' })
  })

  it('reads an action row', () => {
    const parsed = parseWatchLine('conflict   .github/dependabot.yml                   [managed_hash_mismatch] contract_drift')
    expect(parsed).toEqual({
      type: 'row',
      row: { kind: 'conflict', path: '.github/dependabot.yml', code: 'managed_hash_mismatch', family: 'contract_drift' }
    })
  })

  it('reads the tally footer', () => {
    const parsed = parseWatchLine('0 create, 0 update, 0 adopt, 29 unchanged, 1 conflict')
    expect(parsed.type).toBe('tally')
    if (parsed.type === 'tally') expect(parsed.tally).toEqual({ create: 0, update: 0, adopt: 0, unchanged: 29, conflict: 1 })
  })

  it('ignores separators and unknown lines', () => {
    expect(parseWatchLine('─────────────────────').type).toBe('other')
    expect(parseWatchLine('').type).toBe('other')
  })
})

describe('parseWatchBuffer', () => {
  const buffer = `bob watch: 2026-09-28 06:19:34 — watching bob.yaml (Ctrl+C to stop)
─────────────────────────────────────────
unchanged  .gitignore                               [in_sync] convergence
conflict   .github/dependabot.yml                   [managed_hash_mismatch] contract_drift

0 create, 0 update, 0 adopt, 1 unchanged, 1 conflict

bob watch: 2026-09-28 06:20:01 — watching bob.yaml (Ctrl+C to stop)
─────────────────────────────────────────
unchanged  .gitignore                               [in_sync] convergence

1 create, 0 update, 0 adopt, 1 unchanged, 0 conflict
`

  it('folds a stream into ordered ticks', () => {
    const ticks = parseWatchBuffer(buffer)
    expect(ticks).toHaveLength(2)
    expect(ticks[0]?.rows).toHaveLength(2)
    expect(ticks[0]?.tally?.conflict).toBe(1)
    expect(ticks[1]?.timestamp).toBe('2026-09-28 06:20:01')
    expect(ticks[1]?.tally?.create).toBe(1)
  })

  it('tolerates a buffer that starts mid-tick', () => {
    const ticks = parseWatchBuffer('unchanged  README.md  [seed_exists] convergence\n')
    expect(ticks).toHaveLength(1)
    expect(ticks[0]?.rows[0]?.path).toBe('README.md')
  })
})
