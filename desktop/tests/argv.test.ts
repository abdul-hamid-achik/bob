import { describe, expect, it } from 'vitest'
import { buildArgv, jsonCompatible, missingPlaceholders, placeholdersFor, splitPathList, tokenizeArgv } from '../src/shared/argv'
import { FEATURES, featureById } from '../src/shared/features'

const workspace = '/tmp/bob-workspace'

describe('buildArgv', () => {
  it('resolves the workspace placeholder and appends --json for CLI features', () => {
    const argv = buildArgv(featureById('plan')!, { workspace })
    expect(argv).toEqual(['plan', workspace, '--json'])
  })

  it('does not add --json to streaming features', () => {
    const argv = buildArgv(featureById('watch')!, { workspace })
    expect(argv).toEqual(['plan', workspace, '--watch'])
    expect(jsonCompatible(featureById('watch')!)).toBe(false)
  })

  it('keeps --batch paths after the -- separator', () => {
    const argv = buildArgv(featureById('path-batch')!, {
      workspace,
      values: { paths: ['package.json', 'tsconfig.json'] }
    })
    expect(argv).toEqual(['path', '--batch', '--workspace', workspace, '--', 'package.json', 'tsconfig.json', '--json'])
  })

  it('renders boolean, string, enum, and repeatable flags', () => {
    const argv = buildArgv(featureById('plan')!, {
      workspace,
      flags: { content: true, diff: false, 'conflicts-only': true }
    })
    expect(argv).toContain('--content')
    expect(argv).not.toContain('--diff')
    expect(argv).toContain('--conflicts-only')

    const contextArgv = buildArgv(featureById('context')!, { workspace, flags: { profile: 'full' } })
    expect(contextArgv).toEqual(['context', workspace, '--profile', 'full', '--json'])

    const playbookArgv = buildArgv(featureById('playbook-plan')!, {
      workspace,
      values: { playbook: 'resolve-ownership-conflict' },
      flags: { set: ['path=README.md', 'action_code=managed_hash_mismatch'] }
    })
    expect(playbookArgv).toEqual([
      'playbook',
      'plan',
      'resolve-ownership-conflict',
      workspace,
      '--set',
      'path=README.md',
      '--set',
      'action_code=managed_hash_mismatch',
      '--json'
    ])
  })

  it('binds apply to an explicit plan digest', () => {
    const digest = `sha256:${'a'.repeat(64)}`
    const argv = buildArgv(featureById('apply')!, { workspace, flags: { 'expect-plan-digest': digest } })
    expect(argv).toEqual(['apply', workspace, '--expect-plan-digest', digest, '--json'])
  })

  it('drops placeholders with no value and reports them as missing', () => {
    const feature = featureById('recipe-show')!
    expect(placeholdersFor(feature)).toEqual(['recipe'])
    expect(missingPlaceholders(feature, { workspace })).toEqual(['recipe'])
    expect(buildArgv(feature, { workspace })).toEqual(['recipe', 'show', '--json'])
    expect(missingPlaceholders(feature, { workspace, values: { recipe: 'ts-app' } })).toEqual([])
  })

  it('never emits --json for app-level surfaces', () => {
    for (const feature of FEATURES.filter((entry) => entry.surface === 'app' || entry.surface === 'tui')) {
      expect(jsonCompatible(feature)).toBe(false)
    }
  })
})

describe('tokenizeArgv', () => {
  it('splits on whitespace and keeps quoted paths together', () => {
    expect(tokenizeArgv('plan . --json')).toEqual(['plan', '.', '--json'])
    expect(tokenizeArgv('path --workspace /tmp/ws --json -- "my dir/bob.yaml"')).toEqual([
      'path',
      '--workspace',
      '/tmp/ws',
      '--json',
      '--',
      'my dir/bob.yaml'
    ])
    expect(tokenizeArgv("playbook plan id . --set 'path=a b'")).toEqual(['playbook', 'plan', 'id', '.', '--set', 'path=a b'])
    expect(tokenizeArgv('')).toEqual([])
  })
})

describe('splitPathList', () => {
  it('accepts newline or comma separated input and trims blanks', () => {
    expect(splitPathList('a\n b ,, c\n')).toEqual(['a', 'b', 'c'])
  })
})
