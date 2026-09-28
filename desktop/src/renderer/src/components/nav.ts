import {
  Activity,
  BarChart3,
  BookMarked,
  BookOpen,
  Boxes,
  Cable,
  Compass,
  Crosshair,
  Eraser,
  FileCode2,
  FolderTree,
  GraduationCap,
  Grid3x3,
  Hammer,
  Info,
  LayoutDashboard,
  ListChecks,
  Lock,
  Monitor,
  Search,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Sparkles,
  Stethoscope,
  Terminal,
  ArrowUpCircle,
  GitBranch,
  type LucideIcon
} from 'lucide-react'
import type { PanelId } from '@shared/features'

export interface NavItem {
  panel: PanelId
  label: string
  icon: LucideIcon
  /** Registry feature that best represents the panel, used for its header. */
  featureId: string
  hint: string
}

export interface NavSection {
  id: string
  label: string
  items: NavItem[]
}

export const NAV_SECTIONS: NavSection[] = [
  {
    id: 'workspace',
    label: 'Workspace',
    items: [
      { panel: 'overview', label: 'Overview', icon: LayoutDashboard, featureId: 'overview', hint: 'Convergence state and the next move' },
      { panel: 'plan', label: 'Plan', icon: ListChecks, featureId: 'plan', hint: 'Classified actions and the plan digest' },
      { panel: 'check', label: 'Check', icon: ShieldCheck, featureId: 'check', hint: 'Drift and convergence gate' },
      { panel: 'apply', label: 'Apply', icon: Sparkles, featureId: 'apply', hint: 'Write one conflict-free reviewed plan' },
      { panel: 'upgrade', label: 'Upgrade', icon: ArrowUpCircle, featureId: 'upgrade', hint: 'Migrate bob.lock to the current recipe' },
      { panel: 'remove', label: 'Remove', icon: Eraser, featureId: 'remove', hint: 'Stop Bob managing this workspace' }
    ]
  },
  {
    id: 'ownership',
    label: 'Ownership',
    items: [
      { panel: 'path', label: 'Path', icon: Crosshair, featureId: 'path', hint: 'Classify one exact path or a batch' },
      { panel: 'context', label: 'Context', icon: BookOpen, featureId: 'context', hint: 'Bounded workspace contract' },
      { panel: 'lock', label: 'Lock ledger', icon: Lock, featureId: 'lock-inspect', hint: 'Recipe version and whole-file digests' },
      { panel: 'manifest', label: 'Manifest', icon: FileCode2, featureId: 'manifest-view', hint: 'Read, edit, and validate bob.yaml' },
      { panel: 'files', label: 'Files', icon: FolderTree, featureId: 'files', hint: 'Owned, seeded, and unmanaged paths' },
      { panel: 'recipes', label: 'Recipes', icon: Boxes, featureId: 'recipe-list', hint: 'The embedded recipe catalog' },
      { panel: 'scaffold', label: 'Init & new', icon: Sparkles, featureId: 'init', hint: 'Detect a stack, seed a manifest, scaffold' }
    ]
  },
  {
    id: 'guidance',
    label: 'Guidance',
    items: [
      { panel: 'playbooks', label: 'Playbooks', icon: Compass, featureId: 'playbook-list', hint: 'Closed procedures with typed inputs' },
      { panel: 'learn', label: 'Agent brief', icon: GraduationCap, featureId: 'learn', hint: 'Commands, exit codes, invariants' },
      { panel: 'explain', label: 'Boundaries', icon: Info, featureId: 'explain', hint: 'What Bob owns and refuses to own' }
    ]
  },
  {
    id: 'environment',
    label: 'Environment',
    items: [
      { panel: 'doctor', label: 'Doctor', icon: Stethoscope, featureId: 'doctor', hint: 'Required and optional tool probes' },
      { panel: 'inspect', label: 'Inspect', icon: Search, featureId: 'inspect', hint: 'Bob state and offline binary availability' },
      { panel: 'settings', label: 'Bob config', icon: SlidersHorizontal, featureId: 'config-show', hint: 'XDG paths, settings, telemetry' },
      { panel: 'stats', label: 'Stats', icon: BarChart3, featureId: 'stats', hint: 'Aggregate opt-in local usage' },
      { panel: 'studio', label: 'Studio', icon: Monitor, featureId: 'studio', hint: 'Read-only board, native and TUI' },
      { panel: 'repository', label: 'Repository', icon: GitBranch, featureId: 'repository', hint: 'Read-only git state' }
    ]
  },
  {
    id: 'console',
    label: 'Console',
    items: [
      { panel: 'mcp', label: 'MCP', icon: Cable, featureId: 'mcp-serve', hint: 'Drive the nine read-only tools' },
      { panel: 'console', label: 'Command console', icon: Terminal, featureId: 'console', hint: 'Run any Bob command, inspect the envelope' },
      { panel: 'tasks', label: 'Dev gates', icon: Hammer, featureId: 'tasks', hint: 'Taskfile gates with streamed output' },
      { panel: 'docs', label: 'Docs', icon: BookMarked, featureId: 'docs', hint: 'Published reference pages and guides' },
      { panel: 'activity', label: 'Activity', icon: Activity, featureId: 'activity', hint: 'Local ledger of every invocation' },
      { panel: 'features', label: 'Features', icon: Grid3x3, featureId: 'features', hint: 'Coverage matrix with live probes' },
      { panel: 'app-settings', label: 'Settings', icon: Settings, featureId: 'app-settings', hint: 'Binary, workspace, theme, safety' }
    ]
  }
]

export const NAV_BY_PANEL: Map<PanelId, NavItem> = new Map(
  NAV_SECTIONS.flatMap((section) => section.items).map((item) => [item.panel, item])
)

export const ALL_NAV_ITEMS: NavItem[] = NAV_SECTIONS.flatMap((section) => section.items)
