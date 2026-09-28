import { useEffect, useMemo, useRef, useState } from 'react'
import { CornerDownLeft, Search } from 'lucide-react'
import { FEATURES } from '@shared/features'
import { exitCodeMeaning } from '@shared/envelope'
import { formatDuration, formatTimestamp } from '@shared/format'
import { Badge, CodeBlock, IconButton, JsonView, Label, Mono, Tabs, cx } from './ui'
import { ALL_NAV_ITEMS } from './nav'
import { useStore } from '../state/store'

interface PaletteRow {
  id: string
  title: string
  subtitle: string
  group: string
  keywords: string
  run: () => void
}

export function CommandPalette(): JSX.Element {
  const { paletteOpen, setPaletteOpen, navigate } = useStore()
  const [query, setQuery] = useState('')
  const [cursor, setCursor] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLUListElement>(null)

  const rows = useMemo<PaletteRow[]>(() => {
    const panelRows: PaletteRow[] = ALL_NAV_ITEMS.map((item) => ({
      id: `panel:${item.panel}`,
      title: item.label,
      subtitle: item.hint,
      group: 'Go to',
      keywords: `${item.label} ${item.panel}`,
      run: () => navigate(item.panel)
    }))
    const featureRows: PaletteRow[] = FEATURES.map((feature) => ({
      id: `feature:${feature.id}`,
      title: feature.title,
      subtitle: feature.command,
      group: feature.group,
      keywords: `${feature.title} ${feature.command} ${feature.summary} ${feature.id}`,
      run: () => navigate(feature.panel)
    }))
    return [...panelRows, ...featureRows]
  }, [navigate])

  const matches = useMemo(() => {
    const needle = query.trim().toLowerCase()
    if (!needle) return rows.slice(0, 40)
    const scored = rows
      .map((row) => {
        const haystack = `${row.title} ${row.subtitle} ${row.keywords}`.toLowerCase()
        if (!haystack.includes(needle)) return null
        const at = haystack.indexOf(needle)
        const titleHit = row.title.toLowerCase().indexOf(needle)
        return { row, score: titleHit === 0 ? 0 : titleHit > 0 ? 1 : at + 2 }
      })
      .filter((entry): entry is { row: PaletteRow; score: number } => entry !== null)
    return scored.sort((a, b) => a.score - b.score).slice(0, 40).map((entry) => entry.row)
  }, [query, rows])

  useEffect(() => {
    if (paletteOpen) {
      setQuery('')
      setCursor(0)
      setTimeout(() => inputRef.current?.focus(), 0)
    }
  }, [paletteOpen])

  useEffect(() => {
    if (!paletteOpen) return
    const onKey = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') {
        event.preventDefault()
        setPaletteOpen(false)
        return
      }
      if (event.key === 'ArrowDown') {
        event.preventDefault()
        setCursor((prev) => Math.min(prev + 1, matches.length - 1))
        return
      }
      if (event.key === 'ArrowUp') {
        event.preventDefault()
        setCursor((prev) => Math.max(prev - 1, 0))
        return
      }
      if (event.key === 'Enter') {
        event.preventDefault()
        const row = matches[cursor]
        if (row) {
          row.run()
          setPaletteOpen(false)
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [paletteOpen, matches, cursor, setPaletteOpen])

  useEffect(() => {
    const active = listRef.current?.children[cursor] as HTMLElement | undefined
    active?.scrollIntoView({ block: 'nearest' })
  }, [cursor])

  if (!paletteOpen) return <></>
  return (
    <div className="fixed inset-0 z-70 flex items-start justify-center bg-black/55 pt-[12vh]" role="presentation" onClick={() => setPaletteOpen(false)}>
      <div
        className="w-full max-w-2xl overflow-hidden rounded-[10px] border border-line-strong bg-panel shadow-2xl"
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="flex items-center gap-2 border-b border-line px-3">
          <Search className="size-4 text-ink-dim" aria-hidden />
          <input
            ref={inputRef}
            value={query}
            onChange={(event) => {
              setQuery(event.target.value)
              setCursor(0)
            }}
            placeholder="Search features, commands, panels…"
            aria-label="Search features"
            className="h-11 flex-1 bg-transparent text-[13px] text-ink placeholder:text-ink-dim focus:outline-none"
          />
          <kbd className="rounded border border-line bg-raise px-1.5 py-0.5 font-mono text-[10px] text-ink-dim">esc</kbd>
        </div>
        <ul ref={listRef} className="max-h-[50vh] overflow-y-auto py-1" role="listbox" aria-label="Results">
          {matches.length === 0 ? (
            <li className="px-3 py-6 text-[12px] text-ink-dim">No feature matches “{query}”.</li>
          ) : null}
          {matches.map((row, index) => (
            <li
              key={row.id}
              role="option"
              aria-selected={index === cursor}
              onMouseEnter={() => setCursor(index)}
              onClick={() => {
                row.run()
                setPaletteOpen(false)
              }}
              className={cx(
                'flex cursor-pointer items-center gap-3 px-3 py-2',
                index === cursor ? 'bg-copper/12' : 'hover:bg-raise/60'
              )}
            >
              <div className="min-w-0 flex-1">
                <p className="truncate text-[12.5px] text-ink">{row.title}</p>
                <p className="truncate font-mono text-[11px] text-ink-dim">{row.subtitle}</p>
              </div>
              <Badge tone="neutral">{row.group}</Badge>
              {index === cursor ? <CornerDownLeft className="size-3.5 shrink-0 text-copper" aria-hidden /> : null}
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}

/* -------------------------------------------------------------- inspector -- */

type InspectorTab = 'envelope' | 'stdout' | 'stderr' | 'argv'

export function Inspector({ onClose }: { onClose: () => void }): JSX.Element {
  const { inspector } = useStore()
  const [tab, setTab] = useState<InspectorTab>('envelope')
  const result = inspector.result

  useEffect(() => {
    setTab('envelope')
  }, [result?.requestId])

  return (
    <aside
      aria-label="Command output inspector"
      className="flex w-[26rem] shrink-0 flex-col border-l border-line bg-panel"
    >
      <div className="flex items-center justify-between gap-2 border-b border-line px-3 py-2">
        <div className="min-w-0">
          <Label>Output</Label>
          <p className="truncate text-[12.5px] text-ink">{inspector.title || 'No command yet'}</p>
        </div>
        <IconButton label="Close inspector" icon={<span className="text-[13px]">×</span>} onClick={onClose} />
      </div>

      {!result ? (
        <div className="flex flex-1 items-center justify-center px-6 text-center">
          <p className="text-[12px] leading-relaxed text-ink-dim">
            Run any command and its envelope, stdout, stderr, and argv land here.
          </p>
        </div>
      ) : (
        <>
          <div className="flex flex-wrap items-center gap-2 border-b border-line px-3 py-2">
            <Badge tone={result.exitCode === 0 ? 'positive' : result.exitCode === 3 || result.exitCode === 5 ? 'caution' : 'danger'}>
              exit {result.exitCode}
            </Badge>
            <Mono className="text-ink-dim">{exitCodeMeaning(result.exitCode)}</Mono>
            <Mono className="ml-auto text-ink-dim">{formatDuration(result.durationMs)}</Mono>
          </div>
          <div className="px-3 pt-2">
            <Tabs
              ariaLabel="Output views"
              value={tab}
              onChange={setTab}
              items={[
                { value: 'envelope', label: 'Envelope' },
                { value: 'stdout', label: 'stdout' },
                { value: 'stderr', label: 'stderr' },
                { value: 'argv', label: 'argv' }
              ]}
            />
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto p-3">
            {tab === 'envelope' ? (
              result.envelope ? (
                <div className="flex flex-col gap-2">
                  <div className="flex flex-wrap items-center gap-2">
                    <Badge tone={result.envelope.ok ? 'positive' : 'danger'}>{result.envelope.ok ? 'ok' : 'failed'}</Badge>
                    <Mono className="text-ink-dim">schema_version {result.envelope.schema_version}</Mono>
                    <Mono className="text-ink-dim">command {result.envelope.command}</Mono>
                  </div>
                  {result.envelope.warnings.length > 0 ? (
                    <ul className="flex flex-col gap-1">
                      {result.envelope.warnings.map((warning) => (
                        <li key={warning} className="rounded border border-brass/40 bg-brass/10 px-2 py-1 text-[11.5px] text-brass">
                          {warning}
                        </li>
                      ))}
                    </ul>
                  ) : null}
                  <JsonView value={result.envelope.data} defaultOpen={2} />
                  {result.envelope.next_actions.length > 0 ? (
                    <div className="flex flex-col gap-1">
                      <Label>next actions</Label>
                      <ul className="flex flex-col gap-1">
                        {result.envelope.next_actions.map((action) => (
                          <li key={action} className="text-[11.5px] text-ink-muted">
                            {action}
                          </li>
                        ))}
                      </ul>
                    </div>
                  ) : null}
                </div>
              ) : (
                <p className="text-[12px] text-ink-dim">
                  No JSON envelope. {result.parseError ? result.parseError : 'This command did not request --json.'}
                </p>
              )
            ) : null}
            {tab === 'stdout' ? (
              <CodeBlock maxHeight="none" copy>
                {result.stdout || '(empty)'}
              </CodeBlock>
            ) : null}
            {tab === 'stderr' ? (
              <CodeBlock maxHeight="none" copy className={result.stderr ? 'text-clay/85' : ''}>
                {result.stderr || '(empty)'}
              </CodeBlock>
            ) : null}
            {tab === 'argv' ? (
              <div className="flex flex-col gap-3">
                <div>
                  <Label>binary</Label>
                  <Mono className="mt-1 block break-all text-ink-muted">{result.binaryPath || '—'}</Mono>
                </div>
                <div>
                  <Label>cwd</Label>
                  <Mono className="mt-1 block break-all text-ink-muted">{result.cwd}</Mono>
                </div>
                <div>
                  <Label>argv</Label>
                  <ol className="mt-1 flex flex-col gap-0.5">
                    {result.argv.map((token, index) => (
                      <li key={`${token}-${index}`} className="flex gap-2">
                        <span className="w-5 shrink-0 text-right font-mono text-[11px] text-ink-dim">{index}</span>
                        <Mono className="break-all text-ink">{token}</Mono>
                      </li>
                    ))}
                  </ol>
                </div>
                <div>
                  <Label>started</Label>
                  <Mono className="mt-1 block text-ink-muted">{formatTimestamp(result.startedAt)}</Mono>
                </div>
                <div className="flex gap-2">
                  {result.timedOut ? <Badge tone="caution">timed out</Badge> : null}
                  {result.cancelled ? <Badge tone="caution">cancelled</Badge> : null}
                </div>
              </div>
            ) : null}
          </div>
        </>
      )}
    </aside>
  )
}
