import { existsSync, readFileSync } from 'node:fs'
import { join } from 'node:path'
import { parse as parseYaml } from 'yaml'
import type { TaskEntry } from '../shared/ipc'

/** Tasks that rewrite source or install outside the workspace. */
export const MUTATING_TASKS = new Set(['fmt', 'install', 'clean'])

/** Tasks that keep running until stopped. */
export const LONG_RUNNING_TASKS = new Set(['docs', 'docs-preview'])

interface TaskfileShape {
  tasks?: Record<string, { desc?: string; cmds?: unknown }>
}

export function listTasks(cwd: string): TaskEntry[] {
  const file = join(cwd, 'Taskfile.yml')
  if (!existsSync(file)) return []
  try {
    const parsed = parseYaml(readFileSync(file, 'utf8')) as TaskfileShape
    const tasks = parsed?.tasks ?? {}
    return Object.entries(tasks)
      .filter(([name]) => name !== 'default')
      .map(([name, task]) => ({
        name,
        desc: typeof task?.desc === 'string' ? task.desc : '',
        mutates: MUTATING_TASKS.has(name)
      }))
      .sort((a, b) => a.name.localeCompare(b.name))
  } catch {
    return []
  }
}

export function isLongRunningTask(name: string): boolean {
  return LONG_RUNNING_TASKS.has(name)
}
