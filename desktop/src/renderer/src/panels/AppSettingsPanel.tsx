import { useCallback, useEffect, useState } from 'react'
import { FolderPlus, RefreshCw, Settings, Trash2 } from 'lucide-react'
import type { BinaryInfo } from '@shared/ipc'
import { Badge, Button, Callout, Field, KeyValue, Mono, Section, Segmented, TextInput, Toggle, cx } from '../components/ui'
import { PanelShell } from '../components/shell'
import { bridge, hasNativeBridge } from '../state/bridge'
import { useStore } from '../state/store'

export function AppSettingsPanel(): JSX.Element {
  const { settings, updateSettings, binary, refreshBinary, workspaces, forgetWorkspace, chooseWorkspace, workspace, selectWorkspace, appInfo, repoRoot, toast } =
    useStore()
  const [path, setPath] = useState(settings.bobBinaryPath)
  const [info, setInfo] = useState<BinaryInfo | null>(binary)
  const [resolving, setResolving] = useState(false)

  useEffect(() => {
    setPath(settings.bobBinaryPath)
  }, [settings.bobBinaryPath])

  useEffect(() => {
    setInfo(binary)
  }, [binary])

  const resolve = useCallback(async () => {
    setResolving(true)
    try {
      setInfo(await refreshBinary())
    } finally {
      setResolving(false)
    }
  }, [refreshBinary])

  const savePath = async (): Promise<void> => {
    await updateSettings({ bobBinaryPath: path.trim() })
    await resolve()
    toast('Bob binary path saved', 'positive')
  }

  return (
    <PanelShell
      featureId="app-settings"
      actions={
        <Button variant="quiet" icon={<RefreshCw className={cx('size-3.5', resolving && 'animate-spin')} aria-hidden />} loading={resolving} onClick={() => void resolve()}>
          Re-resolve binary
        </Button>
      }
      subtitle="Which Bob the console drives, which workspace it opens, and the safety defaults every mutating panel inherits."
    >
      <Section title="Bob binary" hint="Only this binary is ever spawned for Bob commands. Resolution order: this path, a source-checkout ./bin/bob, the Go bin, common prefixes, then PATH.">
        <div className="flex flex-col gap-3 rounded-[8px] border border-line bg-panel px-3 py-3">
          <div className="flex items-end gap-2">
            <Field label="Explicit path" hint="Leave empty to auto-resolve." className="flex-1">
              <TextInput value={path} onChange={(event) => setPath(event.target.value)} placeholder="/absolute/path/to/bob" className="font-mono text-[11.5px]" />
            </Field>
            <Button variant="primary" onClick={() => void savePath()} disabled={path.trim() === settings.bobBinaryPath}>
              Save
            </Button>
            <Button variant="quiet" onClick={() => setPath('')}>
              Auto
            </Button>
          </div>

          {info ? (
            <KeyValue
              entries={[
                ['resolved', <Mono className={cx('break-all', info.exists ? 'text-jade' : 'text-clay')}>{info.path || 'none'}</Mono>],
                ['source', <Badge tone={info.source === 'settings' ? 'accent' : 'info'}>{info.source}</Badge>],
                ['version', <Mono>{info.version ?? '—'}</Mono>],
                ['commit', <Mono>{info.commit ?? '—'}</Mono>],
                ['build date', <Mono>{info.date ?? '—'}</Mono>],
                ['source checkout', <Mono className="break-all">{repoRoot ?? 'not resolved'}</Mono>]
              ]}
            />
          ) : null}

          {info?.error ? <Callout tone="caution" title="Binary note">{info.error}</Callout> : null}

          {info && info.candidates.length > 0 ? (
            <div>
              <p className="mb-1.5 text-[10.5px] uppercase text-ink-dim">candidates</p>
              <ul className="flex flex-col gap-1">
                {info.candidates.map((candidate) => (
                  <li key={candidate.path} className="flex items-center gap-2">
                    <span className={cx('size-1.5 rounded-full', candidate.exists ? 'bg-jade' : 'bg-ink-dim')} aria-hidden />
                    <Mono className="min-w-0 flex-1 truncate text-ink-muted">{candidate.path}</Mono>
                    <Badge tone="neutral">{candidate.source}</Badge>
                    <Button
                      size="sm"
                      variant="ghost"
                      disabled={!candidate.exists}
                      onClick={() => {
                        setPath(candidate.path)
                        void updateSettings({ bobBinaryPath: candidate.path }).then(() => resolve())
                      }}
                    >
                      Use
                    </Button>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}
        </div>
      </Section>

      <Section title="Safety defaults" hint="These are the console's own guardrails. Bob enforces its ownership rules regardless.">
        <div className="grid grid-cols-3 gap-4 rounded-[8px] border border-line bg-panel px-3 py-3">
          <Toggle
            checked={settings.requireDigestBoundApply}
            onChange={(next) => void updateSettings({ requireDigestBoundApply: next })}
            label="Bind apply to the reviewed digest"
            hint="Apply and upgrade pass --expect-plan-digest from the plan you actually reviewed."
          />
          <Toggle
            checked={settings.requireMutationConfirmation}
            onChange={(next) => void updateSettings({ requireMutationConfirmation: next })}
            label="Confirm before any mutation"
            hint="Every mutating panel asks first and shows exactly what will change."
          />
          <Toggle
            checked={settings.allowIntegrationProbes}
            onChange={(next) => void updateSettings({ allowIntegrationProbes: next })}
            tone="danger"
            label="Allow expanded authority"
            hint="Unlocks --probe-integrations and MCP --allow-any-workspace. Off by default; each use still confirms."
          />
        </div>
      </Section>

      <Section title="Appearance">
        <div className="flex items-center gap-3 rounded-[8px] border border-line bg-panel px-3 py-3">
          <Segmented
            ariaLabel="Theme"
            value={settings.theme}
            onChange={(theme) => void updateSettings({ theme })}
            options={[
              { value: 'dark', label: 'Dark' },
              { value: 'light', label: 'Light' },
              { value: 'system', label: 'System' }
            ]}
          />
          <span className="text-[11.5px] text-ink-dim">Graphite and copper by default; the light theme keeps the same semantic action colors.</span>
        </div>
      </Section>

      <Section
        title="Workspaces"
        actions={
          <Button size="sm" variant="quiet" icon={<FolderPlus className="size-3.5" aria-hidden />} onClick={() => void chooseWorkspace()}>
            Add folder
          </Button>
        }
      >
        {workspaces.length === 0 ? (
          <Callout tone="info" title="No workspace tracked yet">
            Add a repository folder so Bob commands have something to answer about.
          </Callout>
        ) : (
          <ul className="flex flex-col gap-1">
            {workspaces.map((entry) => (
              <li
                key={entry.path}
                className={cx(
                  'flex items-center gap-3 rounded-[8px] border px-3 py-2',
                  entry.path === workspace ? 'border-copper/50 bg-copper/8' : 'border-line bg-panel'
                )}
              >
                <button type="button" className="min-w-0 flex-1 text-left" onClick={() => void selectWorkspace(entry.path)}>
                  <Mono className="block truncate text-ink">{entry.name}</Mono>
                  <Mono className="block truncate text-[10.5px] text-ink-dim">{entry.path}</Mono>
                </button>
                <span className="flex shrink-0 items-center gap-1.5">
                  {entry.recipe ? <Badge tone="info">{entry.recipe.id}@{entry.recipe.version}</Badge> : <Badge tone="neutral">no recipe</Badge>}
                  <Badge tone={entry.hasManifest ? 'positive' : 'caution'}>{entry.hasManifest ? 'bob.yaml' : 'no manifest'}</Badge>
                  <Badge tone={entry.hasLock ? 'info' : 'neutral'}>{entry.managedFiles} locked</Badge>
                  {entry.path === settings.defaultWorkspace ? <Badge tone="accent">default</Badge> : null}
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={async () => {
                      await updateSettings({ defaultWorkspace: entry.path })
                      toast('Default workspace updated', 'positive')
                    }}
                  >
                    Set default
                  </Button>
                  <Button size="sm" variant="ghost" icon={<Trash2 className="size-3" aria-hidden />} onClick={() => void forgetWorkspace(entry.path)}>
                    Forget
                  </Button>
                </span>
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section title="Demo mode" hint="Replays real captured Bob envelopes so every panel is reviewable without a binary.">
        <div className="flex flex-col gap-3 rounded-[8px] border border-line bg-panel px-3 py-3">
          <Toggle
            checked={settings.demoMode || !hasNativeBridge()}
            onChange={(next) => void updateSettings({ demoMode: next })}
            disabled={!hasNativeBridge()}
            label="Use captured demo data"
            hint={
              hasNativeBridge()
                ? 'Reloads the console; nothing is executed while demo mode is on.'
                : 'Running in a browser without the Electron bridge, so demo data is the only source.'
            }
          />
          <p className="text-[11.5px] leading-relaxed text-ink-dim">
            Captures come from a real Bob checkout: plans with conflicts, apply receipts, refusal envelopes, context profiles, playbooks, and
            stats. Demo mode never writes files, starts servers, or runs tasks.
          </p>
        </div>
      </Section>

      <Section title="Runtime" actions={<Settings className="size-4 text-ink-dim" aria-hidden />}>
        <KeyValue
          entries={[
            ['console version', <Mono>{appInfo?.appVersion ?? '—'}</Mono>],
            ['electron', <Mono>{appInfo?.electron ?? '—'}</Mono>],
            ['node', <Mono>{appInfo?.node ?? '—'}</Mono>],
            ['chromium', <Mono>{appInfo?.chrome ?? '—'}</Mono>],
            ['platform', <Mono>{appInfo?.platform ?? '—'} {appInfo?.arch ?? ''}</Mono>],
            ['user data', <Mono className="break-all">{appInfo?.userData ?? '—'}</Mono>],
            ['settings file', <Mono className="break-all">{appInfo ? `${appInfo.userData}/settings.json` : '—'}</Mono>],
            ['bridge', <Badge tone={hasNativeBridge() ? 'positive' : 'caution'}>{hasNativeBridge() ? 'electron preload' : 'demo replayer'}</Badge>]
          ]}
        />
        <div className="flex flex-wrap gap-2">
          <Button size="sm" variant="quiet" onClick={async () => { const outcome = await bridge().exportActivity(); if (outcome.path) toast(`Exported to ${outcome.path}`, 'positive') }}>
            Export activity ledger
          </Button>
          <Button size="sm" variant="quiet" onClick={async () => { if (appInfo) await bridge().openExternal(appInfo.userData, 'finder') }}>
            Reveal user data
          </Button>
        </div>
      </Section>
    </PanelShell>
  )
}

export default AppSettingsPanel
