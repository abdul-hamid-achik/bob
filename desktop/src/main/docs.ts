import { existsSync, readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative } from 'node:path'
import type { DocContent, DocEntry } from '../shared/ipc'

const SKIP_DIRS = new Set(['node_modules', '.vitepress', 'dist', 'cache', '.vercel', 'public'])
const MAX_DOC_BYTES = 2 * 1024 * 1024
const ROOT_MARKDOWN = ['README.md', 'AGENTS.md', 'CONTRIBUTING.md', 'SECURITY.md', 'CHANGELOG.md']

function groupFor(relativePath: string): string {
  if (relativePath.startsWith('docs/reference/')) return 'Reference'
  if (relativePath.startsWith('docs/guides/')) return 'Guides'
  if (relativePath.startsWith('docs/')) return 'Handbook'
  return 'Repository'
}

function titleFrom(markdown: string, fallback: string): string {
  const match = markdown.match(/^#\s+(.+)$/m)
  return match?.[1]?.trim() || fallback
}

function walkMarkdown(dir: string, root: string, out: string[]): void {
  let items: string[]
  try {
    items = readdirSync(dir)
  } catch {
    return
  }
  for (const item of items.sort()) {
    if (SKIP_DIRS.has(item) || item.startsWith('.')) continue
    const absolute = join(dir, item)
    let stats
    try {
      stats = statSync(absolute)
    } catch {
      continue
    }
    if (stats.isDirectory()) {
      walkMarkdown(absolute, root, out)
      continue
    }
    if (item.endsWith('.md')) out.push(relative(root, absolute))
  }
}

/** Enumerates the published docs pages plus the root orientation Markdown. */
export function listDocs(repoRoot: string): DocEntry[] {
  if (!repoRoot || !existsSync(repoRoot)) return []
  const found: string[] = []
  const docsDir = join(repoRoot, 'docs')
  if (existsSync(docsDir)) walkMarkdown(docsDir, repoRoot, found)
  for (const name of ROOT_MARKDOWN) {
    if (existsSync(join(repoRoot, name))) found.push(name)
  }
  const entries: DocEntry[] = []
  for (const relativePath of found) {
    const absolute = join(repoRoot, relativePath)
    let markdown = ''
    try {
      markdown = readFileSync(absolute, 'utf8').slice(0, 4096)
    } catch {
      continue
    }
    const id = relativePath.replace(/\.md$/, '')
    entries.push({
      id,
      title: titleFrom(markdown, id.split('/').pop() ?? id),
      relativePath,
      group: groupFor(relativePath)
    })
  }
  return entries.sort((a, b) => a.relativePath.localeCompare(b.relativePath))
}

export function readDoc(repoRoot: string, id: string): DocContent | null {
  if (!repoRoot || !id || id.includes('..') || id.startsWith('/')) return null
  // ids are repo-relative without the extension; callers may also pass the
  // page path without the docs/ prefix, as the reference links do.
  const candidates = [`${id}.md`, id.startsWith('docs/') ? null : `docs/${id}.md`].filter((entry): entry is string => Boolean(entry))
  for (const relativePath of candidates) {
    const absolute = join(repoRoot, relativePath)
    if (!existsSync(absolute)) continue
    try {
      const stats = statSync(absolute)
      if (!stats.isFile() || stats.size > MAX_DOC_BYTES) continue
      const markdown = readFileSync(absolute, 'utf8')
      return {
        id,
        title: titleFrom(markdown.slice(0, 4096), id.split('/').pop() ?? id),
        relativePath,
        markdown
      }
    } catch {
      continue
    }
  }
  return null
}
