import { join } from 'node:path'
import type { ActivityEntry } from '../shared/ipc'
import { readJson, writeJsonAtomic } from './jsonFile'

export interface ActivityLog {
  readonly file: string
  record(entry: ActivityEntry): ActivityEntry[]
  list(): ActivityEntry[]
  clear(): number
  exportTo(file: string): string
}

const MAX_ENTRIES = 500

function sanitizeEntry(value: unknown): ActivityEntry | null {
  if (!value || typeof value !== 'object') return null
  const entry = value as ActivityEntry
  if (typeof entry.id !== 'string' || !Array.isArray(entry.argv)) return null
  return entry
}

/**
 * Append-only local ledger of every invocation the console makes.
 * Newest first, capped, persisted under userData. Never leaves the machine.
 */
export function createActivityLog(dir: string): ActivityLog {
  const file = join(dir, 'activity.json')
  let entries = (readJson<unknown[]>(file) ?? [])
    .map(sanitizeEntry)
    .filter((entry): entry is ActivityEntry => entry !== null)

  function persist(): void {
    writeJsonAtomic(file, entries)
  }

  return {
    file,
    record(entry) {
      entries = [entry, ...entries].slice(0, MAX_ENTRIES)
      persist()
      return this.list()
    },
    list() {
      return entries.map((entry) => ({ ...entry }))
    },
    clear() {
      const cleared = entries.length
      entries = []
      persist()
      return cleared
    },
    exportTo(target) {
      writeJsonAtomic(target, entries)
      return target
    }
  }
}
