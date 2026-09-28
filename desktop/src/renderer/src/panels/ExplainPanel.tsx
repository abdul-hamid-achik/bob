import { useEffect } from 'react'
import { Info } from 'lucide-react'
import type { ExplainData } from '@shared/types'
import { Badge, Mono, Section } from '../components/ui'
import { PanelShell } from '../components/shell'
import { FailureNotice, RunButton, RunMeta } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

export function ExplainPanel(): JSX.Element {
  const { revision, navigate } = useStore()
  const explain = useRun<ExplainData>('explain')

  useEffect(() => {
    void explain.run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [revision])

  const data = explain.data

  return (
    <PanelShell
      featureId="explain"
      actions={<RunButton label="Reload" pending={explain.pending} onClick={() => void explain.run()} />}
      subtitle="Bob's own statement of its product contract and boundaries — the shortest honest answer to “what is this tool allowed to do?”"
    >
      {data ? (
        <>
          <Section title="Product" actions={<Info className="size-4 text-copper" aria-hidden />}>
            <p className="max-w-3xl rounded-[8px] border border-line bg-panel px-4 py-3 text-[13.5px] leading-relaxed text-ink">
              {data.product}
            </p>
          </Section>

          <div className="grid grid-cols-2 gap-5">
            <Section title={`Bob owns (${data.owns.length})`} hint="Deterministic, reviewable, and reversible through the ownership ledger.">
              <ul className="flex flex-col gap-1">
                {data.owns.map((item) => (
                  <li key={item} className="flex items-start gap-2 rounded-[6px] border border-jade/25 bg-jade/8 px-2.5 py-1.5">
                    <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-jade" aria-hidden />
                    <span className="text-[12.5px] leading-snug text-ink-muted">{item}</span>
                  </li>
                ))}
              </ul>
            </Section>

            <Section title={`Bob does not own (${data.does_not_own.length})`} hint="These stay with the operator, the agent runtime, or a separate tool with its own public contract.">
              <ul className="flex flex-col gap-1">
                {data.does_not_own.map((item) => (
                  <li key={item} className="flex items-start gap-2 rounded-[6px] border border-clay/25 bg-clay/8 px-2.5 py-1.5">
                    <span className="mt-1.5 size-1.5 shrink-0 rounded-full bg-clay" aria-hidden />
                    <span className="text-[12.5px] leading-snug text-ink-muted">{item}</span>
                  </li>
                ))}
              </ul>
            </Section>
          </div>

          <Section
            title={`Embedded recipes (${data.recipe.length})`}
            hint="Each one is versioned and rendered deterministically."
            actions={
              <Badge tone="info">schema v{data.schema_version}</Badge>
            }
          >
            <ul className="flex flex-wrap gap-1.5">
              {data.recipe.map((recipe) => (
                <li key={recipe}>
                  <button
                    type="button"
                    onClick={() => navigate('recipes')}
                    className="rounded-[6px] border border-line bg-panel px-2 py-1 hover:border-copper/50"
                  >
                    <Mono className="text-ink-muted">{recipe}</Mono>
                  </button>
                </li>
              ))}
            </ul>
          </Section>
        </>
      ) : null}

      <FailureNotice result={explain.result} error={explain.error} errorCode={explain.errorCode} />
      <RunMeta result={explain.result} />
    </PanelShell>
  )
}

export default ExplainPanel
