import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { join, resolve, sep } from 'node:path'
import type { FileContent, LockEntryView, PathEntry } from '../shared/ipc'

const SKIP_DIRS = new Set([
  '.git',
  'node_modules',
  'dist',
  'out',
  'release',
  'bin',
  'coverage',
  '.vitepress',
  '.qwen',
  '.glyphrun',
  '.bob-tmp'
])
const MAX_ENTRIES = 4000
const MAX_DEPTH = 8
const MAX_FILE_BYTES = 512 * 1024

/**
 * Resolves a repository-relative path inside a workspace, refusing absolute
 * paths and traversal, mirroring the containment rule Bob itself enforces.
 */
export function safeJoin(workspace: string, relativePath: string): string | null {
  const trimmed = relativePath.trim()
  if (!trimmed) return null
  if (trimmed.startsWith('/') || trimmed.includes('\0')) return null
  const root = resolve(workspace)
  const target = resolve(root, trimmed)
  if (target !== root && !target.startsWith(root + sep)) return null
  return target
}

interface WalkState {
  entries: PathEntry[]
  count: number
}

function walk(dir: string, root: string, depth: number, state: WalkState): void {
  if (depth > MAX_DEPTH || state.count >= MAX_ENTRIES) return
  let items: string[]
  try {
    items = readdirSync(dir)
  } catch {
    return
  }
  for (const item of items.sort()) {
    if (state.count >= MAX_ENTRIES) return
    if (SKIP_DIRS.has(item) || item.startsWith('.bob-tmp')) continue
    const absolute = join(dir, item)
    const relative = absolute.slice(root.length + 1)
    let stats
    try {
      stats = statSync(absolute)
    } catch {
      continue
    }
    if (stats.isDirectory()) {
      state.entries.push({
        path: relative,
        kind: 'directory',
        bytes: null,
        mtime: stats.mtime.toISOString(),
        lockedSha256: null,
        exists: true
      })
      state.count += 1
      walk(absolute, root, depth + 1, state)
      continue
    }
    if (!stats.isFile()) continue
    state.entries.push({
      path: relative,
      kind: 'unmanaged',
      bytes: stats.size,
      mtime: stats.mtime.toISOString(),
      lockedSha256: null,
      exists: true
    })
    state.count += 1
  }
}

/**
 * Lists workspace paths and marks the ones bob.lock owns.
 * Heavy build and dependency directories are skipped so the view stays useful.
 */
export function listPaths(workspace: string, lockFiles: LockEntryView[]): PathEntry[] {
  const root = resolve(workspace)
  if (!existsSync(root)) return []
  const state: WalkState = { entries: [], count: 0 }
  walk(root, root, 1, state)

  const byPath = new Map(state.entries.map((entry) => [entry.path, entry]))
  for (const locked of lockFiles) {
    const existing = byPath.get(locked.path)
    if (existing) {
      existing.kind = 'managed'
      existing.lockedSha256 = locked.sha256
      continue
    }
    const absolute = safeJoin(root, locked.path)
    byPath.set(locked.path, {
      path: locked.path,
      kind: absolute && existsSync(absolute) ? 'managed' : 'lock-only',
      bytes: null,
      mtime: null,
      lockedSha256: locked.sha256,
      exists: Boolean(absolute && existsSync(absolute))
    })
  }
  return [...byPath.values()].sort((a, b) => a.path.localeCompare(b.path))
}

export function readFileBounded(workspace: string, relativePath: string, maxBytes = MAX_FILE_BYTES): FileContent | null {
  const absolute = safeJoin(workspace, relativePath)
  if (!absolute || !existsSync(absolute)) return null
  let stats
  try {
    stats = statSync(absolute)
  } catch {
    return null
  }
  if (!stats.isFile()) return null
  const buffer = readFileSync(absolute)
  const binary = buffer.subarray(0, 8192).includes(0)
  const truncated = buffer.byteLength > maxBytes
  return {
    path: relativePath,
    content: binary ? '' : buffer.subarray(0, Math.min(maxBytes, buffer.byteLength)).toString('utf8'),
    bytes: buffer.byteLength,
    truncated,
    binary
  }
}
