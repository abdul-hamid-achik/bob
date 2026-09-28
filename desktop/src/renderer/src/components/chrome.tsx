import { useCallback, useEffect, useState } from 'react'
import {
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Command,
  FolderPlus,
  Moon,
  PanelRightClose,
  PanelRightOpen,
  RefreshCw,
  Sun,
  X
} from 'lucide-react'
import { featureById } from '@shared/features'
import { shortDigest } from '@shared/format'
import type { PanelId } from '@shared/features'
import { Badge, Button, IconButton, Label, Mono, cx } from './ui'
import { BrandMark } from './BrandMark'
import { ALL_NAV_ITEMS, NAV_BY_PANEL, NAV_SECTIONS } from './nav'
import { useStore } from '../state/store'
import { bridge } from '../state/bridge'

const IS_MAC = typeof navigator !== 'undefined' && /Mac|iPod|iPhone|iPad/.test(navigator.userAgent)

/* ------------------------------------------------------------- titlebar --- */

export function Titlebar(): JSX.Element {
  const { workspaces, workspace, selectWorkspace, chooseWorkspace, binary, plan, demo, settings, updateSettings, navigate, setPaletteOpen, reloadFiles, revision, bump } =
    useStore()
  const [busy, setBusy] = useState(false)

  const onRefresh = useCallback(async () => {
    setBusy(true)
    try {
      await reloadFiles()
      bump()
    } finally {
      setBusy(false)
    }
  }, [reloadFiles, bump])

  return (
    <header
      className={cx(
        'flex h-11 shrink-0 items-center gap-3 border-b border-line bg-panel pr-3',
        IS_MAC ? 'pl-[86px]' : 'pl-3'
      )}
    >
      <div className="drag-region flex min-w-0 flex-1 items-center gap-3">
        <div className="no-drag flex items-center gap-2">
          <BrandMark size={26} className="shrink-0 rounded-[6px]" />
          <div className="leading-none">
            <p className="text-[13px] font-semibold text-ink">Bob Console</p>
            <p className="mt-0.5 text-[10px] text-ink-dim">deterministic repository factory</p>
          </div>
        </div>

        <div className="no-drag ml-2 flex min-w-0 items-center gap-1.5">
          <div className="relative">
            <select
              aria-label="Active workspace"
              value={workspace}
              onChange={(event) => void selectWorkspace(event.target.value)}
              className="h-7 max-w-[22rem] appearance-none rounded-[6px] border border-line-strong bg-raise py-0 pr-7 pl-2.5 font-mono text-[11.5px] text-ink hover:border-ink-dim focus:border-copper focus:outline-none"
            >
              {workspace === '' ? <option value="">No workspace selected</option> : null}
              {workspaces.map((entry) => (
                <option key={entry.path} value={entry.path}>
                  {entry.name} — {entry.path}
                </option>
              ))}
            </select>
            <ChevronDown className="pointer-events-none absolute top-1/2 right-2 size-3.5 -translate-y-1/2 text-ink-dim" aria-hidden />
          </div>
          <IconButton label="Choose a workspace folder" icon={<FolderPlus className="size-4" aria-hidden />} onClick={() => void chooseWorkspace()} />
        </div>
      </div>

      <div className="no-drag flex shrink-0 items-center gap-2">
        {demo ? <Badge tone="caution">Demo replay</Badge> : null}
        {plan ? (
          <button
            type="button"
            onClick={() => navigate('plan')}
            title={`Last reviewed plan digest: ${plan.digest}`}
            className="inline-flex h-7 items-center gap-1.5 rounded-[6px] border border-line bg-raise px-2 hover:border-line-strong"
          >
            <Label>digest</Label>
            <Mono className={plan.conflictCount > 0 ? 'text-clay' : 'text-jade'}>{shortDigest(plan.digest)}</Mono>
            {plan.conflictCount > 0 ? <Badge tone="danger">{plan.conflictCount} conflicts</Badge> : null}
          </button>
        ) : null}
        <button
          type="button"
          onClick={() => navigate('app-settings')}
          title={binary?.path ?? 'no bob binary resolved'}
          className="inline-flex h-7 items-center gap-1.5 rounded-[6px] border border-line bg-raise px-2 hover:border-line-strong"
        >
          <span className={cx('size-1.5 rounded-full', binary?.exists ? 'bg-jade' : 'bg-clay')} aria-hidden />
          <Mono className="text-ink-muted">{binary?.version ?? 'no binary'}</Mono>
        </button>
        <IconButton
          label="Command palette"
          icon={<Command className="size-4" aria-hidden />}
          onClick={() => setPaletteOpen(true)}
        />
        <IconButton
          label={settings.theme === 'light' ? 'Switch to dark theme' : 'Switch to light theme'}
          icon={settings.theme === 'light' ? <Moon className="size-4" aria-hidden /> : <Sun className="size-4" aria-hidden />}
          onClick={() => void updateSettings({ theme: settings.theme === 'light' ? 'dark' : 'light' })}
        />
        <IconButton label="Reload workspace files" icon={<RefreshCw className={cx('size-4', busy && 'animate-spin')} aria-hidden />} onClick={() => void onRefresh()} />
      </div>
      <span className="sr-only" aria-live="polite">
        revision {revision}
      </span>
    </header>
  )
}

