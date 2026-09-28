import { useCallback, useMemo, useState } from 'react'
import { CheckCircle2, Circle, CircleAlert, Grid3x3, Play, TriangleAlert } from 'lucide-react'
import { buildArgv } from '@shared/argv'
import { FEATURES, GROUP_LABEL, featuresByGroup, type Feature } from '@shared/features'
import { displayArgv } from '@shared/format'
import type { McpServerInfo } from '@shared/types'
import { Badge, Button, Callout, Mono, Section, cx } from '../components/ui'
import { PanelShell } from '../components/shell'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'

export type ProbeStatus = 'untested' | 'running' | 'ok' | 'failed' | 'skipped'

export interface ProbeResult {
  status: ProbeStatus
  detail: string
  durationMs: number
  at: string
}

/** Placeholder values that make a read-only feature probeable in any workspace. */
const PROBE_VALUES: Record<string, Record<string, string | string[]>> = {
  'recipe-show': { recipe: 'go-agent-tool' },
  path: { path: 'bob.yaml' },
  'path-batch': { paths: ['bob.yaml', 'README.md'] },
  'playbook-show': { playbook: 'resolve-ownership-conflict' }
}

const MCP_TOOL_FOR_FEATURE: Record<string, string> = Object.fromEntries(
  FEATURES.filter((feature) => feature.id.startsWith('mcp-bob_')).map((feature) => [feature.id, feature.id.slice('mcp-'.length)])
)

function isProbeable(feature: Feature): boolean {
  if (feature.mutates || feature.authority) return false
  if (feature.id === 'demo-mode' || feature.id === 'watch') return false
  return true
}

function probeLabel(feature: Feature): string {
  if (feature.id.startsWith('mcp-bob_')) return `call ${MCP_TOOL_FOR_FEATURE[feature.id]}`
  if (feature.argvTemplate) return `bob ${displayArgv(buildArgv(feature, { workspace: '{workspace}', values: PROBE_VALUES[feature.id] }))}`
  switch (feature.id) {
    case 'workspaces':
      return 'list workspaces'
    case 'overview':
      return 'bob inspect --json'
    case 'manifest-view':
      return 'read bob.yaml + bob.lock'
    case 'manifest-edit':
      return 'validate buffer via bob_validate_manifest'
    case 'lock-inspect':
      return 'parse bob.lock'
    case 'next-actions':
      return 'bob plan --json → next_actions'
    case 'console':
      return 'bob version --json'
    case 'activity':
      return 'read activity ledger'
    case 'diff':
      return 'bob plan --content --json'
    case 'files':
      return 'list workspace paths'
    case 'repository':
      return 'git status --porcelain'
    case 'tasks':
      return 'parse Taskfile.yml'
    case 'docs':
      return 'list docs pages'
    case 'features':
      return 'read feature registry'
    case 'app-settings':
      return 'read console settings'
    case 'studio':
      return 'bob inspect + plan + stats'
    case 'mcp-serve':
      return 'mcp status / tools'
    default:
      return 'no probe defined'
  }
}

