import { useCallback, useEffect, useMemo, useState } from 'react'
import { Lock } from 'lucide-react'
import { shortDigest } from '@shared/format'
import type { PlanAction, PlanData } from '@shared/types'
import { Badge, Button, CodeBlock, Mono, Section, Segmented, TextInput, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, RunButton, RunMeta } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

type Status = 'in_sync' | 'drifted' | 'would_update' | 'missing' | 'unlocked' | 'unknown'

interface LedgerRow {
  path: string
  lockedSha256: string | null
  currentSha256: string | null
  desiredSha256: string | null
  status: Status
  kind: string
  code: string
  mode: string
}

const STATUS_TONE: Record<Status, 'positive' | 'caution' | 'danger' | 'info' | 'neutral'> = {
  in_sync: 'positive',
  drifted: 'danger',
  would_update: 'caution',
  missing: 'danger',
  unlocked: 'info',
  unknown: 'neutral'
}

export function LockPanel(): JSX.Element {
  const { workspace, files, navigate, revision } = useStore()
  const plan = useRun<PlanData>('plan')
  const [filter, setFilter] = useState<'all' | 'attention'>('all')
  const [query, setQuery] = useState('')

  const run = useCallback(() => plan.run({ flags: {} }), [plan])

  useEffect(() => {
    if (workspace) void run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, revision])

  const rows = useMemo<LedgerRow[]>(() => {
    const lockEntries = files?.lockFiles ?? []
    const actions = new Map<string, PlanAction>((plan.data?.actions ?? []).map((action) => [action.path, action]))
    const seen = new Set<string>()
    const out: LedgerRow[] = []

    for (const entry of lockEntries) {
      seen.add(entry.path)
      const action = actions.get(entry.path)
      let status: Status = 'unknown'
      if (!action) status = 'unlocked'
      else if (action.kind === 'conflict') status = action.code === 'managed_missing' ? 'missing' : 'drifted'
      else if (action.kind === 'update') status = 'would_update'
      else if (action.kind === 'unchanged') status = 'in_sync'
      else status = 'unknown'
      out.push({
        path: entry.path,
        lockedSha256: entry.sha256,
        currentSha256: action?.current_sha256 ?? null,
        desiredSha256: action?.desired_sha256 ?? null,
        status,
        kind: action?.kind ?? '—',
        code: action?.code ?? 'not in plan',
        mode: action?.desired_mode !== undefined ? `0${(action.desired_mode & 0o777).toString(8)}` : '—'
      })
    }

    for (const action of plan.data?.actions ?? []) {
      if (seen.has(action.path)) continue
      if (action.kind === 'unchanged' && action.code === 'seed_exists') continue
      out.push({
        path: action.path,
        lockedSha256: action.locked_sha256 ?? null,
        currentSha256: action.current_sha256 ?? null,
        desiredSha256: action.desired_sha256 ?? null,
        status: action.kind === 'conflict' ? 'drifted' : action.kind === 'create' ? 'unlocked' : 'unknown',
        kind: action.kind,
        code: action.code,
        mode: action.desired_mode !== undefined ? `0${(action.desired_mode & 0o777).toString(8)}` : '—'
      })
    }
    return out.sort((a, b) => a.path.localeCompare(b.path))
  }, [files, plan.data])

  const visible = rows.filter((row) => {
    if (filter === 'attention' && row.status === 'in_sync') return false
    const needle = query.trim().toLowerCase()
    return !needle || row.path.toLowerCase().includes(needle)
  })

  const attention = rows.filter((row) => row.status !== 'in_sync').length

  return (
    <PanelShell
      featureId="lock-inspect"
      actions={
        <>
          <Button variant="quiet" onClick={() => navigate('plan')}>
            Open plan
          </Button>
          <RunButton label="Re-read ledger" pending={plan.pending} onClick={() => void run()} />
        </>
      }
      subtitle="bob.lock records the recipe version and the SHA-256 digest of every Bob-owned whole file. A managed file may update only when its current hash still matches this ledger."
    >
      <WorkspaceGate>
        <Section
          title="Ledger"
          hint={`${rows.length} tracked paths · ${attention} need attention`}
          actions={
            <>
              <Segmented
                ariaLabel="Ledger filter"
                value={filter}
                onChange={setFilter}
                options={[
                  { value: 'all', label: 'All', count: rows.length },
                  { value: 'attention', label: 'Attention', count: attention }
                ]}
              />
              <TextInput
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Filter paths…"
                aria-label="Filter ledger paths"
                className="h-7 w-48 py-0 font-mono text-[11.5px]"
              />
            </>
          }
        >
          {files?.lockRecipe ? (
            <div className="flex flex-wrap items-center gap-3 rounded-[8px] border border-line bg-panel px-3 py-2">
              <Lock className="size-4 text-copper" aria-hidden />
              <Mono className="text-ink">
                {files.lockRecipe.id}@{files.lockRecipe.version}
              </Mono>
              <Badge tone="neutral">{files.lockFiles.length} lock entries</Badge>
              {plan.data ? (
                <Badge tone={plan.data.lock_changed ? 'caution' : 'positive'}>
                  {plan.data.lock_changed ? 'apply would rewrite the lock' : 'lock unchanged'}
                </Badge>
              ) : null}
              <Mono className="ml-auto text-ink-dim">{files.lockPath}</Mono>
            </div>
          ) : (
            <p className="rounded-[8px] border border-dashed border-line px-3 py-4 text-[12px] text-ink-dim">
              No bob.lock yet. Bob writes it on the first successful apply.
            </p>
          )}

          {visible.length > 0 ? (
            <div className="max-h-[34rem] overflow-auto rounded-[8px] border border-line">
              <table className="w-full border-collapse text-[12px]">
                <thead className="sticky top-0 z-10 bg-raise">
                  <tr className="text-left text-[10.5px] uppercase text-ink-dim">
                    <th className="px-2.5 py-1.5 font-medium">Path</th>
                    <th className="px-2.5 py-1.5 font-medium">Status</th>
                    <th className="px-2.5 py-1.5 font-medium">Locked</th>
                    <th className="px-2.5 py-1.5 font-medium">Current</th>
                    <th className="px-2.5 py-1.5 font-medium">Desired</th>
                    <th className="px-2.5 py-1.5 font-medium">Code</th>
                  </tr>
                </thead>
                <tbody>
                  {visible.map((row) => (
                    <tr key={row.path} className="border-b border-line/70 last:border-b-0 hover:bg-raise/60">
                      <td className="px-2.5 py-1">
                        <button type="button" className="text-left" onClick={() => navigate('path', { path: row.path })}>
                          <Mono className="text-ink hover:text-copper">{row.path}</Mono>
                        </button>
                      </td>
                      <td className="px-2.5 py-1">
                        <Badge tone={STATUS_TONE[row.status]}>{row.status.replace(/_/g, ' ')}</Badge>
                      </td>
                      <td className="px-2.5 py-1">
                        <Mono className={cx('text-ink-dim', row.lockedSha256 && row.currentSha256 && row.lockedSha256 !== row.currentSha256 && 'text-clay')} title={row.lockedSha256 ?? ''}>
                          {shortDigest(row.lockedSha256 ?? undefined, 10)}
                        </Mono>
                      </td>
                      <td className="px-2.5 py-1">
                        <Mono className="text-ink-dim" title={row.currentSha256 ?? ''}>
                          {shortDigest(row.currentSha256 ?? undefined, 10)}
                        </Mono>
                      </td>
                      <td className="px-2.5 py-1">
                        <Mono className="text-ink-dim" title={row.desiredSha256 ?? ''}>
                          {shortDigest(row.desiredSha256 ?? undefined, 10)}
                        </Mono>
                      </td>
                      <td className="px-2.5 py-1">
                        <Mono className="text-ink-muted">{row.code}</Mono>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p className="rounded-[8px] border border-dashed border-line px-3 py-4 text-[12px] text-ink-dim">Nothing matches this filter.</p>
          )}
        </Section>

        <FailureNotice result={plan.result} error={plan.error} errorCode={plan.errorCode} title="Could not read the plan behind the ledger" />
        <RunMeta result={plan.result} />

        {files?.lock ? (
          <Section title="Raw bob.lock" hint="Exactly what is on disk.">
            <CodeBlock maxHeight="18rem" copy>
              {files.lock}
            </CodeBlock>
          </Section>
        ) : null}
      </WorkspaceGate>
    </PanelShell>
  )
}

export default LockPanel
