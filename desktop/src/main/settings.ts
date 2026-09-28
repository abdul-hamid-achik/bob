import { existsSync } from 'node:fs'
import { join } from 'node:path'
import { DEFAULT_SETTINGS, type AppSettings } from '../shared/types'
import { readJson, writeJsonAtomic } from './jsonFile'

export interface SettingsStore {
  readonly file: string
  get(): AppSettings
  set(patch: Partial<AppSettings>): AppSettings
  reset(): AppSettings
}

function sanitize(candidate: unknown): AppSettings {
  const value = (candidate ?? {}) as Partial<AppSettings>
  const settings: AppSettings = { ...DEFAULT_SETTINGS }
  if (typeof value.bobBinaryPath === 'string') settings.bobBinaryPath = value.bobBinaryPath
  if (typeof value.defaultWorkspace === 'string') settings.defaultWorkspace = value.defaultWorkspace
  if (Array.isArray(value.recentWorkspaces)) {
    settings.recentWorkspaces = value.recentWorkspaces.filter((item): item is string => typeof item === 'string').slice(0, 24)
  }
  if (value.theme === 'dark' || value.theme === 'light' || value.theme === 'system') settings.theme = value.theme
  if (typeof value.requireDigestBoundApply === 'boolean') settings.requireDigestBoundApply = value.requireDigestBoundApply
  if (typeof value.requireMutationConfirmation === 'boolean') settings.requireMutationConfirmation = value.requireMutationConfirmation
  if (typeof value.allowIntegrationProbes === 'boolean') settings.allowIntegrationProbes = value.allowIntegrationProbes
  if (typeof value.demoMode === 'boolean') settings.demoMode = value.demoMode
  return settings
}

/**
 * Per-user console settings, stored as JSON under Electron's userData dir.
 * Kept electron-free so the store is testable in plain Node.
 */
export function createSettingsStore(dir: string): SettingsStore {
  const file = join(dir, 'settings.json')
  let current = load()

  function load(): AppSettings {
    if (!existsSync(file)) return { ...DEFAULT_SETTINGS }
    return sanitize(readJson<Partial<AppSettings>>(file))
  }

  function persist(next: AppSettings): void {
    writeJsonAtomic(file, next)
  }

  return {
    file,
    get: () => ({ ...current, recentWorkspaces: [...current.recentWorkspaces] }),
    set(patch) {
      current = sanitize({ ...current, ...patch })
      persist(current)
      return this.get()
    },
    reset() {
      current = { ...DEFAULT_SETTINGS }
      persist(current)
      return this.get()
    }
  }
}