/* --------------------------------------------------------------- nav rail -- */

export function NavRail({
  inspectorOpen,
  onToggleInspector
}: {
  inspectorOpen: boolean
  onToggleInspector: () => void
}): JSX.Element {
  const { panel, navigate } = useStore()
  const [collapsed, setCollapsed] = useState(false)

  useEffect(() => {
    const onKey = (event: KeyboardEvent): void => {
      const index = Number(event.key)
      if (!event.altKey || Number.isNaN(index) || index < 1 || index > 9) return
      const item = ALL_NAV_ITEMS[index - 1]
      if (item) navigate(item.panel)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navigate])

  return (
    <nav
      aria-label="Bob features"
      className={cx('flex shrink-0 flex-col border-r border-line bg-panel transition-[width] duration-150', collapsed ? 'w-14' : 'w-52')}
    >
      <div className="flex-1 overflow-y-auto py-2">
        {NAV_SECTIONS.map((section) => (
          <div key={section.id} className="mb-1">
            {!collapsed ? (
              <p className="px-3 py-1.5">
                <Label>{section.label}</Label>
              </p>
            ) : (
              <div className="mx-3 my-2 border-t border-line" />
            )}
            <ul>
              {section.items.map((item) => {
                const active = panel === item.panel
                const Icon = item.icon
                const feature = featureById(item.featureId)
                return (
                  <li key={item.panel}>
                    <button
                      type="button"
                      onClick={() => navigate(item.panel as PanelId)}
                      aria-current={active ? 'page' : undefined}
                      title={`${item.label}${feature ? ` — ${feature.command}` : ''}`}
                      className={cx(
                        'group flex w-full items-center gap-2.5 border-l-2 py-1.5 text-left transition-colors duration-100',
                        collapsed ? 'justify-center px-0' : 'px-2.5',
                        active
                          ? 'border-copper bg-copper/10 text-ink'
                          : 'border-transparent text-ink-muted hover:border-line-strong hover:bg-raise/60 hover:text-ink'
                      )}
                    >
                      <Icon className={cx('size-4 shrink-0', active ? 'text-copper' : 'text-ink-dim group-hover:text-ink-muted')} aria-hidden />
                      {!collapsed ? <span className="truncate text-[12.5px]">{item.label}</span> : null}
                      {!collapsed && feature?.mutates ? <span className="ml-auto size-1.5 shrink-0 rounded-full bg-brass" title="Can mutate" aria-hidden /> : null}
                    </button>
                  </li>
                )
              })}
            </ul>
          </div>
        ))}
      </div>
      <div className={cx('flex shrink-0 items-center border-t border-line py-1.5', collapsed ? 'justify-center' : 'justify-between px-2')}>
        {!collapsed ? <span className="text-[10.5px] text-ink-dim">{ALL_NAV_ITEMS.length} surfaces</span> : null}
        <div className="flex items-center gap-0.5">
          <IconButton
            label={inspectorOpen ? 'Hide output inspector' : 'Show output inspector'}
            icon={inspectorOpen ? <PanelRightClose className="size-4" aria-hidden /> : <PanelRightOpen className="size-4" aria-hidden />}
            onClick={onToggleInspector}
          />
          <IconButton
            label={collapsed ? 'Expand navigation' : 'Collapse navigation'}
            icon={collapsed ? <ChevronRight className="size-4" aria-hidden /> : <ChevronLeft className="size-4" aria-hidden />}
            onClick={() => setCollapsed((prev) => !prev)}
          />
        </div>
      </div>
    </nav>
  )
}

/* ------------------------------------------------------------- panel head -- */

export function PanelHeader({
  featureId,
  title,
  subtitle,
  actions
}: {
  featureId: string
  title?: string
  subtitle?: string
  actions?: React.ReactNode
}): JSX.Element {
  const { navigate } = useStore()
  const feature = featureById(featureId)
  const item = feature ? NAV_BY_PANEL.get(feature.panel) : undefined
  const heading = title ?? item?.label ?? feature?.title ?? featureId

  return (
    <div className="flex shrink-0 items-start justify-between gap-4 border-b border-line bg-panel px-5 py-3">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <h1 className="text-[15px] font-semibold text-ink">{heading}</h1>
          {feature ? <Mono className="rounded border border-line bg-raise px-1.5 py-0.5 text-ink-muted">{feature.command}</Mono> : null}
          {feature?.mutates ? <Badge tone="caution">can mutate</Badge> : null}
          {feature?.authority ? <Badge tone="danger">explicit authority</Badge> : null}
          {feature?.surface === 'mcp' ? <Badge tone="info">MCP</Badge> : null}
        </div>
        <p className="mt-1 max-w-3xl text-[12px] leading-relaxed text-ink-muted">
          {subtitle ?? feature?.summary ?? item?.hint ?? ''}
        </p>
        {feature?.docs && feature.docs.length > 0 ? (
          <div className="mt-1.5 flex flex-wrap items-center gap-2">
            {feature.docs.map((doc) => (
              <button
                key={doc}
                type="button"
                onClick={() => navigate('docs', { doc: doc.replace(/^docs\//, '').replace(/\.md$/, '') })}
                className="text-[11px] text-ink-dim underline decoration-line underline-offset-2 hover:text-copper"
              >
                {doc}
              </button>
            ))}
          </div>
        ) : null}
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
    </div>
  )
}

/* ------------------------------------------------------------- status bar -- */

export function StatusBar(): JSX.Element {
  const { activity, workspace, plan, binary, demo, settings } = useStore()
  const last = activity[0]
  return (
    <footer className="flex h-7 shrink-0 items-center gap-4 border-t border-line bg-panel px-3 text-[11px] text-ink-dim">
      <span className="inline-flex items-center gap-1.5">
        <span className={cx('size-1.5 rounded-full', demo ? 'bg-brass' : binary?.exists ? 'bg-jade' : 'bg-clay')} aria-hidden />
        {demo ? 'demo replay — no binary executed' : binary?.exists ? binary.path : 'no bob binary'}
      </span>
      {last ? (
        <span className="inline-flex min-w-0 items-center gap-2">
          <Mono className={cx('shrink-0', last.ok ? 'text-jade' : 'text-clay')}>exit {last.exitCode}</Mono>
          <Mono className="truncate text-ink-muted">{last.displayCommand}</Mono>
          <span className="shrink-0">{last.durationMs} ms</span>
        </span>
      ) : (
        <span>no command run yet</span>
      )}
      <span className="ml-auto inline-flex shrink-0 items-center gap-4">
        {plan ? (
          <span className="inline-flex items-center gap-1.5">
            <Label>plan</Label>
            <Mono className={plan.conflictCount > 0 ? 'text-clay' : 'text-jade'}>{shortDigest(plan.digest)}</Mono>
          </span>
        ) : null}
        {settings.requireDigestBoundApply ? <span title="Apply is bound to the reviewed plan digest">digest-bound apply</span> : null}
        {demo ? <span className="text-brass">demo</span> : null}
        <Mono className="max-w-[24rem] truncate" title={workspace}>
          {workspace || '—'}
        </Mono>
      </span>
    </footer>
  )
}

/* ---------------------------------------------------------------- toasts --- */

export function Toaster(): JSX.Element {
  const { toasts, dismissToast } = useStore()
  if (toasts.length === 0) return <></>
  return (
    <div className="pointer-events-none fixed right-4 bottom-10 z-60 flex flex-col gap-2" role="status" aria-live="polite">
      {toasts.map((entry) => (
        <div
          key={entry.id}
          className={cx(
            'pointer-events-auto flex items-center gap-2 rounded-[8px] border bg-panel px-3 py-2 text-[12px] shadow-lg',
            entry.tone === 'danger' ? 'border-clay/50 text-clay' : entry.tone === 'positive' ? 'border-jade/40 text-jade' : 'border-line-strong text-ink'
          )}
        >
          <span>{entry.message}</span>
          <IconButton label="Dismiss" icon={<X className="size-3.5" aria-hidden />} onClick={() => dismissToast(entry.id)} />
        </div>
      ))}
    </div>
  )
}

/* -------------------------------------------------------------- helpers --- */

export function panelTitle(panel: PanelId): string {
  return NAV_BY_PANEL.get(panel)?.label ?? panel
}
