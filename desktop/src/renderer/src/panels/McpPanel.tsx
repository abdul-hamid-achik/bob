import { useCallback, useEffect, useMemo, useState } from 'react'
import { Cable, CircleStop, Play, Plug } from 'lucide-react'
import type { McpCallResult, McpLogLine } from '@shared/ipc'
import type { McpServerInfo, McpTool } from '@shared/types'
import { Badge, Button, Callout, CodeBlock, Disclosure, EmptyState, KeyValue, Mono, Section, TextArea, TextInput, Toggle, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'

interface ToolForm {
  tool: McpTool
  values: Record<string, string>
}

function defaultValue(type: string): string {
  if (type === 'boolean') return 'false'
  if (type === 'integer') return '0'
  return ''
}

function coerce(tool: McpTool, values: Record<string, string>): Record<string, unknown> {
  const out: Record<string, unknown> = {}
  const properties = tool.inputSchema.properties ?? {}
  for (const [key, raw] of Object.entries(values)) {
    const spec = properties[key]
    if (!spec) continue
    if (raw.trim() === '' && !(tool.inputSchema.required ?? []).includes(key)) continue
    if (spec.type === 'boolean') out[key] = raw === 'true'
    else if (spec.type === 'integer') out[key] = Number(raw)
    else if (spec.type === 'object') {
      try {
        out[key] = JSON.parse(raw)
      } catch {
        out[key] = raw
      }
    } else out[key] = raw
  }
  return out
}

export function McpPanel(): JSX.Element {
  const { workspace, binary, settings, updateSettings, toast } = useStore()
  const [info, setInfo] = useState<McpServerInfo | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [extraWorkspaces, setExtraWorkspaces] = useState('')
  const [allowAny, setAllowAny] = useState(false)
  const [form, setForm] = useState<ToolForm | null>(null)
  const [call, setCall] = useState<McpCallResult | null>(null)
  const [calling, setCalling] = useState(false)
  const [log, setLog] = useState<McpLogLine[]>([])

  const refresh = useCallback(async () => {
    const [status, lines] = await Promise.all([bridge().mcpStatus(), bridge().mcpLog()])
    setInfo(status)
    setLog(lines)
  }, [])

  useEffect(() => {
    void refresh()
    const timer = setInterval(() => void refresh(), 4000)
    return () => clearInterval(timer)
  }, [refresh])

  const allowList = useMemo(
    () =>
      extraWorkspaces
        .split('\n')
        .map((line) => line.trim())
        .filter(Boolean),
    [extraWorkspaces]
  )

  const start = async (): Promise<void> => {
    setBusy(true)
    setError(null)
    try {
      const started = await bridge().mcpStart({ workspace, allowWorkspaces: allowList, allowAnyWorkspace: allowAny })
      setInfo(started)
      if (started.error) setError(started.error)
      else toast(`MCP server started with ${started.tools.length} tools`, 'positive')
    } catch (caught) {
      setError(caught instanceof Error ? caught.message : String(caught))
    } finally {
      setBusy(false)
      await refresh()
    }
  }

  const stop = async (): Promise<void> => {
    setBusy(true)
    await bridge().mcpStop()
    setForm(null)
    setBusy(false)
    await refresh()
  }

  const runTool = async (): Promise<void> => {
    if (!form) return
    setCalling(true)
    try {
      const result = await bridge().mcpCallTool(form.tool.name, coerce(form.tool, form.values))
      setCall(result)
    } finally {
      setCalling(false)
      await refresh()
    }
  }

  const registration = [
    `mcphub add bob "${binary?.path ?? '/absolute/path/to/bob'}" \\`,
    `  --description "Deterministic agent-ready repository builder" \\`,
    `  --tag builder --tag code -- \\`,
    `  mcp serve --workspace ${workspace}`,
    `mcphub pin bob__bob_context bob__bob_plan bob__bob_check`,
    `mcphub doctor --server bob --probe`
  ].join('\n')

  return (
    <PanelShell
      featureId="mcp-serve"
      actions={
        info?.started ? (
          <Button variant="danger" loading={busy} icon={<CircleStop className="size-3.5" aria-hidden />} onClick={() => void stop()}>
            Stop server
          </Button>
        ) : (
          <Button variant="primary" loading={busy} icon={<Play className="size-3.5" aria-hidden />} onClick={() => void start()}>
            Start server
          </Button>
        )
      }
      subtitle="Bob's typed stdio MCP projection: nine repository-read-only tools over newline-delimited JSON-RPC. stdout is protocol-only; diagnostics stay on stderr."
    >
      <WorkspaceGate>
        <Section
          title="Server"
          actions={
            <span className="flex items-center gap-2">
              <span className={cx('size-1.5 rounded-full', info?.started ? 'bg-jade' : 'bg-ink-dim')} aria-hidden />
              <Badge tone={info?.started ? 'positive' : 'neutral'}>{info?.started ? `running (pid ${info.pid})` : 'stopped'}</Badge>
            </span>
          }
        >
          <div className="grid grid-cols-[minmax(0,1fr)_20rem] gap-4">
            <div className="flex flex-col gap-3 rounded-[8px] border border-line bg-panel px-3 py-3">
              <KeyValue
                entries={[
                  ['binary', <Mono className="break-all">{binary?.path ?? 'not resolved'}</Mono>],
                  ['startup workspace', <Mono className="break-all">{workspace}</Mono>],
                  ['authority', <Badge tone="info">repository read-only</Badge>],
                  ['tools', <Mono>{info?.tools.length ?? 0}</Mono>]
                ]}
              />
              <TextArea
                rows={2}
                value={extraWorkspaces}
                onChange={(event) => setExtraWorkspaces(event.target.value)}
                placeholder="--allow-workspace: one absolute path per line"
                aria-label="Additional allowed workspaces"
                className="font-mono text-[11.5px]"
              />
              <Toggle
                checked={allowAny}
                tone="danger"
                onChange={setAllowAny}
                disabled={!settings.allowIntegrationProbes}
                label="--allow-any-workspace"
                hint={
                  settings.allowIntegrationProbes
                    ? 'Expands read authority to any workspace Bob can reach. Must be an explicit choice.'
                    : 'Enable “allow expanded authority” in Settings before widening the allowlist.'
                }
              />
              {error ? <Callout tone="danger" title="Server did not start">{error}</Callout> : null}
            </div>

            <div className="flex flex-col gap-3">
              <Callout tone="info" title="Allowlist model">
                MCP defaults to the exact startup workspace. Broader read authority requires --allow-workspace or an explicit
                --allow-any-workspace.
              </Callout>
              <Disclosure label="MCPHub registration">
                <CodeBlock maxHeight="14rem" copy>
                  {registration}
                </CodeBlock>
                <p className="mt-2 text-[11.5px] leading-relaxed text-ink-dim">
                  The minimal pin set keeps orientation, review, and convergence visible to a small model. bob_path and bob_playbook stay
                  available through lazy discovery. Pinning never grants authority or executes a tool.
                </p>
              </Disclosure>
            </div>
          </div>

          {info?.instructions ? (
            <Disclosure label="Server instructions (from initialize)">
              <pre className="argv max-h-56 overflow-auto whitespace-pre-wrap text-ink-muted">{info.instructions}</pre>
            </Disclosure>
          ) : null}
        </Section>

        <Section title={`Tools (${info?.tools.length ?? 0})`} hint="Every tool is repository read-only and never runs a specialist probe.">
          {info?.started && info.tools.length > 0 ? (
            <div className="grid grid-cols-[20rem_minmax(0,1fr)] gap-4">
              <ul className="max-h-[32rem] overflow-auto rounded-[8px] border border-line">
                {info.tools.map((tool) => (
                  <li key={tool.name}>
                    <button
                      type="button"
                      onClick={() => {
                        setForm({ tool, values: Object.fromEntries(Object.keys(tool.inputSchema.properties ?? {}).map((key) => [key, ''])) })
                        setCall(null)
                      }}
                      className={cx(
                        'flex w-full flex-col gap-1 border-b border-line/70 px-3 py-2 text-left last:border-b-0',
                        form?.tool.name === tool.name ? 'bg-copper/10' : 'hover:bg-raise/60'
                      )}
                    >
                      <span className="flex items-center gap-2">
                        <Cable className="size-3.5 shrink-0 text-copper" aria-hidden />
                        <Mono className="truncate text-ink">{tool.name}</Mono>
                      </span>
                      <span className="line-clamp-2 text-[11px] leading-snug text-ink-muted">{tool.description}</span>
                    </button>
                  </li>
                ))}
              </ul>

              <div className="flex min-w-0 flex-col gap-3">
                {form ? (
                  <>
                    <div className="rounded-[8px] border border-line bg-panel px-3 py-3">
                      <div className="flex items-center gap-2">
                        <Plug className="size-4 text-copper" aria-hidden />
                        <Mono className="text-[13px] text-ink">{form.tool.name}</Mono>
                        <Button size="sm" variant="primary" className="ml-auto" loading={calling} onClick={() => void runTool()}>
                          Call tool
                        </Button>
                      </div>
                      <p className="mt-1.5 text-[12px] leading-relaxed text-ink-muted">{form.tool.description}</p>
                      <div className="mt-3 grid grid-cols-2 gap-3">
                        {Object.entries(form.tool.inputSchema.properties ?? {}).map(([key, spec]) => (
                          <label key={key} className="flex flex-col gap-1">
                            <span className="flex items-center gap-1.5 text-[11px] text-ink-muted">
                              <Mono className="text-ink">{key}</Mono>
                              <Badge tone="neutral">{spec.type}</Badge>
                              {(form.tool.inputSchema.required ?? []).includes(key) ? <Badge tone="caution">required</Badge> : null}
                            </span>
                            {spec.type === 'boolean' ? (
                              <select
                                value={form.values[key] ?? 'false'}
                                onChange={(event) => setForm({ ...form, values: { ...form.values, [key]: event.target.value } })}
                                className="h-8 rounded-[6px] border border-line-strong bg-sunken px-2 font-mono text-[12px] text-ink focus:border-copper focus:outline-none"
                              >
                                <option value="">omit</option>
                                <option value="true">true</option>
                                <option value="false">false</option>
                              </select>
                            ) : (
                              <TextInput
                                value={form.values[key] ?? ''}
                                placeholder={spec.description?.slice(0, 60) ?? defaultValue(spec.type)}
                                onChange={(event) => setForm({ ...form, values: { ...form.values, [key]: event.target.value } })}
                                className="font-mono text-[11.5px]"
                              />
                            )}
                            {spec.description ? <span className="text-[10.5px] leading-snug text-ink-dim">{spec.description}</span> : null}
                          </label>
                        ))}
                      </div>
                    </div>

                    {call ? (
                      <div className="flex flex-col gap-2">
                        <div className="flex flex-wrap items-center gap-2">
                          <Badge tone={call.ok ? 'positive' : 'danger'}>{call.ok ? 'ok' : 'error'}</Badge>
                          <Mono className="text-ink-muted">{call.tool}</Mono>
                          <Mono className="text-ink-dim">{call.durationMs} ms</Mono>
                        </div>
                        {call.error ? <Callout tone="danger" title={call.error} /> : null}
                        <CodeBlock maxHeight="20rem" copy>
                          {call.content.map((part) => part.text ?? '').join('\n') || JSON.stringify(call.structured ?? {}, null, 2)}
                        </CodeBlock>
                        <Disclosure label="Raw JSON-RPC response">
                          <CodeBlock maxHeight="16rem" copy>
                            {call.rawResponse ?? '(none)'}
                          </CodeBlock>
                        </Disclosure>
                      </div>
                    ) : null}
                  </>
                ) : (
                  <EmptyState title="No tool selected" hint="Pick a tool to see its input schema and call it against the allowed workspace." />
                )}
              </div>
            </div>
          ) : (
            <EmptyState
              title="Server is not running"
              hint="Start the server to list and call the nine read-only tools. The console spawns bob mcp serve with this workspace as the exact allowlist."
              action={
                <Button variant="primary" loading={busy} onClick={() => void start()}>
                  Start bob mcp serve
                </Button>
              }
            />
          )}
        </Section>

        <Section
          title="Protocol log"
          hint="stderr diagnostics only. stdout is reserved for JSON-RPC and never appears here as text."
          actions={
            <Button size="sm" variant="quiet" onClick={() => void updateSettings({ allowIntegrationProbes: settings.allowIntegrationProbes })}>
              Refresh log
            </Button>
          }
        >
          {log.length > 0 ? (
            <ul className="max-h-48 overflow-auto rounded-[8px] border border-line bg-sunken px-3 py-2">
              {log.slice(-80).map((line, index) => (
                <li key={`${line.at}-${index}`} className="flex gap-2 py-px">
                  <Mono className="shrink-0 text-ink-dim">{line.at.slice(11, 19)}</Mono>
                  <Badge tone={line.stream === 'stderr' ? 'caution' : 'info'}>{line.stream}</Badge>
                  <span className="min-w-0 flex-1 break-words text-[11.5px] text-ink-muted">{line.line}</span>
                </li>
              ))}
            </ul>
          ) : (
            <p className="text-[12px] text-ink-dim">No diagnostics recorded yet.</p>
          )}
        </Section>
      </WorkspaceGate>
    </PanelShell>
  )
}

export default McpPanel
