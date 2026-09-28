import { useCallback, useEffect, useState } from 'react'
import { Boxes } from 'lucide-react'
import type { RecipeSummary } from '@shared/types'
import { Badge, Button, Callout, EmptyState, Mono, Section, TextInput, cx } from '../components/ui'
import { PanelShell } from '../components/shell'
import { FailureNotice, RunMeta } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

export function RecipesPanel(): JSX.Element {
  const { files, revision, navigate } = useStore()
  const list = useRun<RecipeSummary[]>('recipe-list')
  const detail = useRun<RecipeSummary>('recipe-show')
  const [selected, setSelected] = useState<string | null>(null)
  const [query, setQuery] = useState('')

  const load = useCallback(async () => {
    const result = await list.run()
    const recipes = (result.envelope?.data ?? []) as unknown as RecipeSummary[]
    const active = files?.lockRecipe?.id
    if (Array.isArray(recipes) && recipes.length > 0) {
      setSelected((prev) => prev ?? (active && recipes.some((entry) => entry.id === active) ? active : (recipes[0]?.id ?? null)))
    }
  }, [list, files])

  useEffect(() => {
    void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [revision, files?.lockRecipe?.id])

  useEffect(() => {
    if (selected) void detail.run({ values: { recipe: selected } })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selected])

  const recipes = Array.isArray(list.data) ? list.data : []
  const needle = query.trim().toLowerCase()
  const visible = recipes.filter(
    (recipe) =>
      !needle ||
      recipe.id.includes(needle) ||
      recipe.language.toLowerCase().includes(needle) ||
      recipe.stacks.some((stack) => stack.includes(needle))
  )
  const active = detail.data ?? recipes.find((recipe) => recipe.id === selected) ?? null

  return (
    <PanelShell
      featureId="recipe-list"
      actions={
        <Button variant="quiet" loading={list.pending} onClick={() => void load()}>
          Reload catalog
        </Button>
      }
      subtitle="The embedded, versioned recipe catalog. Recipes render desired files deterministically; a published recipe version is never changed in place."
    >
      <Section
        title={`Catalog (${recipes.length})`}
        actions={
          <TextInput value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter recipes…" aria-label="Filter recipes" className="h-7 w-52 py-0 text-[11.5px]" />
        }
      >
        {recipes.length === 0 && !list.pending ? (
          <EmptyState title="No catalog loaded" hint="Run bob recipe list to read the embedded recipes from the resolved binary." action={<Button onClick={() => void load()}>Load catalog</Button>} />
        ) : null}
        <div className="grid grid-cols-[18rem_minmax(0,1fr)] gap-4">
          <ul className="max-h-[36rem] overflow-auto rounded-[8px] border border-line">
            {visible.map((recipe) => (
              <li key={recipe.id}>
                <button
                  type="button"
                  onClick={() => setSelected(recipe.id)}
                  className={cx(
                    'flex w-full flex-col gap-1 border-b border-line/70 px-3 py-2 text-left last:border-b-0',
                    selected === recipe.id ? 'bg-copper/10' : 'hover:bg-raise/60'
                  )}
                >
                  <span className="flex items-center gap-2">
                    <Mono className="text-ink">{recipe.id}</Mono>
                    <Badge tone="neutral">v{recipe.version}</Badge>
                    {files?.lockRecipe?.id === recipe.id ? <Badge tone="positive">active</Badge> : null}
                  </span>
                  <span className="text-[11.5px] leading-snug text-ink-muted">{recipe.language}</span>
                </button>
              </li>
            ))}
          </ul>

          {active ? (
            <div className="flex min-w-0 flex-col gap-4">
              <div className="rounded-[8px] border border-line bg-panel px-4 py-3">
                <div className="flex flex-wrap items-center gap-2">
                  <Boxes className="size-4 text-copper" aria-hidden />
                  <h3 className="text-[15px] font-semibold text-ink">{active.id}</h3>
                  <Badge tone="info">version {active.version}</Badge>
                  {active.stacks.map((stack) => (
                    <Badge key={stack} tone="neutral">
                      stack: {stack}
                    </Badge>
                  ))}
                  {active.surfaces.map((surface) => (
                    <Badge key={surface} tone="accent">
                      {surface}
                    </Badge>
                  ))}
                </div>
                <p className="mt-2 max-w-3xl text-[12.5px] leading-relaxed text-ink-muted">{active.description}</p>
                <p className="mt-1 text-[12px] text-ink-dim">{active.language}</p>
              </div>

              <Callout tone="info" title="Ownership note">
                {active.ownership_note}
              </Callout>

              <Section title={`Seeded paths (${active.seeded_paths.length})`} hint="Every whole file this recipe renders. Anything outside this list is yours.">
                <ul className="grid grid-cols-2 gap-x-4 gap-y-0.5">
                  {active.seeded_paths.map((path) => (
                    <li key={path} className="flex items-center gap-2">
                      <button type="button" onClick={() => navigate('path', { path })} className="min-w-0 flex-1 text-left">
                        <Mono className="block truncate text-ink-muted hover:text-copper">{path}</Mono>
                      </button>
                    </li>
                  ))}
                </ul>
              </Section>

              <div className="flex flex-wrap gap-2">
                <Button variant="quiet" onClick={() => navigate('scaffold', { recipe: active.id })}>
                  Use this recipe in init
                </Button>
                <Button variant="quiet" onClick={() => navigate('playbooks')}>
                  See its playbooks
                </Button>
              </div>
            </div>
          ) : (
            <EmptyState title="No recipe selected" hint="Pick a recipe from the catalog to read its contract." />
          )}
        </div>
      </Section>

      <FailureNotice result={list.result ?? detail.result} error={list.error ?? detail.error} errorCode={list.errorCode ?? detail.errorCode} />
      <RunMeta result={detail.result ?? list.result} />
    </PanelShell>
  )
}

export default RecipesPanel
