import { useCallback, useEffect, useState } from 'react'
import { FileCode2, Save } from 'lucide-react'
import type { InitData, Manifest } from '@shared/types'
import { Badge, Button, Callout, CodeBlock, KeyValue, Modal, Mono, Section, Tabs, TextArea, cx } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, RunMeta, WarningList } from '../components/run'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

type Tab = 'typed' | 'yaml' | 'validate'

interface ValidatedManifest {
  ok?: boolean
  data?: { manifest?: Manifest; error?: { message?: string } }
}

interface Validation {
  ok: boolean
  at: string
  source: 'mcp-inline' | 'init-preview'
  manifest: Manifest | null
  error: string | null
  detail: string | null
}

function ManifestView({ manifest }: { manifest: Manifest }): JSX.Element {
  return (
    <div className="flex flex-col gap-4">
      <Section title="Product">
        <KeyValue
          entries={[
            ['name', <Mono className="text-ink">{manifest.product.name || '—'}</Mono>],
            ['module', <Mono>{manifest.product.module || '—'}</Mono>],
            ['description', manifest.product.description || '—'],
            ['visibility', manifest.product.visibility || '—'],
            ['license', manifest.product.license || '—'],
            ['runtime', <Mono>{manifest.runtime.language} · {manifest.runtime.kind}</Mono>],
            ['recipe', <Badge tone="info">{manifest.recipe}</Badge>]
          ]}
        />
      </Section>
      <Section title="Surfaces" hint="What this product exposes; Bob seeds the matching hygiene artifacts.">
        <ul className="flex flex-wrap gap-2">
          {(Object.entries(manifest.surfaces) as [string, boolean][]).map(([key, value]) => (
            <li key={key}>
              <Badge tone={value ? 'positive' : 'neutral'}>
                {key}: {String(value)}
              </Badge>
            </li>
          ))}
        </ul>
      </Section>
      <Section title="Intelligence seams" hint="Optional tools stay separate and are reached through explicit public contracts.">
        <KeyValue
          entries={(Object.entries(manifest.integrations) as [string, string][]).map(([key, value]) => [
            key.replace(/_/g, ' '),
            <Mono className={value && value !== 'none' ? 'text-ink' : 'text-ink-dim'}>{value || 'unset'}</Mono>
          ])}
        />
      </Section>
      <Section title="Distribution">
        <KeyValue
          entries={[
            ...((Object.entries(manifest.distribution) as [string, string | boolean][]).map(([key, value]) => [
              key.replace(/_/g, ' '),
              typeof value === 'boolean' ? <Badge tone={value ? 'positive' : 'neutral'}>{String(value)}</Badge> : <Mono>{value || '—'}</Mono>
            ]) as [string, React.ReactNode][])
          ]}
        />
      </Section>
      <Section title="Ownership" hint="Human-owned adjustments such as released paths that Bob seeds once and never lock-owns again.">
        {manifest.ownership && Object.keys(manifest.ownership).length > 0 ? (
          <CodeBlock maxHeight="12rem">{JSON.stringify(manifest.ownership, null, 2)}</CodeBlock>
        ) : (
          <p className="text-[12px] text-ink-dim">No ownership overrides declared.</p>
        )}
      </Section>
    </div>
  )
}

