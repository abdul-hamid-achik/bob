import { useCallback, useEffect, useState } from 'react'
import { Eraser } from 'lucide-react'
import type { RemoveData, RemoveResult } from '@shared/types'
import { Badge, Button, Callout, KeyValue, Modal, Mono, Section, Toggle } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, NextActionsList, RunButton, RunMeta, useRunCommand } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

function normalize(data: RemoveData | null): RemoveResult | null {
  if (!data) return null
  if (data.result) return data.result
  return {
    removed: data.removed ?? [],
    skipped: data.skipped ?? [],
    conflicts: data.conflicts ?? [],
    lock_removed: Boolean(data.lock_removed)
  }
}

export function RemovePanel(): JSX.Element {
  const { workspace, reloadFiles, bump } = useStore()
  const dry = useRun<RemoveData>('remove')
  const live = useRun<RemoveData>('remove')
  const runCommand = useRunCommand()
  const [force, setForce] = useState(false)
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [acknowledge, setAcknowledge] = useState('')

  const runDry = useCallback(() => dry.run({ flags: { 'dry-run': true } }), [dry])

  useEffect(() => {
    if (workspace) void runDry()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, force])

  const preview = normalize(dry.data) ?? normalize(live.data)
  const receipt = normalize(live.data)
  const blocking = (preview?.skipped.length ?? 0) > 0

  const runLive = async (): Promise<void> => {
    setConfirmOpen(false)
    const result = await live.run({ confirmed: true, flags: force ? { force: true } : {} })
    if (result.envelope) {
      await reloadFiles()
      bump()
    }
  }

  return (
    <PanelShell
      featureId="remove"
      actions={
        <>
          <Button variant="quiet" loading={dry.pending} onClick={() => void runDry()}>
            Re-run dry run
          </Button>
          <Button variant="danger" icon={<Eraser className="size-3.5" aria-hidden />} disabled={!preview} onClick={() => setConfirmOpen(true)}>
            Remove management
          </Button>
        </>
      }
    >
      <WorkspaceGate>
        <Callout tone="info" title="What remove does and does not touch">
          Bob deletes only lock-owned files whose ownership still holds. Seed-once and unmanaged files stay, bob.yaml is preserved, and
          bob.lock is removed last. Nothing else in the repository is touched.
        </Callout>

        <Section
          title="Dry run"
          hint="Preview exactly which paths would be deleted before removing anything."
          actions={<Toggle checked={force} onChange={setForce} tone="danger" label="--force" hint="Remove managed files even when their content drifted from bob.lock." />}
        >
          {preview ? (
            <div className="grid grid-cols-3 gap-3">
              {(['removed', 'skipped', 'conflicts'] as const).map((key) => {
                const items = preview[key] as unknown[]
                return (
                  <div key={key} className="rounded-[8px] border border-line bg-panel px-3 py-2.5">
                    <p className="text-[10.5px] uppercase text-ink-dim">{key}</p>
                    <p
                      className={`font-mono text-[20px] ${
                        key === 'removed' ? 'text-ink' : key === 'skipped' ? 'text-brass' : 'text-clay'
                      }`}
                    >
                      {items.length}
                    </p>
                    <ul className="mt-1 max-h-40 overflow-auto">
                      {items.slice(0, 40).map((item, index) => (
                        <li key={`${String(item)}-${index}`}>
                          <Mono className="block truncate text-ink-muted">{typeof item === 'string' ? item : JSON.stringify(item)}</Mono>
                        </li>
                      ))}
                      {items.length === 0 ? <li className="text-[11.5px] text-ink-dim">none</li> : null}
                    </ul>
                  </div>
                )
              })}
            </div>
          ) : (
            <p className="text-[12px] text-ink-dim">Run the dry run to preview removal.</p>
          )}

          {blocking && !force ? (
            <Callout tone="caution" title="Some managed files drifted">
              Bob skips files whose content no longer matches bob.lock. Enable --force to remove them anyway, or restore them first with an
              apply.
            </Callout>
          ) : null}
        </Section>

        <FailureNotice result={dry.result ?? live.result} error={dry.error ?? live.error} errorCode={dry.errorCode ?? live.errorCode} title="Remove refused" />

        {receipt && live.result ? (
          <Section title="Removal receipt">
            <KeyValue
              entries={[
                ['removed', <Mono>{receipt.removed.length}</Mono>],
                ['skipped', <Mono className={receipt.skipped.length > 0 ? 'text-brass' : ''}>{receipt.skipped.length}</Mono>],
                ['conflicts', <Mono className={receipt.conflicts.length > 0 ? 'text-clay' : ''}>{receipt.conflicts.length}</Mono>],
                ['bob.lock removed', <Badge tone={receipt.lock_removed ? 'positive' : 'neutral'}>{String(receipt.lock_removed)}</Badge>],
                ['bob.yaml', <Badge tone="info">preserved</Badge>]
              ]}
            />
          </Section>
        ) : null}

        {dry.result?.envelope ? <NextActionsList envelope={dry.result.envelope} onRunCommand={(argv) => void runCommand(argv)} /> : null}
        <RunMeta result={live.result ?? dry.result} />

        <Modal
          open={confirmOpen}
          title="Remove Bob management from this workspace?"
          description="Lock-owned files are deleted. Seed-once files, unmanaged files, and bob.yaml stay."
          onClose={() => setConfirmOpen(false)}
          footer={
            <>
              <Button variant="ghost" onClick={() => setConfirmOpen(false)}>
                Cancel
              </Button>
              <Button variant="danger" disabled={acknowledge.trim().toUpperCase() !== 'REMOVE'} onClick={() => void runLive()}>
                Delete {preview?.removed.length ?? 0} file{(preview?.removed.length ?? 0) === 1 ? '' : 's'}
              </Button>
            </>
          }
        >
          <div className="flex flex-col gap-3">
            <KeyValue
              entries={[
                ['workspace', <Mono className="break-all">{workspace}</Mono>],
                ['would remove', <Mono>{preview?.removed.length ?? 0}</Mono>],
                ['would skip', <Mono>{preview?.skipped.length ?? 0}</Mono>],
                ['force drifted files', <Badge tone={force ? 'danger' : 'neutral'}>{String(force)}</Badge>]
              ]}
            />
            <label className="flex flex-col gap-1.5">
              <span className="text-[11.5px] text-ink-muted">Type REMOVE to confirm</span>
              <input
                value={acknowledge}
                onChange={(event) => setAcknowledge(event.target.value)}
                autoFocus
                className="h-8 rounded-[6px] border border-line-strong bg-sunken px-2.5 font-mono text-[12.5px] text-ink focus:border-clay focus:outline-none"
                aria-label="Type REMOVE to confirm"
              />
            </label>
          </div>
        </Modal>
      </WorkspaceGate>
    </PanelShell>
  )
}

export default RemovePanel
