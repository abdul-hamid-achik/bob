import type { ActionKind } from './types'

/**
 * Parser for `bob plan --watch` human output.
 *
 * --watch and --json are mutually exclusive, so the streaming watch surface is
 * text. Each reprint starts with a `bob watch:` header, lists one row per
 * action, and ends with the action tally.
 */

export interface WatchRow {
  kind: ActionKind
  path: string
  code: string
  family: string
}

export interface WatchTick {
  timestamp: string
  rows: WatchRow[]
  tally: Record<ActionKind, number> | null
}

const KINDS = new Set(['create', 'update', 'adopt', 'unchanged', 'conflict'])
const ROW_RE = /^(create|update|adopt|unchanged|conflict)\s+(.+?)\s+\[([a-z_]+)\]\s*([a-z_]*)$/
const HEADER_RE = /^bob watch:\s*(.+?)\s*—/
const TALLY_RE = /^(\d+) create, (\d+) update, (\d+) adopt, (\d+) unchanged, (\d+) conflict$/

export function parseWatchLine(line: string): { type: 'header'; timestamp: string } | { type: 'row'; row: WatchRow } | { type: 'tally'; tally: Record<ActionKind, number> } | { type: 'other' } {
  const header = line.match(HEADER_RE)
  if (header?.[1]) return { type: 'header', timestamp: header[1] }
  const row = line.match(ROW_RE)
  if (row?.[1] && KINDS.has(row[1])) {
    return {
      type: 'row',
      row: { kind: row[1] as ActionKind, path: row[2] ?? '', code: row[3] ?? '', family: row[4] ?? '' }
    }
  }
  const tally = line.match(TALLY_RE)
  if (tally) {
    return {
      type: 'tally',
      tally: {
        create: Number(tally[1]),
        update: Number(tally[2]),
        adopt: Number(tally[3]),
        unchanged: Number(tally[4]),
        conflict: Number(tally[5])
      }
    }
  }
  return { type: 'other' }
}

/** Folds a raw watch buffer into ordered ticks, newest last. */
export function parseWatchBuffer(buffer: string): WatchTick[] {
  const ticks: WatchTick[] = []
  let current: WatchTick | null = null
  for (const rawLine of buffer.split('\n')) {
    const line = rawLine.replace(/\s+$/, '')
    if (!line) continue
    const parsed = parseWatchLine(line)
    if (parsed.type === 'header') {
      current = { timestamp: parsed.timestamp, rows: [], tally: null }
      ticks.push(current)
      continue
    }
    if (!current) {
      current = { timestamp: '', rows: [], tally: null }
      ticks.push(current)
    }
    if (parsed.type === 'row') current.rows.push(parsed.row)
    if (parsed.type === 'tally') current.tally = parsed.tally
  }
  return ticks
}
