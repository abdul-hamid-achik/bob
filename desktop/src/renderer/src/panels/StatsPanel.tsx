import { useCallback, useEffect, useState } from 'react'
import { BarChart3 } from 'lucide-react'
import type { StatsData } from '@shared/types'
import { Badge, Button, Callout, DataTable, KeyValue, Mono, Section, Segmented, Toggle, cx, type Column } from '../components/ui'
import { PanelShell } from '../components/shell'
import { FailureNotice, RunButton, RunMeta } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

type Window = '24h' | '7d' | '30d'

export function StatsPanel(): JSX.Element {
  const { workspace, revision, navigate } = useStore()
  const stats = useRun<StatsData>('stats')
  const [since, setSince] = useState<Window>('7d')
  const [all, setAll] = useState(false)

  const run = useCallback(
    () =>
      stats.run({
        // stats rejects a workspace argument together with --all
        argv: all ? ['stats', '--all', '--since', since, '--json'] : ['stats', workspace, '--since', since, '--json']
      }),
    [stats, all, since, workspace]
  )

  useEffect(() => {
    void run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, since, all, revision])

  const data = stats.data
  const summary = data?.stats

  const operationColumns: Column<NonNullable<typeof summary>['by_operation'][number]>[] = [
    { key: 'operation', header: 'Operation', render: (row) => <Mono className="text-ink">bob {row.operation}</Mono>, sortValue: (row) => row.operation },
    { key: 'events', header: 'Events', align: 'right', render: (row) => <Mono>{row.events}</Mono>, sortValue: (row) => row.events },
    { key: 'successes', header: 'Successes', align: 'right', render: (row) => <Mono className="text-jade">{row.successes}</Mono>, sortValue: (row) => row.successes },
    { key: 'failures', header: 'Failures', align: 'right', render: (row) => <Mono className={row.failures > 0 ? 'text-clay' : 'text-ink-dim'}>{row.failures}</Mono>, sortValue: (row) => row.failures },
    { key: 'conflicts', header: 'Conflicts', align: 'right', render: (row) => <Mono className={row.conflict_events > 0 ? 'text-clay' : 'text-ink-dim'}>{row.conflict_events}</Mono>, sortValue: (row) => row.conflict_events },
    { key: 'drift', header: 'Drift', align: 'right', render: (row) => <Mono className={row.drift_events > 0 ? 'text-brass' : 'text-ink-dim'}>{row.drift_events}</Mono>, sortValue: (row) => row.drift_events },
    { key: 'duration', header: 'Total ms', align: 'right', render: (row) => <Mono className="text-ink-muted">{row.duration_ms}</Mono>, sortValue: (row) => row.duration_ms }
  ]

  return (
    <PanelShell
      featureId="stats"
      actions={<RunButton label="Read stats" pending={stats.pending} onClick={() => void run()} />}
      subtitle="Aggregate opt-in local usage. No individual events, no stored raw paths, arguments, filenames, manifest content, or raw errors ever leave this view."
    >
      <Section
        title="Window"
        actions={
          <Segmented
            ariaLabel="Lookback window"
            value={since}
            onChange={setSince}
            options={[
              { value: '24h', label: '24h' },
              { value: '7d', label: '7d' },
              { value: '30d', label: '30d' }
            ]}
          />
        }
      >
        <div className="flex flex-col gap-3 rounded-[8px] border border-line bg-panel px-3 py-3">
          <Toggle
            checked={all}
            onChange={setAll}
            label="--all: aggregate every retained pseudonymous workspace"
            hint="Mutually exclusive with a workspace argument. Workspaces are identified by a pseudonymous id, never by path."
          />
          {data ? (
            <KeyValue
              entries={[
                ['telemetry', <Badge tone={data.enabled ? 'caution' : 'positive'}>{data.enabled ? 'enabled' : 'disabled'}</Badge>],
                ['transport', <Badge tone="info">{data.local_only ? 'local only' : 'unknown'}</Badge>],
                ['selection', <Mono className="break-all">{data.selection}</Mono>]
              ]}
            />
          ) : null}
        </div>
      </Section>

      {data && !data.enabled ? (
        <Callout
          tone="info"
          title="Telemetry is disabled, so there is nothing to summarize"
          actions={
            <Button size="sm" variant="quiet" onClick={() => navigate('settings')}>
              Bob config
            </Button>
          }
        >
          Enable privacy-bounded local telemetry in Bob's own settings to start recording bounded events. The console never enables it for
          you.
        </Callout>
      ) : null}

      {summary ? (
        <>
          <Section title="Totals" actions={<BarChart3 className="size-4 text-ink-dim" aria-hidden />}>
            <div className="grid grid-cols-6 gap-3">
              {[
                ['events', summary.events, 'text-ink'],
                ['successes', summary.successes, 'text-jade'],
                ['failures', summary.failures, summary.failures > 0 ? 'text-clay' : 'text-ink-dim'],
                ['conflicts', summary.conflict_events, summary.conflict_events > 0 ? 'text-clay' : 'text-ink-dim'],
                ['drift', summary.drift_events, summary.drift_events > 0 ? 'text-brass' : 'text-ink-dim'],
                ['skipped', summary.skipped, 'text-ink-dim']
              ].map(([label, value, tone]) => (
                <div key={String(label)} className="rounded-[8px] border border-line bg-panel px-3 py-2.5">
                  <p className="text-[10.5px] uppercase text-ink-dim">{label}</p>
                  <p className={cx('font-mono text-[20px] tabular-nums', String(tone))}>{String(value)}</p>
                </div>
              ))}
            </div>
            <KeyValue
              entries={[
                ['window', <Mono>{new Date(summary.since).toLocaleString()} → {new Date(summary.until).toLocaleString()}</Mono>],
                ['workspace id', <Mono>{summary.workspace_id}</Mono>],
                ['total duration', <Mono>{summary.duration_ms} ms</Mono>],
                ['schema version', <Mono>{summary.schema_version}</Mono>]
              ]}
            />
          </Section>

          <Section title={`By operation (${summary.by_operation.length})`}>
            <DataTable
              rows={summary.by_operation}
              columns={operationColumns}
              keyFor={(row) => row.operation}
              dense
              empty={<p className="text-[12px] text-ink-dim">No events recorded in this window.</p>}
            />
          </Section>

          {Object.keys(summary.actions).length > 0 ? (
            <Section title="Action tally" hint="Aggregate planner action counts, never per-path detail.">
              <ul className="flex flex-wrap gap-2">
                {Object.entries(summary.actions).map(([key, value]) => (
                  <li key={key} className="rounded-[6px] border border-line bg-panel px-2.5 py-1">
                    <Mono className="text-ink-muted">{key}</Mono>
                    <Mono className="ml-2 text-ink">{String(value)}</Mono>
                  </li>
                ))}
              </ul>
            </Section>
          ) : null}
        </>
      ) : null}

      <FailureNotice result={stats.result} error={stats.error} errorCode={stats.errorCode} title="Could not read stats" />
      <RunMeta result={stats.result} />
    </PanelShell>
  )
}

export default StatsPanel
