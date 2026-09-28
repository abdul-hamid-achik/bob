import { useCallback, useEffect, useState } from 'react'
import { Radar, ShieldAlert } from 'lucide-react'
import type { InspectData } from '@shared/types'
import { Badge, Button, Callout, KeyValue, Modal, Mono, Section, Toggle, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, NextActionsList, RunButton, RunMeta, WarningList, useRunCommand } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

const PROBE_WARNING =
  'Bob will call the public Codemap and Vecgrep status commands. Those commands may open their tool-owned stores, and Vecgrep may contact its configured embedding provider. Bob never searches, indexes, resets, repairs, or declares verification.'

export function InspectPanel(): JSX.Element {
  const { workspace, settings, updateSettings, navigate, revision } = useStore()
  const inspect = useRun<InspectData>('inspect')
  const probe = useRun<InspectData>('inspect-probe')
  const runCommand = useRunCommand()
  const [probeConfirm, setProbeConfirm] = useState(false)

  const run = useCallback(() => inspect.run(), [inspect])

  useEffect(() => {
    if (workspace) void run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, revision])

  const data = probe.data ?? inspect.data
  const repository = data?.repository

  return (
    <PanelShell
      featureId="inspect"
      actions={
        <>
          <Button variant="quiet" onClick={() => navigate('doctor')}>
            Tool probes
          </Button>
          <RunButton label="Inspect" pending={inspect.pending} onClick={() => void run()} />
        </>
      }
      subtitle="Offline workspace inventory: Bob manifest and drift state plus whether selected specialist binaries exist. No specialist process runs unless you explicitly authorize the probe."
    >
      <WorkspaceGate>
        {repository ? (
          <Section title="Repository state" actions={<Radar className={cx('size-4', repository.ready ? 'text-jade' : 'text-brass')} aria-hidden />}>
            <div
              className={cx(
                'rounded-[8px] border px-4 py-3',
                repository.converged ? 'border-jade/40 bg-jade/8' : repository.state === 'missing_manifest' ? 'border-line bg-panel' : 'border-brass/40 bg-brass/8'
              )}
            >
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-[15px] font-semibold text-ink">{repository.state.replace(/_/g, ' ')}</p>
                <Badge tone={repository.ready ? 'positive' : 'neutral'}>{repository.ready ? 'ready' : 'not ready'}</Badge>
                <Badge tone={repository.converged ? 'positive' : 'caution'}>{repository.converged ? 'converged' : 'not converged'}</Badge>
                <Badge tone={repository.conflict_count > 0 ? 'danger' : 'neutral'}>{repository.conflict_count} conflicts</Badge>
                <Badge tone={repository.lock_changed ? 'caution' : 'positive'}>{repository.lock_changed ? 'lock would change' : 'lock stable'}</Badge>
                {data?.degraded ? <Badge tone="caution">degraded read</Badge> : null}
              </div>
              {repository.error ? <p className="mt-1.5 text-[12px] text-clay">{repository.error}</p> : null}
              <div className="mt-3 grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-4 border-t border-line/70 pt-3">
                <KeyValue
                  entries={[
                    ['manifest', <Mono className="break-all">{repository.manifest_path}</Mono>],
                    ['managed files', <Mono>{repository.managed_files}</Mono>]
                  ]}
                />
                <div>
                  <p className="text-[10.5px] uppercase text-ink-dim">action counts</p>
                  <ul className="mt-1 flex flex-wrap gap-x-4 gap-y-1">
                    {Object.entries(repository.actions).map(([kind, count]) => (
                      <li key={kind} className="inline-flex items-baseline gap-1.5">
                        <Mono className={cx('text-[13px]', kind === 'conflict' && Number(count) > 0 ? 'text-clay' : 'text-ink')}>{count}</Mono>
                        <span className="text-[11px] text-ink-dim">{kind}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            </div>
          </Section>
        ) : null}

        {data ? (
          <Section
            title="Integrations"
            hint="Offline availability only. Bob checks whether the binary exists; it does not run it here."
          >
            <ul className="flex flex-col gap-2">
              {data.integrations.map((integration) => (
                <li key={integration.name} className="rounded-[8px] border border-line bg-panel px-3 py-2.5">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={cx('size-1.5 rounded-full', integration.available ? 'bg-jade' : 'bg-ink-dim')} aria-hidden />
                    <Mono className="text-[13px] text-ink">{integration.name}</Mono>
                    <Badge tone={integration.selected ? 'info' : 'neutral'}>{integration.selected ? 'selected by manifest' : 'not selected'}</Badge>
                    <Badge tone={integration.available ? 'positive' : 'neutral'}>{integration.available ? 'binary available' : 'binary not found'}</Badge>
                    <Badge tone={integration.probe.state === 'not_selected' ? 'neutral' : 'caution'}>probe: {integration.probe.state.replace(/_/g, ' ')}</Badge>
                    <Badge tone="neutral">index: {integration.index.state}</Badge>
                  </div>
                  <Mono className="mt-1.5 block break-all text-ink-dim">{integration.binary_path || 'not resolved'}</Mono>
                  {integration.probe.argv.length > 0 ? (
                    <Mono className="mt-1 block break-all text-ink-dim">probe argv: {integration.probe.argv.join(' ')}</Mono>
                  ) : null}
                </li>
              ))}
            </ul>
          </Section>
        ) : null}

        <Section
          title="Explicit integration probe"
          hint="This is the only Bob surface that starts a specialist subprocess from the console."
          actions={
            <Button
              variant="danger"
              disabled={!settings.allowIntegrationProbes}
              loading={probe.pending}
              icon={<ShieldAlert className="size-3.5" aria-hidden />}
              onClick={() => setProbeConfirm(true)}
            >
              Run --probe-integrations
            </Button>
          }
        >
          <Callout tone={settings.allowIntegrationProbes ? 'caution' : 'info'} title={settings.allowIntegrationProbes ? 'Probe authority is enabled' : 'Probe authority is disabled'}>
            {PROBE_WARNING}
            <div className="mt-2">
              <Toggle
                checked={settings.allowIntegrationProbes}
                tone="danger"
                onChange={(next) => void updateSettings({ allowIntegrationProbes: next })}
                label="Allow the console to run --probe-integrations"
                hint="Off by default. Enabling it does not run anything by itself."
              />
            </div>
          </Callout>
          {probe.data ? (
            <ul className="mt-3 flex flex-col gap-2">
              {probe.data.integrations.map((integration) => (
                <li key={integration.name} className="rounded-[8px] border border-line bg-sunken px-3 py-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <Mono className="text-ink">{integration.name}</Mono>
                    <Badge tone={integration.probe.state === 'ok' ? 'positive' : 'caution'}>probe: {integration.probe.state.replace(/_/g, ' ')}</Badge>
                    <Badge tone="neutral">index: {integration.index.state}</Badge>
                  </div>
                  {integration.probe.argv.length > 0 ? (
                    <Mono className="mt-1 block break-all text-ink-dim">{integration.probe.argv.join(' ')}</Mono>
                  ) : null}
                </li>
              ))}
            </ul>
          ) : null}
          <FailureNotice result={probe.result} error={probe.error} errorCode={probe.errorCode} title="Probe refused or failed" />
        </Section>

        {data && data.warnings.length > 0 ? <WarningList warnings={data.warnings} /> : null}
        <WarningList warnings={inspect.result?.envelope?.warnings} />

        {data && data.next_actions.length > 0 ? (
          <Section title="Typed next actions" hint="Bob proposes argv. Steps needing explicit authority are gated.">
            <ul className="flex flex-col gap-1.5">
              {data.next_actions.map((action, index) => (
                <li key={`${action.id ?? action.reason}-${index}`} className="flex items-start gap-2 rounded-[6px] border border-line bg-panel px-2.5 py-2">
                  <Button
                    size="sm"
                    variant={action.requires_explicit_authority || action.effect !== 'read_only' ? 'quiet' : 'default'}
                    disabled={action.requires_explicit_authority || action.effect !== 'read_only'}
                    onClick={() => void runCommand(action.argv)}
                  >
                    {action.requires_explicit_authority ? 'Needs authority' : 'Run'}
                  </Button>
                  <div className="min-w-0 flex-1">
                    <p className="text-[12px] text-ink">{action.reason ?? action.id}</p>
                    <Mono className="mt-0.5 block break-all text-ink-dim">{action.argv.join(' ')}</Mono>
                    <div className="mt-1 flex flex-wrap gap-1.5">
                      <Badge tone={action.effect === 'read_only' ? 'positive' : 'caution'}>{action.effect.replace(/_/g, ' ')}</Badge>
                      {action.requires_explicit_authority ? <Badge tone="danger">requires explicit authority</Badge> : null}
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          </Section>
        ) : null}

        <FailureNotice result={inspect.result} error={inspect.error} errorCode={inspect.errorCode} />
        {inspect.result?.envelope ? <NextActionsList envelope={inspect.result.envelope} onRunCommand={(argv) => void runCommand(argv)} /> : null}
        <RunMeta result={probe.result ?? inspect.result} />

        <Modal
          open={probeConfirm}
          title="Run the explicit integration probe?"
          description={PROBE_WARNING}
          onClose={() => setProbeConfirm(false)}
          footer={
            <>
              <Button variant="ghost" onClick={() => setProbeConfirm(false)}>
                Cancel
              </Button>
              <Button
                variant="danger"
                onClick={async () => {
                  setProbeConfirm(false)
                  await probe.run({ confirmed: true })
                }}
              >
                Run probe
              </Button>
            </>
          }
        >
          <KeyValue
            entries={[
              ['workspace', <Mono className="break-all">{workspace}</Mono>],
              ['command', <Mono>bob inspect --probe-integrations --json</Mono>]
            ]}
          />
        </Modal>
      </WorkspaceGate>
    </PanelShell>
  )
}

export default InspectPanel
