import { useCallback, useEffect, useState } from 'react'
import { ArrowRight, ListChecks, Radar, ShieldCheck } from 'lucide-react'
import { envelopePayload } from '@shared/envelope'
import { KIND_META, shortDigest } from '@shared/format'
import type { CheckData, InspectData, PlanAction } from '@shared/types'
import { Badge, Button, Callout, EmptyState, KeyValue, Mono, Section, SectionTitle, Skeleton, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, NextActionsList, RefreshButton, RunMeta, useRunCommand } from '../components/run'
import { PlanSummaryBar } from '../components/plan'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

type Verdict = 'converged' | 'drift' | 'conflict' | 'unmanaged' | 'unknown'

const VERDICT_COPY: Record<Verdict, { title: string; tone: 'positive' | 'caution' | 'danger' | 'info'; hint: string }> = {
  converged: {
    title: 'Converged',
    tone: 'positive',
    hint: 'Every managed path matches the recipe and bob.lock. A repeated apply is a no-op.'
  },
  drift: {
    title: 'Drift',
    tone: 'caution',
    hint: 'Managed state would change. Review the plan before applying anything.'
  },
  conflict: {
    title: 'Conflicts block apply',
    tone: 'danger',
    hint: 'Bob cannot prove ownership for at least one path, so the complete apply is refused.'
  },
  unmanaged: {
    title: 'Not a Bob workspace yet',
    tone: 'info',
    hint: 'No bob.yaml was found. Initialize a manifest to start tracking ownership.'
  },
  unknown: {
    title: 'Not checked yet',
    tone: 'info',
    hint: 'Run inspect and check to read the current convergence state.'
  }
}

