import { useCallback, useEffect, useState } from 'react'
import { Crosshair, Layers } from 'lucide-react'
import { splitPathList } from '@shared/argv'
import { shortDigest } from '@shared/format'
import type { PathBatchData, PathData, TypedAction } from '@shared/types'
import { Badge, Button, Callout, KeyValue, Mono, Section, Segmented, TextArea, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, NextActionsList, RunButton, RunMeta, useRunCommand } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

const BATCH_LIMIT = 7

function classificationTone(classification: string): 'positive' | 'info' | 'caution' | 'danger' | 'neutral' {
  switch (classification) {
    case 'managed':
      return 'info'
    case 'seed':
      return 'positive'
    case 'reserved':
      return 'caution'
    case 'extension_point':
      return 'accent' as 'info'
    default:
      return 'neutral'
  }
}

function TypedActionRow({ action, onRun }: { action: TypedAction; onRun: (argv: string[]) => void }): JSX.Element {
  const blocked = action.requires_explicit_authority || action.effect !== 'read_only' || action.blocked_by.length > 0
  return (
    <li className="flex items-start gap-2 rounded-[6px] border border-line bg-sunken px-2.5 py-2">
      <Button
        size="sm"
        variant={blocked ? 'quiet' : 'default'}
        disabled={blocked}
        className="shrink-0"
        onClick={() => onRun(action.argv)}
        title={blocked ? 'This step needs explicit authority or mutates the repository' : 'Run this read-only command'}
      >
        {blocked ? 'Gated' : 'Run'}
      </Button>
      <div className="min-w-0 flex-1">
        <p className="text-[12px] text-ink">{action.id}</p>
        <Mono className="mt-0.5 block break-all text-ink-dim">{action.argv.join(' ')}</Mono>
        <div className="mt-1 flex flex-wrap items-center gap-1.5">
          <Badge tone={action.effect === 'read_only' ? 'positive' : 'caution'}>{action.effect}</Badge>
          {action.reason_code ? <Badge tone="neutral">{action.reason_code}</Badge> : null}
          {action.blocked_by.map((blocker) => (
            <Badge key={blocker} tone="danger">
              blocked: {blocker}
            </Badge>
          ))}
        </div>
      </div>
    </li>
  )
}

