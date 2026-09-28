import { useCallback, useEffect, useState } from 'react'
import { Hammer, Sparkles } from 'lucide-react'
import type { InitData, NewData, RecipeSummary } from '@shared/types'
import { Badge, Button, Callout, Field, KeyValue, Modal, Mono, Section, Tabs, TextInput, Toggle } from '../components/ui'
import { PanelShell, WorkspaceGate } from '../components/shell'
import { FailureNotice, RunMeta, WarningList } from '../components/run'
import { useStore } from '../state/store'
import { useRun } from '../state/useRun'

type Tab = 'init' | 'new'

export function ScaffoldPanel(): JSX.Element {
  const { workspace, navParams, files, reloadFiles, bump, toast, chooseWorkspace, addWorkspace } = useStore()
  const catalog = useRun<RecipeSummary[]>('recipe-list')
  const init = useRun<InitData>('init')
  const create = useRun<NewData>('new')
  const [tab, setTab] = useState<Tab>(files?.manifestPath ? 'init' : 'init')

  const [initName, setInitName] = useState('')
  const [initModule, setInitModule] = useState('')
  const [initDescription, setInitDescription] = useState('')
  const [initRecipe, setInitRecipe] = useState(navParams.recipe ?? '')
  const [initForce, setInitForce] = useState(false)
  const [initConfirm, setInitConfirm] = useState(false)

  const [newName, setNewName] = useState('')
  const [newDir, setNewDir] = useState('')
  const [newModule, setNewModule] = useState('')
  const [newDescription, setNewDescription] = useState('')
  const [newRecipe, setNewRecipe] = useState('')
  const [newConfirm, setNewConfirm] = useState(false)

  useEffect(() => {
    void catalog.run()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  useEffect(() => {
    if (navParams.recipe) {
      setInitRecipe(navParams.recipe)
      setNewRecipe(navParams.recipe)
    }
  }, [navParams.recipe])

  const recipes = Array.isArray(catalog.data) ? catalog.data : []

  const previewInit = useCallback(() => init.run({
    values: {},
    flags: {
      name: initName || undefined,
      module: initModule || undefined,
      description: initDescription || undefined,
      recipe: initRecipe || undefined,
      force: initForce || undefined
    }
  }), [init, initName, initModule, initDescription, initRecipe, initForce])

  const writeInit = useCallback(async () => {
    setInitConfirm(false)
    const result = await init.run({
      confirmed: true,
      flags: {
        name: initName || undefined,
        module: initModule || undefined,
        description: initDescription || undefined,
        recipe: initRecipe || undefined,
        force: initForce || undefined,
        write: true
      }
    })
    if (result.envelope?.ok) {
      toast('bob.yaml written', 'positive')
      await reloadFiles()
      bump()
    }
  }, [init, initName, initModule, initDescription, initRecipe, initForce, toast, reloadFiles, bump])

  const previewNew = useCallback(
    () =>
      create.run({
        values: { name: newName || 'new-project' },
        flags: {
          dir: newDir || undefined,
          module: newModule || undefined,
          description: newDescription || undefined,
          recipe: newRecipe || undefined
        }
      }),
    [create, newName, newDir, newModule, newDescription, newRecipe]
  )

  const writeNew = useCallback(async () => {
    setNewConfirm(false)
    const result = await create.run({
      confirmed: true,
      values: { name: newName },
      flags: {
        dir: newDir || undefined,
        module: newModule || undefined,
        description: newDescription || undefined,
        recipe: newRecipe || undefined,
        write: true
      }
    })
    if (result.envelope?.ok) {
      toast('Repository created', 'positive')
      const data = result.envelope.data as NewData
      if (data?.target) {
        await addWorkspace(data.target)
        bump()
      }
    }
  }, [create, newName, newDir, newModule, newDescription, newRecipe, toast, addWorkspace, bump])

  return (
    <PanelShell
      featureId="init"
      title="Init & new"
      subtitle="Two ways in: initialize a manifest in an existing repository, or create a whole repository from a built-in recipe. Both preview by default."
    >
      <Tabs
        ariaLabel="Scaffolding mode"
        value={tab}
        onChange={setTab}
        items={[
          { value: 'init', label: 'bob init', badge: <Badge tone="neutral">existing repository</Badge> },
          { value: 'new', label: 'bob new', badge: <Badge tone="neutral">fresh repository</Badge> }
        ]}
      />

      {tab === 'init' ? (
        <WorkspaceGate>
          <Section
            title="Initialize a manifest"
            hint="Bob detects the stack and defaults to the matching recipe. A mismatch warns in preview and refuses --write without --force."
            actions={
              <>
                <Button variant="quiet" loading={init.pending} onClick={() => void previewInit()}>
                  Preview
                </Button>
                <Button variant="primary" icon={<Sparkles className="size-3.5" aria-hidden />} onClick={() => setInitConfirm(true)}>
                  Write bob.yaml
                </Button>
              </>
            }
          >
            <div className="grid grid-cols-2 gap-4 rounded-[8px] border border-line bg-panel px-3 py-3">
              <Field label="Project name" hint="Defaults to the directory name.">
                <TextInput value={initName} onChange={(event) => setInitName(event.target.value)} placeholder={workspace.split('/').pop() ?? 'acme-tool'} />
              </Field>
              <Field label="Module path" hint="Required by go-agent-tool; optional repository identity for stack recipes.">
                <TextInput value={initModule} onChange={(event) => setInitModule(event.target.value)} placeholder="github.com/acme/acme-tool" className="font-mono text-[12px]" />
              </Field>
              <Field label="Description" hint="One-line product description.">
                <TextInput value={initDescription} onChange={(event) => setInitDescription(event.target.value)} placeholder="Agent-ready Acme CLI" />
              </Field>
              <Field label="Recipe" hint={`Defaults to the detected stack. ${recipes.length} recipes available.`}>
                <select
                  value={initRecipe}
                  onChange={(event) => setInitRecipe(event.target.value)}
                  className="h-8 w-full rounded-[6px] border border-line-strong bg-sunken px-2 text-[12.5px] text-ink focus:border-copper focus:outline-none"
                  aria-label="Recipe"
                >
                  <option value="">detected automatically</option>
                  {recipes.map((recipe) => (
                    <option key={recipe.id} value={recipe.id}>
                      {recipe.id} · v{recipe.version} · {recipe.language}
                    </option>
                  ))}
                </select>
              </Field>
              <div className="col-span-2">
                <Toggle
                  checked={initForce}
                  onChange={setInitForce}
                  tone="danger"
                  label="--force"
                  hint="Write even when the chosen recipe does not match the detected stack."
                />
              </div>
            </div>

            {init.data?.detection ? (
              <div className="flex flex-col gap-3">
                <KeyValue
                  entries={[
                    ['primary stack', <Badge tone="info">{init.data.detection.primary}</Badge>],
                    ['detected', init.data.detection.stacks.map((stack) => (
                      <span key={stack.id} className="inline-flex items-center gap-1.5">
                        <Mono>{stack.id}</Mono>
                        <span className="text-[11px] text-ink-dim">({stack.markers.join(', ')})</span>
                      </span>
                    ))],
                    ['resolved recipe', <Badge tone="accent">{init.data.manifest.recipe}</Badge>],
                    ['target', <Mono className="break-all">{init.data.path}</Mono>],
                    ['written', <Badge tone={init.data.written ? 'positive' : 'neutral'}>{String(init.data.written)}</Badge>]
                  ]}
                />
                {init.data.detection.kind_hint ? (
                  <Callout tone="info" title={`Kind hint: ${init.data.detection.kind_hint}`}>
                    A plain Go module selects go-hygiene; only a Cobra cmd/ plus internal/cli layout selects the go-agent-tool factory.
                  </Callout>
                ) : null}
              </div>
            ) : null}

            <FailureNotice result={init.result} error={init.error} errorCode={init.errorCode} title="Init refused" />
            <WarningList warnings={init.result?.envelope?.warnings} />
            <RunMeta result={init.result} />
          </Section>
        </WorkspaceGate>
      ) : (
        <Section
          title="Create a repository"
          hint="Preview lists every artifact the recipe renders. Seed-once stack recipes never scaffold application source — initialize the application first, then run init."
          actions={
            <>
              <Button variant="quiet" loading={create.pending} onClick={() => void previewNew()} disabled={!newName}>
                Preview artifacts
              </Button>
              <Button variant="primary" icon={<Hammer className="size-3.5" aria-hidden />} disabled={!newName} onClick={() => setNewConfirm(true)}>
                Create repository
              </Button>
            </>
          }
        >
          <div className="grid grid-cols-2 gap-4 rounded-[8px] border border-line bg-panel px-3 py-3">
            <Field label="Project name" hint="Required. Also the default target directory name.">
              <TextInput value={newName} onChange={(event) => setNewName(event.target.value)} placeholder="acme-tool" />
            </Field>
            <Field label="Target directory" hint="Defaults to the project name. Use an absolute path to place it anywhere.">
              <TextInput value={newDir} onChange={(event) => setNewDir(event.target.value)} placeholder="/absolute/path/to/acme-tool" className="font-mono text-[12px]" />
            </Field>
            <Field label="Go module path" hint="Required by go-agent-tool; rejected by every other recipe.">
              <TextInput value={newModule} onChange={(event) => setNewModule(event.target.value)} placeholder="github.com/acme/acme-tool" className="font-mono text-[12px]" />
            </Field>
            <Field label="Description">
              <TextInput value={newDescription} onChange={(event) => setNewDescription(event.target.value)} placeholder="Agent-ready Acme CLI" />
            </Field>
            <Field label="Recipe" hint="Defaults to the detected stack of the target, else go-agent-tool.">
              <select
                value={newRecipe}
                onChange={(event) => setNewRecipe(event.target.value)}
                className="h-8 w-full rounded-[6px] border border-line-strong bg-sunken px-2 text-[12.5px] text-ink focus:border-copper focus:outline-none"
                aria-label="Recipe"
              >
                <option value="">default</option>
                {recipes.map((recipe) => (
                  <option key={recipe.id} value={recipe.id}>
                    {recipe.id} · v{recipe.version} · {recipe.language}
                  </option>
                ))}
              </select>
            </Field>
            <div className="flex items-end">
              <Button variant="quiet" onClick={() => void chooseWorkspace()}>
                Choose a parent folder as workspace
              </Button>
            </div>
          </div>

          {create.data?.artifacts ? (
            <div className="flex flex-col gap-2">
              <KeyValue
                entries={[
                  ['recipe', <Badge tone="accent">{create.data.manifest.recipe}</Badge>],
                  ['target', <Mono className="break-all">{create.data.target}</Mono>],
                  ['artifacts', <Mono>{create.data.artifacts.length}</Mono>],
                  ['written', <Badge tone={create.data.written ? 'positive' : 'neutral'}>{String(create.data.written)}</Badge>]
                ]}
              />
              <ul className="max-h-72 overflow-auto rounded-[8px] border border-line bg-sunken px-3 py-2">
                {create.data.artifacts.map((artifact) => (
                  <li key={artifact}>
                    <Mono className="block truncate text-ink-muted">{artifact}</Mono>
                  </li>
                ))}
              </ul>
            </div>
          ) : null}

          <FailureNotice result={create.result} error={create.error} errorCode={create.errorCode} title="New refused" />
          <WarningList warnings={create.result?.envelope?.warnings} />
          <RunMeta result={create.result} />
        </Section>
      )}

      <Modal
        open={initConfirm}
        title="Write bob.yaml?"
        description="Bob creates the human-owned manifest. It writes nothing else; the repository files come from a separate plan and apply."
        onClose={() => setInitConfirm(false)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setInitConfirm(false)}>
              Cancel
            </Button>
            <Button variant="primary" onClick={() => void writeInit()}>
              Write manifest
            </Button>
          </>
        }
      >
        <KeyValue
          entries={[
            ['workspace', <Mono className="break-all">{workspace}</Mono>],
            ['recipe', <Mono>{initRecipe || 'detected automatically'}</Mono>],
            ['force mismatch', <Badge tone={initForce ? 'danger' : 'neutral'}>{String(initForce)}</Badge>]
          ]}
        />
      </Modal>

      <Modal
        open={newConfirm}
        title="Create this repository?"
        description="Bob renders the complete recipe artifact set into the target directory and writes bob.yaml plus bob.lock."
        onClose={() => setNewConfirm(false)}
        footer={
          <>
            <Button variant="ghost" onClick={() => setNewConfirm(false)}>
              Cancel
            </Button>
            <Button variant="primary" disabled={!newName} onClick={() => void writeNew()}>
              Create repository
            </Button>
          </>
        }
      >
        <KeyValue
          entries={[
            ['name', <Mono>{newName || '—'}</Mono>],
            ['target', <Mono className="break-all">{newDir || `(defaults to ${newName || 'name'})`}</Mono>],
            ['module', <Mono>{newModule || '—'}</Mono>],
            ['recipe', <Mono>{newRecipe || 'default'}</Mono>]
          ]}
        />
      </Modal>
    </PanelShell>
  )
}

export default ScaffoldPanel
