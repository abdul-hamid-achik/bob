import type { ComponentType } from 'react'
import type { PanelId } from '@shared/features'
import OverviewPanel from './OverviewPanel'
import PlanPanel from './PlanPanel'
import CheckPanel from './CheckPanel'
import ApplyPanel from './ApplyPanel'
import UpgradePanel from './UpgradePanel'
import RemovePanel from './RemovePanel'
import PathPanel from './PathPanel'
import ContextPanel from './ContextPanel'
import LockPanel from './LockPanel'
import ManifestPanel from './ManifestPanel'
import FilesPanel from './FilesPanel'
import RecipesPanel from './RecipesPanel'
import ScaffoldPanel from './ScaffoldPanel'
import PlaybooksPanel from './PlaybooksPanel'
import LearnPanel from './LearnPanel'
import ExplainPanel from './ExplainPanel'
import DoctorPanel from './DoctorPanel'
import InspectPanel from './InspectPanel'
import ConfigPanel from './ConfigPanel'
import StatsPanel from './StatsPanel'
import StudioPanel from './StudioPanel'
import RepositoryPanel from './RepositoryPanel'
import McpPanel from './McpPanel'
import ConsolePanel from './ConsolePanel'
import TasksPanel from './TasksPanel'
import DocsPanel from './DocsPanel'
import ActivityPanel from './ActivityPanel'
import FeaturesPanel from './FeaturesPanel'
import AppSettingsPanel from './AppSettingsPanel'

export const PANELS: Record<PanelId, ComponentType> = {
  overview: OverviewPanel,
  plan: PlanPanel,
  check: CheckPanel,
  apply: ApplyPanel,
  upgrade: UpgradePanel,
  remove: RemovePanel,
  path: PathPanel,
  context: ContextPanel,
  lock: LockPanel,
  manifest: ManifestPanel,
  files: FilesPanel,
  recipes: RecipesPanel,
  scaffold: ScaffoldPanel,
  playbooks: PlaybooksPanel,
  learn: LearnPanel,
  explain: ExplainPanel,
  doctor: DoctorPanel,
  inspect: InspectPanel,
  settings: ConfigPanel,
  stats: StatsPanel,
  studio: StudioPanel,
  repository: RepositoryPanel,
  mcp: McpPanel,
  console: ConsolePanel,
  tasks: TasksPanel,
  docs: DocsPanel,
  activity: ActivityPanel,
  features: FeaturesPanel,
  'app-settings': AppSettingsPanel
}

export { OverviewPanel }