export function FeaturesPanel(): JSX.Element {
  const { workspace, run, navigate, repoRoot, toast } = useStore()
  const [results, setResults] = useState<Record<string, ProbeResult>>({})
  const [mcpInfo, setMcpInfo] = useState<McpServerInfo | null>(null)

  const record = (id: string, result: ProbeResult): void => {
    setResults((prev) => ({ ...prev, [id]: result }))
  }

  const probe = useCallback(
    async (feature: Feature): Promise<ProbeResult> => {
      const started = Date.now()
      const finish = (status: ProbeStatus, detail: string): ProbeResult => {
        const result = { status, detail, durationMs: Date.now() - started, at: new Date().toISOString() }
        record(feature.id, result)
        return result
      }
      if (!isProbeable(feature)) return finish('skipped', feature.mutates ? 'mutating: run it from its own panel' : 'authority-gated or streaming')

      try {
        if (feature.id.startsWith('mcp-bob_')) {
          const tool = MCP_TOOL_FOR_FEATURE[feature.id] ?? ''
          const status = await bridge().mcpStatus()
          setMcpInfo(status)
          if (!status?.started) return finish('skipped', 'start the MCP server first')
          const args: Record<string, unknown> = {}
          if (tool === 'bob_path') args.path = 'bob.yaml'
          if (tool === 'bob_playbook') args.operation = 'list'
          if (tool === 'bob_recipe_describe') args.recipe = 'go-agent-tool'
          if (tool === 'bob_validate_manifest') args.workspace = workspace
          const call = await bridge().mcpCallTool(tool, args)
          return finish(call.ok ? 'ok' : 'failed', `${tool} → ${call.ok ? 'ok' : call.error ?? 'error'} (${call.durationMs} ms)`)
        }

        if (feature.argvTemplate) {
          const featureId = feature.id === 'overview' ? 'inspect' : feature.id === 'diff' ? 'plan' : feature.id
          const argv =
            feature.id === 'overview'
              ? ['inspect', workspace, '--json']
              : feature.id === 'diff'
                ? buildArgv(featureByIdOrThrow('plan'), { workspace, flags: { content: true } })
                : buildArgv(feature, { workspace, values: PROBE_VALUES[feature.id] })
          const result = await run(featureId, { argv, cwd: workspace })
          const ok = Boolean(result.envelope?.ok)
          return finish(
            ok ? 'ok' : 'failed',
            `exit ${result.exitCode} · ${result.envelope?.ok ? 'ok' : (result.envelope?.data as { error?: { code?: string } })?.error?.code ?? 'failed'} · ${result.durationMs} ms`
          )
        }

        switch (feature.id) {
          case 'workspaces': {
            const list = await bridge().listWorkspaces()
            return finish('ok', `${list.length} workspace(s) tracked`)
          }
          case 'manifest-view':
          case 'lock-inspect': {
            const files = await bridge().readWorkspace(workspace)
            return finish(
              files.manifest ? 'ok' : 'failed',
              feature.id === 'lock-inspect'
                ? `${files.lockFiles.length} lock entries · recipe ${files.lockRecipe ? `${files.lockRecipe.id}@${files.lockRecipe.version}` : 'none'}`
                : `${files.manifest ? files.manifest.length : 0} bytes of bob.yaml`
            )
          }
          case 'manifest-edit': {
            const files = await bridge().readWorkspace(workspace)
            if (!files.manifest) return finish('skipped', 'no bob.yaml to validate')
            const status = await bridge().mcpStatus()
            if (!status?.started) return finish('skipped', 'start the MCP server to validate inline YAML')
            const call = await bridge().mcpCallTool('bob_validate_manifest', { manifest_yaml: files.manifest })
            return finish(call.ok ? 'ok' : 'failed', call.ok ? 'inline manifest validates' : (call.error ?? 'rejected'))
          }
          case 'next-actions': {
            const result = await run('plan', { cwd: workspace })
            const count = result.envelope?.next_actions.length ?? 0
            return finish(result.envelope ? 'ok' : 'failed', `${count} guidance line(s) parsed`)
          }
          case 'console': {
            const result = await run('version', { cwd: workspace })
            return finish(result.envelope?.ok ? 'ok' : 'failed', `exit ${result.exitCode} · ${result.durationMs} ms`)
          }
          case 'activity': {
            const entries = await bridge().activity()
            return finish('ok', `${entries.length} ledger entries`)
          }
          case 'files': {
            const paths = await bridge().listPaths(workspace)
            return finish(paths.length > 0 ? 'ok' : 'failed', `${paths.length} paths listed`)
          }
          case 'repository': {
            const status = await bridge().gitStatus(workspace)
            return finish(status.available ? 'ok' : 'skipped', status.available ? `${status.branch ?? 'detached'} · ${status.entries.length} changed` : (status.error ?? 'no git'))
          }
          case 'tasks': {
            const list = await bridge().listTasks(repoRoot ?? workspace)
            return finish(list.length > 0 ? 'ok' : 'skipped', `${list.length} Taskfile task(s)`)
          }
          case 'docs': {
            const list = await bridge().listDocs()
            return finish(list.length > 0 ? 'ok' : 'skipped', `${list.length} documentation page(s)`)
          }
          case 'features':
            return finish('ok', `${FEATURES.length} features registered`)
          case 'app-settings': {
            const settings = await bridge().getSettings()
            const info = await bridge().appInfo()
            return finish('ok', `theme ${settings.theme} · electron ${info.electron || 'n/a'}`)
          }
          case 'studio': {
            const [inspectResult, checkResult] = await Promise.all([run('inspect', { cwd: workspace }), run('check', { cwd: workspace })])
            return finish(inspectResult.envelope?.ok && checkResult.envelope ? 'ok' : 'failed', 'overview + plan projections read')
          }
          case 'mcp-serve': {
            const status = await bridge().mcpStatus()
            setMcpInfo(status)
            return finish(status?.started ? 'ok' : 'skipped', status?.started ? `${status.tools.length} tools live` : 'server not started')
          }
          default:
            return finish('skipped', 'no probe defined')
        }
      } catch (error) {
        return finish('failed', error instanceof Error ? error.message : String(error))
      }
    },
    [run, workspace, repoRoot]
  )

  const runAll = async (): Promise<void> => {
    if (!workspace) {
      toast('Select a workspace first', 'caution')
      return
    }
    const probeable = FEATURES.filter(isProbeable)
    for (const feature of probeable) {
      record(feature.id, { status: 'running', detail: 'probing…', durationMs: 0, at: new Date().toISOString() })
      // Sequential on purpose: Bob takes one workspace lock for mutating work,
      // and serial reads keep the evidence attributable.
      await probe(feature)
    }
  }

  const groups = featuresByGroup()
  const tally = useMemo(() => {
    const counts: Record<ProbeStatus, number> = { untested: 0, running: 0, ok: 0, failed: 0, skipped: 0 }
    for (const feature of FEATURES) {
      const status = results[feature.id]?.status ?? 'untested'
      counts[status] += 1
    }
    return counts
  }, [results])

  return (
    <PanelShell
      featureId="features"
      actions={
        <>
          <Button variant="quiet" onClick={() => setResults({})}>
            Reset evidence
          </Button>
          <Button variant="primary" icon={<Play className="size-3.5" aria-hidden />} onClick={() => void runAll()}>
            Probe every read-only feature
          </Button>
        </>
      }
      subtitle={`Every Bob surface the console integrates: ${FEATURES.length} features across ${groups.length} groups. A probe runs the real underlying command and records what came back.`}
    >
      <Section
        title="Coverage"
        actions={
          <span className="flex items-center gap-2">
            <Badge tone="positive">{tally.ok} ok</Badge>
            <Badge tone="danger">{tally.failed} failed</Badge>
            <Badge tone="caution">{tally.skipped} gated</Badge>
            <Badge tone="neutral">{tally.untested} untested</Badge>
          </span>
        }
      >
        <Callout tone="info" title="What this matrix proves">
          Each row is a real Bob surface — a CLI command and flag set, one of the nine MCP tools, or a console capability built on Bob's
          contracts. Probing executes the underlying read-only call against the active workspace and records the exit code, so integration is
          demonstrated rather than claimed. Mutating and authority-gated features are never probed automatically; open their panel to drive
          them deliberately.
        </Callout>
        {mcpInfo && !mcpInfo.started ? (
          <Callout tone="caution" title="MCP server is stopped">
            Start it on the MCP panel to probe the nine tools. {mcpInfo.error ?? ''}
          </Callout>
        ) : null}
      </Section>

      {groups.map((group) => (
        <Section key={group.group} title={group.label} hint={`${group.features.length} features`}>
          <ul className="flex flex-col gap-1">
            {group.features.map((feature) => {
              const result = results[feature.id]
              const status = result?.status ?? 'untested'
              const probeable = isProbeable(feature)
              return (
                <li
                  key={feature.id}
                  className={cx(
                    'grid grid-cols-[minmax(0,1fr)_16rem_9rem_7rem] items-start gap-3 rounded-[8px] border px-3 py-2',
                    status === 'failed' ? 'border-clay/40 bg-clay/6' : status === 'ok' ? 'border-jade/25 bg-panel' : 'border-line bg-panel'
                  )}
                >
                  <div className="min-w-0">
                    <p className="flex flex-wrap items-center gap-2">
                      <span className="text-[12.5px] font-medium text-ink">{feature.title}</span>
                      <Mono className="text-ink-muted">{feature.command}</Mono>
                      {feature.mutates ? <Badge tone="caution">mutates</Badge> : null}
                      {feature.authority ? <Badge tone="danger">explicit authority</Badge> : null}
                      {feature.streaming ? <Badge tone="info">streaming</Badge> : null}
                    </p>
                    <p className="mt-0.5 text-[11.5px] leading-snug text-ink-muted">{feature.summary}</p>
                  </div>

                  <div className="min-w-0">
                    <Mono className="block truncate text-[10.5px] text-ink-dim" title={probeLabel(feature)}>
                      {probeLabel(feature)}
                    </Mono>
                    {result ? (
                      <p className={cx('mt-0.5 break-words text-[11px]', status === 'failed' ? 'text-clay' : status === 'ok' ? 'text-jade' : 'text-ink-dim')}>
                        {result.detail}
                      </p>
                    ) : null}
                  </div>

                  <div className="flex flex-col gap-1">
                    <span className="flex items-center gap-1.5">
                      {status === 'ok' ? (
                        <CheckCircle2 className="size-3.5 text-jade" aria-hidden />
                      ) : status === 'failed' ? (
                        <CircleAlert className="size-3.5 text-clay" aria-hidden />
                      ) : status === 'skipped' ? (
                        <TriangleAlert className="size-3.5 text-brass" aria-hidden />
                      ) : status === 'running' ? (
                        <Circle className="size-3.5 animate-pulse text-copper" aria-hidden />
                      ) : (
                        <Circle className="size-3.5 text-ink-dim" aria-hidden />
                      )}
                      <span className={cx('text-[11px]', status === 'ok' ? 'text-jade' : status === 'failed' ? 'text-clay' : 'text-ink-dim')}>{status}</span>
                    </span>
                    <span className="flex items-center gap-1.5">
                      <Badge tone="neutral">{feature.surface}</Badge>
                      {result && result.durationMs > 0 ? <Mono className="text-[10px] text-ink-dim">{result.durationMs}ms</Mono> : null}
                    </span>
                  </div>

                  <div className="flex flex-col items-end gap-1">
                    <Button size="sm" variant="quiet" onClick={() => navigate(feature.panel)}>
                      Open
                    </Button>
                    {probeable ? (
                      <Button
                        size="sm"
                        variant="default"
                        icon={<Grid3x3 className="size-3" aria-hidden />}
                        disabled={!workspace && Boolean(feature.argvTemplate)}
                        onClick={() => void probe(feature)}
                      >
                        Probe
                      </Button>
                    ) : null}
                  </div>
                </li>
              )
            })}
          </ul>
        </Section>
      ))}

      <Section title="Group coverage">
        <ul className="flex flex-wrap gap-2">
          {Object.entries(GROUP_LABEL).map(([key, label]) => (
            <li key={key} className="rounded-[6px] border border-line bg-panel px-2.5 py-1">
              <span className="text-[11.5px] text-ink-muted">{label}</span>
              <Mono className="ml-2 text-ink">{FEATURES.filter((feature) => feature.group === key).length}</Mono>
            </li>
          ))}
        </ul>
      </Section>
    </PanelShell>
  )
}

function featureByIdOrThrow(id: string): Feature {
  const feature = FEATURES.find((entry) => entry.id === id)
  if (!feature) throw new Error(`missing feature: ${id}`)
  return feature
}

export default FeaturesPanel
