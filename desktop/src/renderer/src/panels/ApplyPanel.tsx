import { useCallback, useEffect, useMemo, useState } from 'react'
import { KeyRound, ShieldAlert } from 'lucide-react'
import { isPlanDigest, shortDigest } from '@shared/format'
import type { ApplyData, PlanData } from '@shared/types'
import { Badge, Button, Callout, KeyValue, Modal, Mono, Section, Toggle, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, NextActionsList, RunMeta, WarningList, useRunCommand } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

export function ApplyPanel(): JSX.Element {
  const { workspace, plan: snapshot, settings, updateSettings, navigate, navParams, reloadFiles, bump } = useStore()
  const fresh = useRun<PlanData>('plan')
  const apply = useRun<ApplyData>('apply')
  const runCommand = useRunCommand()
  const [bindDigest, setBindDigest] = useState(settings.requireDigestBoundApply)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [acknowledge, setAcknowledge] = useState('')

  const reviewedDigest = navParams.digest ?? snapshot?.digest ?? ''
  const freshDigest = fresh.data?.plan_digest ?? ''
  const actions = useMemo(() => fresh.data?.actions ?? snapshot?.actions ?? [], [fresh.data, snapshot])
  const conflicts = actions.filter((action) => action.kind === 'conflict')
  const writes = actions.filter((action) => action.kind === 'create' || action.kind === 'update')
  const adopts = actions.filter((action) => action.kind === 'adopt')

  const digestToBind = bindDigest ? (freshDigest || reviewedDigest) : ''
  const digestMatches = !bindDigest || !freshDigest || !reviewedDigest || freshDigest === reviewedDigest

  const preflight = useCallback(async () => {
    await fresh.run()
  }, [fresh])

  useEffect(() => {
    if (workspace) void preflight()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace])

  const doApply = async (): Promise<void> => {
    setConfirmOpen(false)
    const result = await apply.run({
      confirmed: true,
      flags: digestToBind && isPlanDigest(digestToBind) ? { 'expect-plan-digest': digestToBind } : {}
    })
    if (result.envelope?.ok) {
      await reloadFiles()
      bump()
      await preflight()
    }
  }

  const blocked = conflicts.length > 0
  const receipt = apply.data

  return (
    <PanelShell
      featureId="apply"
      actions={
        <>
          <Button variant="quiet" onClick={() => void preflight()} loading={fresh.pending}>
            Re-run preflight plan
          </Button>
          <Button variant="primary" disabled={blocked || !digestMatches || actions.length === 0} onClick={() => setConfirmOpen(true)}>
            Apply plan
          </Button>
        </>
      }
    >
      <WorkspaceGate>
        <Section
          title="Digest binding"
          hint="Apply only the plan you actually reviewed. If the workspace changed underneath, Bob refuses with exit 5 instead of writing a different plan."
        >
          <div className="flex flex-col gap-3 rounded-[8px] border border-line bg-panel px-3 py-3">
            <Toggle
              checked={bindDigest}
              onChange={(next) => {
                setBindDigest(next)
                void updateSettings({ requireDigestBoundApply: next })
              }}
              label="Bind apply to the reviewed plan digest"
              hint="Passes --expect-plan-digest sha256:<64-hex> from the fresh preflight plan."
            />
            <KeyValue
              entries={[
                ['reviewed digest', <Mono className={reviewedDigest ? 'text-ink' : 'text-ink-dim'}>{reviewedDigest || 'none captured yet'}</Mono>],
                ['fresh preflight digest', <Mono className={freshDigest ? 'text-ink' : 'text-ink-dim'}>{freshDigest || 'run preflight'}</Mono>],
                [
                  'binding',
                  bindDigest ? (
                    digestMatches ? (
                      <Badge tone="positive">will apply {shortDigest(digestToBind)}</Badge>
                    ) : (
                      <Badge tone="caution">digest changed — re-review the plan</Badge>
                    )
                  ) : (
                    <Badge tone="caution">unbound apply</Badge>
                  )
                ]
              ]}
            />
            {bindDigest && !digestMatches ? (
              <Callout tone="caution" title="The workspace moved">
                The fresh plan digest no longer matches the one you reviewed. Open the plan, review the new actions, then apply again.
                <div className="mt-2">
                  <Button size="sm" variant="quiet" onClick={() => navigate('plan')}>
                    Review the new plan
                  </Button>
                </div>
              </Callout>
            ) : null}
          </div>
        </Section>

        {blocked ? (
          <Callout
            tone="danger"
            title={`${conflicts.length} conflict${conflicts.length === 1 ? ' blocks' : 's block'} the complete apply`}
          >
            Bob preflights the entire plan and writes nothing when any target conflicts. Resolve each conflict first — the ownership
            playbook walks through the bounded choices.
            <ul className="mt-2 flex flex-col gap-1">
              {conflicts.slice(0, 10).map((action) => (
                <li key={action.path} className="flex items-center gap-2">
                  <Mono className="text-clay">{action.code}</Mono>
                  <Mono className="min-w-0 flex-1 truncate text-ink-muted">{action.path}</Mono>
                  <button type="button" className="text-[11px] text-copper underline" onClick={() => navigate('path', { path: action.path, code: action.code })}>
                    classify
                  </button>
                </li>
              ))}
            </ul>
            <div className="mt-2">
              <Button size="sm" variant="quiet" onClick={() => navigate('playbooks', { playbook: 'resolve-ownership-conflict' })}>
                Open the conflict playbook
              </Button>
            </div>
          </Callout>
        ) : null}

        <FailureNotice result={fresh.result} error={fresh.error} errorCode={fresh.errorCode} title="Preflight plan failed" />
        <WarningList warnings={fresh.result?.envelope?.warnings} />

        {actions.length > 0 ? (
          <Section title="What apply will do" hint="Whole-file writes only, and only where ownership is proven.">
            <div className="grid grid-cols-3 gap-3">
              <div className="rounded-[8px] border border-line bg-panel px-3 py-2.5">
                <p className="text-[10.5px] uppercase text-ink-dim">writes</p>
                <p className="font-mono text-[20px] text-jade">{writes.length}</p>
                <p className="text-[11px] text-ink-dim">create + update</p>
              </div>
              <div className="rounded-[8px] border border-line bg-panel px-3 py-2.5">
                <p className="text-[10.5px] uppercase text-ink-dim">adopts</p>
                <p className="font-mono text-[20px] text-steel">{adopts.length}</p>
                <p className="text-[11px] text-ink-dim">identical unmanaged files</p>
              </div>
              <div className="rounded-[8px] border border-line bg-panel px-3 py-2.5">
                <p className="text-[10.5px] uppercase text-ink-dim">conflicts</p>
                <p className={cx('font-mono text-[20px]', conflicts.length > 0 ? 'text-clay' : 'text-ink-dim')}>{conflicts.length}</p>
                <p className="text-[11px] text-ink-dim">one blocks everything</p>
              </div>
            </div>
            <ul className="max-h-64 overflow-auto rounded-[8px] border border-line bg-sunken px-3 py-2">
              {actions
                .filter((action) => action.kind !== 'unchanged')
                .map((action) => (
                  <li key={action.path} className="flex items-center gap-2 py-0.5">
                    <Badge tone={action.kind === 'conflict' ? 'danger' : action.kind === 'update' ? 'caution' : action.kind === 'adopt' ? 'info' : 'positive'}>
                      {action.kind}
                    </Badge>
                    <Mono className="min-w-0 flex-1 truncate text-ink-muted">{action.path}</Mono>
                    <Mono className="shrink-0 text-ink-dim">{action.code}</Mono>
                  </li>
                ))}
              {actions.every((action) => action.kind === 'unchanged') ? (
                <li className="py-1 text-[12px] text-ink-dim">Every path is unchanged — apply converges to a no-op.</li>
              ) : null}
            </ul>
          </Section>
        ) : null}

        {receipt ? (
          <Section title="Apply receipt" hint="Bob returns an immediate receipt. It does not persist it and it is not behavioral verification.">
            <KeyValue
              entries={[
                ['applied digest', <Mono className={receipt.converged_after_apply ? 'text-jade' : 'text-brass'}>{receipt.applied_plan_digest}</Mono>],
                ['written', <Mono>{receipt.written_count}</Mono>],
                ['adopted', <Mono>{receipt.adopted_count}</Mono>],
                ['unchanged', <Mono>{receipt.unchanged_count}</Mono>],
                ['lock written', <Mono>{String(receipt.lock_written)}</Mono>],
                ['converged after apply', <Badge tone={receipt.converged_after_apply ? 'positive' : 'caution'}>{String(receipt.converged_after_apply)}</Badge>],
                ['next check', <Mono>{receipt.next_check?.argv?.join(' ') ?? '—'}</Mono>],
                ...(receipt.truncation?.truncated ? ([['truncated', <Badge tone="caution">output truncated at {receipt.truncation.byte_limit} bytes</Badge>]] as [string, React.ReactNode][]) : [])
              ]}
            />
            <div className="grid grid-cols-3 gap-3">
              {(['written', 'adopted', 'unchanged'] as const).map((key) => (
                <div key={key} className="rounded-[8px] border border-line bg-sunken px-3 py-2">
                  <p className="text-[10.5px] uppercase text-ink-dim">{key}</p>
                  <ul className="mt-1 max-h-40 overflow-auto">
                    {(receipt[key] ?? []).map((path) => (
                      <li key={path}>
                        <Mono className="block truncate text-ink-muted">{path}</Mono>
                      </li>
                    ))}
                    {(receipt[key] ?? []).length === 0 ? <li className="text-[11.5px] text-ink-dim">none</li> : null}
                  </ul>
                </div>
              ))}
            </div>
            <div className="flex flex-wrap gap-2">
              <Button variant="quiet" onClick={() => navigate('check')}>
                Run check
              </Button>
              <Button variant="quiet" onClick={() => navigate('repository')}>
                Review the git diff
              </Button>
            </div>
          </Section>
        ) : null}

        <FailureNotice result={apply.result} error={apply.error} errorCode={apply.errorCode} title="Apply refused" />
        {apply.result && apply.result.exitCode === 5 ? (
          <Callout tone="caution" title="Guarded apply refused" actions={<Button size="sm" variant="quiet" onClick={() => void preflight()}>Re-run preflight</Button>}>
            <span className="inline-flex items-center gap-1.5">
              <ShieldAlert className="size-3.5" aria-hidden />
              The fresh plan no longer matches the digest you reviewed. Nothing was written.
            </span>
          </Callout>
        ) : null}
        {apply.result?.envelope ? <NextActionsList envelope={apply.result.envelope} onRunCommand={(argv) => void runCommand(argv)} /> : null}
        <RunMeta result={apply.result ?? fresh.result} />

        <Modal
          open={confirmOpen}
          title="Apply this plan?"
          description="Bob writes only absent, identical, or previously managed files. One conflict refuses the whole operation."
          onClose={() => setConfirmOpen(false)}
          footer={
            <>
              <Button variant="ghost" onClick={() => setConfirmOpen(false)}>
                Cancel
              </Button>
              <Button
                variant="primary"
                disabled={acknowledge.trim().toUpperCase() !== 'APPLY'}
                icon={<KeyRound className="size-3.5" aria-hidden />}
                onClick={() => void doApply()}
              >
                Apply {writes.length + adopts.length} file{writes.length + adopts.length === 1 ? '' : 's'}
              </Button>
            </>
          }
        >
          <div className="flex flex-col gap-3">
            <KeyValue
              entries={[
                ['workspace', <Mono className="break-all">{workspace}</Mono>],
                ['digest', <Mono className="break-all">{bindDigest ? digestToBind || 'unbound' : 'unbound'}</Mono>],
                ['writes', <Mono>{writes.length}</Mono>],
                ['adopts', <Mono>{adopts.length}</Mono>],
                ['unchanged', <Mono>{actions.length - writes.length - adopts.length - conflicts.length}</Mono>]
              ]}
            />
            <label className="flex flex-col gap-1.5">
              <span className="text-[11.5px] text-ink-muted">Type APPLY to confirm</span>
              <input
                value={acknowledge}
                onChange={(event) => setAcknowledge(event.target.value)}
                autoFocus
                className="h-8 rounded-[6px] border border-line-strong bg-sunken px-2.5 font-mono text-[12.5px] text-ink focus:border-copper focus:outline-none"
                aria-label="Type APPLY to confirm"
              />
            </label>
          </div>
        </Modal>
      </WorkspaceGate>
    </PanelShell>
  )
}

export default ApplyPanel
