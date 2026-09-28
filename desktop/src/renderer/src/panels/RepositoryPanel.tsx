import { useCallback, useEffect, useState } from 'react'
import { GitBranch } from 'lucide-react'
import type { GitCommit, GitStatus } from '@shared/ipc'
import { Badge, Button, CodeBlock, EmptyState, Mono, Section, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'

export function RepositoryPanel(): JSX.Element {
  const { workspace, revision, plan, navigate } = useStore()
  const [status, setStatus] = useState<GitStatus | null>(null)
  const [commits, setCommits] = useState<GitCommit[]>([])
  const [diff, setDiff] = useState<{ text: string; path: string | null } | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!workspace) return
    setLoading(true)
    setError(null)
    try {
      const [nextStatus, nextCommits] = await Promise.all([bridge().gitStatus(workspace), bridge().gitLog(workspace, 15)])
      setStatus(nextStatus)
      setCommits(nextCommits)
      if (!nextStatus.available) setError(nextStatus.error)
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught))
    } finally {
      setLoading(false)
    }
  }, [workspace])

  useEffect(() => {
    void load()
    setDiff(null)
  }, [load, revision])

  const changedPaths = new Set((plan?.actions ?? []).filter((action) => action.kind !== 'unchanged').map((action) => action.path))

  return (
    <PanelShell
      featureId="repository"
      actions={
        <>
          <Button variant="quiet" loading={loading} onClick={() => void load()}>
            Refresh
          </Button>
          <Button variant="default" onClick={() => void bridge().gitDiff(workspace).then((result) => setDiff({ text: result.diff, path: null }))}>
            Full working diff
          </Button>
        </>
      }
      subtitle="Read-only git state so you can review what an apply actually changed. The console never stages, commits, resets, or pushes."
    >
      <WorkspaceGate>
        {error ? <EmptyState title="No git state available" hint={error} action={<Button onClick={() => void load()}>Try again</Button>} /> : null}

        {status?.available ? (
          <Section
            title="Working tree"
            actions={
              <span className="flex items-center gap-2">
                <GitBranch className="size-4 text-ink-dim" aria-hidden />
                <Mono className="text-ink">{status.branch ?? 'detached'}</Mono>
                <Badge tone={status.dirty ? 'caution' : 'positive'}>{status.dirty ? `${status.entries.length} changed` : 'clean'}</Badge>
              </span>
            }
          >
            {status.entries.length > 0 ? (
              <ul className="max-h-72 overflow-auto rounded-[8px] border border-line">
                {status.entries.map((entry) => (
                  <li key={entry.path} className="flex items-center gap-2 border-b border-line/70 px-2.5 py-1 last:border-b-0 hover:bg-raise/60">
                    <Mono className={cx('w-6 shrink-0', entry.worktree || entry.index ? 'text-brass' : 'text-ink-dim')}>
                      {(entry.index || ' ') + (entry.worktree || ' ')}
                    </Mono>
                    <button type="button" className="min-w-0 flex-1 text-left" onClick={async () => setDiff({ text: (await bridge().gitDiff(workspace, entry.path)).diff, path: entry.path })}>
                      <Mono className={cx('block truncate', changedPaths.has(entry.path) ? 'text-copper' : 'text-ink')}>{entry.path}</Mono>
                    </button>
                    {changedPaths.has(entry.path) ? <Badge tone="accent">in plan</Badge> : null}
                    <Button size="sm" variant="ghost" onClick={() => navigate('path', { path: entry.path })}>
                      Classify
                    </Button>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-[12px] text-ink-dim">Working tree is clean.</p>
            )}
          </Section>
        ) : null}

        {diff ? (
          <Section title={diff.path ? `Diff — ${diff.path}` : 'Working diff'} hint="Exactly what git reports, unmodified.">
            {diff.text ? (
              <CodeBlock maxHeight="24rem" copy className={cx('text-[11.5px]')}>
                {diff.text}
              </CodeBlock>
            ) : (
              <p className="text-[12px] text-ink-dim">No differences.</p>
            )}
          </Section>
        ) : null}

        <Section title="Recent commits" hint="History is authoritative; the console only reads it.">
          {commits.length > 0 ? (
            <ul className="rounded-[8px] border border-line">
              {commits.map((commit) => (
                <li key={commit.hash} className="flex items-baseline gap-3 border-b border-line/70 px-3 py-1.5 last:border-b-0">
                  <Mono className="shrink-0 text-copper">{commit.shortHash}</Mono>
                  <span className="min-w-0 flex-1 truncate text-[12px] text-ink">{commit.subject}</span>
                  <span className="shrink-0 text-[11px] text-ink-dim">{commit.author}</span>
                  <Mono className="shrink-0 text-ink-dim">{commit.date.slice(0, 10)}</Mono>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-[12px] text-ink-dim">No commits read.</p>
          )}
        </Section>
      </WorkspaceGate>
    </PanelShell>
  )
}

export default RepositoryPanel
