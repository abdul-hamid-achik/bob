import { useMemo, useState } from 'react'
import { diffLines, parseUnifiedDiff, summarizeDiff, type DiffLine } from '@shared/diff'
import {
  ACTION_CODE_MEANING,
  FAMILY_META,
  KIND_META,
  actionCounts,
  conflictClass,
  familyForCode,
  formatMode,
  shortDigest
} from '@shared/format'
import type { ActionKind, PlanAction } from '@shared/types'
import { Badge, DataTable, KeyValue, Mono, Segmented, TextInput, cx, type Column, type Tone } from './ui'

const KIND_ORDER: ActionKind[] = ['conflict', 'update', 'create', 'adopt', 'unchanged']

export function KindBadge({ kind }: { kind: ActionKind }): JSX.Element {
  const meta = KIND_META[kind]
  return (
    <Badge tone={meta.tone as Tone} className="font-mono normal-case">
      {kind}
    </Badge>
  )
}

export function CodeChip({ code }: { code: string }): JSX.Element {
  const family = familyForCode(code)
  return (
    <span title={family ? FAMILY_META[family].detail : undefined} className="inline-flex items-center gap-1.5">
      <Mono className="text-ink-muted">{code}</Mono>
      {family ? <span className="text-[10.5px] text-ink-dim">{family.replace('_', ' ')}</span> : null}
    </span>
  )
}

/** Compact ledger bar: the five action counts, in severity order. */
export function PlanSummaryBar({ actions, digest }: { actions: PlanAction[]; digest?: string }): JSX.Element {
  const counts = actionCounts(actions)
  const klass = conflictClass(actions)
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 rounded-[8px] border border-line bg-panel px-3 py-2">
      <div className="flex items-center gap-3">
        {KIND_ORDER.map((kind) => (
          <span key={kind} className="inline-flex items-baseline gap-1.5">
            <span
              className={cx(
                'font-mono text-[13px] tabular-nums',
                counts[kind] === 0
                  ? 'text-ink-dim'
                  : kind === 'conflict'
                    ? 'text-clay'
                    : kind === 'update'
                      ? 'text-brass'
                      : kind === 'create'
                        ? 'text-jade'
                        : kind === 'adopt'
                          ? 'text-steel'
                          : 'text-ink-muted'
              )}
            >
              {counts[kind]}
            </span>
            <span className="text-[11px] text-ink-dim">{kind}</span>
          </span>
        ))}
      </div>
      <span className="ml-auto inline-flex items-center gap-2">
        {klass !== 'none' ? <Badge tone="danger">conflict class: {klass.replace(/_/g, ' ')}</Badge> : <Badge tone="positive">conflict-free</Badge>}
        {digest ? (
          <Mono className="text-ink-muted" title={digest}>
            {shortDigest(digest)}
          </Mono>
        ) : null}
      </span>
    </div>
  )
}

