import { useCallback, useEffect, useState } from 'react'
import { BookOpen } from 'lucide-react'
import { shortDigest } from '@shared/format'
import type { ContextData } from '@shared/types'
import { Badge, Button, Callout, DataTable, KeyValue, Mono, Section, Segmented, cx, type Column } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, NextActionsList, RunButton, RunMeta, useRunCommand } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

type Profile = 'compact' | 'standard' | 'full'

const FACET_TONE: Record<string, 'positive' | 'caution' | 'danger' | 'info' | 'neutral'> = {
  in_sync: 'positive',
  enabled: 'info',
  disabled: 'neutral',
  not_applicable: 'neutral',
  not_assessed: 'neutral',
  missing: 'danger',
  drifted: 'caution',
  available: 'positive',
  unavailable: 'caution'
}

export function ContextPanel(): JSX.Element {
  const { workspace, navigate, revision } = useStore()
  const context = useRun<ContextData>('context')
  const runCommand = useRunCommand()
  const [profile, setProfile] = useState<Profile>('standard')

  const run = useCallback(() => context.run({ flags: { profile } }), [context, profile])

  useEffect(() => {
    if (workspace) void run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, profile, revision])

  const data = context.data

  const capabilityColumns: Column<ContextData['capabilities'][number]>[] = [
    { key: 'id', header: 'Capability', render: (row) => <Mono className="text-ink">{row.id}</Mono>, sortValue: (row) => row.id },
    {
      key: 'selection',
      header: 'Selection',
      render: (row) => <Badge tone={FACET_TONE[row.selection] ?? 'neutral'}>{row.selection.replace(/_/g, ' ')}</Badge>,
      sortValue: (row) => row.selection
    },
    {
      key: 'materialization',
      header: 'Materialization',
      render: (row) => <Badge tone={FACET_TONE[row.materialization] ?? 'neutral'}>{row.materialization.replace(/_/g, ' ')}</Badge>,
      sortValue: (row) => row.materialization
    },
    {
      key: 'availability',
      header: 'Availability',
      render: (row) => <Badge tone={FACET_TONE[row.availability] ?? 'neutral'}>{row.availability.replace(/_/g, ' ')}</Badge>,
      sortValue: (row) => row.availability
    },
    {
      key: 'verification',
      header: 'Verification',
      render: (row) => <span className="text-[11.5px] text-ink-muted">{row.verification.replace(/_/g, ' ')}</span>,
      sortValue: (row) => row.verification
    },
    ...(profile === 'full'
      ? ([
          {
            key: 'summary',
            header: 'Summary',
            render: (row) => <span className="text-[11.5px] text-ink-muted">{row.summary ?? '—'}</span>
          }
        ] as Column<ContextData['capabilities'][number]>[])
      : [])
  ]

  return (
    <PanelShell
      featureId="context"
      actions={<RunButton label="Read context" pending={context.pending} onClick={() => void run()} />}
      subtitle="One bounded offline call that reports recipe identity, ownership-relevant entry and extension points, capability facets, invariants, and the exact current plan digest — without running any specialist tool."
    >
      <WorkspaceGate>
        <Section
          title="Profile"
          hint="Each profile has an explicit byte limit and reports its own truncation, so a capped agent harness never receives a silent cut."
          actions={
            <Segmented
              ariaLabel="Context profile"
              value={profile}
              onChange={setProfile}
              options={[
                { value: 'compact', label: 'compact' },
                { value: 'standard', label: 'standard' },
                { value: 'full', label: 'full' }
              ]}
            />
          }
        >
          {data ? (
            <div className="flex flex-wrap items-center gap-4 rounded-[8px] border border-line bg-panel px-3 py-2.5">
              <Badge tone="info">{data.profile}</Badge>
              <KeyValue
                className="flex-1"
                entries={[
                  ['recipe', <Mono>{data.recipe.id}@{data.recipe.version}</Mono>],
                  ['product', <Mono>{data.product.name} · {data.product.runtime} · {data.product.kind}</Mono>],
                  ['contract digest', <Mono title={data.contract_digest}>{shortDigest(data.contract_digest)}</Mono>],
                  ['context digest', <Mono title={data.context_digest}>{shortDigest(data.context_digest)}</Mono>],
                  ['plan digest', <Mono title={data.repository.plan_digest}>{shortDigest(data.repository.plan_digest)}</Mono>]
                ]}
              />
            </div>
          ) : (
            <p className="text-[12px] text-ink-dim">Read the context to see the bounded contract.</p>
          )}
          {data?.truncation.truncated ? (
            <Callout tone="caution" title={`Truncated at ${data.truncation.byte_limit} bytes`}>
              Omitted sections: {Object.entries(data.truncation.omitted).map(([key, value]) => `${key}=${String(value)}`).join(', ') || 'none reported'}
            </Callout>
          ) : data ? (
            <p className="text-[11.5px] text-ink-dim">
              byte limit {data.truncation.byte_limit} · not truncated
            </p>
          ) : null}
        </Section>

        <FailureNotice result={context.result} error={context.error} errorCode={context.errorCode} />

        {data ? (
          <>
            <Section title="Repository verdict" actions={<BookOpen className="size-4 text-ink-dim" aria-hidden />}>
              <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)] gap-4">
                <KeyValue
                  entries={[
                    ['state', <Badge tone={data.repository.clean ? 'positive' : 'caution'}>{data.repository.state}</Badge>],
                    ['lock exists', String(data.repository.lock_exists)],
                    ['lock changed', String(data.repository.lock_changed)],
                    ['managed files', <Mono>{data.repository.managed_files}</Mono>],
                    ['conflicts', <Mono className={data.repository.conflict_count > 0 ? 'text-clay' : ''}>{data.repository.conflict_count}</Mono>],
                    ['conflict class', <Mono>{data.repository.conflict_class}</Mono>]
                  ]}
                />
                <div className="flex flex-col gap-2">
                  <p className="text-[10.5px] uppercase text-ink-dim">action counts</p>
                  <ul className="flex flex-wrap gap-x-4 gap-y-1">
                    {Object.entries(data.repository.action_counts).map(([kind, count]) => (
                      <li key={kind} className="inline-flex items-baseline gap-1.5">
                        <Mono className={cx('text-[13px]', Number(count) > 0 && kind === 'conflict' ? 'text-clay' : 'text-ink')}>{count}</Mono>
                        <span className="text-[11px] text-ink-dim">{kind}</span>
                      </li>
                    ))}
                  </ul>
                  <p className="mt-1 text-[10.5px] uppercase text-ink-dim">conflict families</p>
                  <ul className="flex flex-wrap gap-x-4 gap-y-1">
                    {Object.entries(data.repository.conflict_family_counts).map(([family, count]) => (
                      <li key={family} className="inline-flex items-baseline gap-1.5">
                        <Mono className={cx('text-[13px]', Number(count) > 0 ? 'text-clay' : 'text-ink-dim')}>{count}</Mono>
                        <span className="text-[11px] text-ink-dim">{family.replace(/_/g, ' ')}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            </Section>

            <Section title={`Capabilities (${data.capabilities.length})`} hint="Selection, materialization, availability, and verification — reported, never assessed by Bob.">
              <DataTable
                rows={data.capabilities}
                columns={capabilityColumns}
                keyFor={(row) => row.id}
                dense
                empty={<p className="text-[12px] text-ink-dim">This profile carries no capability facets.</p>}
              />
            </Section>

            {data.invariants.length > 0 ? (
              <Section title="Invariants" hint="The rules Bob enforces on this workspace.">
                <ul className="flex flex-col gap-1.5">
                  {data.invariants.map((invariant) => (
                    <li key={invariant.id} className="rounded-[6px] border border-line bg-panel px-3 py-2">
                      <Mono className="text-copper">{invariant.id}</Mono>
                      <p className="mt-0.5 text-[12px] leading-relaxed text-ink-muted">{invariant.statement}</p>
                    </li>
                  ))}
                </ul>
              </Section>
            ) : null}

            {data.playbooks.length > 0 ? (
              <Section title="Playbooks" hint="Closed procedures that apply to this workspace.">
                <ul className="flex flex-col gap-1">
                  {data.playbooks.map((playbook) => (
                    <li key={playbook.id}>
                      <button
                        type="button"
                        onClick={() => navigate('playbooks', { playbook: playbook.id })}
                        className="flex w-full items-center gap-3 rounded-[6px] border border-line bg-panel px-3 py-1.5 text-left hover:border-line-strong"
                      >
                        <Mono className="text-ink">{playbook.id}</Mono>
                        <span className="min-w-0 flex-1 truncate text-[11.5px] text-ink-muted">{playbook.title}</span>
                        <Badge tone={playbook.risk === 'high' ? 'danger' : playbook.risk === 'medium' ? 'caution' : 'neutral'}>{playbook.risk}</Badge>
                        <Badge tone={playbook.applicable ? 'positive' : 'neutral'}>{playbook.applicable ? 'applicable' : 'not applicable'}</Badge>
                      </button>
                    </li>
                  ))}
                </ul>
              </Section>
            ) : null}

            {profile === 'full' ? (
              <>
                <Section title={`Artifacts (${data.artifacts?.length ?? 0})`} hint="Recipe artifacts, their roles, and their ownership class.">
                  <DataTable
                    rows={data.artifacts ?? []}
                    dense
                    keyFor={(row) => row.id}
                    columns={[
                      { key: 'id', header: 'Artifact', render: (row) => <Mono className="text-ink">{row.id}</Mono>, sortValue: (row) => row.id },
                      { key: 'path', header: 'Path', render: (row) => <Mono className="text-ink-muted">{row.path}</Mono>, sortValue: (row) => row.path },
                      { key: 'ownership', header: 'Ownership', render: (row) => <Badge tone={row.ownership === 'bob_whole_file' ? 'info' : 'positive'}>{row.ownership.replace(/_/g, ' ')}</Badge>, sortValue: (row) => row.ownership },
                      { key: 'roles', header: 'Roles', render: (row) => <span className="text-[11.5px] text-ink-muted">{row.roles.join(', ')}</span> }
                    ]}
                    empty={<p className="text-[12px] text-ink-dim">No artifacts in this projection.</p>}
                  />
                </Section>

                <Section title="Entry and extension points">
                  <div className="grid grid-cols-2 gap-4">
                    <div>
                      <p className="mb-1.5 text-[10.5px] uppercase text-ink-dim">entry points ({data.entry_points.length})</p>
                      <ul className="flex flex-col gap-1">
                        {data.entry_points.map((point, index) => (
                          <li key={`${String(point.id)}-${index}`} className="rounded-[6px] border border-line bg-sunken px-2.5 py-1.5">
                            <Mono className="text-ink">{String(point.id)}</Mono>
                            {point.path ? <Mono className="mt-0.5 block text-ink-dim">{String(point.path)}</Mono> : null}
                          </li>
                        ))}
                        {data.entry_points.length === 0 ? <li className="text-[11.5px] text-ink-dim">none</li> : null}
                      </ul>
                    </div>
                    <div>
                      <p className="mb-1.5 text-[10.5px] uppercase text-ink-dim">extension points ({data.extension_points.length})</p>
                      <ul className="flex flex-col gap-1">
                        {data.extension_points.map((point, index) => (
                          <li key={`${String(point.id)}-${index}`} className="rounded-[6px] border border-line bg-sunken px-2.5 py-1.5">
                            <Mono className="text-ink">{String(point.id)}</Mono>
                            {point.path ? <Mono className="mt-0.5 block text-ink-dim">{String(point.path)}</Mono> : null}
                          </li>
                        ))}
                        {data.extension_points.length === 0 ? <li className="text-[11.5px] text-ink-dim">none</li> : null}
                      </ul>
                    </div>
                  </div>
                </Section>
              </>
            ) : null}

            {data.notices.length > 0 ? (
              <Section title="Notices">
                <ul className="flex flex-col gap-1">
                  {data.notices.map((notice) => (
                    <li key={notice} className="rounded-[6px] border border-brass/40 bg-brass/10 px-2.5 py-1.5 text-[11.5px] text-brass">
                      {notice}
                    </li>
                  ))}
                </ul>
              </Section>
            ) : null}

            {data.actions.length > 0 ? (
              <Section title="Typed actions">
                <ul className="flex flex-col gap-1.5">
                  {data.actions.map((action) => (
                    <li key={action.id} className="flex items-start gap-2 rounded-[6px] border border-line bg-sunken px-2.5 py-2">
                      <Button
                        size="sm"
                        variant={action.effect === 'read_only' && !action.requires_explicit_authority ? 'default' : 'quiet'}
                        disabled={action.effect !== 'read_only' || action.requires_explicit_authority}
                        onClick={() => void runCommand(action.argv)}
                      >
                        {action.effect === 'read_only' && !action.requires_explicit_authority ? 'Run' : 'Gated'}
                      </Button>
                      <div className="min-w-0 flex-1">
                        <Mono className="text-ink">{action.id}</Mono>
                        <Mono className="mt-0.5 block break-all text-ink-dim">{action.argv.join(' ')}</Mono>
                      </div>
                    </li>
                  ))}
                </ul>
              </Section>
            ) : null}
          </>
        ) : null}

        {context.result?.envelope ? <NextActionsList envelope={context.result.envelope} onRunCommand={(argv) => void runCommand(argv)} /> : null}
        <RunMeta result={context.result} />
      </WorkspaceGate>
    </PanelShell>
  )
}

export default ContextPanel
