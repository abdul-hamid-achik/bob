import { existsSync, readFileSync, statSync } from 'node:fs'
import { basename, isAbsolute, join, resolve } from 'node:path'
import { parse as parseYaml } from 'yaml'
import type { LockEntryView, WorkspaceFiles } from '../shared/ipc'
import type { RecipeRef, WorkspaceInfo } from '../shared/types'

interface LockShape {
  recipe?: { id?: string; version?: number }
  files?: { path?: string; sha256?: string }[]
}

interface ManifestShape {
  recipe?: string | { id?: string; version?: number }
  product?: { name?: string }
}

function readTextOrNull(path: string): string | null {
  try {
    return existsSync(path) ? readFileSync(path, 'utf8') : null
  } catch {
    return null
  }
}

/** Reads bob.yaml and bob.lock for one workspace without mutating anything. */
export function readWorkspaceFiles(workspacePath: string): WorkspaceFiles {
  const root = resolve(workspacePath)
  const manifestPath = join(root, 'bob.yaml')
  const lockPath = join(root, 'bob.lock')
  const manifest = readTextOrNull(manifestPath)
  const lock = readTextOrNull(lockPath)

  let lockRecipe: RecipeRef | null = null
  let lockFiles: LockEntryView[] = []
  let lockError: string | null = null
  if (lock) {
    try {
      const parsed = parseYaml(lock) as LockShape
      if (parsed?.recipe?.id) {
        lockRecipe = { id: parsed.recipe.id, version: parsed.recipe.version ?? 0 }
      }
      lockFiles = (parsed?.files ?? [])
        .filter((entry) => typeof entry?.path === 'string' && typeof entry?.sha256 === 'string')
        .map((entry) => ({ path: entry.path as string, sha256: entry.sha256 as string }))
    } catch (error) {
      lockError = error instanceof Error ? error.message : String(error)
    }
  }

  let manifestError: string | null = null
  let recipeFromManifest: RecipeRef | null = null
  if (manifest) {
    try {
      const parsed = parseYaml(manifest) as ManifestShape
      if (typeof parsed?.recipe === 'string') recipeFromManifest = { id: parsed.recipe, version: 0 }
      else if (parsed?.recipe?.id) recipeFromManifest = { id: parsed.recipe.id, version: parsed.recipe.version ?? 0 }
    } catch (error) {
      manifestError = error instanceof Error ? error.message : String(error)
    }
  }

  return {
    path: root,
    manifest,
    manifestPath: existsSync(manifestPath) ? manifestPath : null,
    lock,
    lockPath: existsSync(lockPath) ? lockPath : null,
    lockRecipe: lockRecipe ?? recipeFromManifest,
    lockFiles,
    manifestError,
    lockError
  }
}

export function isDirectory(path: string): boolean {
  try {
    return existsSync(path) && statSync(path).isDirectory()
  } catch {
    return false
  }
}

/** Builds the workspace descriptor used by the switcher and the overview. */
export function inspectWorkspace(workspacePath: string, lastOpenedAt: string | null = null): WorkspaceInfo {
  const root = resolve(workspacePath)
  const files = readWorkspaceFiles(root)
  return {
    path: root,
    name: basename(root) || root,
    hasManifest: files.manifestPath !== null,
    hasLock: files.lockPath !== null,
    recipe: files.lockRecipe ?? undefined,
    managedFiles: files.lockFiles.length,
    lastOpenedAt: lastOpenedAt ?? new Date().toISOString()
  }
}

export function normalizeWorkspaceInput(input: string): string | null {
  const trimmed = input.trim()
  if (!trimmed) return null
  const candidate = isAbsolute(trimmed) ? resolve(trimmed) : resolve(process.cwd(), trimmed)
  return isDirectory(candidate) ? candidate : null
}

/** Moves a path to the front of the recent list, capped. */
export function touchRecent(recents: string[], path: string, limit = 24): string[] {
  return [path, ...recents.filter((entry) => entry !== path)].slice(0, limit)
}