export function DiffView({ lines, maxHeight = '22rem' }: { lines: DiffLine[]; maxHeight?: string }): JSX.Element {
  const summary = summarizeDiff(lines)
  return (
    <div className="overflow-hidden rounded-[8px] border border-line bg-sunken">
      <div className="flex items-center gap-3 border-b border-line bg-panel px-3 py-1.5">
        <Label className="text-[10.5px] text-ink-dim">diff</Label>
        <Mono className="text-jade">+{summary.added}</Mono>
        <Mono className="text-clay">−{summary.removed}</Mono>
        <Mono className="text-ink-dim">{summary.context} context</Mono>
      </div>
      <div className="overflow-auto" style={{ maxHeight }}>
        <table className="w-full border-collapse font-mono text-[11.5px] leading-[1.45]">
          <tbody>
            {lines.map((line, index) => (
              <tr
                key={index}
                className={cx(
                  line.type === 'add' && 'bg-jade/10',
                  line.type === 'remove' && 'bg-clay/10',
                  line.type === 'hunk' && 'bg-steel/10',
                  line.type === 'meta' && 'bg-raise'
                )}
              >
                <td className="w-10 shrink-0 px-1.5 text-right text-ink-dim select-none">{line.oldNo ?? ''}</td>
                <td className="w-10 shrink-0 px-1.5 text-right text-ink-dim select-none">{line.newNo ?? ''}</td>
                <td
                  className={cx(
                    'w-4 shrink-0 px-1 select-none',
                    line.type === 'add' ? 'text-jade' : line.type === 'remove' ? 'text-clay' : 'text-ink-dim'
                  )}
                >
                  {line.type === 'add' ? '+' : line.type === 'remove' ? '−' : line.type === 'hunk' ? '@' : ' '}
                </td>
                <td className={cx('px-2 whitespace-pre-wrap break-all', line.type === 'add' ? 'text-jade' : line.type === 'remove' ? 'text-clay' : 'text-ink-muted')}>
                  {line.text}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

function Label({ children, className }: { children: React.ReactNode; className?: string }): JSX.Element {
  return <span className={cx('text-[10.5px] uppercase', className)}>{children}</span>
}

export function ActionDetail({ action }: { action: PlanAction }): JSX.Element {
  const family = familyForCode(action.code)
  const diffLinesView = useMemo<DiffLine[] | null>(() => {
    if (action.diff) return parseUnifiedDiff(action.diff)
    if (action.current_preview !== undefined || action.desired_preview !== undefined) {
      return diffLines(action.current_preview ?? '', action.desired_preview ?? '')
    }
    return null
  }, [action])

  return (
    <div className="flex flex-col gap-3">
      <KeyValue
        entries={[
          ['path', <Mono className="text-ink">{action.path}</Mono>],
          ['action', <KindBadge kind={action.kind} />],
          ['code', <CodeChip code={action.code} />],
          ...(family ? ([['family', <span className="text-[12px]">{FAMILY_META[family].label}</span>]] as [string, React.ReactNode][]) : []),
          ['meaning', <span className="text-[12px] text-ink-muted">{ACTION_CODE_MEANING[action.code] ?? 'undocumented code'}</span>],
          ['current sha256', <Mono title={action.current_sha256}>{action.current_sha256 ?? '—'}</Mono>],
          ['locked sha256', <Mono title={action.locked_sha256}>{action.locked_sha256 ?? '—'}</Mono>],
          ['desired sha256', <Mono title={action.desired_sha256}>{action.desired_sha256 ?? '—'}</Mono>],
          ['mode', <Mono>{formatMode(action.current_mode)} → {formatMode(action.desired_mode)}</Mono>],
          ...(action.reason ? ([['reason', <span className="text-[12px] text-ink-muted">{action.reason}</span>]] as [string, React.ReactNode][]) : [])
        ]}
      />
      {family ? <p className="text-[11.5px] leading-relaxed text-ink-dim">{FAMILY_META[family].detail}</p> : null}
      {diffLinesView && diffLinesView.length > 0 ? (
        <DiffView lines={diffLinesView} />
      ) : action.desired_preview ? (
        <div>
          <Label className="text-ink-dim">desired content preview</Label>
          <pre className="argv mt-1 max-h-64 overflow-auto rounded-[8px] border border-line bg-sunken px-3 py-2 text-ink-muted">
            {action.desired_preview}
          </pre>
        </div>
      ) : (
        <p className="text-[11.5px] text-ink-dim">
          No bounded content in this plan. Re-run with <Mono className="text-ink-muted">--content</Mono> or{' '}
          <Mono className="text-ink-muted">--diff</Mono> to include previews.
        </p>
      )}
    </div>
  )
}

export interface PlanFilterState {
  kinds: Set<ActionKind>
  family: string
  query: string
}

export function PlanFilters({
  actions,
  state,
  onChange
}: {
  actions: PlanAction[]
  state: PlanFilterState
  onChange: (next: PlanFilterState) => void
}): JSX.Element {
  const counts = actionCounts(actions)
  return (
    <div className="flex flex-wrap items-center gap-3">
      <Segmented
        ariaLabel="Filter by action kind"
        value={[...state.kinds].join(',') || 'all'}
        onChange={(value) => {
          if (value === 'all') onChange({ ...state, kinds: new Set(KIND_ORDER) })
          else onChange({ ...state, kinds: new Set([value as ActionKind]) })
        }}
        options={[
          { value: 'all', label: 'all', count: actions.length },
          ...KIND_ORDER.map((kind) => ({ value: kind, label: kind, count: counts[kind] }))
        ]}
      />
      <Segmented
        ariaLabel="Filter by conflict family"
        value={state.family}
        onChange={(family) => onChange({ ...state, family })}
        options={[
          { value: 'all', label: 'any family' },
          { value: 'ownership_hazard', label: 'hazard' },
          { value: 'contract_drift', label: 'contract' },
          { value: 'unmanaged_divergence', label: 'divergence' },
          { value: 'scaffold', label: 'scaffold' },
          { value: 'convergence', label: 'convergence' }
        ]}
      />
      <TextInput
        value={state.query}
        onChange={(event) => onChange({ ...state, query: event.target.value })}
        placeholder="Filter paths…"
        aria-label="Filter paths"
        className="h-7 w-52 py-0 font-mono text-[11.5px]"
      />
    </div>
  )
}

export function filterActions(actions: PlanAction[], state: PlanFilterState): PlanAction[] {
  const query = state.query.trim().toLowerCase()
  return actions.filter((action) => {
    if (!state.kinds.has(action.kind)) return false
    if (state.family !== 'all') {
      const family = familyForCode(action.code)
      if (family !== state.family) return false
    }
    if (query && !action.path.toLowerCase().includes(query) && !action.code.includes(query)) return false
    return true
  })
}

export const DEFAULT_PLAN_FILTERS: PlanFilterState = { kinds: new Set(KIND_ORDER), family: 'all', query: '' }

export function PlanTable({
  actions,
  selectedPath,
  onSelect
}: {
  actions: PlanAction[]
  selectedPath?: string | null
  onSelect?: (action: PlanAction) => void
}): JSX.Element {
  const columns: Column<PlanAction>[] = useMemo(
    () => [
      {
        key: 'kind',
        header: 'Action',
        width: '7.5rem',
        render: (row) => <KindBadge kind={row.kind} />,
        sortValue: (row) => KIND_ORDER.indexOf(row.kind)
      },
      {
        key: 'path',
        header: 'Path',
        render: (row) => <Mono className="text-ink">{row.path}</Mono>,
        sortValue: (row) => row.path
      },
      {
        key: 'code',
        header: 'Code',
        width: '15rem',
        render: (row) => <CodeChip code={row.code} />,
        sortValue: (row) => row.code
      },
      {
        key: 'current',
        header: 'Current',
        width: '7rem',
        render: (row) => <Mono className="text-ink-dim">{shortDigest(row.current_sha256, 8)}</Mono>,
        sortValue: (row) => row.current_sha256 ?? ''
      },
      {
        key: 'locked',
        header: 'Locked',
        width: '7rem',
        render: (row) => <Mono className="text-ink-dim">{shortDigest(row.locked_sha256, 8)}</Mono>,
        sortValue: (row) => row.locked_sha256 ?? ''
      },
      {
        key: 'desired',
        header: 'Desired',
        width: '7rem',
        render: (row) => <Mono className="text-ink-dim">{shortDigest(row.desired_sha256, 8)}</Mono>,
        sortValue: (row) => row.desired_sha256 ?? ''
      }
    ],
    []
  )

  const [expanded, setExpanded] = useState<string | null>(null)

  return (
    <div className="flex flex-col gap-2">
      <DataTable
        rows={actions}
        columns={columns}
        dense
        keyFor={(row) => row.path}
        selectedKey={selectedPath ?? expanded}
        onRowClick={
          onSelect
            ? (row) => {
                setExpanded(row.path)
                onSelect(row)
              }
            : (row) => setExpanded((prev) => (prev === row.path ? null : row.path))
        }
      />
      {expanded && !onSelect ? (
        <div className="rounded-[8px] border border-line bg-panel p-3">
          {(() => {
            const action = actions.find((entry) => entry.path === expanded)
            return action ? <ActionDetail action={action} /> : null
          })()}
        </div>
      ) : null}
    </div>
  )
}
