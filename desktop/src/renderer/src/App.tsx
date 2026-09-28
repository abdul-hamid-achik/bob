import { useEffect, useState } from 'react'
import { CommandPalette, Inspector } from './components/overlays'
import { NavRail, StatusBar, Titlebar, Toaster } from './components/chrome'
import { PANELS } from './panels'
import { StoreProvider, useStore } from './state/store'
import { bridge } from './state/bridge'
import { Button, Callout } from './components/ui'

function Shell(): JSX.Element {
  const { panel, ready, bootstrapError, inspector, closeInspector, showInspector, setPaletteOpen, navigate, workspace, chooseWorkspace, reloadFiles, bump, settings, updateSettings } =
    useStore()
  const [inspectorOpen, setInspectorOpen] = useState(false)

  useEffect(() => {
    const unsubscribe = bridge().onCommand((command) => {
      switch (command) {
        case 'open-workspace':
          void chooseWorkspace()
          break
        case 'command-palette':
          setPaletteOpen(true)
          break
        case 'run-plan':
          navigate('plan')
          bump()
          break
        case 'run-check':
          navigate('check')
          bump()
          break
        case 'refresh':
          void reloadFiles()
          bump()
          break
        case 'toggle-theme':
          void updateSettings({ theme: settings.theme === 'light' ? 'dark' : 'light' })
          break
        case 'focus-console':
          navigate('console')
          break
      }
    })
    return unsubscribe
  }, [bump, chooseWorkspace, navigate, reloadFiles, setPaletteOpen, settings.theme, updateSettings])

  useEffect(() => {
    if (inspector.open) setInspectorOpen(true)
  }, [inspector.open])

  const Panel = PANELS[panel] ?? PANELS.overview

  return (
    <div className="flex h-dvh flex-col overflow-hidden bg-canvas">
      <Titlebar />
      <div className="flex min-h-0 flex-1">
        <NavRail
          inspectorOpen={inspectorOpen}
          onToggleInspector={() => {
            setInspectorOpen((prev) => {
              const next = !prev
              if (next && !inspector.result) showInspector('Command output', null)
              return next
            })
          }}
        />
        <main className="flex min-w-0 flex-1 flex-col">
          {!ready ? (
            <div className="flex flex-1 items-center justify-center">
              <p className="text-[12.5px] text-ink-dim">Starting Bob Console…</p>
            </div>
          ) : (
            <>
              {bootstrapError ? (
                <div className="border-b border-clay/40 bg-clay/10 px-5 py-2">
                  <Callout tone="danger" title="Bootstrap problem">{bootstrapError}</Callout>
                </div>
              ) : null}
              {!workspace ? (
                <div className="border-b border-line bg-panel px-5 py-2">
                  <div className="flex items-center gap-3">
                    <p className="text-[12px] text-ink-muted">
                      No workspace selected. Every Bob command answers for one repository at a time.
                    </p>
                    <Button size="sm" variant="primary" className="ml-auto" onClick={() => void chooseWorkspace()}>
                      Choose workspace
                    </Button>
                  </div>
                </div>
              ) : null}
              <Panel />
            </>
          )}
        </main>
        {inspectorOpen ? (
          <Inspector
            onClose={() => {
              setInspectorOpen(false)
              closeInspector()
            }}
          />
        ) : null}
      </div>
      <StatusBar />
      <CommandPalette />
      <Toaster />
    </div>
  )
}

export default function App(): JSX.Element {
  return (
    <StoreProvider>
      <Shell />
    </StoreProvider>
  )
}
