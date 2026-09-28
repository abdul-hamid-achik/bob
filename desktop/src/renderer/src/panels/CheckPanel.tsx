import { useCallback, useEffect, useState } from 'react'
import { ShieldCheck } from 'lucide-react'
import { exitCodeMeaning } from '@shared/envelope'
import { shortDigest } from '@shared/format'
import type { CheckData } from '@shared/types'
import { Badge, Button, CodeBlock, KeyValue, Mono, Section, Toggle, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, NextActionsList, RunButton, RunMeta, WarningList, useRunCommand } from '../components/run'
import { PlanSummaryBar, PlanTable } from '../components/plan'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

export function CheckPanel(): JSX.Element {
  const { workspace, plan: snapshot, navigate, revision } = useStore()
  const check = useRun<CheckData>('check')
  const runCommand = useRunCommand()
  const [conflictsOnly, setConflictsOnly] = useState(false)

  const run = useCallback(() => check.run({ flags: { 'conflicts-only': conflictsOnly } }), [check, conflictsOnly])

  useEffect(() => {
    if (workspace) void run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, revision, conflictsOnly])

  const data = check.data
  const actions = data?.plan?.actions ?? []
  const digest = data?.plan_digest ?? ''
  const matchesSnapshot = Boolean(snapshot && digest && snapshot.digest === digest)

  const verdict = !data
    ? null
    : data.plan.conflict_count > 0
      ? { label: 'Conflicts block apply', tone: 'danger' as const, exit: 2 }
      : data.clean
        ? { label: 'Converged', tone: 'positive' as const, exit: 0 }
        : { label: 'Drift', tone: 'caution' as const, exit: 3 }

  return (
    <PanelShell
      featureId="check"
      actions={<RunButton label="Run check" pending={check.pending} onClick={() => void run()} />}
    >
      <WorkspaceGate>
        <Section
          title="CI gate"
          hint="check fails when managed repository state would change. It never writes."
          actions={
            <Toggle checked={conflictsOnly} onChange={setConflictsOnly} label="--conflicts-only" hint="Compact output for capped agent harnesses." />
          }
        >
          <div
            className={cx(
              'flex flex-wrap items-center gap-4 rounded-[8px] border px-4 py-3',
              verdict?.tone === 'positive' && 'border-jade/40 bg-jade/8',
              verdict?.tone === 'caution' && 'border-brass/40 bg-brass/8',
              verdict?.tone === 'danger' && 'border-clay/45 bg-clay/8',
              !verdict && 'border-line bg-panel'
            )}
          >
            <ShieldCheck
              className={cx(
                'size-6 shrink-0',
                verdict?.tone === 'positive' && 'text-jade',
                verdict?.tone === 'caution' && 'text-brass',
                verdict?.tone === 'danger' && 'text-clay',
                !verdict && 'text-ink-dim'
              )}
              aria-hidden
            />
            <div className="min-w-0">
              <p className="text-[16px] font-semibold text-ink">{verdict?.label ?? 'Not checked yet'}</p>
              <p className="mt-0.5 text-[12px] text-ink-muted">
                {verdict ? `exit ${verdict.exit} · ${exitCodeMeaning(verdict.exit)}` : 'Run check to read the convergence gate.'}
              </p>
            </div>
            <div className="ml-auto flex flex-col items-end gap-1">
              <Mono className={cx('text-[12px]', matchesSnapshot ? 'text-jade' : 'text-ink-muted')} title={digest}>
                {shortDigest(digest)}
              </Mono>
              {snapshot ? (
                <Badge tone={matchesSnapshot ? 'positive' : 'caution'}>
                  {matchesSnapshot ? 'matches the reviewed plan' : 'differs from the reviewed plan'}
                </Badge>
              ) : null}
            </div>
          </div>
        </Section>

        <FailureNotice
          result={data ? null : check.result}
          error={data ? null : check.error}
          errorCode={data ? null : check.errorCode}
          title="Check could not run"
        />
        <WarningList warnings={check.result?.envelope?.warnings} />

        {data ? (
          <Section title="Plan behind the verdict" hint="check carries the same complete-plan digest the planner produced.">
            <PlanSummaryBar actions={actions} digest={digest} />
            <div className="max-h-[26rem] overflow-auto">
              <PlanTable actions={actions} />
            </div>
            <KeyValue
              className="mt-1"
              entries={[
                ['clean', <Mono>{String(data.clean)}</Mono>],
                ['conflicts', <Mono className={data.plan.conflict_count > 0 ? 'text-clay' : ''}>{data.plan.conflict_count}</Mono>],
                ['lock changed', <Mono>{String(data.plan.lock_changed)}</Mono>],
                ['recipe', <Mono>{data.plan.recipe ? `${data.plan.recipe.id}@${data.plan.recipe.version}` : '—'}</Mono>],
                ['digest version', <Mono>{data.plan_digest_version}</Mono>]
              ]}
            />
            <div className="flex flex-wrap gap-2">
              <Button variant="quiet" onClick={() => navigate('plan')}>
                Open in plan
              </Button>
              {data.plan.conflict_count > 0 ? (
                <Button variant="danger" onClick={() => navigate('plan')}>
                  Resolve conflicts
                </Button>
              ) : (
                <Button variant="primary" onClick={() => navigate('apply', { digest })}>
                  Apply this plan
                </Button>
              )}
            </div>
          </Section>
        ) : null}

        <Section title="Use it in CI" hint="The same command, verbatim, for a workflow step.">
          <CodeBlock copy>{`bob check --json\n# exit 0 converged · exit 2 conflicts · exit 3 drift`}</CodeBlock>
        </Section>

        {check.result?.envelope ? <NextActionsList envelope={check.result.envelope} onRunCommand={(argv) => void runCommand(argv)} /> : null}
        <RunMeta result={check.result} />
      </WorkspaceGate>
    </PanelShell>
  )
}

export default CheckPanel
