import type { ReactNode } from 'react'
import { FolderPlus } from 'lucide-react'
import { Button, EmptyState } from './ui'
import { PanelHeader } from './chrome'
import { useStore } from '../state/store'

export function PanelShell({
  featureId,
  title,
  subtitle,
  actions,
  children
}: {
  featureId: string
  title?: string
  subtitle?: string
  actions?: ReactNode
  children: ReactNode
}): JSX.Element {
  return (
    <>
      <PanelHeader featureId={featureId} title={title} subtitle={subtitle} actions={actions} />
      <div className="flex min-h-0 flex-1 flex-col gap-5 overflow-y-auto px-5 py-4">{children}</div>
    </>
  )
}

/** Blocks a panel that needs a workspace and offers the way to pick one. */
export function WorkspaceGate({ children }: { children: ReactNode }): JSX.Element {
  const { workspace, chooseWorkspace, addWorkspace } = useStore()
  if (workspace) return <>{children}</>
  return (
    <div className="flex flex-1 items-center justify-center p-8">
      <EmptyState
        title="No workspace selected"
        hint="Bob commands answer for one repository at a time. Choose a folder that contains a bob.yaml — or any repository you want Bob to inspect — and every panel will drive that workspace."
        action={
          <div className="flex gap-2">
            <Button variant="primary" icon={<FolderPlus className="size-3.5" aria-hidden />} onClick={() => void chooseWorkspace()}>
              Choose workspace
            </Button>
            <Button
              variant="quiet"
              onClick={async () => {
                const path = window.prompt('Absolute path to a workspace')
                if (path) await addWorkspace(path)
              }}
            >
              Type a path
            </Button>
          </div>
        }
      />
    </div>
  )
}

export function PanelBody({ children, className }: { children: ReactNode; className?: string }): JSX.Element {
  return <div className={`flex flex-col gap-5 ${className ?? ''}`}>{children}</div>
}