function PathDetail({ data, onRun }: { data: PathData; onRun: (argv: string[]) => void }): JSX.Element {
  const { navigate } = useStore()
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <Mono className="text-[13px] text-ink">{data.path}</Mono>
        <Badge tone={classificationTone(data.classification)}>{data.classification}</Badge>
        <Badge tone="neutral">{data.state}</Badge>
        <Badge tone={data.human_edit_effect === 'will_conflict' ? 'danger' : data.human_edit_effect === 'outside_bob_ownership' ? 'neutral' : 'caution'}>
          human edit: {data.human_edit_effect.replace(/_/g, ' ')}
        </Badge>
        <Badge tone={data.exists ? 'positive' : 'neutral'}>{data.exists ? 'exists' : 'absent'}</Badge>
      </div>

      <KeyValue
        entries={[
          ['recipe', data.ownership.recipe ? <Mono>{data.ownership.recipe.id}@{data.ownership.recipe.version}</Mono> : <span className="text-ink-dim">—</span>],
          ['locked sha256', <Mono title={data.ownership.locked_sha256}>{data.ownership.locked_sha256 ?? '—'}</Mono>],
          ['current sha256', <Mono title={data.ownership.current_sha256}>{data.ownership.current_sha256 ?? '—'}</Mono>],
          [
            'plan action',
            data.plan_action ? (
              <span className="inline-flex items-center gap-1.5">
                <Badge tone={data.plan_action.kind === 'conflict' ? 'danger' : data.plan_action.kind === 'unchanged' ? 'positive' : 'caution'}>
                  {data.plan_action.kind}
                </Badge>
                <Mono className="text-ink-muted">{data.plan_action.code}</Mono>
              </span>
            ) : (
              <span className="text-ink-dim">none</span>
            )
          ],
          ['artifact', data.artifact ? <Mono>{data.artifact.id}</Mono> : <span className="text-ink-dim">not a recipe artifact</span>],
          ...(data.artifact?.roles?.length ? ([['roles', data.artifact.roles.join(', ')]] as [string, React.ReactNode][]) : []),
          ...(data.artifact?.capability_ids?.length
            ? ([['capabilities', <Mono className="text-ink-muted">{data.artifact.capability_ids.join(', ')}</Mono>]] as [string, React.ReactNode][])
            : [])
        ]}
      />

      {data.extension_points.length > 0 ? (
        <Section title="Extension points" hint="Places where human or generated content is expected to live.">
          <ul className="flex flex-col gap-1">
            {data.extension_points.map((point, index) => (
              <li key={`${String(point.id)}-${index}`} className="rounded-[6px] border border-line bg-sunken px-2.5 py-1.5">
                <Mono className="text-ink">{String(point.id)}</Mono>
                {point.summary ? <p className="mt-0.5 text-[11.5px] text-ink-muted">{String(point.summary)}</p> : null}
              </li>
            ))}
          </ul>
        </Section>
      ) : null}

      {data.related_playbooks.length > 0 ? (
        <Section title="Related playbooks">
          <div className="flex flex-wrap gap-2">
            {data.related_playbooks.map((id) => (
              <Button key={id} size="sm" variant="quiet" onClick={() => navigate('playbooks', { playbook: id })}>
                {id}
              </Button>
            ))}
          </div>
        </Section>
      ) : null}

      {data.notices.length > 0 ? (
        <ul className="flex flex-col gap-1">
          {data.notices.map((notice) => (
            <li key={notice} className="rounded-[6px] border border-brass/40 bg-brass/10 px-2.5 py-1.5 text-[11.5px] text-brass">
              {notice}
            </li>
          ))}
        </ul>
      ) : null}

      {data.actions.length > 0 ? (
        <Section title="Typed next actions" hint="Bob proposes argv, never runs it for you. Gated steps need explicit authority.">
          <ul className="flex flex-col gap-1.5">
            {data.actions.map((action) => (
              <TypedActionRow key={action.id} action={action} onRun={onRun} />
            ))}
          </ul>
        </Section>
      ) : null}

      {data.truncation?.truncated ? (
        <Callout tone="caution" title="Projection truncated">
          This path projection was capped at {data.truncation.byte_limit} bytes.
        </Callout>
      ) : null}
    </div>
  )
}

