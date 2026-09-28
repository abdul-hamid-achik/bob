import { useCallback, useEffect, useMemo, useState } from 'react'
import { Download, History, Trash2 } from 'lucide-react'
import { exitCodeMeaning } from '@shared/envelope'
import { formatDuration, formatTimestamp, shortDigest } from '@shared/format'
import type { ActivityEntry } from '@shared/ipc'
import { Badge, Button, DataTable, KeyValue, Modal, Mono, Section, Segmented, TextInput, cx, type Column } from '../components/ui'
import { PanelShell } from '../components/shell'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'

type KindFilter = 'all' | 'bob' | 'task' | 'git' | 'mcp'

export function ActivityPanel(): JSX.Element {
  const { activity, reloadActivity, clearActivity, toast, revision } = useStore()
  const [kind, setKind] = useState<KindFilter>('all')
  const [onlyFailures, setOnlyFailures] = useState(false)
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<ActivityEntry | null>(null)
  const [confirmClear, setConfirmClear] = useState(false)

  useEffect(() => {
    void reloadActivity()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [revision])

  const rows = useMemo(() => {
    const needle = query.trim().toLowerCase()
    return activity.filter((entry) => {
      if (kind !== 'all' && entry.kind !== kind) return false
      if (onlyFailures && entry.ok) return false
      if (needle && !`${entry.displayCommand} ${entry.featureId} ${entry.cwd}`.toLowerCase().includes(needle)) return false
      return true
    })
  }, [activity, kind, onlyFailures, query])

  const onExport = useCallback(async () => {
    const outcome = await bridge().exportActivity()
    if (outcome.error) toast(outcome.error, 'danger')
    else if (outcome.path) toast(`Exported to ${outcome.path}`, 'positive')
  }, [toast])

  const columns: Column<ActivityEntry>[] = [
    {
      key: 'at',
      header: 'When',
      width: '10rem',
      render: (row) => <Mono className="text-ink-dim">{formatTimestamp(row.at)}</Mono>,
      sortValue: (row) => row.at
    },
    { key: 'kind', header: 'Kind', width: '5rem', render: (row) => <Badge tone={row.kind === 'bob' ? 'info' : row.kind === 'task' ? 'accent' : 'neutral'}>{row.kind}</Badge>, sortValue: (row) => row.kind },
    {
      key: 'command',
      header: 'Command',
      render: (row) => <Mono className={cx('break-all', row.ok ? 'text-ink' : 'text-clay')}>{row.displayCommand}</Mono>,
      sortValue: (row) => row.displayCommand
    },
    { key: 'feature', header: 'Feature', width: '9rem', render: (row) => <span className="text-[11.5px] text-ink-muted">{row.featureId}</span>, sortValue: (row) => row.featureId },
    {
      key: 'exit',
      header: 'Exit',
      width: '4rem',
      align: 'right',
      render: (row) => <Mono className={row.exitCode === 0 ? 'text-jade' : row.exitCode === 3 || row.exitCode === 5 ? 'text-brass' : 'text-clay'}>{row.exitCode}</Mono>,
      sortValue: (row) => row.exitCode ?? 0
    },
    { key: 'duration', header: 'Duration', width: '6rem', align: 'right', render: (row) => <Mono className="text-ink-dim">{formatDuration(row.durationMs)}</Mono>, sortValue: (row) => row.durationMs },
    {
      key: 'digest',
      header: 'Plan digest',
      width: '8rem',
      render: (row) => <Mono className="text-ink-dim">{shortDigest(row.planDigest ?? undefined, 10)}</Mono>,
      sortValue: (row) => row.planDigest ?? ''
    }
  ]

  return (
    <PanelShell
      featureId="activity"
      actions={
        <>
          <Button variant="quiet" icon={<Download className="size-3.5" aria-hidden />} onClick={() => void onExport()}>
            Export
          </Button>
          <Button variant="danger" icon={<Trash2 className="size-3.5" aria-hidden />} onClick={() => setConfirmClear(true)} disabled={activity.length === 0}>
            Clear
          </Button>
          <Button variant="default" icon={<History className="size-3.5" aria-hidden />} loading={false} onClick={() => void reloadActivity()}>
            Reload
          </Button>
        </>
      }
      subtitle="An append-only local ledger of every invocation this console made: argv, cwd, exit code, duration, and the plan digest it observed. It never leaves the machine."
    >
      <Section
        title={`Ledger (${rows.length} of ${activity.length})`}
        actions={
          <>
            <Segmented
              ariaLabel="Filter by kind"
              value={kind}
              onChange={setKind}
              options={[
                { value: 'all', label: 'all' },
                { value: 'bob', label: 'bob' },
                { value: 'task', label: 'task' },
                { value: 'git', label: 'git' },
                { value: 'mcp', label: 'mcp' }
              ]}
            />
            <Button size="sm" variant={onlyFailures ? 'primary' : 'quiet'} onClick={() => setOnlyFailures((prev) => !prev)}>
              Failures only
            </Button>
            <TextInput value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter…" aria-label="Filter ledger" className="h-7 w-44 py-0 text-[11.5px]" />
          </>
        }
      >
        <DataTable
          rows={rows}
          columns={columns}
          dense
          keyFor={(row) => row.id}
          selectedKey={selected?.id ?? null}
          onRowClick={setSelected}
          empty={<p className="rounded-[8px] border border-dashed border-line px-3 py-6 text-[12px] text-ink-dim">Nothing recorded yet. Run a command and it lands here.</p>}
        />
      </Section>

      {selected ? (
        <Section title="Entry detail">
          <KeyValue
            entries={[
              ['id', <Mono className="break-all">{selected.id}</Mono>],
              ['command', <Mono className="break-all text-ink">{selected.displayCommand}</Mono>],
              ['cwd', <Mono className="break-all">{selected.cwd}</Mono>],
              ['binary', <Mono className="break-all">{selected.binaryPath || '—'}</Mono>],
              ['exit', <Mono className={selected.exitCode === 0 ? 'text-jade' : 'text-clay'}>{selected.exitCode} · {selected.exitCode !== null ? exitCodeMeaning(selected.exitCode) : ''}</Mono>],
              ['duration', <Mono>{formatDuration(selected.durationMs)}</Mono>],
              ['ok', <Badge tone={selected.ok ? 'positive' : 'danger'}>{String(selected.ok)}</Badge>],
              ...(selected.errorCode ? ([['error code', <Mono className="text-clay">{selected.errorCode}</Mono>]] as [string, React.ReactNode][]) : []),
              ...(selected.planDigest ? ([['plan digest', <Mono className="break-all">{selected.planDigest}</Mono>]] as [string, React.ReactNode][]) : []),
              ['bytes out / err', <Mono>{selected.bytesOut} / {selected.bytesErr}</Mono>],
              ['timed out', String(selected.timedOut)],
              ['cancelled', String(selected.cancelled)],
              ['argv', <ol className="flex flex-col gap-0.5">{selected.argv.map((token, index) => <li key={`${token}-${index}`} className="flex gap-2"><span className="w-4 text-right text-ink-dim">{index}</span><Mono className="break-all text-ink-muted">{token}</Mono></li>)}</ol>]
            ]}
          />
        </Section>
      ) : null}

      <Modal
        open={confirmClear}
        title="Clear the activity ledger?"
        description="This deletes every recorded invocation from local storage. Export first if you want an audit copy."
        onClose={() => setConfirmClear(false)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmClear(false)}>
              Cancel
            </Button>
            <Button
              variant="danger"
              onClick={async () => {
                setConfirmClear(false)
                const { cleared } = await clearActivity()
                toast(`Cleared ${cleared} entries`, 'positive')
              }}
            >
              Clear ledger
            </Button>
          </>
        }
      />
    </PanelShell>
  )
}

export default ActivityPanel
