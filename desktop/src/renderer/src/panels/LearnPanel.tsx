import { useEffect } from 'react'
import { GraduationCap } from 'lucide-react'
import type { LearnData } from '@shared/types'
import { Badge, Button, CodeBlock, DataTable, Mono, Section, cx, type Column } from '../components/ui'
import { PanelShell } from '../components/shell'
import { FailureNotice, RunButton, RunMeta } from '../components/run'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

export function LearnPanel(): JSX.Element {
  const { revision, navigate, toast } = useStore()
  const learn = useRun<LearnData>('learn')

  useEffect(() => {
    void learn.run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [revision])

  const data = learn.data
  const commandColumns: Column<LearnData['commands'][number]>[] = [
    { key: 'name', header: 'Command', render: (row) => <Mono className="text-ink">bob {row.name}</Mono>, sortValue: (row) => row.name },
    { key: 'purpose', header: 'Purpose', render: (row) => <span className="text-[12px] text-ink-muted">{row.purpose}</span> },
    { key: 'json', header: 'JSON', width: '5rem', render: (row) => <Badge tone={row.json ? 'info' : 'neutral'}>{String(row.json)}</Badge> },
    {
      key: 'mutates',
      header: 'Mutates',
      width: '6rem',
      render: (row) => <Badge tone={row.mutates ? 'caution' : 'positive'}>{String(row.mutates)}</Badge>,
      sortValue: (row) => String(row.mutates)
    }
  ]

  return (
    <PanelShell
      featureId="learn"
      actions={
        <>
          <Button
            variant="quiet"
            onClick={async () => {
              const raw = learn.result?.stdout ?? ''
              await navigator.clipboard.writeText(raw)
              toast('Raw learn JSON copied', 'positive')
            }}
            disabled={!learn.result?.stdout}
          >
            Copy raw JSON
          </Button>
          <RunButton label="Reload brief" pending={learn.pending} onClick={() => void learn.run()} />
        </>
      }
      subtitle="The one-shot onboarding brief Bob emits for coding agents: commands, exit codes, error codes, invariants, lifecycle, and the MCP surface."
    >
      {data ? (
        <>
          <Section title="What Bob is" actions={<GraduationCap className="size-4 text-copper" aria-hidden />}>
            <div className="rounded-[8px] border border-line bg-panel px-4 py-3">
              <p className="text-[13px] font-medium text-ink">{data.product}</p>
              <p className="mt-1.5 max-w-4xl text-[12.5px] leading-relaxed text-ink-muted">{data.summary}</p>
            </div>
          </Section>

          <Section title="Boundaries" hint="What Bob deliberately does not own.">
            <ul className="flex flex-wrap gap-2">
              {data.boundaries.map((boundary) => (
                <li key={boundary}>
                  <Badge tone="danger">{boundary}</Badge>
                </li>
              ))}
            </ul>
          </Section>

          <Section title={`Commands (${data.commands.length})`} hint="Every command with its purpose, JSON support, and whether it mutates.">
            <DataTable rows={data.commands} columns={commandColumns} keyFor={(row) => row.name} dense />
          </Section>

          <div className="grid grid-cols-2 gap-5">
            <Section title="Exit codes">
              <ul className="flex flex-col gap-1.5">
                {Object.entries(data.exit_codes)
                  .sort(([a], [b]) => Number(a) - Number(b))
                  .map(([code, meaning]) => (
                    <li key={code} className="flex gap-3 rounded-[6px] border border-line bg-panel px-2.5 py-1.5">
                      <Mono
                        className={cx(
                          'w-4 shrink-0 text-[13px]',
                          code === '0' ? 'text-jade' : code === '3' || code === '5' ? 'text-brass' : 'text-clay'
                        )}
                      >
                        {code}
                      </Mono>
                      <span className="text-[12px] leading-snug text-ink-muted">{meaning}</span>
                    </li>
                  ))}
              </ul>
            </Section>

            <Section title="Error codes">
              <ul className="flex flex-col gap-1.5">
                {Object.entries(data.error_codes).map(([code, meaning]) => (
                  <li key={code} className="rounded-[6px] border border-line bg-panel px-2.5 py-1.5">
                    <Mono className="text-clay">{code}</Mono>
                    <p className="mt-0.5 text-[12px] leading-snug text-ink-muted">{meaning}</p>
                  </li>
                ))}
              </ul>
            </Section>
          </div>

          <Section title="Invariants">
            <ul className="flex flex-col gap-1.5">
              {data.invariants.map((invariant) => (
                <li key={invariant} className="rounded-[6px] border border-line bg-sunken px-3 py-2 text-[12.5px] leading-relaxed text-ink-muted">
                  {invariant}
                </li>
              ))}
            </ul>
          </Section>

          <div className="grid grid-cols-2 gap-5">
            <Section title="Lifecycle">
              <ol className="flex flex-col gap-1.5">
                {data.lifecycle.map((step, index) => (
                  <li key={step} className="flex gap-2.5">
                    <span className="mt-0.5 flex size-4.5 shrink-0 items-center justify-center rounded-full border border-line-strong font-mono text-[10px] text-ink-dim">
                      {index + 1}
                    </span>
                    <span className="text-[12px] leading-snug text-ink-muted">{step}</span>
                  </li>
                ))}
              </ol>
            </Section>

            <Section title="JSON envelope">
              <div className="flex flex-col gap-2">
                <p className="text-[12px] text-ink-muted">
                  Flag: <Mono className="text-copper">{data.json_envelope.flag}</Mono>
                </p>
                <ul className="flex flex-wrap gap-1.5">
                  {data.json_envelope.fields.map((field) => (
                    <li key={field}>
                      <Mono className="rounded border border-line bg-sunken px-1.5 py-0.5 text-ink-muted">{field}</Mono>
                    </li>
                  ))}
                </ul>
                <p className="text-[11.5px] leading-relaxed text-ink-dim">{data.json_envelope.notes}</p>
              </div>
            </Section>
          </div>

          <Section title="MCP surface" hint="Repository read-only, with an exact startup workspace allowlist by default.">
            <div className="flex flex-col gap-3">
              <div className="flex flex-wrap items-center gap-2">
                <Mono className="rounded border border-line bg-sunken px-2 py-1 text-ink">{data.mcp.serve}</Mono>
                <Button size="sm" variant="quiet" onClick={() => navigate('mcp')}>
                  Open the MCP console
                </Button>
              </div>
              <p className="text-[12px] leading-relaxed text-ink-muted">{data.mcp.authority}</p>
              <ul className="grid grid-cols-3 gap-1.5">
                {data.mcp.tools.map((tool) => (
                  <li key={tool}>
                    <Mono className="block rounded border border-line bg-panel px-2 py-1 text-ink-muted">{tool}</Mono>
                  </li>
                ))}
              </ul>
            </div>
          </Section>

          <Section title={`Recipes in this binary (${data.recipes.length})`}>
            <ul className="grid grid-cols-2 gap-1">
              {data.recipes.map((recipe) => (
                <li key={recipe.id} className="flex items-baseline gap-2">
                  <Mono className="text-ink">{recipe.id}</Mono>
                  <Mono className="text-ink-dim">v{recipe.version}</Mono>
                  <span className="min-w-0 flex-1 truncate text-[11.5px] text-ink-muted">{recipe.description}</span>
                </li>
              ))}
            </ul>
          </Section>

          <Section title="Agent bootstrap" hint="What Bob recommends an agent run before driving it.">
            <ul className="flex flex-col gap-1.5">
              {data.recommended_agent_bootstrap.map((step) => (
                <li key={step} className="rounded-[6px] border border-line bg-sunken px-2.5 py-1.5">
                  <Mono className="text-ink-muted">{step}</Mono>
                </li>
              ))}
            </ul>
            <div className="flex flex-wrap gap-2">
              <Button size="sm" variant="quiet" onClick={async () => { await bridge().openExternal(data.docs.site, 'browser') }}>
                {data.docs.site}
              </Button>
              <Button size="sm" variant="quiet" onClick={async () => { await bridge().openExternal(data.docs.reference, 'browser') }}>
                CLI reference
              </Button>
              <Button size="sm" variant="quiet" onClick={async () => { await bridge().openExternal(data.docs.agents, 'browser') }}>
                Agent guide
              </Button>
            </div>
          </Section>

          <Section title="Raw brief">
            <CodeBlock maxHeight="20rem" copy>
              {learn.result?.stdout ?? ''}
            </CodeBlock>
          </Section>
        </>
      ) : null}

      <FailureNotice result={learn.result} error={learn.error} errorCode={learn.errorCode} title="Could not read the brief" />
      <RunMeta result={learn.result} />
    </PanelShell>
  )
}

export default LearnPanel
