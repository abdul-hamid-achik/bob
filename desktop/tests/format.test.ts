import { describe, expect, it } from 'vitest'
import {
  ACTION_CODE_MEANING,
  FAMILY_BY_CODE,
  KIND_META,
  actionCounts,
  conflictClass,
  conflictFamilyCounts,
  dominantConflictFamily,
  displayArgv,
  formatDuration,
  formatMode,
  isPlanDigest,
  shortDigest
} from '../src/shared/format'
import type { PlanAction } from '../src/shared/types'

/** The closed action-code vocabulary mirrored from internal/engine/engine.go. */
const CLOSED_CODES = [
  'symlink',
  'special_file',
  'managed_hash_mismatch',
  'managed_missing',
  'retired_owned',
  'unmanaged_differs',
  'unmanaged_mode_differs',
  'missing',
  'mode_drift',
  'content_update',
  'in_sync',
  'identical_content',
  'seed_exists'
]

function action(kind: PlanAction['kind'], code: string, path = 'file'): PlanAction {
  return { path, kind, code }
}

describe('family map', () => {
  it('covers the whole closed code vocabulary', () => {
    for (const code of CLOSED_CODES) {
      expect(FAMILY_BY_CODE[code], `${code} has no family`).toBeTruthy()
      expect(ACTION_CODE_MEANING[code], `${code} has no meaning`).toBeTruthy()
    }
    expect(Object.keys(FAMILY_BY_CODE).sort()).toEqual([...CLOSED_CODES].sort())
  })

  it('classifies hazards, drift, divergence, scaffold, and convergence', () => {
    expect(FAMILY_BY_CODE.symlink).toBe('ownership_hazard')
    expect(FAMILY_BY_CODE.special_file).toBe('ownership_hazard')
    expect(FAMILY_BY_CODE.managed_hash_mismatch).toBe('contract_drift')
    expect(FAMILY_BY_CODE.retired_owned).toBe('contract_drift')
    expect(FAMILY_BY_CODE.unmanaged_differs).toBe('unmanaged_divergence')
    expect(FAMILY_BY_CODE.missing).toBe('scaffold')
    expect(FAMILY_BY_CODE.in_sync).toBe('convergence')
    expect(FAMILY_BY_CODE.seed_exists).toBe('convergence')
  })

  it('returns an empty family for an unknown code instead of guessing', () => {
    expect(FAMILY_BY_CODE['not-a-code']).toBeUndefined()
  })

  it('has display metadata for all five action kinds', () => {
    expect(Object.keys(KIND_META).sort()).toEqual(['adopt', 'conflict', 'create', 'unchanged', 'update'])
  })
})

describe('conflict classification', () => {
  it('mirrors engine.ConflictFamilyCounts by only counting conflict kinds', () => {
    const counts = conflictFamilyCounts([
      action('conflict', 'symlink', 'a'),
      action('conflict', 'managed_hash_mismatch', 'b'),
      action('update', 'content_update', 'c')
    ])
    expect(counts).toEqual({ ownership_hazard: 1, contract_drift: 1, unmanaged_divergence: 0 })
  })

  it('mirrors engine.ConflictClass: none, one family, or mixed', () => {
    expect(conflictClass([action('unchanged', 'in_sync')])).toBe('none')
    expect(conflictClass([action('conflict', 'managed_hash_mismatch')])).toBe('contract_drift')
    expect(conflictClass([action('conflict', 'symlink'), action('conflict', 'unmanaged_differs')])).toBe('mixed')
  })

  it('mirrors engine.DominantConflictFamily precedence', () => {
    expect(dominantConflictFamily([action('conflict', 'unmanaged_differs'), action('conflict', 'symlink')])).toBe('ownership_hazard')
    expect(dominantConflictFamily([action('conflict', 'unmanaged_differs'), action('conflict', 'managed_missing')])).toBe('contract_drift')
    expect(dominantConflictFamily([action('unchanged', 'in_sync')])).toBe('')
  })

  it('tallies every action kind', () => {
    expect(
      actionCounts([
        action('create', 'missing', 'a'),
        action('update', 'content_update', 'b'),
        action('adopt', 'identical_content', 'c'),
        action('unchanged', 'in_sync', 'd'),
        action('conflict', 'symlink', 'e')
      ])
    ).toEqual({ create: 1, update: 1, adopt: 1, unchanged: 1, conflict: 1 })
  })
})

describe('digests and formatting', () => {
  it('accepts only sha256:<64 lowercase hex>', () => {
    expect(isPlanDigest(`sha256:${'a'.repeat(64)}`)).toBe(true)
    expect(isPlanDigest(`sha256:${'A'.repeat(64)}`)).toBe(false)
    expect(isPlanDigest(`sha256:${'a'.repeat(63)}`)).toBe(false)
    expect(isPlanDigest('a'.repeat(64))).toBe(false)
  })

  it('shortens digests and renders file modes', () => {
    expect(shortDigest(`sha256:${'a'.repeat(64)}`, 8)).toBe('aaaaaaaa')
    expect(shortDigest(undefined)).toBe('—')
    expect(formatMode(420)).toBe('0644')
    expect(formatMode(493)).toBe('0755')
    expect(formatMode(undefined)).toBe('—')
  })

  it('formats durations across scales', () => {
    expect(formatDuration(42)).toBe('42 ms')
    expect(formatDuration(1500)).toBe('1.50 s')
    expect(formatDuration(125_000)).toBe('2m 5s')
  })

  it('quotes argv the way a shell needs', () => {
    expect(displayArgv(['plan', '.', '--json'])).toBe('plan . --json')
    expect(displayArgv(['path', '--', 'my dir/bob.yaml'])).toBe("path -- 'my dir/bob.yaml'")
  })
})
