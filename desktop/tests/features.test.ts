import { describe, expect, it } from 'vitest'
import {
  AUTHORITY_FEATURE_IDS,
  FEATURES,
  FEATURE_BY_ID,
  GROUP_LABEL,
  MCP_TOOL_FEATURE_IDS,
  MUTATING_FEATURE_IDS,
  featureById,
  featuresByGroup
} from '../src/shared/features'

/** The complete CLI surface reported by `bob --help` on v0.11.0, mapped to its registry id. */
const CLI_COMMANDS: [command: string, featureId: string][] = [
  ['apply', 'apply'],
  ['check', 'check'],
  ['config init', 'config-init'],
  ['config show', 'config-show'],
  ['context', 'context'],
  ['doctor', 'doctor'],
  ['explain', 'explain'],
  ['init', 'init'],
  ['inspect', 'inspect'],
  ['inspect --probe-integrations', 'inspect-probe'],
  ['learn', 'learn'],
  ['mcp serve', 'mcp-serve'],
  ['new', 'new'],
  ['path', 'path'],
  ['path --batch', 'path-batch'],
  ['plan', 'plan'],
  ['plan --watch', 'watch'],
  ['playbook list', 'playbook-list'],
  ['playbook plan', 'playbook-plan'],
  ['playbook show', 'playbook-show'],
  ['recipe list', 'recipe-list'],
  ['recipe show', 'recipe-show'],
  ['remove', 'remove'],
  ['stats', 'stats'],
  ['studio', 'studio'],
  ['upgrade', 'upgrade'],
  ['version', 'version']
]

const MCP_TOOLS = [
  'bob_context',
  'bob_path',
  'bob_playbook',
  'bob_inspect',
  'bob_plan',
  'bob_check',
  'bob_validate_manifest',
  'bob_recipe_describe',
  'bob_stats'
]

describe('feature registry', () => {
  it('has unique ids and titles', () => {
    const ids = FEATURES.map((feature) => feature.id)
    expect(new Set(ids).size).toBe(ids.length)
    expect(FEATURE_BY_ID.size).toBe(FEATURES.length)
  })

  it('covers every bob CLI command and subcommand', () => {
    for (const [command, featureId] of CLI_COMMANDS) {
      const feature = featureById(featureId)
      expect(feature, `${command} → ${featureId} is not registered`).toBeTruthy()
      const argv = feature?.argvTemplate?.join(' ') ?? ''
      const head = command.split(' ')[0] ?? ''
      expect(argv.startsWith(head), `${featureId} argv "${argv}" does not start with "${head}"`).toBe(true)
    }
    expect(CLI_COMMANDS.length).toBeGreaterThanOrEqual(27)
  })

  it('covers all nine MCP tools', () => {
    expect(MCP_TOOL_FEATURE_IDS.sort()).toEqual([...MCP_TOOLS].sort())
  })

  it('assigns every feature to a known group and panel', () => {
    for (const feature of FEATURES) {
      expect(GROUP_LABEL[feature.group], `${feature.id} group`).toBeTruthy()
      expect(feature.panel, `${feature.id} panel`).toBeTruthy()
      expect(feature.summary.length, `${feature.id} summary`).toBeGreaterThan(10)
      expect(feature.detail.length, `${feature.id} detail`).toBeGreaterThan(20)
    }
  })

  it('groups features without losing any', () => {
    const grouped = featuresByGroup().flatMap((group) => group.features)
    expect(grouped.length).toBe(FEATURES.length)
  })

  it('marks exactly the mutating surfaces', () => {
    expect([...MUTATING_FEATURE_IDS].sort()).toEqual(
      ['apply', 'config-init', 'init', 'manifest-edit', 'new', 'remove', 'tasks', 'upgrade'].sort()
    )
  })

  it('gates the specialist probe behind explicit authority', () => {
    expect(AUTHORITY_FEATURE_IDS).toEqual(['inspect-probe'])
    expect(featureById('inspect-probe')?.authority).toBe(true)
    expect(featureById('inspect')?.authority).toBeUndefined()
  })

  it('flags streaming surfaces so the runner never adds --json', () => {
    const streaming = FEATURES.filter((feature) => feature.streaming).map((feature) => feature.id)
    expect(streaming.sort()).toEqual(['mcp-serve', 'studio', 'tasks', 'watch'].sort())
  })

  it('documents flags only for features that accept them', () => {
    for (const feature of FEATURES) {
      for (const flag of feature.flags ?? []) {
        expect(flag.name.startsWith('--'), `${feature.id} ${flag.name}`).toBe(true)
        expect(flag.help.length).toBeGreaterThan(5)
        if (flag.kind === 'enum') expect(flag.values?.length).toBeGreaterThan(0)
      }
    }
  })

  it('keeps read-only invariants on the read-only surfaces', () => {
    const readOnly = ['context', 'path', 'path-batch', 'playbook-list', 'playbook-show', 'playbook-plan', 'plan', 'check', 'stats', 'inspect', 'explain', 'learn']
    for (const id of readOnly) {
      expect(featureById(id)?.mutates, `${id} must not mutate`).toBe(false)
    }
  })

  it('resolves features by id', () => {
    expect(featureById('plan')?.command).toBe('bob plan')
    expect(featureById('nope')).toBeUndefined()
  })
})