export function OverviewPanel(): JSX.Element {
  const { workspace, files, plan, navigate, revision } = useStore()
  const inspect = useRun<InspectData>('inspect')
  const check = useRun<CheckData>('check')
  const runCommand = useRunCommand()
  const [verdict, setVerdict] = useState<Verdict>('unknown')

  const refresh = useCallback(async () => {
    if (!workspace) return
    const [inspectResult, checkResult] = await Promise.all([inspect.run(), check.run()])
    const inspectData = envelopePayload<InspectData>(inspectResult.envelope)
    const checkData = envelopePayload<CheckData>(checkResult.envelope)
    if (!inspectData && !checkData) {
      setVerdict('unknown')
      return
    }
    if (inspectData?.repository.state === 'missing_manifest') {
      setVerdict('unmanaged')
      return
    }
    const conflicts = checkData?.plan?.conflict_count ?? 0
    if (conflicts > 0) {
      setVerdict('conflict')
      return
    }
    setVerdict(checkData?.clean ? 'converged' : 'drift')
  }, [workspace, inspect, check])

  useEffect(() => {
    void refresh()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, revision])

  const checkData = check.data
  const inspectData = inspect.data
  const actions: PlanAction[] = checkData?.plan?.actions ?? plan?.actions ?? []
  const conflicts = actions.filter((action) => action.kind === 'conflict')
  const copy = VERDICT_COPY[verdict]

  return (
    <PanelShell
      featureId="overview"
      actions={
        <>
          <RefreshButton pending={inspect.pending || check.pending} onClick={() => void refresh()} />
          <Button variant="quiet" icon={<ListChecks className="size-3.5" aria-hidden />} onClick={() => navigate('plan')}>
            Open plan
          </Button>
          <Button variant="primary" icon={<ShieldCheck className="size-3.5" aria-hidden />} onClick={() => void check.run()}>
            Run check
          </Button>
        </>
      }
    >
      <WorkspaceGate>
        <div className="grid grid-cols-[minmax(0,1fr)_20rem] items-start gap-5">
          {/* anchor: the convergence verdict */}
          <section className="flex flex-col gap-4">
            <div
              className={cx(
                'rounded-[10px] border px-5 py-4',
                copy.tone === 'positive' && 'border-jade/40 bg-jade/8',
                copy.tone === 'caution' && 'border-brass/40 bg-brass/8',
                copy.tone === 'danger' && 'border-clay/45 bg-clay/8',
                copy.tone === 'info' && 'border-line bg-panel'
              )}
            >
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0">
                  <p className="text-[10.5px] uppercase text-ink-dim" style={{ letterSpacing: '0.05em' }}>
                    {files?.lockRecipe ? `${files.lockRecipe.id}@${files.lockRecipe.version}` : 'no recipe'}
                  </p>
                  <h2 className="mt-1 text-[22px] leading-tight font-semibold text-ink">{copy.title}</h2>
                  <p className="mt-1.5 max-w-xl text-[12.5px] leading-relaxed text-ink-muted">{copy.hint}</p>
                </div>
                <Radar
                  className={cx(
                    'size-8 shrink-0',
                    copy.tone === 'positive' && 'text-jade',
                    copy.tone === 'caution' && 'text-brass',
                    copy.tone === 'danger' && 'text-clay',
                    copy.tone === 'info' && 'text-ink-dim'
                  )}
                  aria-hidden
                />
              </div>

              <dl className="mt-4 flex flex-wrap gap-x-8 gap-y-2 border-t border-line/70 pt-3">
                {[
                  ['conflicts', String(checkData?.plan?.conflict_count ?? plan?.conflictCount ?? 0)],
                  ['managed files', String(files?.lockFiles.length ?? inspectData?.repository.managed_files ?? 0)],
                  ['lock changed', inspectData ? String(inspectData.repository.lock_changed) : '—'],
                  ['plan digest', shortDigest(checkData?.plan_digest ?? plan?.digest ?? '')]
                ].map(([label, value]) => (
                  <div key={label}>
                    <dt className="text-[10.5px] uppercase text-ink-dim">{label}</dt>
                    <dd className={cx('font-mono text-[13px]', label === 'conflicts' && Number(value) > 0 ? 'text-clay' : 'text-ink')}>{value}</dd>
                  </div>
                ))}
              </dl>
            </div>

            {inspect.pending || check.pending ? (
              <div className="flex flex-col gap-2">
                <Skeleton className="h-9 w-full" />
                <Skeleton className="h-40 w-full" />
              </div>
            ) : null}

            {actions.length > 0 ? (
              <Section title="Plan actions" hint="From the most recent check. Open the plan panel for content previews and diffs.">
                <PlanSummaryBar actions={actions} digest={checkData?.plan_digest ?? plan?.digest} />
                {conflicts.length > 0 ? (
                  <ul className="flex flex-col gap-1.5">
                    {conflicts.slice(0, 8).map((action) => (
                      <li key={action.path}>
                        <button
                          type="button"
                          onClick={() => navigate('path', { path: action.path, code: action.code })}
                          className="flex w-full items-center gap-3 rounded-[8px] border border-clay/35 bg-clay/8 px-3 py-2 text-left hover:border-clay/60"
                        >
                          <Badge tone="danger">{action.code}</Badge>
                          <Mono className="min-w-0 flex-1 truncate text-ink">{action.path}</Mono>
                          <Mono className="shrink-0 text-ink-dim">
                            {shortDigest(action.current_sha256, 8)} ≠ {shortDigest(action.locked_sha256 ?? action.desired_sha256, 8)}
                          </Mono>
                          <ArrowRight className="size-3.5 shrink-0 text-clay" aria-hidden />
                        </button>
                      </li>
                    ))}
                    {conflicts.length > 8 ? (
                      <li className="text-[11.5px] text-ink-dim">
                        and {conflicts.length - 8} more — <button type="button" className="text-copper underline" onClick={() => navigate('plan')}>open the plan</button>
                      </li>
                    ) : null}
                  </ul>
                ) : null}
                {conflicts.length === 0 && (actions.some((action) => action.kind !== 'unchanged') || inspectData?.repository.lock_changed) ? (
                  <Callout tone="info" title="Safe to apply">
                    Every action is conflict-free. Apply writes only absent, identical, or previously managed files, and it can be bound to
                    the digest you just reviewed.
                  </Callout>
                ) : null}
              </Section>
            ) : null}

            <FailureNotice result={check.data ? null : check.result} error={check.data ? null : check.error} errorCode={check.data ? null : check.errorCode} title="Check could not run" />
            <FailureNotice result={inspect.data ? null : inspect.result} error={inspect.data ? null : inspect.error} errorCode={inspect.data ? null : inspect.errorCode} title="Inspect failed" />

            {check.result?.envelope ? <NextActionsList envelope={check.result.envelope} onRunCommand={(argv) => void runCommand(argv)} /> : null}
            <RunMeta result={check.result ?? inspect.result} />
          </section>

          {/* side column: facts, not filler */}
          <aside className="flex flex-col gap-5">
            <Section title="Workspace">
              {files ? (
                <KeyValue
                  entries={[
                    ['path', <Mono className="break-all">{workspace}</Mono>],
                    ['bob.yaml', files.manifestPath ? <Badge tone="positive">present</Badge> : <Badge tone="danger">missing</Badge>],
                    ['bob.lock', files.lockPath ? <Badge tone="positive">present</Badge> : <Badge tone="neutral">absent</Badge>],
                    ['recipe', files.lockRecipe ? <Mono>{files.lockRecipe.id}@{files.lockRecipe.version}</Mono> : <span className="text-ink-dim">—</span>],
                    ['lock entries', <Mono>{files.lockFiles.length}</Mono>]
                  ]}
                />
              ) : (
                <EmptyState title="Nothing read yet" hint="Select a workspace to read its manifest and lock ledger." />
              )}
            </Section>

            <Section title="Integrations" hint="Offline binary availability. No specialist process runs here.">
              {inspectData ? (
                <ul className="flex flex-col gap-2">
                  {inspectData.integrations.map((integration) => (
                    <li key={integration.name} className="flex items-center gap-2">
                      <span className={cx('size-1.5 rounded-full', integration.available ? 'bg-jade' : 'bg-ink-dim')} aria-hidden />
                      <Mono className="text-ink">{integration.name}</Mono>
                      <Badge tone={integration.selected ? 'info' : 'neutral'}>{integration.selected ? 'selected' : 'not selected'}</Badge>
                      <Mono className="ml-auto max-w-[9rem] truncate text-ink-dim" title={integration.binary_path}>
                        {integration.binary_path || 'not found'}
                      </Mono>
                    </li>
                  ))}
                  {inspectData.degraded ? <Badge tone="caution">degraded read</Badge> : null}
                </ul>
              ) : (
                <p className="text-[12px] text-ink-dim">Run inspect to read integration availability.</p>
              )}
            </Section>

            {actions.length > 0 ? (
              <Section title="Action legend">
                <ul className="flex flex-col gap-1.5">
                  {(Object.keys(KIND_META) as (keyof typeof KIND_META)[]).map((kind) => (
                    <li key={kind} className="flex items-start gap-2">
                      <Badge tone={KIND_META[kind].tone}>{kind}</Badge>
                      <span className="text-[11.5px] leading-snug text-ink-muted">{KIND_META[kind].detail}</span>
                    </li>
                  ))}
                </ul>
              </Section>
            ) : null}

            {inspectData?.repository.state ? (
              <Section title="Repository state">
                <KeyValue
                  entries={[
                    ['state', <Mono>{inspectData.repository.state}</Mono>],
                    ['ready', String(inspectData.repository.ready)],
                    ['converged', String(inspectData.repository.converged)],
                    ['lock changed', String(inspectData.repository.lock_changed)]
                  ]}
                />
              </Section>
            ) : null}
          </aside>
        </div>
      </WorkspaceGate>
    </PanelShell>
  )
}

export default OverviewPanel
