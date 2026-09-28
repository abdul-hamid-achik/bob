import { useCallback, useEffect, useMemo, useState } from 'react'
import { Compass, Play } from 'lucide-react'
import type { Playbook, PlaybookListData, PlaybookShowData } from '@shared/types'
import { Badge, Button, Callout, EmptyState, KeyValue, Mono, Section, TextInput, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, RunButton, RunMeta, useRunCommand } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

const RISK_TONE = { high: 'danger', medium: 'caution', low: 'positive' } as const

function effectTone(effect: string): 'positive' | 'caution' | 'danger' | 'neutral' {
  if (effect === 'read_only') return 'positive'
  if (effect === 'repository_mutation') return 'danger'
  if (effect === 'subprocess') return 'caution'
  return 'neutral'
}

function StepRow({ step, index, onRun }: { step: Playbook['steps'][number]; index: number; onRun: (argv: string[]) => void }): JSX.Element {
  const runnable = step.effect === 'read_only' && !step.requires_explicit_authority && step.blocked_by.length === 0 && step.argv.length > 0
  return (
    <li className="flex gap-3 rounded-[8px] border border-line bg-panel px-3 py-2.5">
      <span className="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full border border-line-strong font-mono text-[10.5px] text-ink-dim">
        {index + 1}
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <Mono className="text-ink">{step.id}</Mono>
          <Badge tone="neutral">{step.kind}</Badge>
          <Badge tone={effectTone(step.effect)}>{step.effect.replace(/_/g, ' ')}</Badge>
          {step.requires_explicit_authority ? <Badge tone="danger">explicit authority</Badge> : null}
          {step.blocked_by.map((blocker) => (
            <Badge key={blocker} tone="caution">
              blocked: {blocker.replace(/_/g, ' ')}
            </Badge>
          ))}
        </div>
        <p className="mt-1 text-[12.5px] leading-relaxed text-ink-muted">{step.summary}</p>
        {step.argv.length > 0 ? (
          <div className="mt-1.5 flex items-start gap-2">
            <code className="argv min-w-0 flex-1 rounded border border-line bg-sunken px-2 py-1 text-ink-muted">{step.argv.join(' ')}</code>
            <Button size="sm" variant={runnable ? 'default' : 'quiet'} disabled={!runnable} icon={<Play className="size-3" aria-hidden />} onClick={() => onRun(step.argv)}>
              {runnable ? 'Run' : 'Gated'}
            </Button>
          </div>
        ) : null}
        {step.paths.length > 0 ? (
          <p className="mt-1 text-[11px] text-ink-dim">
            paths: <Mono>{step.paths.join(', ')}</Mono>
          </p>
        ) : null}
        {step.depends_on.length > 0 ? (
          <p className="mt-0.5 text-[11px] text-ink-dim">
            depends on: <Mono>{step.depends_on.join(', ')}</Mono>
          </p>
        ) : null}
        <p className="mt-1 text-[11px] text-ink-dim">
          success when: <span className="text-ink-muted">{step.success_condition}</span>
        </p>
      </div>
    </li>
  )
}

