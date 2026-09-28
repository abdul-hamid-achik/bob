import type { Feature } from './features'

export type ArgvValue = string | boolean | string[] | number

export interface BuildArgvOptions {
  workspace: string
  /** Placeholder values, e.g. { path: 'README.md', paths: ['a', 'b'] }. */
  values?: Record<string, ArgvValue | undefined>
  /** Flag values keyed by flag name without leading dashes, e.g. { diff: true }. */
  flags?: Record<string, ArgvValue | undefined>
  /** Append --json. Defaults to true for non-streaming CLI features. */
  json?: boolean
}

export const PLACEHOLDER_RE = /\{(\w+)\}/g

/** Placeholder names a feature template needs, in template order. */
export function placeholdersFor(feature: Feature): string[] {
  const seen = new Set<string>()
  for (const token of feature.argvTemplate ?? []) {
    for (const match of token.matchAll(PLACEHOLDER_RE)) {
      const name = match[1]
      if (name && name !== 'workspace') seen.add(name)
    }
  }
  return [...seen]
}

function expand(token: string, values: Record<string, ArgvValue | undefined>, workspace: string): string[] {
  const fullPlaceholder = token.match(/^\{(\w+)\}$/)
  if (fullPlaceholder) {
    const name = fullPlaceholder[1] ?? ''
    const value = name === 'workspace' ? workspace : values[name]
    if (Array.isArray(value)) return value
    if (value === undefined || value === '') return []
    return [String(value)]
  }
  return [token.replace(PLACEHOLDER_RE, (_match, name: string) => {
    const value = name === 'workspace' ? workspace : values[name]
    if (Array.isArray(value)) return value.join(' ')
    return value === undefined ? '' : String(value)
  })]
}

/** True when the feature streams and must not receive --json. */
export function jsonCompatible(feature: Feature): boolean {
  if (feature.surface === 'app' || feature.surface === 'tui') return false
  if (feature.streaming) return false
  return Boolean(feature.argvTemplate)
}

/**
 * Builds the argv (without the leading binary) for one registry feature.
 * Placeholders with no value are dropped, so `bob path --workspace W --` stays
 * syntactically harmless; callers check missingPlaceholders before running.
 */
export function buildArgv(feature: Feature, options: BuildArgvOptions): string[] {
  const values = options.values ?? {}
  const flags = options.flags ?? {}
  const template = feature.argvTemplate ?? []
  const argv: string[] = []
  for (const token of template) {
    argv.push(...expand(token, values, options.workspace))
  }
  for (const flag of feature.flags ?? []) {
    const key = flag.name.replace(/^-+/, '')
    const value = flags[key]
    if (value === undefined || value === false || value === '') continue
    if (flag.kind === 'boolean') {
      argv.push(flag.name)
      continue
    }
    if (Array.isArray(value)) {
      for (const item of value) argv.push(flag.name, String(item))
      continue
    }
    argv.push(flag.name, String(value))
  }
  const wantJson = options.json ?? jsonCompatible(feature)
  if (wantJson && !argv.includes('--json')) argv.push('--json')
  return argv
}

/** Placeholder names that still have no value. */
export function missingPlaceholders(feature: Feature, options: BuildArgvOptions): string[] {
  const values = options.values ?? {}
  return placeholdersFor(feature).filter((name) => {
    const value = values[name]
    return value === undefined || value === '' || (Array.isArray(value) && value.length === 0)
  })
}

/** Splits a multi-line or comma separated path list into argv tokens. */
export function splitPathList(input: string): string[] {
  return input
    .split(/[\n,]+/)
    .map((token) => token.trim())
    .filter(Boolean)
}

/**
 * Tokenizes a pasted command line into argv, honouring single and double
 * quotes so paths with spaces survive a round trip from the console.
 */
export function tokenizeArgv(input: string): string[] {
  const tokens: string[] = []
  let current = ''
  let quote: '"' | "'" | null = null
  let started = false
  for (const char of input) {
    if (quote) {
      if (char === quote) {
        quote = null
        continue
      }
      current += char
      continue
    }
    if (char === '"' || char === "'") {
      quote = char
      started = true
      continue
    }
    if (/\s/.test(char)) {
      if (started) tokens.push(current)
      current = ''
      started = false
      continue
    }
    current += char
    started = true
  }
  if (started) tokens.push(current)
  return tokens
}
