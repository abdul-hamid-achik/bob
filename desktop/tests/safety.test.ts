import { describe, expect, it } from 'vitest'
import { invocationMutates, invocationNeedsAuthority } from '../src/shared/safety'
import { featureById } from '../src/shared/features'

const workspace = '/tmp/ws'

describe('invocationMutates', () => {
  it('never gates a read-only feature', () => {
    expect(invocationMutates(featureById('plan')!, ['plan', workspace, '--json'])).toBe(false)
    expect(invocationMutates(featureById('context')!, ['context', workspace, '--json'])).toBe(false)
  })

  it('gates apply unconditionally', () => {
    expect(invocationMutates(featureById('apply')!, ['apply', workspace, '--json'])).toBe(true)
  })

  it('treats init and new as previews until --write appears', () => {
    expect(invocationMutates(featureById('init')!, ['init', workspace, '--json'])).toBe(false)
    expect(invocationMutates(featureById('init')!, ['init', workspace, '--write', '--json'])).toBe(true)
    expect(invocationMutates(featureById('new')!, ['new', 'demo', '--json'])).toBe(false)
    expect(invocationMutates(featureById('new')!, ['new', 'demo', '--write', '--json'])).toBe(true)
    expect(invocationMutates(featureById('config-init')!, ['config', 'init', '--json'])).toBe(false)
    expect(invocationMutates(featureById('config-init')!, ['config', 'init', '--write', '--telemetry', '--json'])).toBe(true)
  })

  it('treats upgrade and remove as mutations unless --dry-run is present', () => {
    expect(invocationMutates(featureById('upgrade')!, ['upgrade', workspace, '--json'])).toBe(true)
    expect(invocationMutates(featureById('upgrade')!, ['upgrade', workspace, '--dry-run', '--json'])).toBe(false)
    expect(invocationMutates(featureById('remove')!, ['remove', workspace, '--json'])).toBe(true)
    expect(invocationMutates(featureById('remove')!, ['remove', workspace, '--dry-run', '--json'])).toBe(false)
    expect(invocationMutates(featureById('remove')!, ['remove', workspace, '--dry-run', '--force', '--json'])).toBe(false)
  })

  it('gates console-side writes', () => {
    expect(invocationMutates(featureById('manifest-edit')!, [])).toBe(true)
    expect(invocationMutates(featureById('tasks')!, ['fmt'])).toBe(true)
  })
})

describe('invocationNeedsAuthority', () => {
  it('requires authority only for the explicit probe', () => {
    expect(invocationNeedsAuthority(featureById('inspect-probe')!, ['inspect', workspace, '--probe-integrations', '--json'])).toBe(true)
    expect(invocationNeedsAuthority(featureById('inspect')!, ['inspect', workspace, '--json'])).toBe(false)
    expect(invocationNeedsAuthority(featureById('plan')!, ['plan', workspace, '--json'])).toBe(false)
  })
})
