import { useCallback, useEffect, useMemo, useState } from 'react'
import { BookMarked, ExternalLink } from 'lucide-react'
import { marked } from 'marked'
import type { DocEntry } from '@shared/ipc'
import { Badge, Button, Callout, EmptyState, Mono, Section, TextInput, cx } from '../components/ui'
import { PanelShell } from '../components/shell'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'

marked.setOptions({ gfm: true, breaks: false })

export function DocsPanel(): JSX.Element {
  const { repoRoot, navParams } = useStore()
  const [docs, setDocs] = useState<DocEntry[]>([])
  const [selected, setSelected] = useState<string | null>(null)
  const [markdown, setMarkdown] = useState<string | null>(null)
  const [title, setTitle] = useState('')
  const [relativePath, setRelativePath] = useState('')
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(false)

  const load = useCallback(async () => {
    const list = await bridge().listDocs()
    setDocs(list)
    return list
  }, [])

  useEffect(() => {
    void load().then((list) => {
      const wanted = navParams.doc
      const match = wanted
        ? list.find((entry) => entry.id === wanted || entry.relativePath === wanted || entry.id.endsWith(`/${wanted}`))
        : null
      setSelected(match?.id ?? list[0]?.id ?? null)
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [repoRoot])

  useEffect(() => {
    if (!selected) {
      setMarkdown(null)
      return
    }
    setLoading(true)
    void bridge()
      .readDoc(selected)
      .then((doc) => {
        setMarkdown(doc?.markdown ?? null)
        setTitle(doc?.title ?? selected)
        setRelativePath(doc?.relativePath ?? '')
      })
      .finally(() => setLoading(false))
  }, [selected])

  const html = useMemo(() => {
    if (!markdown) return ''
    return marked.parse(markdown, { async: false }) as string
  }, [markdown])

  const groups = useMemo(() => {
    const map = new Map<string, DocEntry[]>()
    for (const doc of docs) {
      const needle = query.trim().toLowerCase()
      if (needle && !`${doc.title} ${doc.relativePath}`.toLowerCase().includes(needle)) continue
      const list = map.get(doc.group) ?? []
      list.push(doc)
      map.set(doc.group, list)
    }
    return [...map.entries()]
  }, [docs, query])

  return (
    <PanelShell
      featureId="docs"
      actions={
        <Button
          variant="quiet"
          icon={<ExternalLink className="size-3.5" aria-hidden />}
          disabled={!relativePath}
          onClick={async () => {
            if (relativePath.startsWith('docs/')) await bridge().openExternal('https://bobcli.dev/', 'browser')
          }}
        >
          bobcli.dev
        </Button>
      }
      subtitle="The published reference pages and guides, read from the Bob checkout the console resolved. Normative product behavior lives in docs/reference."
    >
      {!repoRoot ? (
        <Callout tone="info" title="No Bob checkout resolved">
          Docs are read from a Bob source tree. Point Settings at a <Mono>./bin/bob</Mono> inside a checkout, or add the checkout as a
          workspace.
        </Callout>
      ) : null}

      <div className="grid grid-cols-[17rem_minmax(0,1fr)] gap-4">
        <div className="flex flex-col gap-3">
          <TextInput value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Filter docs…" aria-label="Filter docs" className="h-7 py-0 text-[11.5px]" />
          <nav aria-label="Documentation" className="max-h-[38rem] overflow-auto rounded-[8px] border border-line bg-panel">
            {groups.length === 0 ? (
              <p className="px-3 py-4 text-[11.5px] text-ink-dim">No documentation found.</p>
            ) : null}
            {groups.map(([group, entries]) => (
              <div key={group}>
                <p className="border-b border-line bg-raise px-2.5 py-1 text-[10.5px] uppercase text-ink-dim" style={{ letterSpacing: '0.05em' }}>
                  {group}
                </p>
                <ul>
                  {entries.map((doc) => (
                    <li key={doc.id}>
                      <button
                        type="button"
                        onClick={() => setSelected(doc.id)}
                        className={cx(
                          'flex w-full flex-col gap-0.5 border-b border-line/60 px-2.5 py-1.5 text-left last:border-b-0',
                          selected === doc.id ? 'bg-copper/10' : 'hover:bg-raise/60'
                        )}
                      >
                        <span className={cx('truncate text-[12px]', selected === doc.id ? 'text-ink' : 'text-ink-muted')}>{doc.title}</span>
                        <Mono className="truncate text-[10px] text-ink-dim">{doc.relativePath}</Mono>
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </nav>
        </div>

        <div className="min-w-0">
          {loading ? (
            <p className="text-[12px] text-ink-dim">Loading…</p>
          ) : markdown ? (
            <Section
              title={title}
              actions={
                <span className="flex items-center gap-2">
                  <Badge tone="neutral">{relativePath}</Badge>
                  <Button size="sm" variant="quiet" onClick={async () => bridge().openExternal(`${repoRoot}/${relativePath}`, 'editor')}>
                    Open in editor
                  </Button>
                </span>
              }
            >
              <article
                className="bob-doc max-h-[42rem] overflow-auto rounded-[8px] border border-line bg-panel px-6 py-5"
                // Rendered from the local Bob checkout's own Markdown, never remote content.
                dangerouslySetInnerHTML={{ __html: html }}
              />
            </Section>
          ) : (
            <EmptyState title="No page selected" hint="Pick a page from the list to read it here." />
          )}
        </div>
      </div>
    </PanelShell>
  )
}

export default DocsPanel
