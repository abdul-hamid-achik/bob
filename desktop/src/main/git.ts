import type { GitCommit, GitStatus } from '../shared/ipc'
import { spawnCapture } from './runner'

const TIMEOUT_MS = 20_000

async function git(cwd: string, argv: string[]): Promise<{ stdout: string; stderr: string; exitCode: number }> {
  try {
    const outcome = await spawnCapture({ command: 'git', argv, cwd, timeoutMs: TIMEOUT_MS })
    return { stdout: outcome.stdout, stderr: outcome.stderr, exitCode: outcome.exitCode }
  } catch (error) {
    return { stdout: '', stderr: error instanceof Error ? error.message : String(error), exitCode: 1 }
  }
}

/** Read-only git state. The console never stages, commits, resets, or pushes. */
export async function gitStatus(cwd: string): Promise<GitStatus> {
  const result = await git(cwd, ['status', '--porcelain=v1', '--branch'])
  if (result.exitCode !== 0) {
    return { available: false, branch: null, dirty: false, entries: [], error: result.stderr.trim() || 'git failed' }
  }
  let branch: string | null = null
  const entries: GitStatus['entries'] = []
  for (const line of result.stdout.split('\n')) {
    if (!line.trim()) continue
    if (line.startsWith('## ')) {
      branch = line.slice(3).split('...')[0] ?? null
      continue
    }
    const index = line.slice(0, 1)
    const worktree = line.slice(1, 2)
    const path = line.slice(3).trim()
    if (!path) continue
    entries.push({ path, index: index === ' ' ? '' : index, worktree: worktree === ' ' ? '' : worktree })
  }
  return { available: true, branch, dirty: entries.length > 0, entries, error: null }
}

export async function gitLog(cwd: string, limit = 20): Promise<GitCommit[]> {
  const bounded = Math.max(1, Math.min(200, Math.floor(limit)))
  const result = await git(cwd, [
    'log',
    `-n`,
    String(bounded),
    '--pretty=format:%H%x1f%h%x1f%s%x1f%an%x1f%aI'
  ])
  if (result.exitCode !== 0) return []
  return result.stdout
    .split('\n')
    .filter(Boolean)
    .map((line) => {
      const [hash = '', shortHash = '', subject = '', author = '', date = ''] = line.split('\x1f')
      return { hash, shortHash, subject, author, date }
    })
}

export async function gitDiff(cwd: string, relativePath?: string): Promise<{ diff: string; error: string | null }> {
  const argv = relativePath ? ['diff', '--', relativePath] : ['diff']
  const result = await git(cwd, argv)
  if (result.exitCode !== 0) return { diff: '', error: result.stderr.trim() || 'git diff failed' }
  return { diff: result.stdout, error: null }
}
