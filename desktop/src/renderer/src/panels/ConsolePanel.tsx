import { useCallback, useMemo, useState } from 'react'
import { Terminal } from 'lucide-react'
import { buildArgv, missingPlaceholders, placeholdersFor, tokenizeArgv, type ArgvValue } from '@shared/argv'
import { exitCodeMeaning } from '@shared/envelope'
import { FEATURES, featureById } from '@shared/features'
import { displayArgv } from '@shared/format'
import type { RunResult } from '@shared/types'
import { Badge, Button, Callout, CodeBlock, Field, JsonView, KeyValue, Modal, Mono, Section, Tabs, TextArea, TextInput, Toggle, cx } from '../components/ui'
import { PanelShell } from '../components/shell'
import { useStore } from '../state/store'

type ResultTab = 'envelope' | 'stdout' | 'stderr'

interface HistoryRow {
  at: string
  featureId: string
  argv: string[]
  exitCode: number
  durationMs: number
  ok: boolean
}

const CLI_FEATURES = FEATURES.filter((feature) => Boolean(feature.argvTemplate) && !feature.streaming)

export function ConsolePanel(): JSX.Element {
  const { workspace, run, showInspector } = useStore()
  const [featureId, setFeatureId] = useState('plan')
  const [values, setValues] = useState<Record<string, ArgvValue>>({})
  const [flags, setFlags] = useState<Record<string, ArgvValue>>({})
  const [rawMode, setRawMode] = useState(false)
  const [raw, setRaw] = useState('plan . --json')
  const [cwd, setCwd] = useState(workspace)
  const [json, setJson] = useState(true)
  const [pending, setPending] = useState(false)
  const [result, setResult] = useState<RunResult | null>(null)
  const [tab, setTab] = useState<ResultTab>('envelope')
  const [history, setHistory] = useState<HistoryRow[]>([])
  const [confirmOpen, setConfirmOpen] = useState(false)

  const feature = featureById(featureId)
  const effectiveCwd = cwd.trim() || workspace

  const generated = useMemo(() => {
    if (!feature) return []
    return buildArgv(feature, {
      workspace: effectiveCwd,
      values: values as Record<string, ArgvValue | undefined>,
      flags: flags as Record<string, ArgvValue | undefined>,
      json
    })
  }, [feature, effectiveCwd, values, flags, json])

  const argv = rawMode ? tokenizeArgv(raw).filter((token) => token !== 'bob') : generated
  const missing = feature && !rawMode ? missingPlaceholders(feature, { workspace: effectiveCwd, values: values as Record<string, ArgvValue | undefined> }) : []
  const needsConfirm = Boolean(feature?.mutates) || rawMode

  const execute = useCallback(
    async (confirmed: boolean) => {
      setPending(true)
      try {
        const outcome = await run(featureId, { argv, cwd: effectiveCwd, confirmed, json: false })
        setResult(outcome)
        setTab('envelope')
        setHistory((prev) =>
          [
            {
              at: outcome.startedAt,
              featureId,
              argv: outcome.argv,
              exitCode: outcome.exitCode,
              durationMs: outcome.durationMs,
              ok: outcome.envelope?.ok ?? outcome.exitCode === 0
            },
            ...prev
          ].slice(0, 40)
        )
        showInspector(outcome.displayCommand, outcome)
      } finally {
        setPending(false)
      }
    },
    [argv, effectiveCwd, featureId, json, run, showInspector]
  )

  const onRun = (): void => {
    if (needsConfirm) setConfirmOpen(true)
    else void execute(false)
  }

  return (
    <PanelShell
      featureId="console"
      actions={
        <>
          <Button variant="quiet" onClick={() => setRawMode((prev) => !prev)}>
            {rawMode ? 'Use builder' : 'Edit raw argv'}
          </Button>
          <Button variant="primary" icon={<Terminal className="size-3.5" aria-hidden />} loading={pending} disabled={argv.length === 0} onClick={onRun}>
            Run command
          </Button>
        </>
      }
      subtitle="Any Bob command with an argv builder, plus the raw envelope, stdout, stderr, exit code, and duration side by side. The resolved binary is always the one from Settings."
    >
      <div className="grid grid-cols-[22rem_minmax(0,1fr)] gap-5">
        <div className="flex flex-col gap-4">
          <Section title="Command">
            <Field label="Feature">
              <select
                value={featureId}
                onChange={(event) => {
                  const next = event.target.value
                  setFeatureId(next)
                  const chosen = featureById(next)
                  setFlags({})
                  setValues({})
                  setRaw(chosen ? `${chosen.command.replace(/^bob /, '')} --json` : raw)
                }}
                className="h-8 w-full rounded-[6px] border border-line-strong bg-sunken px-2 text-[12.5px] text-ink focus:border-copper focus:outline-none"
                aria-label="Bob feature"
              >
                {CLI_FEATURES.map((entry) => (
                  <option key={entry.id} value={entry.id}>
                    {entry.command}
                    {entry.mutates ? ' · mutates' : ''}
                  </option>
                ))}
              </select>
            </Field>

            {feature ? (
              <>
                <p className="text-[11.5px] leading-snug text-ink-muted">{feature.summary}</p>
                {feature.mutates ? <Badge tone="caution">mutates the workspace</Badge> : <Badge tone="positive">read-only</Badge>}
              </>
            ) : null}

            {!rawMode && feature && placeholdersFor(feature).length > 0 ? (
              <div className="mt-3 flex flex-col gap-3 border-t border-line pt-3">
                {placeholdersFor(feature).map((name) => (
                  <Field key={name} label={name}>
                    <TextInput
                      value={String(values[name] ?? '')}
                      onChange={(event) => setValues((prev) => ({ ...prev, [name]: event.target.value }))}
                      placeholder={name === 'paths' ? 'one path per line' : name}
                      className="font-mono text-[11.5px]"
                    />
                  </Field>
                ))}
              </div>
            ) : null}

            {!rawMode && feature?.flags && feature.flags.length > 0 ? (
              <div className="mt-3 flex flex-col gap-2.5 border-t border-line pt-3">
                {feature.flags.map((flag) => {
                  const key = flag.name.replace(/^-+/, '')
                  if (flag.kind === 'boolean') {
                    return (
                      <Toggle
                        key={flag.name}
                        checked={Boolean(flags[key])}
                        onChange={(next) => setFlags((prev) => ({ ...prev, [key]: next }))}
                        label={flag.name}
                        hint={flag.help}
                      />
                    )
                  }
                  if (flag.kind === 'enum') {
                    return (
                      <Field key={flag.name} label={flag.name} hint={flag.help}>
                        <select
                          value={String(flags[key] ?? '')}
                          onChange={(event) => setFlags((prev) => ({ ...prev, [key]: event.target.value }))}
                          className="h-8 rounded-[6px] border border-line-strong bg-sunken px-2 font-mono text-[12px] text-ink focus:border-copper focus:outline-none"
                        >
                          <option value="">unset</option>
                          {(flag.values ?? []).map((option) => (
                            <option key={option} value={option}>
                              {option}
                            </option>
                          ))}
                        </select>
                      </Field>
                    )
                  }
                  return (
                    <Field key={flag.name} label={flag.name} hint={flag.help}>
                      <TextInput
                        value={String(flags[key] ?? '')}
                        onChange={(event) => setFlags((prev) => ({ ...prev, [key]: event.target.value }))}
                        placeholder={flag.placeholder ?? (flag.kind === 'stringArray' ? 'repeatable value' : 'value')}
                        className="font-mono text-[11.5px]"
                      />
                    </Field>
                  )
                })}
              </div>
            ) : null}

            <div className="mt-3 flex flex-col gap-3 border-t border-line pt-3">
              <Field label="cwd" hint="Defaults to the active workspace.">
                <TextInput value={effectiveCwd} onChange={(event) => setCwd(event.target.value)} className="font-mono text-[11.5px]" />
              </Field>
              {!rawMode ? (
                <Toggle checked={json} onChange={setJson} label="--json" hint="Write a versioned JSON envelope to stdout." />
              ) : null}
            </div>
          </Section>

          <Section title="Session history" hint="Console runs only. The full ledger lives on the Activity panel.">
            {history.length === 0 ? (
              <p className="text-[11.5px] text-ink-dim">Nothing run yet.</p>
            ) : (
              <ul className="max-h-56 overflow-auto rounded-[8px] border border-line">
                {history.map((entry, index) => (
                  <li key={`${entry.at}-${index}`}>
                    <button
                      type="button"
                      onClick={() => {
                        setRawMode(true)
                        setRaw(displayArgv(entry.argv))
                      }}
                      className="flex w-full items-center gap-2 border-b border-line/70 px-2.5 py-1 text-left last:border-b-0 hover:bg-raise/60"
                    >
                      <Mono className={cx('shrink-0', entry.ok ? 'text-jade' : 'text-clay')}>{entry.exitCode}</Mono>
                      <Mono className="min-w-0 flex-1 truncate text-ink-muted">bob {displayArgv(entry.argv)}</Mono>
                      <Mono className="shrink-0 text-ink-dim">{entry.durationMs}ms</Mono>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </Section>
        </div>

        <div className="flex min-w-0 flex-col gap-4">
          <Section title="argv">
            {rawMode ? (
              <TextArea
                rows={3}
                value={raw}
                onChange={(event) => setRaw(event.target.value)}
                aria-label="Raw argv"
                className="font-mono text-[12px]"
                placeholder="plan . --json"
              />
            ) : null}
            <div className="rounded-[8px] border border-line bg-sunken px-3 py-2">
              <Mono className="break-all text-ink">bob {displayArgv(argv)}</Mono>
            </div>
            {missing.length > 0 ? (
              <Callout tone="caution" title={`Missing values: ${missing.join(', ')}`}>
                The command will run without them and Bob will report the invalid argument. Fill them in or switch to raw argv.
              </Callout>
            ) : null}
          </Section>

          {result ? (
            <Section title="Result">
              <div className="flex flex-wrap items-center gap-3 rounded-[8px] border border-line bg-panel px-3 py-2">
                <Badge tone={result.exitCode === 0 ? 'positive' : result.exitCode === 3 || result.exitCode === 5 ? 'caution' : 'danger'}>
                  exit {result.exitCode}
                </Badge>
                <span className="text-[11.5px] text-ink-muted">{exitCodeMeaning(result.exitCode)}</span>
                <Mono className="text-ink-dim">{result.durationMs} ms</Mono>
                {result.envelope ? <Badge tone={result.envelope.ok ? 'positive' : 'danger'}>{result.envelope.ok ? 'ok' : 'failed'}</Badge> : <Badge tone="neutral">no envelope</Badge>}
                <Mono className="ml-auto max-w-[18rem] truncate text-ink-dim" title={result.binaryPath}>
                  {result.binaryPath}
                </Mono>
              </div>

              <Tabs
                ariaLabel="Result views"
                value={tab}
                onChange={setTab}
                items={[
                  { value: 'envelope', label: 'Envelope' },
                  { value: 'stdout', label: 'stdout' },
                  { value: 'stderr', label: 'stderr' }
                ]}
              />

              {tab === 'envelope' ? (
                result.envelope ? (
                  <div className="flex flex-col gap-3">
                    <KeyValue
                      entries={[
                        ['schema_version', <Mono>{result.envelope.schema_version}</Mono>],
                        ['command', <Mono>{result.envelope.command}</Mono>],
                        ['ok', <Badge tone={result.envelope.ok ? 'positive' : 'danger'}>{String(result.envelope.ok)}</Badge>],
                        ...(result.envelope.warnings.length > 0
                          ? ([['warnings', <ul className="flex flex-col gap-0.5">{result.envelope.warnings.map((warning) => <li key={warning} className="text-[11.5px] text-brass">{warning}</li>)}</ul>]] as [string, React.ReactNode][])
                          : []),
                        ...(result.envelope.next_actions.length > 0
                          ? ([['next_actions', <ul className="flex flex-col gap-0.5">{result.envelope.next_actions.map((action) => <li key={action} className="text-[11.5px] text-ink-muted">{action}</li>)}</ul>]] as [string, React.ReactNode][])
                          : [])
                      ]}
                    />
                    <JsonView value={result.envelope.data} defaultOpen={2} />
                  </div>
                ) : (
                  <Callout tone="caution" title="No JSON envelope">
                    {result.parseError ?? 'This command was run without --json, so stdout is human text.'}
                  </Callout>
                )
              ) : null}
              {tab === 'stdout' ? (
                <CodeBlock maxHeight="26rem" copy>
                  {result.stdout || '(empty)'}
                </CodeBlock>
              ) : null}
              {tab === 'stderr' ? (
                <CodeBlock maxHeight="16rem" copy className={result.stderr ? 'text-clay/85' : ''}>
                  {result.stderr || '(empty)'}
                </CodeBlock>
              ) : null}
            </Section>
          ) : (
            <Callout tone="info" title="Nothing run yet">
              Pick a feature, fill in its values, and run it. Mutating commands and raw argv always ask for confirmation first.
            </Callout>
          )}
        </div>
      </div>

      <Modal
        open={confirmOpen}
        title="Run this command?"
        description={
          feature?.mutates || rawMode
            ? 'This may write to the workspace. Bob still applies its own ownership rules and refuses conflicts.'
            : 'This command is read-only.'
        }
        onClose={() => setConfirmOpen(false)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmOpen(false)}>
              Cancel
            </Button>
            <Button
              variant={feature?.mutates || rawMode ? 'danger' : 'primary'}
              onClick={() => {
                setConfirmOpen(false)
                void execute(true)
              }}
            >
              Run
            </Button>
          </>
        }
      >
        <KeyValue
          entries={[
            ['command', <Mono className="break-all">bob {displayArgv(argv)}</Mono>],
            ['cwd', <Mono className="break-all">{effectiveCwd}</Mono>],
            ['feature', <Mono>{featureId}</Mono>],
            ['mutates', <Badge tone={feature?.mutates ? 'caution' : 'positive'}>{String(Boolean(feature?.mutates))}</Badge>]
          ]}
        />
      </Modal>
    </PanelShell>
  )
}

export default ConsolePanel
