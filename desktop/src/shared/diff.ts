export type DiffLineType = 'context' | 'add' | 'remove' | 'hunk' | 'meta'

export interface DiffLine {
  type: DiffLineType
  text: string
  oldNo: number | null
  newNo: number | null
}

const MAX_DIFF_LINES = 4000

/** Parses the unified diff text Bob emits for `plan --diff`. */
export function parseUnifiedDiff(diff: string): DiffLine[] {
  const lines: DiffLine[] = []
  let oldNo = 0
  let newNo = 0
  // Diff text conventionally ends with a newline; that trailing empty line is
  // not a context line.
  const body = diff.endsWith('\n') ? diff.slice(0, -1) : diff
  for (const raw of body.split('\n')) {
    if (raw.startsWith('@@')) {
      const match = raw.match(/@@ -(\d+)(?:,\d+)? \+(\d+)(?:,\d+)? @@/)
      oldNo = match?.[1] ? Number(match[1]) - 1 : 0
      newNo = match?.[2] ? Number(match[2]) - 1 : 0
      lines.push({ type: 'hunk', text: raw, oldNo: null, newNo: null })
      continue
    }
    if (raw.startsWith('---') || raw.startsWith('+++')) {
      lines.push({ type: 'meta', text: raw, oldNo: null, newNo: null })
      continue
    }
    if (raw.startsWith('+')) {
      newNo += 1
      lines.push({ type: 'add', text: raw.slice(1), oldNo: null, newNo })
      continue
    }
    if (raw.startsWith('-')) {
      oldNo += 1
      lines.push({ type: 'remove', text: raw.slice(1), oldNo, newNo: null })
      continue
    }
    oldNo += 1
    newNo += 1
    lines.push({ type: 'context', text: raw.startsWith(' ') ? raw.slice(1) : raw, oldNo, newNo })
  }
  return lines
}

/**
 * Line diff between two bodies, used when Bob returns bounded content previews
 * instead of a unified diff. Falls back to a replace-all view past the cap so
 * a huge file cannot stall the renderer.
 */
export function diffLines(before: string, after: string): DiffLine[] {
  const a = before.split('\n')
  const b = after.split('\n')
  if (a.length > MAX_DIFF_LINES || b.length > MAX_DIFF_LINES) {
    return [
      ...a.map((text, index) => ({ type: 'remove' as const, text, oldNo: index + 1, newNo: null })),
      ...b.map((text, index) => ({ type: 'add' as const, text, oldNo: null, newNo: index + 1 }))
    ]
  }

  const rows = a.length + 1
  const cols = b.length + 1
  const table = new Int32Array(rows * cols)
  for (let i = a.length - 1; i >= 0; i -= 1) {
    for (let j = b.length - 1; j >= 0; j -= 1) {
      const same = a[i] === b[j]
      table[i * cols + j] = same
        ? (table[(i + 1) * cols + j + 1] ?? 0) + 1
        : Math.max(table[(i + 1) * cols + j] ?? 0, table[i * cols + j + 1] ?? 0)
    }
  }

  const out: DiffLine[] = []
  let i = 0
  let j = 0
  while (i < a.length && j < b.length) {
    if (a[i] === b[j]) {
      i += 1
      j += 1
      out.push({ type: 'context', text: a[i - 1] ?? '', oldNo: i, newNo: j })
      continue
    }
    if ((table[(i + 1) * cols + j] ?? 0) >= (table[i * cols + j + 1] ?? 0)) {
      i += 1
      out.push({ type: 'remove', text: a[i - 1] ?? '', oldNo: i, newNo: null })
    } else {
      j += 1
      out.push({ type: 'add', text: b[j - 1] ?? '', oldNo: null, newNo: j })
    }
  }
  while (i < a.length) {
    i += 1
    out.push({ type: 'remove', text: a[i - 1] ?? '', oldNo: i, newNo: null })
  }
  while (j < b.length) {
    j += 1
    out.push({ type: 'add', text: b[j - 1] ?? '', oldNo: null, newNo: j })
  }
  return out
}

export interface DiffSummary {
  added: number
  removed: number
  context: number
}

export function summarizeDiff(lines: DiffLine[]): DiffSummary {
  let added = 0
  let removed = 0
  let context = 0
  for (const line of lines) {
    if (line.type === 'add') added += 1
    else if (line.type === 'remove') removed += 1
    else if (line.type === 'context') context += 1
  }
  return { added, removed, context }
}
