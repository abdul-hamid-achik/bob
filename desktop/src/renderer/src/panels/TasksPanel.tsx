import { useCallback, useEffect, useState } from 'react'
import { Hammer } from 'lucide-react'
import type { TaskEntry } from '@shared/ipc'
import { Badge, Button, Callout, Mono, Section, cx } from '../components/ui'
import { PanelShell } from '../components/shell'
import { StreamConsole, useStream } from '../components/run'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'

const GATE_ORDER = ['build', 'test', 'race', 'lint', 'check', 'specs', 'docs-build', 'ship']

export function TasksPanel(): JSX.Element {
  const { repoRoot, workspace, toast, bump } = useStore()
  const stream = useStream()
  const [tasks, setTasks] = useState<TaskEntry[]>([])
  const [taskBinary, setTaskBinary] = useState<string | null>(null)
  const [cwd, setCwd] = useState(repoRoot ?? workspace)
  const [running, setRunning] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    const target = repoRoot ?? workspace
    if (!target) return
    setCwd(target)
    const [list, binary] = await Promise.all([bridge().listTasks(target), bridge().taskBinary()])
    setTasks(list)
    setTaskBinary(binary.path)
  }, [repoRoot, workspace])

  useEffect(() => {
    void load()
  }, [load])

  const runTask = async (task: TaskEntry): Promise<void> => {
    if (task.mutates) {
      const confirmed = window.confirm(`task ${task.name} rewrites files. Run it?`)
      if (!confirmed) return
    }
    setRunning(task.name)
    setError(null)
    const outcome = await bridge().runTask(task.name, cwd, true)
    if (outcome.error) {
      setError(outcome.error)
      setRunning(null)
      return
    }
    await stream.start('tasks', [task.name], cwd)
    setRunning(null)
    bump()
  }

  const ordered = [
    ...GATE_ORDER.map((name) => tasks.find((task) => task.name === name)).filter((task): task is TaskEntry => Boolean(task)),
    ...tasks.filter((task) => !GATE_ORDER.includes(task.name))
  ]

  return (
    <PanelShell
      featureId="tasks"
      actions={
        <>
          <Button variant="quiet" loading={stream.running} onClick={() => void stream.stop()} disabled={!stream.running}>
            Stop task
          </Button>
          <Button variant="default" onClick={() => void load()}>
            Reload Taskfile
          </Button>
        </>
      }
      subtitle="The repository's own development gates, streamed live. Useful for working on Bob itself: build, test, race, lint, the canonical check gate, specs, docs, and ship."
    >
      {!taskBinary ? (
        <Callout tone="caution" title="task is not on PATH">
          Install go-task, or run the equivalent commands manually: <Mono>go test ./...</Mono>, <Mono>go vet ./...</Mono>,{' '}
          <Mono>go build ./cmd/bob</Mono>.
        </Callout>
      ) : null}

      {!repoRoot ? (
        <Callout tone="info" title="No Bob source checkout resolved">
          The console drives the Taskfile of the Bob checkout it resolved from the binary path. Point Settings at a{' '}
          <Mono>./bin/bob</Mono> inside a source tree, or add the checkout as a workspace.
        </Callout>
      ) : null}

      <Section title={`Tasks (${tasks.length})`} hint={cwd ? `cwd: ${cwd}` : undefined}>
        {tasks.length === 0 ? (
          <p className="text-[12px] text-ink-dim">No Taskfile.yml found in the resolved repository.</p>
        ) : (
          <ul className="grid grid-cols-2 gap-2">
            {ordered.map((task) => (
              <li key={task.name}>
                <div
                  className={cx(
                    'flex items-start gap-3 rounded-[8px] border px-3 py-2',
                    running === task.name ? 'border-copper/60 bg-copper/10' : 'border-line bg-panel'
                  )}
                >
                  <Button
                    size="sm"
                    variant={task.mutates ? 'danger' : 'default'}
                    className="mt-0.5 shrink-0"
                    icon={<Hammer className="size-3" aria-hidden />}
                    disabled={!taskBinary || stream.running}
                    onClick={() => void runTask(task)}
                  >
                    Run
                  </Button>
                  <div className="min-w-0 flex-1">
                    <p className="flex items-center gap-2">
                      <Mono className="text-ink">task {task.name}</Mono>
                      {task.mutates ? <Badge tone="danger">rewrites files</Badge> : <Badge tone="positive">non-mutating</Badge>}
                    </p>
                    <p className="mt-0.5 text-[11.5px] leading-snug text-ink-muted">{task.desc || 'No description in the Taskfile.'}</p>
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </Section>

      {error ? <Callout tone="danger" title="Task refused">{error}</Callout> : null}

      <Section title="Output" hint="Streamed verbatim, including exit status. Nothing is summarized away.">
        <StreamConsole stream={stream} title="task output" height="24rem" />
      </Section>

      <Section title="Notes">
        <ul className="flex flex-col gap-1 text-[12px] text-ink-muted">
          <li>
            <Mono className="text-ink">task check</Mono> is the canonical non-mutating code, security, and build gate.
          </li>
          <li>
            <Mono className="text-ink">task specs</Mono> needs Glyphrun; <Mono className="text-ink">task docs-build</Mono> needs the locked docs
            dependencies.
          </li>
          <li>
            <Mono className="text-ink">task fmt</Mono>, <Mono className="text-ink">install</Mono>, and <Mono className="text-ink">clean</Mono> rewrite
            files or install outside the workspace, so they ask first.
          </li>
          <li>
            After a task run, <button type="button" className="text-copper underline" onClick={() => void load()}>reload</button> or bump the
            workspace to re-read state.{' '}
            <button type="button" className="text-copper underline" onClick={() => toast('Workspace state refreshed', 'positive')}>
              notify
            </button>
          </li>
        </ul>
      </Section>
    </PanelShell>
  )
}

export default TasksPanel
