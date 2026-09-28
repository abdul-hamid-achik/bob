import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode
} from 'react'
import { buildArgv, type ArgvValue } from '@shared/argv'
import { envelopePayload } from '@shared/envelope'
import { FEATURES, featureById, type PanelId } from '@shared/features'
import type { ActivityEntry, AppInfo, BinaryInfo, WorkspaceFiles } from '@shared/ipc'
import type {
  AppSettings,
  CheckData,
  PlanAction,
  PlanData,
  RecipeRef,
  RunResult,
  WorkspaceInfo
} from '@shared/types'
import { DEFAULT_SETTINGS } from '@shared/types'
import { bridge, isDemoActive, setForcedDemo } from './bridge'
import type { Tone } from '../components/ui'

export interface PlanSnapshot {
  digest: string
  digestVersion: number
  recipe: RecipeRef | null
  actions: PlanAction[]
  conflictCount: number
  lockChanged: boolean
  capturedAt: string
  source: 'plan' | 'check'
  workspace: string
}

export interface RunOptions {
  values?: Record<string, ArgvValue | undefined>
  flags?: Record<string, ArgvValue | undefined>
  argv?: string[]
  cwd?: string
  confirmed?: boolean
  json?: boolean
  timeoutMs?: number
}

export interface Toast {
  id: string
  message: string
  tone: Tone
}

export interface InspectorState {
  open: boolean
  title: string
  result: RunResult | null
}

export interface StoreValue {
  ready: boolean
  demo: boolean
  bootstrapError: string | null

  appInfo: AppInfo | null
  binary: BinaryInfo | null
  refreshBinary: () => Promise<BinaryInfo | null>
  repoRoot: string | null

  settings: AppSettings
  updateSettings: (patch: Partial<AppSettings>) => Promise<void>

  workspaces: WorkspaceInfo[]
  workspace: string
  selectWorkspace: (path: string) => Promise<void>
  addWorkspace: (path: string) => Promise<void>
  chooseWorkspace: () => Promise<void>
  forgetWorkspace: (path: string) => Promise<void>
  reloadWorkspaces: () => Promise<WorkspaceInfo[]>

  files: WorkspaceFiles | null
  reloadFiles: () => Promise<void>
  saveManifest: (content: string) => Promise<{ written: boolean; error: string | null }>

  panel: PanelId
  navParams: Record<string, string>
  navigate: (panel: PanelId, params?: Record<string, string>) => void

  plan: PlanSnapshot | null
  recordPlan: (snapshot: PlanSnapshot | null) => void

  run: (featureId: string, options?: RunOptions) => Promise<RunResult>
  running: Record<string, boolean>

  activity: ActivityEntry[]
  reloadActivity: () => Promise<void>
  clearActivity: () => Promise<{ cleared: number }>

  inspector: InspectorState
  showInspector: (title: string, result: RunResult | null) => void
  closeInspector: () => void

  paletteOpen: boolean
  setPaletteOpen: (open: boolean) => void

  toasts: Toast[]
  toast: (message: string, tone?: Tone) => void
  dismissToast: (id: string) => void

  revision: number
  bump: () => void
}

const StoreContext = createContext<StoreValue | null>(null)