export function PlaybooksPanel(): JSX.Element {
  const { workspace, navParams, plan, revision } = useStore()
  const list = useRun<PlaybookListData>('playbook-list')
  const show = useRun<PlaybookShowData>('playbook-show')
  const resolve = useRun<PlaybookShowData>('playbook-plan')
  const runCommand = useRunCommand()
  const [selected, setSelected] = useState<string>(navParams.playbook ?? '')
  const [values, setValues] = useState<Record<string, string>>({})

  const loadList = useCallback(() => list.run(), [list])

  useEffect(() => {
    if (workspace) {
      void loadList().then((result) => {
        const list = (result.envelope?.data as PlaybookListData | undefined)?.playbooks ?? []
        if (!selected && list.length > 0) setSelected(list[0]?.id ?? '')
      })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace, revision])

  useEffect(() => {
    if (navParams.playbook) setSelected(navParams.playbook)
  }, [navParams.playbook])

  useEffect(() => {
    if (selected && workspace) {
      setValues({})
      void show.run({ values: { playbook: selected } })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected, workspace])

  const playbooks = list.data?.playbooks ?? []
  const playbook = show.data?.playbook ?? resolve.data?.playbook ?? null
  const resolved = resolve.data?.values ? resolve.data : null
  // Highlight whatever the detail pane is actually rendering; in live mode that
  // is always the selected id, and in demo replay it is the captured playbook.
  const activeId = playbook?.id ?? selected

  const conflictPaths = useMemo(
    () => (plan?.actions ?? []).filter((action) => action.kind === 'conflict').map((action) => ({ path: action.path, code: action.code })),
    [plan]
  )

  const runResolve = async (): Promise<void> => {
    const pairs = Object.entries(values).filter(([, value]) => value.trim() !== '')
    await resolve.run({ values: { playbook: selected }, flags: { set: pairs.map(([key, value]) => `${key}=${value}`) } })
  }

  return (
    <PanelShell
      featureId="playbook-list"
      title="Playbooks"
      subtitle="Closed, recipe-versioned procedures with typed inputs and argv-shaped steps. Bob resolves them and never executes a step."
      actions={
        <Button variant="quiet" loading={list.pending} onClick={() => void loadList()}>
          Reload
        </Button>
      }
    >
      <WorkspaceGate>
        <Section title={`Available for this recipe (${playbooks.length})`} hint={list.data?.recipe ? `${list.data.recipe.id}@${list.data.recipe.version}` : undefined}>
          {playbooks.length === 0 && !list.pending ? (
            <EmptyState title="No playbooks resolved" hint="Playbooks come from the active recipe. Initialize a manifest first if this workspace has none." />
          ) : null}
          <ul className="grid grid-cols-2 gap-2">
            {playbooks.map((entry) => (
              <li key={entry.id}>
                <button
                  type="button"
                  onClick={() => setSelected(entry.id)}
                  className={cx(
                    'flex w-full flex-col gap-1.5 rounded-[8px] border px-3 py-2 text-left transition-colors',
                    activeId === entry.id ? 'border-copper/60 bg-copper/10' : 'border-line bg-panel hover:border-line-strong'
                  )}
                >
                  <span className="flex items-center gap-2">
                    <Compass className={cx('size-3.5 shrink-0', activeId === entry.id ? 'text-copper' : 'text-ink-dim')} aria-hidden />
                    <Mono className="truncate text-ink">{entry.id}</Mono>
                    <Badge tone={RISK_TONE[entry.risk as keyof typeof RISK_TONE] ?? 'neutral'}>{entry.risk}</Badge>
                  </span>
                  <span className="text-[11.5px] leading-snug text-ink-muted">{entry.title}</span>
                  <span className="flex flex-wrap items-center gap-1.5">
                    <Badge tone={entry.applicable ? 'positive' : 'neutral'}>{entry.applicable ? 'applicable' : 'not applicable'}</Badge>
                    <Badge tone={entry.available ? 'info' : 'caution'}>{entry.available ? 'available' : 'unavailable'}</Badge>
                    <Mono className="text-[10.5px] text-ink-dim">{entry.scope_class}</Mono>
                    {entry.required_inputs.length > 0 ? <Mono className="text-[10.5px] text-ink-dim">inputs: {entry.required_inputs.join(', ')}</Mono> : null}
                    {entry.blocked_by.map((blocker) => (
                      <Badge key={blocker} tone="danger">
                        {blocker}
                      </Badge>
                    ))}
                  </span>
                </button>
              </li>
            ))}
          </ul>
        </Section>

        <FailureNotice result={list.result} error={list.error} errorCode={list.errorCode} title="Could not list playbooks" />

        {playbook ? (
          <>
            <Section title={playbook.title} hint={playbook.purpose}>
              <div className="flex flex-col gap-3 rounded-[8px] border border-line bg-panel px-3 py-3">
                <div className="flex flex-wrap items-center gap-2">
                  <Mono className="text-ink">{playbook.id}</Mono>
                  <Badge tone={RISK_TONE[playbook.risk as keyof typeof RISK_TONE] ?? 'neutral'}>risk: {playbook.risk}</Badge>
                  <Badge tone="neutral">scope: {playbook.scope_class}</Badge>
                  <Badge tone={playbook.applicable ? 'positive' : 'neutral'}>{playbook.applicable ? 'applicable' : 'not applicable'}</Badge>
                  <Badge tone={playbook.available ? 'info' : 'caution'}>{playbook.available ? 'available' : 'unavailable'}</Badge>
                </div>

                <KeyValue
                  entries={[
                    ['boundary: create', playbook.boundary.create.length > 0 ? <Mono className="text-jade">{playbook.boundary.create.join(', ')}</Mono> : <span className="text-ink-dim">nothing</span>],
                    ['boundary: modify', playbook.boundary.modify.length > 0 ? <Mono className="text-brass">{playbook.boundary.modify.join(', ')}</Mono> : <span className="text-ink-dim">nothing</span>],
                    ['boundary: forbidden', <Mono className="text-clay">{playbook.boundary.forbidden.join(', ') || '—'}</Mono>],
                    ...(playbook.preconditions.length > 0
                      ? ([['preconditions', <ul className="flex flex-col gap-0.5">{playbook.preconditions.map((item) => <li key={item} className="text-[12px] text-ink-muted">{item}</li>)}</ul>]] as [string, React.ReactNode][])
                      : [])
                  ]}
                />

                {playbook.inputs.length > 0 ? (
                  <div className="flex flex-col gap-2 border-t border-line pt-3">
                    <p className="text-[10.5px] uppercase text-ink-dim">typed inputs</p>
                    <div className="grid grid-cols-2 gap-3">
                      {playbook.inputs.map((input) => {
                        const value = values[input.name] ?? ''
                        const suggestions = input.name === 'path' ? conflictPaths : []
                        return (
                          <div key={input.name} className="flex flex-col gap-1">
                            <label htmlFor={`input-${input.name}`} className="flex items-center gap-2 text-[11.5px] text-ink-muted">
                              <Mono className="text-ink">{input.name}</Mono>
                              <Badge tone="neutral">{input.type}</Badge>
                              {input.required ? <Badge tone="caution">required</Badge> : null}
                              <Mono className="text-[10px] text-ink-dim">{input.validation}</Mono>
                            </label>
                            {input.enum ? (
                              <select
                                id={`input-${input.name}`}
                                value={value}
                                onChange={(event) => setValues((prev) => ({ ...prev, [input.name]: event.target.value }))}
                                className="h-8 rounded-[6px] border border-line-strong bg-sunken px-2 font-mono text-[12px] text-ink focus:border-copper focus:outline-none"
                              >
                                <option value="">choose…</option>
                                {input.enum.map((option) => (
                                  <option key={option} value={option}>
                                    {option}
                                  </option>
                                ))}
                              </select>
                            ) : (
                              <TextInput
                                id={`input-${input.name}`}
                                value={value}
                                list={suggestions.length > 0 ? `${input.name}-suggestions` : undefined}
                                onChange={(event) => setValues((prev) => ({ ...prev, [input.name]: event.target.value }))}
                                placeholder={input.type === 'repository_path' ? 'relative/path/in/repo' : 'value'}
                                className="font-mono text-[12px]"
                              />
                            )}
                            {suggestions.length > 0 ? (
                              <>
                                <datalist id={`${input.name}-suggestions`}>
                                  {suggestions.map((suggestion) => (
                                    <option key={suggestion.path} value={suggestion.path} />
                                  ))}
                                </datalist>
                                <div className="flex flex-wrap gap-1">
                                  {suggestions.slice(0, 5).map((suggestion) => (
                                    <button
                                      key={suggestion.path}
                                      type="button"
                                      onClick={() => setValues((prev) => ({ ...prev, [input.name]: suggestion.path, action_code: suggestion.code }))}
                                      className="rounded border border-clay/40 bg-clay/10 px-1.5 py-0.5 font-mono text-[10.5px] text-clay hover:bg-clay/20"
                                    >
                                      {suggestion.path} · {suggestion.code}
                                    </button>
                                  ))}
                                </div>
                              </>
                            ) : null}
                          </div>
                        )
                      })}
                    </div>
                    <div>
                      <RunButton label="Resolve playbook" pending={resolve.pending} onClick={() => void runResolve()} />
                    </div>
                  </div>
                ) : null}
              </div>
            </Section>

            <FailureNotice result={resolve.result} error={resolve.error} errorCode={resolve.errorCode} title="Resolution refused" />

            {resolved ? (
              <Callout tone="positive" title="Resolved with typed values">
                <Mono className="text-jade">{JSON.stringify(resolved.values ?? {})}</Mono>
                <p className="mt-1">
                  Steps below carry concrete argv. Running one is still your decision — Bob resolves procedures and never executes them.
                </p>
              </Callout>
            ) : null}

            <Section title={`Procedure (${playbook.steps.length} steps)`} hint="Ordered, typed, and bounded. Steps that mutate or need authority are gated.">
              <ol className="flex flex-col gap-2">
                {playbook.steps.map((step, index) => (
                  <StepRow key={step.id} step={step} index={index} onRun={(argv) => void runCommand(argv)} />
                ))}
              </ol>
            </Section>

            {playbook.verification_hints.length > 0 ? (
              <Section title="Verification hints" hint="What proves the procedure worked. Bob does not declare verification itself.">
                <ul className="flex flex-col gap-1">
                  {playbook.verification_hints.map((hint) => (
                    <li key={hint} className="rounded-[6px] border border-line bg-sunken px-2.5 py-1.5 text-[12px] text-ink-muted">
                      {hint}
                    </li>
                  ))}
                </ul>
              </Section>
            ) : null}

            {playbook.failure_modes.length > 0 ? (
              <Section title="Failure modes">
                <ul className="flex flex-col gap-1">
                  {playbook.failure_modes.map((mode) => (
                    <li key={mode} className="rounded-[6px] border border-clay/35 bg-clay/8 px-2.5 py-1.5 text-[12px] text-clay/90">
                      {mode}
                    </li>
                  ))}
                </ul>
              </Section>
            ) : null}

            <RunMeta result={resolve.result ?? show.result} />
          </>
        ) : null}
      </WorkspaceGate>
    </PanelShell>
  )
}

export default PlaybooksPanel
