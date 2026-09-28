import { useCallback, useEffect } from 'react'
import { Stethoscope } from 'lucide-react'
import type { DoctorData } from '@shared/types'
import { Badge, Callout, DataTable, Mono, Section, cx, type Column } from '../components/ui'
import { PanelShell } from '../components/shell'
import { FailureNotice, RunButton, RunMeta } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

export function DoctorPanel(): JSX.Element {
  const { workspace, revision } = useStore()
  const doctor = useRun<DoctorData>('doctor')
  const run = useCallback(() => doctor.run(), [doctor])

  useEffect(() => {
    void run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, revision])

  const data = doctor.data
  const required = (data?.checks ?? []).filter((check) => check.required)
  const optional = (data?.checks ?? []).filter((check) => !check.required)

  const columns: Column<DoctorData['checks'][number]>[] = [
    {
      key: 'state',
      header: 'State',
      width: '6.5rem',
      render: (row) => (
        <Badge tone={row.usable ? 'positive' : row.found ? 'caution' : row.required ? 'danger' : 'neutral'}>
          {row.usable ? 'usable' : row.found ? 'unusable' : 'missing'}
        </Badge>
      )
    },
    {
      key: 'name',
      header: 'Tool',
      render: (row) => (
        <span className="flex items-center gap-2">
          <Mono className="text-ink">{row.name}</Mono>
          {row.required ? <Badge tone="info">required</Badge> : <Badge tone="neutral">optional</Badge>}
        </span>
      ),
      sortValue: (row) => row.name
    },
    { key: 'command', header: 'Command', width: '8rem', render: (row) => <Mono className="text-ink-muted">{row.command}</Mono>, sortValue: (row) => row.command },
    {
      key: 'path',
      header: 'Resolved path',
      render: (row) => <Mono className={cx('break-all', row.path ? 'text-ink-muted' : 'text-ink-dim')}>{row.path || '—'}</Mono>,
      sortValue: (row) => row.path
    },
    { key: 'version', header: 'Version', render: (row) => <Mono className="text-ink-dim">{row.version || '—'}</Mono> },
    { key: 'note', header: 'Why it matters', render: (row) => <span className="text-[11.5px] text-ink-muted">{row.note}</span> }
  ]

  return (
    <PanelShell
      featureId="doctor"
      actions={<RunButton label="Probe tools" pending={doctor.pending} onClick={() => void run()} />}
      subtitle="Bounded capability probes for required and selected optional development tools. Doctor reports honestly; it never installs, repairs, or initializes anything."
    >
      {data ? (
        <Section
          title="Verdict"
          actions={
            <span className="flex items-center gap-2">
              <Stethoscope className={cx('size-4', data.ready ? 'text-jade' : 'text-clay')} aria-hidden />
              <Badge tone={data.ready && !data.degraded ? 'positive' : data.degraded ? 'caution' : 'danger'}>
                {data.ready ? (data.degraded ? 'ready, degraded' : 'ready') : 'not ready'}
              </Badge>
            </span>
          }
        >
          <div
            className={cx(
              'rounded-[8px] border px-4 py-3',
              data.ready && !data.degraded ? 'border-jade/40 bg-jade/8' : data.degraded ? 'border-brass/40 bg-brass/8' : 'border-clay/45 bg-clay/8'
            )}
          >
            <p className="text-[13px] font-medium text-ink">
              {data.ready
                ? data.degraded
                  ? 'Ready with degraded capability'
                  : 'Every required tool is present and usable'
                : 'A required tool is missing or unusable'}
            </p>
            <p className="mt-1 text-[12px] text-ink-muted">
              {required.filter((check) => check.usable).length} of {required.length} required usable ·{' '}
              {optional.filter((check) => check.found).length} of {optional.length} optional found
            </p>
          </div>
          {!data.ready ? (
            <Callout tone="danger" title="Doctor exits 1 when a required tool is missing">
              Install the missing tool, then probe again. Bob never installs anything for you.
            </Callout>
          ) : null}
        </Section>
      ) : null}

      <Section title={`Required tools (${required.length})`}>
        <DataTable rows={required} columns={columns} keyFor={(row) => row.name} dense empty={<p className="text-[12px] text-ink-dim">No required tools reported.</p>} />
      </Section>

      <Section title={`Optional tools (${optional.length})`} hint="Probed only when the manifest selects them.">
        <DataTable rows={optional} columns={columns} keyFor={(row) => row.name} dense empty={<p className="text-[12px] text-ink-dim">No optional tools selected.</p>} />
      </Section>

      <FailureNotice result={doctor.result} error={doctor.error} errorCode={doctor.errorCode} />
      <RunMeta result={doctor.result} />
    </PanelShell>
  )
}

export default DoctorPanel