function panelFromHash(): PanelId {
  const hash = window.location.hash.replace(/^#\/?/, '')
  const candidate = hash.split('?')[0] as PanelId
  return FEATURES.some((feature) => feature.panel === candidate) ? candidate : 'overview'
}

function paramsFromHash(): Record<string, string> {
  const hash = window.location.hash.replace(/^#\/?/, '')
  const query = hash.split('?')[1]
  const out: Record<string, string> = {}
  if (!query) return out
  for (const pair of query.split('&')) {
    const [key, value = ''] = pair.split('=')
    if (key) out[decodeURIComponent(key)] = decodeURIComponent(value)
  }
  return out
}

function planFromData(data: PlanData | null, workspace: string, source: 'plan' | 'check'): PlanSnapshot | null {
  if (!data || !data.plan_digest) return null
  return {
    digest: data.plan_digest,
    digestVersion: data.plan_digest_version ?? 1,
    recipe: data.recipe ?? null,
    actions: data.actions ?? [],
    conflictCount: data.conflict_count ?? 0,
    lockChanged: Boolean(data.lock_changed),
    capturedAt: new Date().toISOString(),
    source,
    workspace
  }
}

export function StoreProvider({ children }: { children: ReactNode }): JSX.Element {
  const [ready, setReady] = useState(false)
  const [bootstrapError, setBootstrapError] = useState<string | null>(null)
  const [appInfo, setAppInfo] = useState<AppInfo | null>(null)
  const [binary, setBinary] = useState<BinaryInfo | null>(null)
  const [repoRoot, setRepoRoot] = useState<string | null>(null)
  const [settings, setSettings] = useState<AppSettings>(DEFAULT_SETTINGS)
  const [workspaces, setWorkspaces] = useState<WorkspaceInfo[]>([])
  const [workspace, setWorkspace] = useState('')
  const [files, setFiles] = useState<WorkspaceFiles | null>(null)
  const [panel, setPanel] = useState<PanelId>(() => panelFromHash())
  const [navParams, setNavParams] = useState<Record<string, string>>(() => paramsFromHash())
  const [plan, setPlan] = useState<PlanSnapshot | null>(null)
  const [running, setRunning] = useState<Record<string, boolean>>({})
  const [activity, setActivity] = useState<ActivityEntry[]>([])
  const [inspector, setInspector] = useState<InspectorState>({ open: false, title: '', result: null })
  const [paletteOpen, setPaletteOpen] = useState(false)
  const [toasts, setToasts] = useState<Toast[]>([])
  const [revision, setRevision] = useState(0)
  const workspaceRef = useRef('')
  workspaceRef.current = workspace

  const demo = isDemoActive()

  const toast = useCallback((message: string, tone: Tone = 'neutral') => {
    const id = `${Date.now()}-${Math.random().toString(36).slice(2, 7)}`
    setToasts((prev) => [...prev.slice(-3), { id, message, tone }])
    setTimeout(() => setToasts((prev) => prev.filter((entry) => entry.id !== id)), 4200)
  }, [])

  const dismissToast = useCallback((id: string) => {
    setToasts((prev) => prev.filter((entry) => entry.id !== id))
  }, [])

  const reloadActivity = useCallback(async () => {
    try {
      setActivity(await bridge().activity())
    } catch {
      /* the ledger is best-effort */
    }
  }, [])

  const refreshBinary = useCallback(async () => {
    try {
      const info = await bridge().resolveBinary()
      setBinary(info)
      const root = await bridge().repoRoot()
      setRepoRoot(root.path)
      return info
    } catch (error) {
      setBootstrapError(error instanceof Error ? error.message : String(error))
      return null
    }
  }, [])

  const reloadWorkspaces = useCallback(async () => {
    try {
      const list = await bridge().listWorkspaces()
      setWorkspaces(list)
      return list
    } catch {
      return []
    }
  }, [])

  const loadFiles = useCallback(async (path: string) => {
    if (!path) {
      setFiles(null)
      return
    }
    try {
      setFiles(await bridge().readWorkspace(path))
    } catch {
      setFiles(null)
    }
  }, [])

  /* ---------------------------------------------------------- bootstrap --- */
  useEffect(() => {
    let cancelled = false
    void (async () => {
      try {
        const api = bridge()
        const [info, loaded, list] = await Promise.all([api.appInfo(), api.getSettings(), api.listWorkspaces()])
        if (cancelled) return
        setAppInfo(info)
        setSettings(loaded)
        setForcedDemo(loaded.demoMode)
        setWorkspaces(list)
        const initial = loaded.defaultWorkspace || list[0]?.path || ''
        setWorkspace(initial)
        await loadFiles(initial)
        const resolved = await api.resolveBinary()
        const root = await api.repoRoot()
        if (cancelled) return
        setBinary(resolved)
        setRepoRoot(root.path)
        await reloadActivity()
        if (!initial) {
          const recent = list[0]?.path
          if (recent) {
            setWorkspace(recent)
            await loadFiles(recent)
          }
        }
        setReady(true)
      } catch (error) {
        if (!cancelled) {
          setBootstrapError(error instanceof Error ? error.message : String(error))
          setReady(true)
        }
      }
    })()
    return () => {
      cancelled = true
    }
  }, [loadFiles, reloadActivity])

  /* -------------------------------------------------------------- theme --- */
  useEffect(() => {
    const root = document.documentElement
    const apply = (mode: AppSettings['theme']): void => {
      const light = mode === 'light' || (mode === 'system' && window.matchMedia('(prefers-color-scheme: light)').matches)
      root.classList.toggle('light', light)
      root.classList.toggle('dark', !light)
    }
    apply(settings.theme)
    const media = window.matchMedia('(prefers-color-scheme: light)')
    const listener = (): void => apply(settings.theme)
    media.addEventListener('change', listener)
    return () => media.removeEventListener('change', listener)
  }, [settings.theme])

  /* --------------------------------------------------------------- hash --- */
  useEffect(() => {
    const onHashChange = (): void => {
      setPanel(panelFromHash())
      setNavParams(paramsFromHash())
    }
    window.addEventListener('hashchange', onHashChange)
    return () => window.removeEventListener('hashchange', onHashChange)
  }, [])

  const navigate = useCallback((next: PanelId, params?: Record<string, string>) => {
    const query = params && Object.keys(params).length > 0
      ? `?${Object.entries(params).map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`).join('&')}`
      : ''
    window.location.hash = `/${next}${query}`
    setPanel(next)
    setNavParams(params ?? {})
  }, [])

  /* ----------------------------------------------------------- settings --- */
  const updateSettings = useCallback(
    async (patch: Partial<AppSettings>) => {
      const next = await bridge().setSettings(patch)
      setSettings(next)
      if ('demoMode' in patch) {
        setForcedDemo(Boolean(patch.demoMode))
        window.location.reload()
        return
      }
      if ('bobBinaryPath' in patch) await refreshBinary()
    },
    [refreshBinary]
  )

  /* --------------------------------------------------------- workspaces --- */
  const selectWorkspace = useCallback(
    async (path: string) => {
      setWorkspace(path)
      setPlan(null)
      await loadFiles(path)
      await bridge().setSettings({ defaultWorkspace: path })
      setSettings(await bridge().getSettings())
      await refreshBinary()
    },
    [loadFiles, refreshBinary]
  )

  const addWorkspace = useCallback(
    async (path: string) => {
      const info = await bridge().addWorkspace(path)
      await reloadWorkspaces()
      await selectWorkspace(info.path)
      toast(`Added ${info.name}`, 'positive')
    },
    [reloadWorkspaces, selectWorkspace, toast]
  )

  const chooseWorkspace = useCallback(async () => {
    const chosen = await bridge().chooseDirectory()
    if (!chosen) return
    await reloadWorkspaces()
    await selectWorkspace(chosen)
  }, [reloadWorkspaces, selectWorkspace])

  const forgetWorkspace = useCallback(
    async (path: string) => {
      await bridge().forgetWorkspace(path)
      const list = await reloadWorkspaces()
      setWorkspaces(list)
      if (workspaceRef.current === path) {
        const next = list[0]?.path ?? ''
        setWorkspace(next)
        await loadFiles(next)
      }
    },
    [loadFiles, reloadWorkspaces]
  )

  const reloadFiles = useCallback(async () => {
    await loadFiles(workspaceRef.current)
  }, [loadFiles])

  const saveManifest = useCallback(
    async (content: string) => {
      const result = await bridge().writeManifest(workspaceRef.current, content)
      if (result.written) await loadFiles(workspaceRef.current)
      return result
    },
    [loadFiles]
  )

  /* ---------------------------------------------------------------- run --- */
  const run = useCallback(
    async (featureId: string, options: RunOptions = {}): Promise<RunResult> => {
      const feature = featureById(featureId)
      const cwd = options.cwd ?? workspaceRef.current
      const argv =
        options.argv ??
        (feature
          ? buildArgv(feature, {
              workspace: cwd,
              values: options.values,
              flags: options.flags,
              json: options.json
            })
          : [])
      setRunning((prev) => ({ ...prev, [featureId]: true }))
      try {
        const result = await bridge().run({
          featureId,
          argv,
          cwd,
          confirmed: options.confirmed,
          timeoutMs: options.timeoutMs
        })
        void reloadActivity()
        if (featureId === 'plan') {
          const data = envelopePayload<PlanData>(result.envelope)
          setPlan(planFromData(data, cwd, 'plan'))
        }
        if (featureId === 'check') {
          const data = envelopePayload<CheckData>(result.envelope)
          setPlan(planFromData(data?.plan ?? null, cwd, 'check'))
        }
        return result
      } finally {
        setRunning((prev) => ({ ...prev, [featureId]: false }))
      }
    },
    [reloadActivity]
  )

  const bump = useCallback(() => setRevision((prev) => prev + 1), [])

  const value = useMemo<StoreValue>(
    () => ({
      ready,
      demo,
      bootstrapError,
      appInfo,
      binary,
      refreshBinary,
      repoRoot,
      settings,
      updateSettings,
      workspaces,
      workspace,
      selectWorkspace,
      addWorkspace,
      chooseWorkspace,
      forgetWorkspace,
      reloadWorkspaces,
      files,
      reloadFiles,
      saveManifest,
      panel,
      navParams,
      navigate,
      plan,
      recordPlan: setPlan,
      run,
      running,
      activity,
      reloadActivity,
      clearActivity: async () => {
        const outcome = await bridge().clearActivity()
        await reloadActivity()
        return outcome
      },
      inspector,
      showInspector: (title, result) => setInspector({ open: true, title, result }),
      closeInspector: () => setInspector((prev) => ({ ...prev, open: false })),
      paletteOpen,
      setPaletteOpen,
      toasts,
      toast,
      dismissToast,
      revision,
      bump
    }),
    [
      ready, demo, bootstrapError, appInfo, binary, refreshBinary, repoRoot, settings, updateSettings,
      workspaces, workspace, selectWorkspace, addWorkspace, chooseWorkspace, forgetWorkspace, reloadWorkspaces,
      files, reloadFiles, saveManifest, panel, navParams, navigate, plan, run, running, activity,
      reloadActivity, inspector, paletteOpen, toasts, toast, dismissToast, revision, bump
    ]
  )

  return <StoreContext.Provider value={value}>{children}</StoreContext.Provider>
}

export function useStore(): StoreValue {
  const value = useContext(StoreContext)
  if (!value) throw new Error('useStore must be used inside StoreProvider')
  return value
}
