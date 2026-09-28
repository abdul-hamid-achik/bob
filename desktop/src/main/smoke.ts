import { writeFileSync } from 'node:fs'
import type { BrowserWindow } from 'electron'
import type { ConsoleState } from './state'

export interface SmokeCheck {
  name: string
  ok: boolean
  detail: string
}

export interface SmokeReport {
  startedAt: string
  finishedAt: string
  workspace: string
  binary: string
  checks: SmokeCheck[]
  passed: number
  failed: number
}

/**
 * Drives the real renderer bridge through every Bob surface and writes a JSON
 * report. Used by scripts/smoke.mjs; never runs in a normal session.
 */
export async function runSmoke(window: BrowserWindow, state: ConsoleState, workspace: string, reportPath: string): Promise<SmokeReport> {
  const startedAt = new Date().toISOString()
  const checks: SmokeCheck[] = []

  const evaluate = async <T>(name: string, expression: string, verify: (value: T) => string | null): Promise<void> => {
    console.log(`[smoke] → ${name}`)
    try {
      const value = (await window.webContents.executeJavaScript(`(async () => ${expression})()`, true)) as T
      const failure = verify(value)
      console.log(`[smoke] ${failure === null ? '✓' : '✗'} ${name}${failure ? ` — ${failure}` : ''}`)
      checks.push({ name, ok: failure === null, detail: failure ?? describe(value) })
    } catch (error) {
      const detail = error instanceof Error ? error.message : String(error)
      console.log(`[smoke] ✗ ${name} — threw ${detail}`)
      checks.push({ name, ok: false, detail })
    }
  }

  await evaluate<{ electron: string }>('appInfo', 'window.bob.appInfo()', (value) => (value.electron ? null : 'no electron version'))

  await evaluate<{ exists: boolean; version: string | null; path: string }>(
    'resolveBinary',
    'window.bob.resolveBinary()',
    (value) => (value.exists && value.version ? null : `binary not usable: ${value.path} ${value.version ?? ''}`)
  )

  await evaluate<{ theme: string }>('getSettings', 'window.bob.getSettings()', (value) => (value.theme ? null : 'no theme'))
  await evaluate<{ theme: string }>(
    'setSettings round trip',
    'window.bob.setSettings({ theme: "dark" })',
    (value) => (value.theme === 'dark' ? null : `theme is ${value.theme}`)
  )

  await evaluate<{ path: string; hasManifest: boolean }>(
    'addWorkspace',
    `window.bob.addWorkspace(${JSON.stringify(workspace)})`,
    (value) => (value.hasManifest ? null : 'workspace has no bob.yaml')
  )

  await evaluate<{ manifest: string | null; lockFiles: { path: string }[] }>(
    'readWorkspace (bob.yaml + bob.lock)',
    `window.bob.readWorkspace(${JSON.stringify(workspace)})`,
    (value) => (value.manifest && value.lockFiles.length >= 0 ? null : 'manifest not read')
  )

  const bob = (argv: string[]): string =>
    `window.bob.run({ featureId: "plan", argv: ${JSON.stringify(argv)}, cwd: ${JSON.stringify(workspace)} })`

  const featureRun = (featureId: string, argv: string[]): string =>
    `window.bob.run({ featureId: ${JSON.stringify(featureId)}, argv: ${JSON.stringify(argv)}, cwd: ${JSON.stringify(workspace)} })`

  interface Envelope {
    exitCode: number
    envelope: { ok: boolean; command: string; data: Record<string, unknown> } | null
    stderr: string
    parseError: string | null
  }

  const expectOk = (value: Envelope): string | null => {
    if (!value.envelope) return `no envelope (exit ${value.exitCode}, ${value.parseError ?? value.stderr.slice(0, 120)})`
    if (!value.envelope.ok) return `${value.envelope.command} reported a failure envelope`
    return null
  }

  await evaluate<Envelope>('bob version', featureRun('version', ['version', '--json']), expectOk)
  await evaluate<Envelope>('bob explain', featureRun('explain', ['explain', '--json']), expectOk)

  await evaluate<Envelope>(
    'bob learn',
    featureRun('learn', ['learn', '--json']),
    (value) => {
      const base = expectOk(value)
      if (base) return base
      const commands = value.envelope?.data.commands as unknown[] | undefined
      return commands && commands.length >= 20 ? null : `expected at least 20 commands, got ${commands?.length ?? 0}`
    }
  )

  await evaluate<Envelope>(
    'bob recipe list',
    featureRun('recipe-list', ['recipe', 'list', '--json']),
    (value) => {
      const base = expectOk(value)
      if (base) return base
      const recipes = value.envelope?.data as unknown
      return Array.isArray(recipes) && recipes.length >= 14 ? null : `expected at least 14 recipes, got ${Array.isArray(recipes) ? recipes.length : 0}`
    }
  )

  await evaluate<Envelope>('bob recipe show', featureRun('recipe-show', ['recipe', 'show', 'go-agent-tool', '--json']), expectOk)

  await evaluate<Envelope>(
    'bob plan',
    bob(['plan', workspace, '--json']),
    (value) => {
      const base = expectOk(value)
      if (base) return base
      const digest = value.envelope?.data.plan_digest as string | undefined
      return digest && /^sha256:[0-9a-f]{64}$/.test(digest) ? null : `bad plan digest: ${digest ?? 'none'}`
    }
  )

  await evaluate<Envelope>('bob plan --content --diff', bob(['plan', workspace, '--content', '--diff', '--json']), expectOk)
  await evaluate<Envelope>('bob check', featureRun('check', ['check', workspace, '--json']), (value) => (value.envelope ? null : 'no envelope'))
  await evaluate<Envelope>('bob context --profile full', featureRun('context', ['context', workspace, '--profile', 'full', '--json']), expectOk)
  await evaluate<Envelope>('bob inspect', featureRun('inspect', ['inspect', workspace, '--json']), expectOk)
  await evaluate<Envelope>('bob doctor', featureRun('doctor', ['doctor', workspace, '--json']), expectOk)
  await evaluate<Envelope>('bob path', featureRun('path', ['path', '--workspace', workspace, '--json', '--', 'bob.yaml']), expectOk)
  await evaluate<Envelope>('bob path --batch', featureRun('path-batch', ['path', '--batch', '--workspace', workspace, '--json', '--', 'bob.yaml', 'README.md']), expectOk)
  await evaluate<Envelope>('bob playbook list', featureRun('playbook-list', ['playbook', 'list', workspace, '--json']), expectOk)
  await evaluate<Envelope>('bob stats', featureRun('stats', ['stats', workspace, '--json']), (value) => (value.envelope ? null : 'no envelope'))
  await evaluate<Envelope>('bob config show', featureRun('config-show', ['config', 'show', '--json']), expectOk)
  await evaluate<Envelope>('bob config init preview', featureRun('config-init', ['config', 'init', '--json']), expectOk)
  await evaluate<Envelope>('bob init preview', featureRun('init', ['init', workspace, '--json']), expectOk)

  /* safety guards */
  await evaluate<Envelope>(
    'guard: apply without confirmation is refused',
    `window.bob.run({ featureId: "apply", argv: ["apply", ${JSON.stringify(workspace)}, "--json"], cwd: ${JSON.stringify(workspace)} })`,
    (value) => (value.exitCode === -1 && /not confirmed/.test(value.stderr) ? null : `expected a refusal, got exit ${value.exitCode}`)
  )

  await evaluate<Envelope>(
    'guard: --probe-integrations needs explicit authority',
    `window.bob.run({ featureId: "inspect-probe", argv: ["inspect", ${JSON.stringify(workspace)}, "--probe-integrations", "--json"], cwd: ${JSON.stringify(workspace)}, confirmed: false })`,
    (value) => (value.exitCode === -1 && /authority/.test(value.stderr) ? null : `expected an authority refusal, got exit ${value.exitCode}`)
  )

  await evaluate<Envelope>(
    'guard: unknown feature id is refused',
    `window.bob.run({ featureId: "delete-everything", argv: ["plan"], cwd: ${JSON.stringify(workspace)} }).catch((error) => ({ exitCode: -1, envelope: null, stderr: String(error && error.message), parseError: null }))`,
    (value) => (/unknown feature/.test(value.stderr) ? null : `expected an unknown-feature refusal, got ${value.stderr}`)
  )

  await evaluate<Envelope>(
    'guard: remove --dry-run needs no confirmation',
    `window.bob.run({ featureId: "remove", argv: ["remove", ${JSON.stringify(workspace)}, "--dry-run", "--json"], cwd: ${JSON.stringify(workspace)} })`,
    (value) => (value.envelope ? null : `preview was refused: ${value.stderr}`)
  )

  await evaluate<Envelope>(
    'guard: remove without --dry-run is refused',
    `window.bob.run({ featureId: "remove", argv: ["remove", ${JSON.stringify(workspace)}, "--json"], cwd: ${JSON.stringify(workspace)} })`,
    (value) => (value.exitCode === -1 && /not confirmed/.test(value.stderr) ? null : `expected a refusal, got exit ${value.exitCode}`)
  )

  await evaluate<Envelope>(
    'guard: init --write is refused without confirmation',
    `window.bob.run({ featureId: "init", argv: ["init", ${JSON.stringify(workspace)}, "--write", "--json"], cwd: ${JSON.stringify(workspace)} })`,
    (value) => (value.exitCode === -1 && /not confirmed/.test(value.stderr) ? null : `expected a refusal, got exit ${value.exitCode}`)
  )

  /* filesystem, git, docs, tasks */
  await evaluate<{ length: number }>('listPaths', `window.bob.listPaths(${JSON.stringify(workspace)})`, (value) => (value.length > 0 ? null : 'no paths listed'))
  await evaluate<{ content: string }>('readFile bob.yaml', `window.bob.readFile(${JSON.stringify(workspace)}, "bob.yaml")`, (value) => (value.content.includes('recipe') ? null : 'manifest content missing'))
  await evaluate<{ path: string | null }>(
    'readFile refuses traversal',
    `window.bob.readFile(${JSON.stringify(workspace)}, "../package.json").then(() => ({ path: "allowed" })).catch(() => ({ path: null }))`,
    (value) => (value.path === null ? null : 'traversal was not refused')
  )
  await evaluate<{ available: boolean; branch: string | null }>('gitStatus', `window.bob.gitStatus(${JSON.stringify(workspace)})`, (value) => (value.available ? null : 'git unavailable'))
  await evaluate<{ length: number }>('gitLog', `window.bob.gitLog(${JSON.stringify(workspace)}, 5)`, (value) => (value.length > 0 ? null : 'no commits read'))
  await evaluate<{ length: number }>('listDocs', 'window.bob.listDocs()', (value) => (value.length > 5 ? null : `only ${value.length} docs found`))
  await evaluate<{ markdown: string } | null>('readDoc reference/cli', 'window.bob.readDoc("reference/cli")', (value) => (value?.markdown ? null : 'doc not read'))
  await evaluate<{ length: number }>('listTasks', `window.bob.listTasks(${JSON.stringify(state.repoRoot ?? workspace)})`, (value) => (value.length > 5 ? null : `only ${value.length} tasks found`))

  /* MCP */
  await evaluate<{ started: boolean; tools: { name: string }[] }>(
    'mcp serve starts with nine tools',
    `window.bob.mcpStart({ workspace: ${JSON.stringify(workspace)} })`,
    (value) => (value.started && value.tools.length === 9 ? null : `started=${value.started} tools=${value.tools.length}`)
  )

  await evaluate<{ ok: boolean }>(
    'mcp bob_context',
    `window.bob.mcpCallTool("bob_context", { workspace: ${JSON.stringify(workspace)}, profile: "compact" })`,
    (value) => (value.ok ? null : 'bob_context failed')
  )
  await evaluate<{ ok: boolean }>(
    'mcp bob_plan',
    `window.bob.mcpCallTool("bob_plan", { workspace: ${JSON.stringify(workspace)} })`,
    (value) => (value.ok ? null : 'bob_plan failed')
  )
  await evaluate<{ ok: boolean }>(
    'mcp bob_check',
    `window.bob.mcpCallTool("bob_check", { workspace: ${JSON.stringify(workspace)} })`,
    (value) => (value.ok ? null : 'bob_check failed')
  )
  await evaluate<{ ok: boolean }>(
    'mcp bob_path',
    `window.bob.mcpCallTool("bob_path", { workspace: ${JSON.stringify(workspace)}, path: "bob.yaml" })`,
    (value) => (value.ok ? null : 'bob_path failed')
  )
  await evaluate<{ ok: boolean }>(
    'mcp bob_validate_manifest',
    `window.bob.mcpCallTool("bob_validate_manifest", { workspace: ${JSON.stringify(workspace)} })`,
    (value) => (value.ok ? null : 'bob_validate_manifest failed')
  )
  await evaluate<{ ok: boolean }>(
    'mcp bob_recipe_describe',
    'window.bob.mcpCallTool("bob_recipe_describe", { recipe: "go-agent-tool" })',
    (value) => (value.ok ? null : 'bob_recipe_describe failed')
  )
  await evaluate<{ ok: boolean }>(
    'mcp bob_inspect',
    `window.bob.mcpCallTool("bob_inspect", { workspace: ${JSON.stringify(workspace)} })`,
    (value) => (value.ok ? null : 'bob_inspect failed')
  )
  await evaluate<{ ok: boolean }>(
    'mcp bob_playbook list',
    `window.bob.mcpCallTool("bob_playbook", { workspace: ${JSON.stringify(workspace)}, operation: "list" })`,
    (value) => (value.ok ? null : 'bob_playbook failed')
  )
  await evaluate<{ ok: boolean }>(
    'mcp bob_stats',
    `window.bob.mcpCallTool("bob_stats", { workspace: ${JSON.stringify(workspace)} })`,
    (value) => (value.ok ? null : 'bob_stats failed')
  )
  await evaluate<{ stopped: boolean }>('mcp stop', 'window.bob.mcpStop()', () => null)

  await evaluate<{ length: number }>('activity ledger', 'window.bob.activity()', (value) => (value.length > 10 ? null : `only ${value.length} entries`))

  const report: SmokeReport = {
    startedAt,
    finishedAt: new Date().toISOString(),
    workspace,
    binary: state.binary?.path ?? '',
    checks,
    passed: checks.filter((check) => check.ok).length,
    failed: checks.filter((check) => !check.ok).length
  }
  writeFileSync(reportPath, `${JSON.stringify(report, null, 2)}\n`, 'utf8')
  return report
}

function describe(value: unknown): string {
  if (value === null || value === undefined) return 'null'
  if (Array.isArray(value)) return `${value.length} items`
  if (typeof value === 'object') {
    const keys = Object.keys(value as Record<string, unknown>)
    return keys.slice(0, 6).join(', ') + (keys.length > 6 ? ', …' : '')
  }
  return String(value).slice(0, 120)
}
