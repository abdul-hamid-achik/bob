import { useCallback, useEffect, useState } from 'react'
import { ArrowUpCircle } from 'lucide-react'
import { isPlanDigest } from '@shared/format'
import type { UpgradeData } from '@shared/types'
import { Badge, Button, Callout, KeyValue, Modal, Mono, Section, Toggle } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, NextActionsList, RunButton, RunMeta, useRunCommand } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

export function UpgradePanel(): JSX.Element {
  const { workspace, plan: snapshot, settings, reloadFiles, bump } = useStore()
  const dry = useRun<UpgradeData>('upgrade')
  const live = useRun<UpgradeData>('upgrade')
  const runCommand = useRunCommand()
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [bindDigest, setBindDigest] = useState(settings.requireDigestBoundApply)

  const runDry = useCallback(() => dry.run({ flags: { 'dry-run': true } }), [dry])

  useEffect(() => {
    if (workspace) void runDry()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace])

  const data = dry.data
  const alreadyCurrent = data ? data.from_version === data.to_version : false
  const digest = snapshot?.digest ?? ''

  const runLive = async (): Promise<void> => {
    setConfirmOpen(false)
    const result = await live.run({
      confirmed: true,
      flags: bindDigest && isPlanDigest(digest) ? { 'expect-plan-digest': digest } : {}
    })
    if (result.envelope?.ok) {
      await reloadFiles()
      bump()
      await runDry()
    }
  }

  return (
    <PanelShell
      featureId="upgrade"
      actions={
        <>
          <Button variant="quiet" loading={dry.pending} onClick={() => void runDry()}>
            Re-run dry run
          </Button>
          <Button variant="primary" icon={<ArrowUpCircle className="size-3.5" aria-hidden />} disabled={!data || alreadyCurrent} onClick={() => setConfirmOpen(true)}>
            Upgrade lock
          </Button>
        </>
      }
    >
      <WorkspaceGate>
        <Section title="Recipe migration" hint="Upgrade moves bob.lock to the current recipe version. A published recipe version is never changed in place.">
          {data ? (
            <div className="flex flex-col gap-3 rounded-[8px] border border-line bg-panel px-3 py-3">
              <div className="flex flex-wrap items-center gap-4">
                <div className="flex items-center gap-3">
                  <div>
                    <p className="text-[10.5px] uppercase text-ink-dim">from</p>
                    <Mono className="text-[18px] text-ink">v{data.from_version}</Mono>
                  </div>
                  <ArrowUpCircle className="size-5 rotate-90 text-copper" aria-hidden />
                  <div>
                    <p className="text-[10.5px] uppercase text-ink-dim">to</p>
                    <Mono className={cxVersion(data.to_version, data.from_version)}>{data.to_version}</Mono>
                  </div>
                </div>
                <Badge tone={data.recipe ? 'info' : 'neutral'}>{data.recipe || 'no recipe'}</Badge>
                <Badge tone={alreadyCurrent ? 'positive' : 'caution'}>{data.actions} action{data.actions === 1 ? '' : 's'}</Badge>
                {data.applied ? <Badge tone="positive">applied</Badge> : <Badge tone="neutral">dry run</Badge>}
              </div>
              {alreadyCurrent ? (
                <Callout tone="positive" title="Already at the current recipe version">
                  Nothing to migrate. Bob still re-verifies ownership on every plan, so drift shows up there rather than here.
                </Callout>
              ) : (
                <Callout tone="caution" title="Migration will rewrite lock-owned files">
                  Upgrading applies the new recipe version through the same ownership rules as apply: lock-proven updates only, and one
                  conflict refuses the whole operation.
                </Callout>
              )}
            </div>
          ) : (
            <p className="text-[12px] text-ink-dim">Run the dry run to read the migration.</p>
          )}
        </Section>

        <FailureNotice result={dry.result ?? live.result} error={dry.error ?? live.error} errorCode={dry.errorCode ?? live.errorCode} title="Upgrade refused" />

        {live.data ? (
          <Section title="Upgrade result">
            <KeyValue
              entries={[
                ['recipe', <Mono>{live.data.recipe}</Mono>],
                ['from → to', <Mono>v{live.data.from_version} → v{live.data.to_version}</Mono>],
                ['actions', <Mono>{live.data.actions}</Mono>],
                ['applied', <Badge tone={live.data.applied ? 'positive' : 'neutral'}>{String(live.data.applied)}</Badge>]
              ]}
            />
          </Section>
        ) : null}

        {dry.result?.envelope ? <NextActionsList envelope={dry.result.envelope} onRunCommand={(argv) => void runCommand(argv)} /> : null}
        <RunMeta result={live.result ?? dry.result} />

        <Modal
          open={confirmOpen}
          title="Upgrade bob.lock?"
          description="This migrates the lock to the current recipe version and rewrites lock-owned files whose hashes still match."
          onClose={() => setConfirmOpen(false)}
          footer={
            <>
              <Button variant="ghost" onClick={() => setConfirmOpen(false)}>
                Cancel
              </Button>
              <Button variant="primary" onClick={() => void runLive()}>
                Upgrade
              </Button>
            </>
          }
        >
          <div className="flex flex-col gap-3">
            <KeyValue
              entries={[
                ['workspace', <Mono className="break-all">{workspace}</Mono>],
                ['recipe', <Mono>{data?.recipe ?? '—'}</Mono>],
                ['migration', <Mono>v{data?.from_version ?? '?'} → v{data?.to_version ?? '?'}</Mono>],
                ['actions', <Mono>{data?.actions ?? 0}</Mono>]
              ]}
            />
            <Toggle
              checked={bindDigest}
              onChange={setBindDigest}
              label="Bind to the reviewed plan digest"
              hint={digest ? `--expect-plan-digest ${digest}` : 'No reviewed digest captured yet; run a plan first to bind.'}
            />
          </div>
        </Modal>
      </WorkspaceGate>
    </PanelShell>
  )
}

function cxVersion(to: number, from: number): string {
  return `text-[18px] ${to > from ? 'text-copper' : 'text-ink'}`
}

export default UpgradePanel
