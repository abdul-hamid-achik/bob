import { useCallback, useEffect, useState } from 'react'
import { SlidersHorizontal } from 'lucide-react'
import type { ConfigInitData, ConfigShowData } from '@shared/types'
import { Badge, Button, Callout, KeyValue, Modal, Mono, Section, Toggle } from '../components/ui'
import { PanelShell } from '../components/shell'
import { FailureNotice, RunButton, RunMeta, WarningList } from '../components/run'
import { bridge } from '../state/bridge'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

const PRIVACY =
  'Telemetry is disabled by default, has no network transport, and never stores paths, arguments, filenames, manifest content, or raw errors. When enabled it appends a bounded event schema to Bob’s XDG state directory only.'

export function ConfigPanel(): JSX.Element {
  const { revision, toast, bump } = useStore()
  const show = useRun<ConfigShowData>('config-show')
  const init = useRun<ConfigInitData>('config-init')
  const [telemetry, setTelemetry] = useState(false)
  const [confirmWrite, setConfirmWrite] = useState(false)

  const run = useCallback(async () => {
    const result = await show.run()
    const preview = await init.run()
    setTelemetry(Boolean(preview.envelope?.ok) ? Boolean((preview.envelope?.data as ConfigInitData | undefined)?.settings.telemetry.enabled) : false)
    return result
  }, [show, init])

  useEffect(() => {
    void run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [revision])

  const data = show.data
  const preview = init.data

  const write = async (): Promise<void> => {
    setConfirmWrite(false)
    const result = await init.run({ confirmed: true, flags: { write: true, telemetry: telemetry || undefined } })
    if (result.envelope?.ok) {
      toast('Bob settings written', 'positive')
      bump()
      await run()
    }
  }

  return (
    <PanelShell
      featureId="config-show"
      title="Bob config"
      subtitle="Bob's private per-user settings and the XDG paths it resolved. This is Bob's configuration, not the console's."
      actions={
        <>
          <Button variant="quiet" loading={show.pending} onClick={() => void run()}>
            Reload
          </Button>
          <Button variant="primary" icon={<SlidersHorizontal className="size-3.5" aria-hidden />} onClick={() => setConfirmWrite(true)}>
            Write settings
          </Button>
        </>
      }
    >
      <Section title="Resolved XDG paths">
        {data ? (
          <div className="rounded-[8px] border border-line bg-panel px-3 py-3">
            <KeyValue
              entries={[
                ['config file', <Mono className="break-all text-ink">{data.config_file}</Mono>],
                ['exists', <Badge tone={data.config_exists ? 'positive' : 'neutral'}>{String(data.config_exists)}</Badge>],
                ['state dir', <Mono className="break-all">{data.state_dir}</Mono>],
                ['cache dir', <Mono className="break-all">{data.cache_dir}</Mono>],
                ['data dir', <Mono className="break-all">{data.data_dir}</Mono>],
                ['telemetry destination', <Badge tone="info">{data.telemetry_destination}</Badge>]
              ]}
            />
            <div className="mt-3 flex flex-wrap gap-2 border-t border-line pt-3">
              <Button size="sm" variant="quiet" onClick={async () => { await bridge().openExternal(data.state_dir, 'finder') }}>
                Reveal state dir
              </Button>
              <Button size="sm" variant="quiet" onClick={async () => { await bridge().openExternal(data.config_file, 'editor') }}>
                Open config file
              </Button>
            </div>
          </div>
        ) : (
          <p className="text-[12px] text-ink-dim">Run config show to read the resolved paths.</p>
        )}
      </Section>

      <Section title="Effective settings" hint="What Bob will actually do on this machine.">
        {data?.settings ? (
          <div className="flex flex-col gap-3 rounded-[8px] border border-line bg-panel px-3 py-3">
            <KeyValue
              entries={[
                ['schema version', <Mono>{data.settings.schema_version}</Mono>],
                ['telemetry enabled', <Badge tone={data.settings.telemetry.enabled ? 'caution' : 'positive'}>{String(data.settings.telemetry.enabled)}</Badge>],
                ['retention days', <Mono>{data.settings.telemetry.retention_days}</Mono>],
                ['max events per day', <Mono>{data.settings.telemetry.max_events_per_day}</Mono>]
              ]}
            />
          </div>
        ) : null}
      </Section>

      <Section title="Initialize or change settings" hint="Preview is the default; writing creates or updates the private settings file.">
        <div className="flex flex-col gap-3 rounded-[8px] border border-line bg-panel px-3 py-3">
          <Toggle
            checked={telemetry}
            onChange={setTelemetry}
            tone="danger"
            label="--telemetry: enable privacy-bounded local telemetry"
            hint={PRIVACY}
          />
          {preview ? (
            <KeyValue
              entries={[
                ['target', <Mono className="break-all">{preview.path}</Mono>],
                ['would write', <Badge tone={preview.written ? 'positive' : 'neutral'}>{String(preview.written)}</Badge>],
                ['telemetry in preview', <Mono>{String(preview.settings.telemetry.enabled)}</Mono>]
              ]}
            />
          ) : null}
          <Callout tone="info" title="Privacy boundary">
            {PRIVACY}
          </Callout>
        </div>
      </Section>

      <FailureNotice result={show.result ?? init.result} error={show.error ?? init.error} errorCode={show.errorCode ?? init.errorCode} />
      <WarningList warnings={init.result?.envelope?.warnings} />
      <RunMeta result={init.result ?? show.result} />

      <Modal
        open={confirmWrite}
        title="Write Bob's user settings?"
        description="Creates or updates the private XDG settings file for this machine. It never touches a repository."
        onClose={() => setConfirmWrite(false)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setConfirmWrite(false)}>
              Cancel
            </Button>
            <Button variant="primary" onClick={() => void write()}>
              Write settings
            </Button>
          </>
        }
      >
        <KeyValue
          entries={[
            ['target', <Mono className="break-all">{preview?.path ?? data?.config_file ?? '—'}</Mono>],
            ['telemetry', <Badge tone={telemetry ? 'caution' : 'positive'}>{String(telemetry)}</Badge>]
          ]}
        />
      </Modal>
    </PanelShell>
  )
}

export default ConfigPanel