export function ManifestPanel(): JSX.Element {
  const { workspace, files, reloadFiles, saveManifest, bump, toast } = useStore()
  const preview = useRun<InitData>('init')
  const [tab, setTab] = useState<Tab>('typed')
  const [draft, setDraft] = useState('')
  const [confirmSave, setConfirmSave] = useState(false)
  const [validation, setValidation] = useState<Validation | null>(null)
  const [validating, setValidating] = useState(false)
  const [mcpNote, setMcpNote] = useState<string | null>(null)

  const onDisk = files?.manifest ?? ''
  const dirty = draft !== onDisk

  useEffect(() => {
    setDraft(onDisk)
  }, [onDisk, workspace])

  const refreshTyped = useCallback(() => preview.run(), [preview])

  useEffect(() => {
    if (workspace) void refreshTyped()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspace])

  const typedManifest = preview.data?.manifest ?? null

  const validateBuffer = async (): Promise<void> => {
    setValidating(true)
    setMcpNote(null)
    try {
      const status = await bridge().mcpStatus()
      if (!status?.started) {
        await bridge().mcpStart({ workspace })
        setMcpNote('Started bob mcp serve with this workspace as the exact allowlist to validate the inline buffer.')
      }
      const result = await bridge().mcpCallTool('bob_validate_manifest', { manifest_yaml: draft })
      const text = result.content.map((part) => part.text ?? '').join('\n')
      let parsed: ValidatedManifest | null = null
      try {
        parsed = JSON.parse(text) as ValidatedManifest
      } catch {
        parsed = null
      }
      setValidation({
        ok: result.ok && parsed?.ok !== false,
        at: new Date().toISOString(),
        source: 'mcp-inline',
        manifest: parsed?.data?.manifest ?? null,
        error: result.error ?? parsed?.data?.error?.message ?? (result.ok ? null : text.slice(0, 600)),
        detail: text.slice(0, 4000) || null
      })
    } catch (error) {
      setValidation({
        ok: false,
        at: new Date().toISOString(),
        source: 'mcp-inline',
        manifest: null,
        error: error instanceof Error ? error.message : String(error),
        detail: null
      })
    } finally {
      setValidating(false)
    }
  }

  const validateOnDisk = async (): Promise<void> => {
    const result = await preview.run()
    const data = (result.envelope?.data ?? null) as InitData | null
    setValidation({
      ok: Boolean(result.envelope?.ok),
      at: new Date().toISOString(),
      source: 'init-preview',
      manifest: data?.manifest ?? null,
      error: result.envelope?.ok
        ? null
        : ((data?.error?.message as string | undefined) ?? result.stderr.trim() ?? `exit ${result.exitCode}`),
      detail: null
    })
  }

  const doSave = async (): Promise<void> => {
    setConfirmSave(false)
    const outcome = await saveManifest(draft)
    if (outcome.written) {
      toast('bob.yaml saved', 'positive')
      bump()
      await refreshTyped()
    } else {
      toast(outcome.error ?? 'Save failed', 'danger')
    }
  }

  return (
    <PanelShell
      featureId="manifest-view"
      title="Manifest"
      subtitle="bob.yaml is the strict human-owned contract: product identity, surfaces, local intelligence seams, and distribution. Bob reads it, validates it, and never rewrites it."
      actions={
        <>
          <Button variant="quiet" loading={preview.pending} onClick={() => void refreshTyped()}>
            Re-read
          </Button>
          <Button
            variant="primary"
            icon={<Save className="size-3.5" aria-hidden />}
            disabled={!dirty}
            onClick={() => setConfirmSave(true)}
          >
            {dirty ? 'Save bob.yaml' : 'Saved'}
          </Button>
        </>
      }
    >
      <WorkspaceGate>
        <Tabs
          ariaLabel="Manifest views"
          value={tab}
          onChange={setTab}
          items={[
            { value: 'typed', label: 'Typed contract' },
            { value: 'yaml', label: 'YAML', badge: dirty ? <Badge tone="caution">unsaved</Badge> : undefined },
            { value: 'validate', label: 'Validation' }
          ]}
        />

        {tab === 'typed' ? (
          typedManifest ? (
            <ManifestView manifest={typedManifest} />
          ) : (
            <Callout tone="info" title="No typed manifest yet">
              Bob could not read a manifest here. Initialize one on the Init &amp; new panel, or fix the YAML errors on the Validation tab.
            </Callout>
          )
        ) : null}

        {tab === 'yaml' ? (
          <Section
            title="bob.yaml"
            hint="Edits stay local until you save. Validation runs against this buffer without writing anything."
            actions={
              <>
                <Button size="sm" variant="quiet" disabled={!dirty} onClick={() => setDraft(onDisk)}>
                  Revert
                </Button>
                <Button size="sm" variant="default" loading={validating} onClick={() => void validateBuffer()}>
                  Validate buffer
                </Button>
              </>
            }
          >
            {files?.manifestPath ? (
              <Mono className="text-ink-dim">{files.manifestPath}</Mono>
            ) : (
              <Callout tone="caution" title="No bob.yaml on disk">
                Saving will create it. Use the Init panel to detect the stack and seed a matching recipe first.
              </Callout>
            )}
            <TextArea
              value={draft}
              rows={22}
              spellCheck={false}
              onChange={(event) => setDraft(event.target.value)}
              aria-label="bob.yaml contents"
              className="font-mono text-[12px] leading-[1.55]"
            />
            {files?.manifestError ? <Callout tone="danger" title="Stored manifest does not parse">{files.manifestError}</Callout> : null}
          </Section>
        ) : null}

        {tab === 'validate' ? (
          <Section
            title="Strict validation"
            hint="Two honest paths: the read-only MCP tool validates the inline buffer, and Bob's own init preview strictly loads what is on disk."
            actions={
              <>
                <Button size="sm" variant="default" loading={validating} onClick={() => void validateBuffer()}>
                  Validate buffer (MCP)
                </Button>
                <Button size="sm" variant="quiet" loading={preview.pending} onClick={() => void validateOnDisk()}>
                  Validate on disk
                </Button>
              </>
            }
          >
            {mcpNote ? <Callout tone="info" title="MCP server">{mcpNote}</Callout> : null}
            {validation ? (
              <div className="flex flex-col gap-3">
                <div
                  className={cx(
                    'flex flex-wrap items-center gap-3 rounded-[8px] border px-3 py-2.5',
                    validation.ok ? 'border-jade/40 bg-jade/8' : 'border-clay/45 bg-clay/8'
                  )}
                >
                  <FileCode2 className={cx('size-4', validation.ok ? 'text-jade' : 'text-clay')} aria-hidden />
                  <p className={cx('text-[13px] font-medium', validation.ok ? 'text-jade' : 'text-clay')}>
                    {validation.ok ? 'Manifest is valid' : 'Manifest rejected'}
                  </p>
                  <Badge tone="neutral">{validation.source === 'mcp-inline' ? 'bob_validate_manifest (inline)' : 'bob init preview (on disk)'}</Badge>
                  <Mono className="ml-auto text-ink-dim">{validation.at}</Mono>
                </div>
                {validation.error ? <Callout tone="danger" title={validation.error} /> : null}
                {validation.manifest ? <ManifestView manifest={validation.manifest} /> : null}
                {validation.detail ? (
                  <CodeBlock maxHeight="16rem" copy>
                    {validation.detail}
                  </CodeBlock>
                ) : null}
              </div>
            ) : (
              <p className="text-[12px] text-ink-dim">
                Nothing validated yet. Bob rejects unknown fields, wrong types, and unsafe values strictly — validation is the cheapest way
                to find out before a plan.
              </p>
            )}
          </Section>
        ) : null}

        {preview.data?.detection ? (
          <Section title="Stack detection" hint="What Bob detected in this repository, and which recipe it implies.">
            <KeyValue
              entries={[
                ['primary', <Badge tone="info">{preview.data.detection.primary}</Badge>],
                ['stacks', preview.data.detection.stacks.map((stack) => <Badge key={stack.id} tone="neutral">{stack.id}: {stack.markers.join(', ')}</Badge>)],
                ...(preview.data.detection.kind_hint ? ([['kind hint', <Mono>{preview.data.detection.kind_hint}</Mono>]] as [string, React.ReactNode][]) : []),
                ...(preview.data.detection.module ? ([['module', <Mono>{preview.data.detection.module}</Mono>]] as [string, React.ReactNode][]) : []),
                ...(preview.data.detection.signals?.length
                  ? ([['signals', <Mono className="text-ink-muted">{preview.data.detection.signals.join(', ')}</Mono>]] as [string, React.ReactNode][])
                  : [])
              ]}
            />
          </Section>
        ) : null}

        <FailureNotice result={preview.result} error={preview.error} errorCode={preview.errorCode} title="Manifest read failed" />
        <WarningList warnings={preview.result?.envelope?.warnings} />
        <RunMeta result={preview.result} />

        <Modal
          open={confirmSave}
          title="Save bob.yaml?"
          description="bob.yaml is human-owned. Saving rewrites it exactly as edited, then re-plans against the new contract."
          onClose={() => setConfirmSave(false)}
          footer={
            <>
              <Button variant="ghost" onClick={() => setConfirmSave(false)}>
                Cancel
              </Button>
              <Button variant="primary" onClick={() => void doSave()}>
                Save manifest
              </Button>
            </>
          }
        >
          <KeyValue
            entries={[
              ['workspace', <Mono className="break-all">{workspace}</Mono>],
              ['target', <Mono className="break-all">{files?.manifestPath ?? `${workspace}/bob.yaml`}</Mono>],
              ['buffer bytes', <Mono>{draft.length}</Mono>]
            ]}
          />
        </Modal>
      </WorkspaceGate>
    </PanelShell>
  )
}

export default ManifestPanel
