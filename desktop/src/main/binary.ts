import { execFile } from 'node:child_process'
import { existsSync, statSync } from 'node:fs'
import { homedir } from 'node:os'
import { dirname, join, resolve } from 'node:path'
import type { BinaryInfo } from '../shared/ipc'
import { parseEnvelope } from '../shared/envelope'
import type { VersionData } from '../shared/types'

export interface ResolveBobOptions {
  configuredPath?: string
  workspaces?: string[]
  /** Directory of the running main bundle, used to find a source checkout. */
  appDir?: string
}

function isExecutable(path: string): boolean {
  try {
    if (!existsSync(path)) return false
    const stats = statSync(path)
    return stats.isFile() && (stats.mode & 0o111) !== 0
  } catch {
    return false
  }
}

function checkoutBinaries(start: string | undefined): string[] {
  if (!start) return []
  const found: string[] = []
  let dir = resolve(start)
  for (let depth = 0; depth < 6; depth += 1) {
    const candidate = join(dir, 'bin', 'bob')
    if (existsSync(candidate)) found.push(candidate)
    const parent = dirname(dir)
    if (parent === dir) break
    dir = parent
  }
  return found
}

function whichBob(): Promise<string | null> {
  return new Promise((promiseResolve) => {
    execFile('which', ['bob'], { timeout: 4000 }, (error, stdout) => {
      if (error) return promiseResolve(null)
      const first = stdout.split('\n').map((line) => line.trim()).filter(Boolean)[0]
      promiseResolve(first ?? null)
    })
  })
}

async function probeVersion(path: string): Promise<{ version: string | null; commit: string | null; date: string | null; error: string | null }> {
  return new Promise((promiseResolve) => {
    execFile(path, ['version', '--json'], { timeout: 8000 }, (error, stdout, stderr) => {
      if (error && !stdout) {
        return promiseResolve({ version: null, commit: null, date: null, error: error.message || stderr.trim() })
      }
      const { envelope, parseError } = parseEnvelope(stdout)
      if (!envelope || parseError) {
        return promiseResolve({ version: null, commit: null, date: null, error: parseError ?? 'no envelope' })
      }
      const data = envelope.data as unknown as VersionData | undefined
      return promiseResolve({
        version: data?.version ?? null,
        commit: data?.commit ?? null,
        date: data?.date ?? null,
        error: envelope.ok ? null : 'version reported a failure envelope'
      })
    })
  })
}

/**
 * Resolves which bob binary the console drives, in priority order:
 * configured path, a source-checkout ./bin/bob, the Go bin, common prefixes,
 * then PATH. Only the resolved binary is ever spawned.
 */
export async function resolveBobBinary(options: ResolveBobOptions = {}): Promise<BinaryInfo> {
  const candidates: { path: string; source: string }[] = []
  if (options.configuredPath) candidates.push({ path: resolve(options.configuredPath), source: 'settings' })
  for (const workspace of options.workspaces ?? []) {
    for (const path of checkoutBinaries(workspace)) candidates.push({ path, source: 'repository' })
  }
  for (const path of checkoutBinaries(options.appDir)) candidates.push({ path, source: 'repository' })
  candidates.push({ path: join(homedir(), 'go', 'bin', 'bob'), source: 'path' })
  candidates.push({ path: '/opt/homebrew/bin/bob', source: 'path' })
  candidates.push({ path: '/usr/local/bin/bob', source: 'path' })
  const onPath = await whichBob()
  if (onPath) candidates.push({ path: onPath, source: 'path' })

  const seen = new Set<string>()
  const unique = candidates.filter((candidate) => {
    if (seen.has(candidate.path)) return false
    seen.add(candidate.path)
    return true
  })

  const chosen = unique.find((candidate) => isExecutable(candidate.path))
  const info: BinaryInfo = {
    path: chosen?.path ?? '',
    source: chosen ? (chosen.source as BinaryInfo['source']) : 'none',
    exists: Boolean(chosen),
    executable: Boolean(chosen),
    version: null,
    commit: null,
    date: null,
    candidates: unique.map((candidate) => ({
      path: candidate.path,
      source: candidate.source,
      exists: existsSync(candidate.path)
    })),
    error: chosen ? null : 'no bob binary found; set one in Settings'
  }

  if (chosen) {
    const probe = await probeVersion(chosen.path)
    info.version = probe.version
    info.commit = probe.commit
    info.date = probe.date
    if (probe.error) info.error = probe.error
  }
  return info
}

/**
 * Finds the Bob source checkout that owns a binary, so the docs browser and
 * the Taskfile gates have a repository to read. Falls back to any workspace
 * that looks like the Bob source tree.
 */
export function detectBobRepoRoot(binaryPath: string | null, workspaces: string[] = []): string | null {
  const roots: string[] = []
  if (binaryPath) {
    let dir = dirname(resolve(binaryPath))
    for (let depth = 0; depth < 4; depth += 1) {
      roots.push(dir)
      const parent = dirname(dir)
      if (parent === dir) break
      dir = parent
    }
  }
  for (const workspace of workspaces) roots.push(resolve(workspace))

  for (const root of roots) {
    if (existsSync(join(root, 'Taskfile.yml')) && existsSync(join(root, 'cmd', 'bob'))) return root
  }
  return null
}
