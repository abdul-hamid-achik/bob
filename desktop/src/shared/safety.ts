import type { Feature } from './features'

/**
 * Whether one concrete invocation mutates, as opposed to whether the feature
 * can mutate at all.
 *
 * Bob expresses intent through flags: `--write` turns a preview into a write,
 * and `--dry-run` turns a mutation into a preview. Gating on the feature alone
 * would force a confirmation for a read-only preview; gating on the argv keeps
 * the guard exactly as wide as the command.
 */
export function invocationMutates(feature: Feature, argv: string[]): boolean {
  if (!feature.mutates) return false
  const flags = new Set((feature.flags ?? []).map((flag) => flag.name))
  if (flags.has('--write')) return argv.includes('--write')
  if (flags.has('--dry-run')) return !argv.includes('--dry-run')
  return true
}

/** True when a feature needs the operator's explicit subprocess authority. */
export function invocationNeedsAuthority(feature: Feature, argv: string[]): boolean {
  if (!feature.authority) return false
  const flags = new Set((feature.flags ?? []).map((flag) => flag.name))
  if (flags.has('--probe-integrations')) return argv.includes('--probe-integrations')
  return true
}
