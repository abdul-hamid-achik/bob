import { useCallback, useEffect, useMemo, useState } from 'react'
import { ExternalLink, FolderOpen, Search } from 'lucide-react'
import type { PathEntry } from '@shared/ipc'
import { formatBytes, shortDigest } from '@shared/format'
import type { PlanAction } from '@shared/types'
import { Badge, Button, CodeBlock, EmptyState, Mono, Section, Segmented, TextInput, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'

type OwnershipFilter = 'all' | 'managed' | 'unmanaged' | 'lock-only' | 'directory'

const OWNERSHIP_TONE: Record<PathEntry['kind'], 'info' | 'positive' | 'neutral' | 'caution'> = {
  managed: 'info',
  seed: 'positive',
  unmanaged: 'neutral',
  'lock-only': 'caution',
  directory: 'neutral'
}

export function FilesPanel(): JSX.Element {
  const { workspace, plan, navigate, revision, toast, navParams } = useStore()
  const [entries, setEntries] = useState<PathEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [filter, setFilter] = useState<OwnershipFilter>('all')
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState<PathEntry | null>(null)
  const [content, setContent] = useState<{ path: string; text: string; truncated: boolean; binary: boolean } | null>(null)

  const load = useCallback(async () => {
    if (!workspace) return
    setLoading(true)
    setError(null)
    try {
      setEntries(await bridge().listPaths(workspace))
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught))
    } finally {
      setLoading(false)
    }
  }, [workspace])

  useEffect(() => {
    void load()
    setSelected(null)
    setContent(null)
  }, [load, revision])

  useEffect(() => {
    const wanted = navParams.path
    if (!wanted) return
    const match = entries.find((entry) => entry.path === wanted)
    if (match) void open(match)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [entries, navParams.path])

  const actions = useMemo(() => {
    const map = new Map<string, PlanAction>()
    for (const action of plan?.actions ?? []) map.set(action.path, action)
    return map
  }, [plan])

  const counts = useMemo(() => {
    const tally: Record<string, number> = { all: entries.length, managed: 0, unmanaged: 0, 'lock-only': 0, directory: 0 }
    for (const entry of entries) tally[entry.kind] = (tally[entry.kind] ?? 0) + 1
    return tally
  }, [entries])

  const visible = entries.filter((entry) => {
    if (filter !== 'all' && entry.kind !== filter) return false
    const needle = query.trim().toLowerCase()
    return !needle || entry.path.toLowerCase().includes(needle)
  })

  async function open(entry: PathEntry): Promise<void> {
    setSelected(entry)
    if (entry.kind === 'directory' || !entry.exists) {
      setContent(null)
      return
    }
    try {
      const file = await bridge().readFile(workspace, entry.path)
      setContent({ path: file.path, text: file.content, truncated: file.truncated, binary: file.binary })
    } catch {
      setContent(null)
    }
  }

  return (
    <PanelShell
      featureId="files"
      actions={
        <>
          <Button variant="quiet" loading={loading} onClick={() => void load()}>
            Rescan
          </Button>
          <Button variant="default" icon={<FolderOpen className="size-3.5" aria-hidden />} onClick={async () => {
            const outcome = await bridge().openExternal(workspace, 'finder')
            if (!outcome.opened) toast(outcome.error ?? 'Could not open the folder', 'danger')
          }}>
            Reveal workspace
          </Button>
        </>
      }
      subtitle="The working tree joined with the lock ledger and the current plan, so ownership is visible per path. Heavy build and dependency directories are skipped."
    >
      <WorkspaceGate>
        {error ? (
          <EmptyState title="Could not read the workspace" hint={error} action={<Button onClick={() => void load()}>Try again</Button>} />
        ) : null}

        <Section
          title={`Paths (${visible.length} of ${entries.length})`}
          actions={
            <>
              <Segmented
                ariaLabel="Ownership filter"
                value={filter}
                onChange={setFilter}
                options={[
                  { value: 'all', label: 'all', count: counts.all ?? 0 },
                  { value: 'managed', label: 'managed', count: counts.managed ?? 0 },
                  { value: 'unmanaged', label: 'unmanaged', count: counts.unmanaged ?? 0 },
                  { value: 'lock-only', label: 'lock only', count: counts['lock-only'] ?? 0 }
                ]}
              />
              <div className="relative">
                <Search className="pointer-events-none absolute top-1/2 left-2 size-3.5 -translate-y-1/2 text-ink-dim" aria-hidden />
                <TextInput
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Filter paths…"
                  aria-label="Filter paths"
                  className="h-7 w-52 py-0 pl-7 font-mono text-[11.5px]"
                />
              </div>
            </>
          }
        >
          <div className="grid grid-cols-[minmax(0,1fr)_26rem] gap-3">
            <div className="max-h-[38rem] overflow-auto rounded-[8px] border border-line">
              <table className="w-full border-collapse text-[12px]">
                <thead className="sticky top-0 z-10 bg-raise">
                  <tr className="text-left text-[10.5px] uppercase text-ink-dim">
                    <th className="px-2.5 py-1.5 font-medium">Path</th>
                    <th className="px-2.5 py-1.5 font-medium">Ownership</th>
                    <th className="px-2.5 py-1.5 font-medium">Plan</th>
                    <th className="px-2.5 py-1.5 text-right font-medium">Size</th>
                  </tr>
                </thead>
                <tbody>
                  {visible.slice(0, 600).map((entry) => {
                    const action = actions.get(entry.path)
                    return (
                      <tr
                        key={entry.path}
                        onClick={() => void open(entry)}
                        tabIndex={0}
                        onKeyDown={(event) => {
                          if (event.key === 'Enter') void open(entry)
                        }}
                        className={cx(
                          'cursor-pointer border-b border-line/70 last:border-b-0',
                          selected?.path === entry.path ? 'bg-copper/10' : 'hover:bg-raise/60'
                        )}
                      >
                        <td className="px-2.5 py-1">
                          <Mono className={cx(entry.kind === 'directory' ? 'text-ink-dim' : 'text-ink')}>{entry.path}</Mono>
                        </td>
                        <td className="px-2.5 py-1">
                          <Badge tone={OWNERSHIP_TONE[entry.kind]}>{entry.kind}</Badge>
                        </td>
                        <td className="px-2.5 py-1">
                          {action ? (
                            <span className="inline-flex items-center gap-1.5">
                              <Badge tone={action.kind === 'conflict' ? 'danger' : action.kind === 'unchanged' ? 'neutral' : 'caution'}>{action.kind}</Badge>
                              <Mono className="text-ink-dim">{action.code}</Mono>
                            </span>
                          ) : (
                            <span className="text-[11.5px] text-ink-dim">—</span>
                          )}
                        </td>
                        <td className="px-2.5 py-1 text-right">
                          <Mono className="text-ink-dim">{entry.bytes === null ? '—' : formatBytes(entry.bytes)}</Mono>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
              {visible.length > 600 ? (
                <p className="border-t border-line px-2.5 py-1.5 text-[11.5px] text-ink-dim">
                  Showing the first 600 of {visible.length} matches. Narrow the filter to see the rest.
                </p>
              ) : null}
            </div>

            <div className="flex min-w-0 flex-col gap-3">
              {selected ? (
                <div className="rounded-[8px] border border-line bg-panel p-3">
                  <div className="flex items-start justify-between gap-2">
                    <Mono className="min-w-0 break-all text-ink">{selected.path}</Mono>
                    <Badge tone={OWNERSHIP_TONE[selected.kind]}>{selected.kind}</Badge>
                  </div>
                  <dl className="mt-2 grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-[11.5px]">
                    <dt className="text-ink-dim">locked</dt>
                    <dd>
                      <Mono title={selected.lockedSha256 ?? ''}>{shortDigest(selected.lockedSha256 ?? undefined, 12)}</Mono>
                    </dd>
                    <dt className="text-ink-dim">exists</dt>
                    <dd className="text-ink-muted">{String(selected.exists)}</dd>
                    <dt className="text-ink-dim">size</dt>
                    <dd className="text-ink-muted">{selected.bytes === null ? '—' : formatBytes(selected.bytes)}</dd>
                    <dt className="text-ink-dim">modified</dt>
                    <dd className="text-ink-muted">{selected.mtime ? new Date(selected.mtime).toLocaleString() : '—'}</dd>
                  </dl>
                  <div className="mt-3 flex flex-wrap gap-2 border-t border-line pt-2.5">
                    <Button size="sm" variant="quiet" onClick={() => navigate('path', { path: selected.path })}>
                      Classify
                    </Button>
                    <Button
                      size="sm"
                      variant="quiet"
                      icon={<ExternalLink className="size-3.5" aria-hidden />}
                      disabled={selected.kind === 'directory'}
                      onClick={async () => {
                        const outcome = await bridge().openExternal(`${workspace}/${selected.path}`, 'editor')
                        if (!outcome.opened) toast(outcome.error ?? 'Could not open the file', 'danger')
                      }}
                    >
                      Open
                    </Button>
                  </div>
                  {content ? (
                    content.binary ? (
                      <p className="mt-3 text-[11.5px] text-ink-dim">Binary file; no text preview.</p>
                    ) : (
                      <div className="mt-3">
                        <CodeBlock maxHeight="18rem">{content.text || '(empty file)'}</CodeBlock>
                        {content.truncated ? <p className="mt-1 text-[11px] text-brass">Preview truncated to the first 512 KB.</p> : null}
                      </div>
                    )
                  ) : null}
                </div>
              ) : (
                <EmptyState title="No path selected" hint="Pick a path to see its ownership, hashes, and a bounded content preview." />
              )}
            </div>
          </div>
        </Section>
      </WorkspaceGate>
    </PanelShell>
  )
}

export default FilesPanel
