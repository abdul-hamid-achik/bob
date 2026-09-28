import { useCallback, useEffect, useRef, useState } from 'react'
import { Radio } from 'lucide-react'
import { isPlanDigest, shortDigest } from '@shared/format'
import { parseWatchBuffer } from '@shared/parseWatch'
import type { PlanAction, PlanData } from '@shared/types'
import { Badge, Button, CopyButton, Mono, Section, Segmented, Toggle, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, NextActionsList, RunButton, RunMeta, StreamConsole, WarningList, useRunCommand, useStream } from '../components/run'
import { ActionDetail, DEFAULT_PLAN_FILTERS, PlanFilters, PlanSummaryBar, filterActions, type PlanFilterState } from '../components/plan'
import { buildArgv } from '@shared/argv'
import { featureById } from '@shared/features'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

type Detail = 'full' | 'compact'

export function PlanPanel(): JSX.Element {
  const { workspace, navigate, revision, recordPlan } = useStore()
  const plan = useRun<PlanData>('plan')
  const runCommand = useRunCommand()
  const stream = useStream()
  const [filters, setFilters] = useState<PlanFilterState>(DEFAULT_PLAN_FILTERS)
  const [content, setContent] = useState(true)
  const [diff, setDiff] = useState(false)
  const [conflictsOnly, setConflictsOnly] = useState(false)
  const [selected, setSelected] = useState<PlanAction | null>(null)
  const [detail, setDetail] = useState<Detail>('full')
  const [watching, setWatching] = useState(false)
  const tickCount = useRef(0)

  const flags = { content, diff, 'conflicts-only': conflictsOnly }

  const run = useCallback(async () => {
    const result = await plan.run({ flags })
    setSelected(null)
    return result
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [content, diff, conflictsOnly, workspace])

  useEffect(() => {
    if (workspace) void run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, revision])

  const actions = plan.data?.actions ?? []
  const visible = filterActions(actions, filters)
  const digest = plan.data?.plan_digest ?? ''

  /* watch: stream Bob's human re-plan, and refresh typed data on each tick */
  useEffect(() => {
    if (!watching || !stream.text) return
    const ticks = parseWatchBuffer(stream.text).length
    if (ticks > tickCount.current) {
      tickCount.current = ticks
      const timer = setTimeout(() => void run(), 350)
      return () => clearTimeout(timer)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [stream.text, watching])

  const toggleWatch = async (): Promise<void> => {
    if (watching) {
      await stream.stop()
      setWatching(false)
      tickCount.current = 0
      return
    }
    const feature = featureById('watch')
    if (!feature) return
    const argv = buildArgv(feature, { workspace, json: false })
    tickCount.current = 0
    await stream.start('watch', argv, workspace)
    setWatching(true)
  }

  useEffect(() => {
    return () => {
      if (watching) void stream.stop()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return (
    <PanelShell
      featureId="plan"
      actions={
        <>
          <Button
            variant="quiet"
            icon={<Radio className={cx('size-3.5', watching && 'text-jade')} aria-hidden />}
            onClick={() => void toggleWatch()}
            title="bob plan --watch and --json are mutually exclusive, so watch streams the human output"
          >
            {watching ? 'Stop watch' : 'Watch bob.yaml'}
          </Button>
          <RunButton label="Run plan" pending={plan.pending} onClick={() => void run()} />
        </>
      }
    >
      <WorkspaceGate>
        <Section
          title="Planner flags"
          hint="Plan never writes. --content and --diff add bounded previews; --conflicts-only compacts the projection for capped harnesses."
          actions={
            <Segmented
              ariaLabel="Detail level"
              value={detail}
              onChange={setDetail}
              options={[
                { value: 'full', label: 'Full plan' },
                { value: 'compact', label: 'Conflicts only' }
              ]}
            />
          }
        >
          <div className="grid grid-cols-3 gap-4 rounded-[8px] border border-line bg-panel px-3 py-2.5">
            <Toggle checked={content} onChange={setContent} label="--content" hint="Bounded desired content for create/update/conflict, plus current content for conflicts." />
            <Toggle checked={diff} onChange={setDiff} label="--diff" hint="Unified diffs for create and update actions." />
            <Toggle
              checked={conflictsOnly}
              onChange={(next) => {
                setConflictsOnly(next)
                setDetail(next ? 'compact' : 'full')
              }}
              label="--conflicts-only"
              hint="Show only conflicting actions."
            />
          </div>
        </Section>

        {watching || stream.text ? (
          <Section title="Live watch" hint="bob plan --watch re-plans whenever bob.yaml changes. The typed table below refreshes on every tick.">
            <StreamConsole stream={stream} title="bob plan --watch" onStop={() => void toggleWatch()} height="14rem" />
          </Section>
        ) : null}

        <FailureNotice result={plan.result} error={plan.error} errorCode={plan.errorCode} />
        <WarningList warnings={plan.result?.envelope?.warnings} />

        {plan.data ? (
          <>
            <Section
              title="Plan digest"
              hint="Bind apply or upgrade to this exact digest so a workspace that changed under you is refused with exit 5."
              actions={
                <>
                  <CopyButton value={digest} label="Copy digest" />
                  <Button
                    variant="primary"
                    disabled={!isPlanDigest(digest)}
                    onClick={() => {
                      recordPlan({
                        digest,
                        digestVersion: plan.data?.plan_digest_version ?? 1,
                        recipe: plan.data?.recipe ?? null,
                        actions,
                        conflictCount: plan.data?.conflict_count ?? 0,
                        lockChanged: Boolean(plan.data?.lock_changed),
                        capturedAt: new Date().toISOString(),
                        source: 'plan',
                        workspace
                      })
                      navigate('apply', { digest })
                    }}
                  >
                    Review in apply
                  </Button>
                </>
              }
            >
              <div className="flex flex-wrap items-center gap-4 rounded-[8px] border border-line bg-panel px-3 py-2.5">
                <div className="min-w-0">
                  <p className="text-[10.5px] uppercase text-ink-dim">sha256 digest</p>
                  <Mono className={cx('break-all text-[13px]', plan.data.conflict_count > 0 ? 'text-clay' : 'text-jade')}>{digest || '—'}</Mono>
                </div>
                <div className="ml-auto flex flex-wrap items-center gap-2">
                  <Badge tone={plan.data.recipe ? 'info' : 'neutral'}>
                    {plan.data.recipe ? `${plan.data.recipe.id}@${plan.data.recipe.version}` : 'no recipe'}
                  </Badge>
                  <Badge tone={plan.data.lock_changed ? 'caution' : 'positive'}>
                    {plan.data.lock_changed ? 'lock would change' : 'lock unchanged'}
                  </Badge>
                  <Badge tone={plan.data.conflict_count > 0 ? 'danger' : 'positive'}>
                    {plan.data.conflict_count} conflict{plan.data.conflict_count === 1 ? '' : 's'}
                  </Badge>
                  <Mono className="text-ink-dim">digest v{plan.data.plan_digest_version}</Mono>
                </div>
              </div>
            </Section>

            <Section
              title={`Actions (${visible.length} of ${actions.length})`}
              hint="Select a row for hashes, mode, reason, and the bounded content diff."
            >
              <PlanFilters actions={actions} state={filters} onChange={setFilters} />
              <PlanSummaryBar actions={actions} digest={digest} />
              <div className={cx('grid gap-3', detail === 'full' && selected ? 'grid-cols-[minmax(0,1fr)_26rem]' : 'grid-cols-1')}>
                <div className="min-w-0">
                  {visible.length === 0 ? (
                    <p className="rounded-[8px] border border-dashed border-line px-3 py-6 text-[12px] text-ink-dim">
                      No action matches these filters. {detail === 'compact' ? 'Switch to the full plan to see converged paths.' : ''}
                    </p>
                  ) : (
                    <div className="max-h-[32rem] overflow-auto rounded-[8px] border border-line">
                      <table className="w-full border-collapse text-[12px]">
                        <thead className="sticky top-0 z-10 bg-raise">
                          <tr className="text-left text-[10.5px] uppercase text-ink-dim">
                            <th className="px-2.5 py-1.5 font-medium">Action</th>
                            <th className="px-2.5 py-1.5 font-medium">Path</th>
                            <th className="px-2.5 py-1.5 font-medium">Code</th>
                            <th className="px-2.5 py-1.5 font-medium">Current → desired</th>
                          </tr>
                        </thead>
                        <tbody>
                          {visible.map((action) => (
                            <tr
                              key={action.path}
                              onClick={() => setSelected(action)}
                              tabIndex={0}
                              onKeyDown={(event) => {
                                if (event.key === 'Enter' || event.key === ' ') {
                                  event.preventDefault()
                                  setSelected(action)
                                }
                              }}
                              className={cx(
                                'cursor-pointer border-b border-line/70 last:border-b-0',
                                selected?.path === action.path ? 'bg-copper/10' : 'hover:bg-raise/70'
                              )}
                            >
                              <td className="px-2.5 py-1">
                                <Badge tone={action.kind === 'conflict' ? 'danger' : action.kind === 'update' ? 'caution' : action.kind === 'create' ? 'positive' : action.kind === 'adopt' ? 'info' : 'neutral'}>
                                  {action.kind}
                                </Badge>
                              </td>
                              <td className="px-2.5 py-1">
                                <Mono className="text-ink">{action.path}</Mono>
                              </td>
                              <td className="px-2.5 py-1">
                                <Mono className="text-ink-muted">{action.code}</Mono>
                              </td>
                              <td className="px-2.5 py-1">
                                <Mono className="text-ink-dim">
                                  {shortDigest(action.current_sha256, 8)} → {shortDigest(action.desired_sha256, 8)}
                                </Mono>
                              </td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </div>
                {selected ? (
                  <div className="min-w-0 rounded-[8px] border border-line bg-panel p-3">
                    <ActionDetail action={selected} />
                    <div className="mt-3 flex flex-wrap gap-2 border-t border-line pt-3">
                      <Button size="sm" variant="quiet" onClick={() => navigate('path', { path: selected.path })}>
                        Classify this path
                      </Button>
                      <Button size="sm" variant="quiet" onClick={() => navigate('files', { path: selected.path })}>
                        Open in files
                      </Button>
                    </div>
                  </div>
                ) : null}
              </div>
            </Section>
          </>
        ) : null}

        {plan.result?.envelope ? <NextActionsList envelope={plan.result.envelope} onRunCommand={(argv) => void runCommand(argv)} /> : null}
        <RunMeta result={plan.result} />
      </WorkspaceGate>
    </PanelShell>
  )
}

export default PlanPanel