export function PathPanel(): JSX.Element {
  const { workspace, navParams, revision } = useStore()
  const single = useRun<PathData>('path')
  const batch = useRun<PathBatchData>('path-batch')
  const runCommand = useRunCommand()
  const [mode, setMode] = useState<'single' | 'batch'>(navParams.path ? 'single' : 'single')
  const [input, setInput] = useState(navParams.path ?? '')
  const [selected, setSelected] = useState<PathData | null>(null)

  const paths = splitPathList(input)

  const run = useCallback(async () => {
    setSelected(null)
    if (mode === 'single') {
      const [first] = paths
      if (!first) return
      await single.run({ values: { path: first } })
      return
    }
    if (paths.length === 0) return
    await batch.run({ values: { paths: paths.slice(0, BATCH_LIMIT) } })
  }, [mode, paths, single, batch])

  useEffect(() => {
    if (navParams.path) {
      setInput(navParams.path)
      setMode('single')
    }
  }, [navParams.path])

  useEffect(() => {
    if (workspace && navParams.path) void run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, navParams.path, revision])

  const batchResults = batch.data?.results ?? []
  const active = single.data ?? selected

  return (
    <PanelShell
      featureId="path"
      actions={<RunButton label={mode === 'single' ? 'Classify path' : `Classify ${Math.min(paths.length, BATCH_LIMIT)} paths`} pending={single.pending || batch.pending} onClick={() => void run()} disabled={paths.length === 0} />}
    >
      <WorkspaceGate>
        <Section
          title="Ask about an exact path"
          hint="Bob answers for one repository-relative path at a time. --batch classifies up to seven paths against one shared plan."
          actions={
            <Segmented
              ariaLabel="Classification mode"
              value={mode}
              onChange={(next) => setMode(next)}
              options={[
                { value: 'single', label: 'Single path' },
                { value: 'batch', label: 'Batch', count: paths.length }
              ]}
            />
          }
        >
          <TextArea
            rows={mode === 'batch' ? 4 : 1}
            value={input}
            onChange={(event) => setInput(event.target.value)}
            placeholder={mode === 'batch' ? 'one path per line, up to seven\npackage.json\ntsconfig.json' : 'internal/cli/root.go'}
            aria-label={mode === 'batch' ? 'Paths to classify' : 'Path to classify'}
            className="font-mono text-[12px]"
            onKeyDown={(event) => {
              if (event.key === 'Enter' && mode === 'single') {
                event.preventDefault()
                void run()
              }
            }}
          />
          {mode === 'batch' && paths.length > BATCH_LIMIT ? (
            <Callout tone="caution" title={`Bob accepts at most ${BATCH_LIMIT} paths per batch`}>
              The first {BATCH_LIMIT} will be classified; the rest are ignored.
            </Callout>
          ) : null}
        </Section>

        <FailureNotice result={single.result ?? batch.result} error={single.error ?? batch.error} errorCode={single.errorCode ?? batch.errorCode} />

        {active ? (
          <Section title="Classification" actions={<Crosshair className="size-4 text-ink-dim" aria-hidden />}>
            <div className="rounded-[8px] border border-line bg-panel p-3">
              <PathDetail data={active} onRun={(argv) => void runCommand(argv)} />
            </div>
          </Section>
        ) : null}

        {mode === 'batch' && batchResults.length > 0 ? (
          <Section title={`Batch results (${batchResults.length})`} hint="One workspace plan, consistent answers for every path.">
            <ul className="flex flex-col gap-1">
              {batchResults.map((result) => (
                <li key={result.path}>
                  <button
                    type="button"
                    onClick={() => setSelected(result)}
                    className={cx(
                      'flex w-full items-center gap-3 rounded-[6px] border px-2.5 py-1.5 text-left',
                      selected?.path === result.path ? 'border-copper/60 bg-copper/10' : 'border-line bg-panel hover:border-line-strong'
                    )}
                  >
                    <Badge tone={classificationTone(result.classification)}>{result.classification}</Badge>
                    <Mono className="min-w-0 flex-1 truncate text-ink">{result.path}</Mono>
                    <Mono className="shrink-0 text-ink-dim">{result.state}</Mono>
                    <Mono className="shrink-0 text-ink-dim">{shortDigest(result.ownership.current_sha256, 8)}</Mono>
                  </button>
                </li>
              ))}
            </ul>
            {selected ? (
              <div className="mt-3 rounded-[8px] border border-line bg-panel p-3">
                <PathDetail data={selected} onRun={(argv) => void runCommand(argv)} />
              </div>
            ) : null}
          </Section>
        ) : null}

        <Section title="Why this matters" hint="Path classification is the smallest honest answer Bob gives about ownership.">
          <ul className="flex flex-col gap-1.5 text-[12px] text-ink-muted">
            <li className="flex items-start gap-2">
              <Layers className="mt-0.5 size-3.5 shrink-0 text-ink-dim" aria-hidden />
              <span>
                <Mono className="text-ink">managed</Mono> — bob.lock owns the whole file; a human edit becomes a conflict.
              </span>
            </li>
            <li className="flex items-start gap-2">
              <Layers className="mt-0.5 size-3.5 shrink-0 text-ink-dim" aria-hidden />
              <span>
                <Mono className="text-ink">seed</Mono> — Bob wrote it once and never owns it again; edits are yours.
              </span>
            </li>
            <li className="flex items-start gap-2">
              <Layers className="mt-0.5 size-3.5 shrink-0 text-ink-dim" aria-hidden />
              <span>
                <Mono className="text-ink">unmanaged</Mono> — Bob has no relationship with this path.
              </span>
            </li>
          </ul>
        </Section>

        {single.result?.envelope ? <NextActionsList envelope={single.result.envelope} onRunCommand={(argv) => void runCommand(argv)} /> : null}
        <RunMeta result={single.result ?? batch.result} />
      </WorkspaceGate>
    </PanelShell>
  )
}

export default PathPanel
