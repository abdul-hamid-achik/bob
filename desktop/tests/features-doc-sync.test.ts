import { readFileSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, it } from 'vitest'
import { FEATURES } from '../src/shared/features'

const doc = readFileSync(join(import.meta.dirname, '..', 'FEATURES.md'), 'utf8')

/** Rows look like: | `plan` | Plan | `bob plan` | no | Plan | */
const documentedIds = [...doc.matchAll(/^\|\s*`([a-z0-9_-]+)`\s*\|/gm)].map((match) => match[1] ?? '')

describe('FEATURES.md stays in sync with the registry', () => {
  it('documents every registered feature', () => {
    const missing = FEATURES.map((feature) => feature.id).filter((id) => !documentedIds.includes(id))
    expect(missing, `missing from FEATURES.md: ${missing.join(', ')}`).toEqual([])
  })

  it('documents nothing that is not registered', () => {
    const extra = documentedIds.filter((id) => !FEATURES.some((feature) => feature.id === id))
    expect(extra, `stale rows in FEATURES.md: ${extra.join(', ')}`).toEqual([])
  })

  it('has no duplicate rows', () => {
    expect(new Set(documentedIds).size).toBe(documentedIds.length)
  })

  it('documents the feature count claimed in the introduction', () => {
    const claimed = doc.match(/\*\*Surfaces\*\* — (\d+) features/)
    expect(claimed, 'the Surfaces line is missing').toBeTruthy()
    expect(Number(claimed?.[1])).toBe(FEATURES.length)
  })
})
