import type { ActionKind, PlanAction } from './types'

/** Closed action-code vocabulary mirrored from internal/engine/engine.go. */
export type ActionFamily =
  | 'ownership_hazard'
  | 'contract_drift'
  | 'unmanaged_divergence'
  | 'scaffold'
  | 'convergence'
  | ''

export const FAMILY_BY_CODE: Record<string, ActionFamily> = {
  symlink: 'ownership_hazard',
  special_file: 'ownership_hazard',
  managed_hash_mismatch: 'contract_drift',
  managed_missing: 'contract_drift',
  retired_owned: 'contract_drift',
  unmanaged_differs: 'unmanaged_divergence',
  unmanaged_mode_differs: 'unmanaged_divergence',
  missing: 'scaffold',
  mode_drift: 'convergence',
  content_update: 'convergence',
  in_sync: 'convergence',
  identical_content: 'convergence',
  seed_exists: 'convergence'
}

export const CONFLICT_FAMILIES: ActionFamily[] = ['ownership_hazard', 'contract_drift', 'unmanaged_divergence']

export const FAMILY_META: Record<Exclude<ActionFamily, ''>, { label: string; detail: string }> = {
  ownership_hazard: {
    label: 'Ownership hazard',
    detail: 'An unsafe destination regardless of content: an existing symlink or special file Bob must never write through.'
  },
  contract_drift: {
    label: 'Contract drift',
    detail: 'A Bob-owned file drifted from the hash recorded in bob.lock, or a retired file the lock still owns.'
  },
  unmanaged_divergence: {
    label: 'Unmanaged divergence',
    detail: 'A recipe proposal over a file Bob never owned: the repository evolved independently of the recipe.'
  },
  scaffold: {
    label: 'Scaffold',
    detail: 'A create of a file that does not exist yet, including seed creates.'
  },
  convergence: {
    label: 'Convergence',
    detail: 'Conflict-free actions: safe lock-proven updates, adoptions, unchanged files, and seed-satisfied destinations.'
  }
}

export const ACTION_CODE_MEANING: Record<string, string> = {
  symlink: 'the destination is an existing symlink; Bob never writes through it',
  special_file: 'the destination is a special file such as a socket or device',
  managed_hash_mismatch: 'managed file differs from the hash recorded in bob.lock',
  managed_missing: 'a lock-owned file is missing from the working tree',
  retired_owned: 'the lock still owns a file the current recipe no longer renders',
  unmanaged_differs: 'an unmanaged file exists with different content than the recipe proposes',
  unmanaged_mode_differs: 'an unmanaged file exists with a different file mode',
  missing: 'the file does not exist; Bob will create it',
  mode_drift: 'content matches but the file mode differs',
  content_update: 'the managed file can be updated because its hash still matches the lock',
  in_sync: 'managed file already matches the desired content and mode',
  identical_content: 'an unmanaged file already matches the desired content exactly',
  seed_exists: 'seed file exists; Bob seeds it once and never lock-owns it again'
}

export type Tone = 'neutral' | 'positive' | 'caution' | 'danger' | 'info'

export const KIND_META: Record<ActionKind, { label: string; tone: Tone; detail: string }> = {
  create: { label: 'Create', tone: 'positive', detail: 'Absent destination; Bob writes the desired file.' },
  adopt: { label: 'Adopt', tone: 'info', detail: 'An unmanaged file already matches desired content; Bob takes ownership.' },
  update: { label: 'Update', tone: 'caution', detail: 'Lock-proven update: the current hash matches the prior lock.' },
  unchanged: { label: 'Unchanged', tone: 'neutral', detail: 'Already converged; nothing is written.' },
  conflict: { label: 'Conflict', tone: 'danger', detail: 'Ownership is not proven; one conflict blocks the complete apply.' }
}

export function familyForCode(code: string): ActionFamily {
  return FAMILY_BY_CODE[code] ?? ''
}

export function actionCounts(actions: PlanAction[]): Record<ActionKind, number> {
  const counts: Record<ActionKind, number> = { create: 0, update: 0, adopt: 0, unchanged: 0, conflict: 0 }
  for (const action of actions) counts[action.kind] = (counts[action.kind] ?? 0) + 1
  return counts
}

export function conflictFamilyCounts(actions: PlanAction[]): Record<string, number> {
  const counts: Record<string, number> = { ownership_hazard: 0, contract_drift: 0, unmanaged_divergence: 0 }
  for (const action of actions) {
    if (action.kind !== 'conflict') continue
    const family = familyForCode(action.code)
    if (family in counts) counts[family] = (counts[family] ?? 0) + 1
  }
  return counts
}

/** Mirrors engine.PlanResult.ConflictClass: none, one family, or mixed. */
export function conflictClass(actions: PlanAction[]): string {
  const counts = conflictFamilyCounts(actions)
  let present = 0
  let klass = 'none'
  for (const family of CONFLICT_FAMILIES) {
    if ((counts[family] ?? 0) > 0) {
      present += 1
      klass = family
    }
  }
  return present > 1 ? 'mixed' : klass
}

export function dominantConflictFamily(actions: PlanAction[]): ActionFamily {
  const counts = conflictFamilyCounts(actions)
  for (const family of CONFLICT_FAMILIES) {
    if ((counts[family] ?? 0) > 0) return family
  }
  return ''
}

const DIGEST_RE = /^sha256:[0-9a-f]{64}$/

export function isPlanDigest(value: string): boolean {
  return DIGEST_RE.test(value)
}

export function shortDigest(value: string | undefined, chars = 12): string {
  if (!value) return '—'
  const body = value.startsWith('sha256:') ? value.slice(7) : value
  return body.slice(0, chars)
}

export function formatMode(mode: number | undefined): string {
  if (mode === undefined) return '—'
  return `0${(mode & 0o777).toString(8)}`
}

export function formatDuration(ms: number): string {
  if (ms < 1000) return `${Math.round(ms)} ms`
  if (ms < 60_000) return `${(ms / 1000).toFixed(2)} s`
  const minutes = Math.floor(ms / 60_000)
  const seconds = Math.round((ms % 60_000) / 1000)
  return `${minutes}m ${seconds}s`
}

export function formatTimestamp(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false
  })
}

export function formatClock(iso: string): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return iso
  return date.toLocaleTimeString(undefined, { hour12: false })
}

export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export function baseName(path: string): string {
  const trimmed = path.replace(/\/+$/, '')
  const index = trimmed.lastIndexOf('/')
  return index === -1 ? trimmed || '/' : trimmed.slice(index + 1)
}

export function relativeTo(workspace: string, path: string): string {
  if (path.startsWith(workspace)) {
    const rest = path.slice(workspace.length)
    return rest.startsWith('/') ? rest.slice(1) : rest || '.'
  }
  return path
}

/** Quotes argv for display the way a shell would need it. */
export function displayArgv(argv: string[]): string {
  return argv
    .map((token) => (/[^\w@%+=:,./-]/.test(token) ? `'${token.replace(/'/g, "'\\''")}'` : token))
    .join(' ')
}
