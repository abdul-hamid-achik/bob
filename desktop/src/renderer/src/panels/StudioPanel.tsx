import { useCallback, useEffect, useState } from 'react'
import { ExternalLink, Monitor } from 'lucide-react'
import type { CheckData, InspectData, StatsData } from '@shared/types'
import { Badge, Button, Callout, KeyValue, Mono, Section, Tabs, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, RunMeta } from '../components/run'
import { PlanSummaryBar, PlanTable } from '../components/plan'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

type Tab = 'overview' | 'plan' | 'stats'

/**
 * Native reproduction of Bob's read-only Studio board.
 * One coherent offline read per refresh, and no apply, shell, editor,
 * indexing, probing, or repair action anywhere in this panel.
 */
export function StudioPanel(): JSX.Element {
  const { workspace, binary, revision, toast } = useStore()
  const inspect = useRun<InspectData>('inspect')
  const check = useRun<CheckData>('check')
  const stats = useRun<StatsData>('stats')
  const [tab, setTab] = useState<Tab>('overview')
  const [lastRefresh, setLastRefresh] = useState<string | null>(null)
  const [singlePane, setSinglePane] = useState(false)

  const refresh = useCallback(async () => {
    await Promise.all([inspect.run(), check.run(), stats.run({ flags: { since: '7d' } })])
    setLastRefresh(new Date().toISOString())
  }, [inspect, check, stats])

  useEffect(() => {
    if (workspace) void refresh()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, revision])

  const repository = inspect.data?.repository
  const plan = check.data?.plan
  const summary = stats.data?.stats

  const openTui = async (): Promise<void> => {
    if (!binary?.path) {
      toast('No bob binary resolved', 'danger')
      return
    }
    const command = `${binary.path} studio ${workspace}${singlePane ? ' --single-pane' : ''}`
    const outcome = await bridge().launchTerminal(command)
    if (!outcome.launched) toast(outcome.error ?? 'Could not launch a terminal', 'danger')
    else toast('Studio opened in your terminal', 'positive')
  }

  return (
    <PanelShell
      featureId="studio"
      actions={
        <>
          <label className="flex items-center gap-1.5 text-[11.5px] text-ink-muted">
            <input type="checkbox" checked={singlePane} onChange={(event) => setSinglePane(event.target.checked)} className="accent-copper" />
            --single-pane
          </label>
          <Button variant="quiet" loading={inspect.pending || check.pending || stats.pending} onClick={() => void refresh()}>
            Refresh board
          </Button>
          <Button variant="default" icon={<ExternalLink className="size-3.5" aria-hidden />} onClick={() => void openTui()}>
            Open the real TUI
          </Button>
        </>
      }
      subtitle="The read-only Overview, Plan, and Stats board. This panel performs one coherent offline repository read per refresh and exposes no mutating action."
    >
      <WorkspaceGate>
        <Callout tone="info" title="Studio is strictly read-only">
          No apply, shell, editor, indexing, probing, or repair shortcut exists here or in the TUI. Use the Plan and Apply panels when you
          want to change something, with their own confirmations.
        </Callout>

        <Tabs
          ariaLabel="Studio projections"
          value={tab}
          onChange={setTab}
          items={[
            { value: 'overview', label: 'Overview' },
            { value: 'plan', label: 'Plan', badge: plan ? <Badge tone={plan.conflict_count > 0 ? 'danger' : 'positive'}>{plan.conflict_count}</Badge> : undefined },
            { value: 'stats', label: 'Stats' }
          ]}
        />

        {lastRefresh ? (
          <p className="text-[11px] text-ink-dim">
            last coherent read <Mono>{new Date(lastRefresh).toLocaleTimeString()}</Mono>
          </p>
        ) : null}

        {tab === 'overview' ? (
          <Section title="Overview" actions={<Monitor className="size-4 text-ink-dim" aria-hidden />}>
            {repository ? (
              <div className={cx('rounded-[8px] border px-4 py-3', repository.converged ? 'border-jade/40 bg-jade/8' : 'border-brass/40 bg-brass/8')}>
                <p className="text-[16px] font-semibold text-ink">{repository.state.replace(/_/g, ' ')}</p>
                <p className="mt-1 text-[12px] text-ink-muted">
                  {repository.converged ? 'Managed state matches the recipe and the lock ledger.' : 'Managed state would change on the next apply.'}
                </p>
                <KeyValue
                  className="mt-3"
                  entries={[
                    ['managed files', <Mono>{repository.managed_files}</Mono>],
                    ['conflicts', <Mono className={repository.conflict_count > 0 ? 'text-clay' : ''}>{repository.conflict_count}</Mono>],
                    ['lock changed', <Mono>{String(repository.lock_changed)}</Mono>],
                    ['integrations', inspect.data?.integrations.map((entry) => (
                      <span key={entry.name} className="mr-2 inline-flex items-center gap-1">
                        <span className={cx('size-1.5 rounded-full', entry.available ? 'bg-jade' : 'bg-ink-dim')} aria-hidden />
                        <Mono>{entry.name}</Mono>
                      </span>
                    )) ?? '—']
                  ]}
                />
              </div>
            ) : (
              <p className="text-[12px] text-ink-dim">Refresh the board to read the workspace.</p>
            )}
          </Section>
        ) : null}

        {tab === 'plan' ? (
          <Section title="Plan" hint="The same plan the CLI produces, rendered read-only.">
            {plan ? (
              <>
                <PlanSummaryBar actions={plan.actions} digest={plan.plan_digest} />
                <div className="max-h-[30rem] overflow-auto">
                  <PlanTable actions={plan.actions} />
                </div>
                <KeyValue
                  className="mt-1"
                  entries={[
                    ['clean', <Mono>{String(check.data?.clean)}</Mono>],
                    ['digest', <Mono className="break-all">{plan.plan_digest}</Mono>],
                    ['recipe', <Mono>{plan.recipe.id}@{plan.recipe.version}</Mono>]
                  ]}
                />
              </>
            ) : (
              <p className="text-[12px] text-ink-dim">No plan read yet.</p>
            )}
          </Section>
        ) : null}

        {tab === 'stats' ? (
          <Section title="Stats" hint="Aggregate opt-in local usage for the last seven days.">
            {summary ? (
              <>
                <div className="grid grid-cols-5 gap-3">
                  {[
                    ['events', summary.events],
                    ['successes', summary.successes],
                    ['failures', summary.failures],
                    ['conflicts', summary.conflict_events],
                    ['drift', summary.drift_events]
                  ].map(([label, value]) => (
                    <div key={String(label)} className="rounded-[8px] border border-line bg-panel px-3 py-2.5">
                      <p className="text-[10.5px] uppercase text-ink-dim">{label}</p>
                      <p className="font-mono text-[20px] tabular-nums text-ink">{String(value)}</p>
                    </div>
                  ))}
                </div>
                <KeyValue
                  entries={[
                    ['telemetry', <Badge tone={stats.data?.enabled ? 'caution' : 'positive'}>{stats.data?.enabled ? 'enabled' : 'disabled'}</Badge>],
                    ['workspace id', <Mono>{summary.workspace_id}</Mono>],
                    ['window', <Mono>{summary.since.slice(0, 10)} → {summary.until.slice(0, 10)}</Mono>]
                  ]}
                />
              </>
            ) : (
              <p className="text-[12px] text-ink-dim">Telemetry is disabled or nothing was recorded in this window.</p>
            )}
          </Section>
        ) : null}

        <FailureNotice result={inspect.result} error={inspect.error} errorCode={inspect.errorCode} title="Board read failed" />
        <RunMeta result={check.result ?? inspect.result} />
      </WorkspaceGate>
    </PanelShell>
  )
}

export default StudioPanel
