/**
 * Typed contract for every Bob surface Bob Console drives.
 *
 * Shapes mirror the versioned JSON envelope emitted by `bob <command> --json`
 * (schema_version 1) and the newline-delimited JSON-RPC MCP projection.
 */

export const ENVELOPE_SCHEMA_VERSION = 1

export type ExitCode = 0 | 1 | 2 | 3 | 4 | 5

export const EXIT_CODE_MEANING: Record<number, string> = {
  0: 'success; plan always exits 0 even when conflicts exist',
  1: 'unclassified command failure, or doctor found a required tool missing',
  2: 'apply or upgrade refused a conflicted plan',
  3: 'check found drift with no ownership conflict',
  4: 'invalid input: missing or invalid manifest, flag, argument, or workspace',
  5: 'guarded apply or upgrade refused because the reviewed plan digest changed'
}

export type ErrorCode =
  | 'command_failed'
  | 'conflicts'
  | 'input_invalid'
  | 'manifest_invalid'
  | 'missing_manifest'
  | 'plan_digest_mismatch'
  | 'workspace_invalid'
  | string

export interface BobError {
  code: ErrorCode
  message: string
}

export interface Envelope<T = unknown> {
  schema_version: number
  ok: boolean
  command: string
  data: T
  warnings: string[]
  next_actions: string[]
}

export interface RecipeRef {
  id: string
  version: number
}

export type ActionKind = 'create' | 'adopt' | 'unchanged' | 'update' | 'conflict'

export type ConflictFamily = 'contract' | 'convergence' | 'ownership' | string

export interface PlanAction {
  path: string
  kind: ActionKind
  code: string
  family?: ConflictFamily
  reason?: string
  current_sha256?: string
  desired_sha256?: string
  locked_sha256?: string
  current_mode?: number
  desired_mode?: number
  desired_preview?: string
  current_preview?: string
  diff?: string
}

export interface LockFileEntry {
  path: string
  sha256: string
}

export interface DesiredLock {
  schema_version: number
  recipe: RecipeRef
  files: LockFileEntry[]
}

export interface Truncation {
  profile?: string
  byte_limit: number
  truncated: boolean
  omitted: Record<string, unknown>
}

export interface PlanData {
  schema_version: number
  recipe: RecipeRef
  actions: PlanAction[]
  conflict_count: number
  lock_changed: boolean
  desired_lock: DesiredLock
  plan_digest_version: number
  plan_digest: string
  truncation?: Truncation
}

export interface CheckData {
  clean: boolean
  plan: PlanData
  plan_digest: string
  plan_digest_version: number
  error?: BobError
  result?: unknown
}

export interface NextCheck {
  argv: string[]
}

export interface ApplyData {
  schema_version: number
  plan_digest_version: number
  applied_plan_digest: string
  written: string[]
  adopted: string[]
  unchanged: string[]
  written_count: number
  adopted_count: number
  unchanged_count: number
  lock_written: boolean
  converged_after_apply: boolean
  next_check: NextCheck
  truncation?: Truncation
  error?: BobError
  result?: Omit<ApplyData, 'error' | 'result'>
}

export interface UpgradeData {
  from_version: number
  to_version: number
  recipe: string
  applied: boolean
  actions: number
  error?: BobError
  result?: unknown
}

export interface RemoveResult {
  removed: string[]
  skipped: string[]
  conflicts: unknown[]
  lock_removed: boolean
}

export interface RemoveData extends Partial<RemoveResult> {
  error?: BobError
  result?: RemoveResult
}

export interface VersionData {
  name: string
  version: string
  commit: string
  date: string
}

export interface ExplainData {
  schema_version: number
  product: string
  owns: string[]
  does_not_own: string[]
  recipe: string[]
}

export interface RecipeSummary {
  id: string
  version: number
  description: string
  language: string
  stacks: string[]
  surfaces: string[]
  seeded_paths: string[]
  ownership_note: string
}

export interface DoctorCheck {
  name: string
  command: string
  required: boolean
  found: boolean
  usable: boolean
  path: string
  version: string
  note: string
}

export interface DoctorData {
  ready: boolean
  degraded: boolean
  checks: DoctorCheck[]
}

export interface DetectedStack {
  id: string
  markers: string[]
  kind_hint?: string
  module?: string
}

export interface ManifestProduct {
  name: string
  module: string
  description: string
  visibility: string
  license: string
}

export interface ManifestSurfaces {
  cli: boolean
  json: boolean
  mcp: boolean
  studio: boolean
}

export interface ManifestIntegrations {
  code_structure: string
  semantic_search: string
  terminal_verification: string
  browser_verification: string
  secrets: string
  artifacts: string
}

export interface ManifestDistribution {
  github_actions: boolean
  goreleaser: boolean
  homebrew: boolean
  docs: string
}

export interface ManifestOwnership {
  release?: string[]
  [key: string]: unknown
}

export interface Manifest {
  schema_version: number
  recipe: string
  product: ManifestProduct
  runtime: { language: string; kind: string }
  surfaces: ManifestSurfaces
  integrations: ManifestIntegrations
  distribution: ManifestDistribution
  ownership: ManifestOwnership
}

export interface InitData {
  detection: {
    stacks: DetectedStack[]
    primary: string
    signals?: string[]
    kind_hint?: string
    module?: string
  }
  manifest: Manifest
  path: string
  written: boolean
  error?: BobError
}

export interface NewData {
  artifacts: string[]
  detection: Record<string, unknown>
  manifest: Manifest
  target: string
  written: boolean
  error?: BobError
}

export type PathClassification =
  | 'managed'
  | 'seed'
  | 'unmanaged'
  | 'reserved'
  | 'extension_point'
  | string

export interface TypedAction {
  id: string
  kind: string
  effect: 'read_only' | 'repository_mutation' | 'subprocess' | string
  reason?: string
  reason_code?: string
  cwd: string
  argv: string[]
  requires_explicit_authority: boolean
  blocked_by: string[]
}

export interface ArtifactRef {
  id: string
  roles: string[]
  capability_ids: string[]
}

export interface ExtensionPoint {
  id: string
  path?: string
  kind?: string
  summary?: string
  [key: string]: unknown
}

export interface PathData {
  schema_version: number
  workspace: string
  path: string
  exists: boolean
  classification: PathClassification
  state: string
  human_edit_effect: string
  ownership: {
    recipe?: RecipeRef
    locked_sha256?: string
    current_sha256?: string
    desired_sha256?: string
    [key: string]: unknown
  }
  plan_action?: { kind: ActionKind; code: string }
  artifact?: ArtifactRef
  extension_points: ExtensionPoint[]
  related_playbooks: string[]
  notices: string[]
  actions: TypedAction[]
  truncation?: Truncation
}

export interface PathBatchData {
  schema_version: number
  workspace: string
  results: PathData[]
}

export interface CapabilityFacet {
  id: string
  category?: string
  selection: string
  materialization: string
  availability: string
  verification: string
  summary?: string
  evidence?: { manifest_fields?: string[]; [key: string]: unknown }
}

export interface Invariant {
  id: string
  statement: string
}

export interface PlaybookSummary {
  id: string
  title: string
  applicable: boolean
  available: boolean
  blocked_by: string[]
  required_inputs: string[]
  scope_class: string
  risk: 'low' | 'medium' | 'high' | string
}

export interface ContextArtifact {
  id: string
  path: string
  roles: string[]
  ownership: string
  capability_ids: string[]
}

export interface ContextData {
  schema_version: number
  profile: 'compact' | 'standard' | 'full' | string
  workspace: string
  contract_digest: string
  context_digest: string
  recipe: RecipeRef
  product: { name: string; module?: string; runtime: string; kind: string }
  repository: {
    state: string
    clean: boolean
    lock_changed: boolean
    lock_exists: boolean
    conflict_count: number
    conflict_class: string
    conflict_family_counts: Record<string, number>
    action_counts: Record<ActionKind, number>
    managed_files: number
    plan_digest_version: number
    plan_digest: string
  }
  capabilities: CapabilityFacet[]
  entry_points: ExtensionPoint[]
  extension_points: ExtensionPoint[]
  invariants: Invariant[]
  playbooks: PlaybookSummary[]
  artifacts?: ContextArtifact[]
  notices: string[]
  actions: TypedAction[]
  truncation: Truncation
}

export interface PlaybookInput {
  name: string
  required: boolean
  type: string
  validation: string
  enum?: string[]
}

export interface PlaybookStep {
  id: string
  kind: string
  effect: 'read_only' | 'repository_mutation' | 'subprocess' | string
  summary: string
  paths: string[]
  argv: string[]
  depends_on: string[]
  requires_explicit_authority: boolean
  success_condition: string
  blocked_by: string[]
}

export interface Playbook {
  id: string
  title: string
  purpose: string
  applicable: boolean
  available: boolean
  blocked_by: string[]
  scope_class: string
  risk: string
  inputs: PlaybookInput[]
  preconditions: string[]
  boundary: { create: string[]; modify: string[]; forbidden: string[] }
  steps: PlaybookStep[]
  verification_hints: string[]
  failure_modes: string[]
  capability_ids: string[]
  extension_point_ids: string[]
}

export interface PlaybookListData {
  schema_version: number
  workspace: string
  recipe: RecipeRef
  playbooks: PlaybookSummary[]
  truncation?: Truncation
}

export interface PlaybookShowData {
  schema_version: number
  workspace: string
  recipe: RecipeRef
  observations: string[]
  playbook: Playbook
  values?: Record<string, string>
  truncation?: Truncation
}

export interface IntegrationProbe {
  state: string
  cwd: string
  argv: string[]
  [key: string]: unknown
}

export interface IntegrationStatus {
  name: string
  selected: boolean
  available: boolean
  binary_path: string
  probe: IntegrationProbe
  index: { state: string; [key: string]: unknown }
}

export interface InspectData {
  schema_version: number
  workspace: string
  repository: {
    state: string
    manifest_path: string
    ready: boolean
    converged: boolean
    lock_changed: boolean
    managed_files: number
    conflict_count: number
    actions: Record<ActionKind, number>
    error?: string
  }
  integrations: IntegrationStatus[]
  degraded: boolean
  warnings: string[]
  next_actions: TypedAction[]
}

export interface ConfigShowData {
  config_exists: boolean
  config_file: string
  cache_dir: string
  data_dir: string
  state_dir: string
  telemetry_destination: string
  settings: {
    schema_version: number
    telemetry: { enabled: boolean; retention_days: number; max_events_per_day: number }
  }
}

export interface ConfigInitData {
  path: string
  settings: ConfigShowData['settings']
  written: boolean
}

export interface StatsOperation {
  operation: string
  events: number
  successes: number
  failures: number
  conflict_events: number
  drift_events: number
  duration_ms: number
  actions: Record<string, number>
}

export interface StatsData {
  enabled: boolean
  local_only: boolean
  selection: string
  stats: {
    schema_version: number
    since: string
    until: string
    workspace_id: string
    events: number
    successes: number
    failures: number
    conflict_events: number
    drift_events: number
    duration_ms: number
    actions: Record<string, number>
    skipped: number
    by_operation: StatsOperation[]
  }
  error?: BobError
}

export interface LearnCommand {
  name: string
  purpose: string
  json: boolean
  mutates: boolean
}

export interface LearnData {
  schema_version: number
  product: string
  summary: string
  boundaries: string[]
  commands: LearnCommand[]
  docs: { site: string; reference: string; agents: string }
  error_codes: Record<string, string>
  exit_codes: Record<string, string>
  invariants: string[]
  json_envelope: { flag: string; fields: string[]; notes: string }
  lifecycle: string[]
  mcp: { serve: string; authority: string; tools: string[] }
  recipes: { id: string; version: number; description: string }[]
  recommended_agent_bootstrap: string[]
}

/* ------------------------------------------------------------------ MCP ---- */

export interface McpToolSchema {
  type: string
  properties?: Record<string, { type: string; description?: string; enum?: string[] }>
  required?: string[]
  additionalProperties?: boolean
}

export interface McpTool {
  name: string
  description: string
  inputSchema: McpToolSchema
}

export interface McpServerInfo {
  started: boolean
  workspace: string
  allowAnyWorkspace: boolean
  allowWorkspaces: string[]
  pid?: number
  instructions?: string
  tools: McpTool[]
  error?: string
}

/* ------------------------------------------------------- app-level types --- */

export interface WorkspaceInfo {
  path: string
  name: string
  hasManifest: boolean
  hasLock: boolean
  recipe?: RecipeRef
  managedFiles: number
  lastOpenedAt: string
}

export interface RunRequest {
  /** Registry feature id, used for the activity log and safety gating. */
  featureId: string
  argv: string[]
  cwd: string
  /** Required for features flagged as mutating. */
  confirmed?: boolean
  /** Milliseconds before the runner kills the process; 0 means no timeout. */
  timeoutMs?: number
  /** Stream stdout/stderr chunks instead of buffering (watch, studio). */
  stream?: boolean
}

export interface RunResult {
  requestId: string
  featureId: string
  argv: string[]
  displayCommand: string
  cwd: string
  exitCode: number
  durationMs: number
  stdout: string
  stderr: string
  envelope: Envelope<unknown> | null
  parseError: string | null
  binaryPath: string
  startedAt: string
  timedOut: boolean
  cancelled: boolean
}

export interface AppSettings {
  bobBinaryPath: string
  defaultWorkspace: string
  recentWorkspaces: string[]
  theme: 'dark' | 'light' | 'system'
  requireDigestBoundApply: boolean
  requireMutationConfirmation: boolean
  allowIntegrationProbes: boolean
  demoMode: boolean
}

export const DEFAULT_SETTINGS: AppSettings = {
  bobBinaryPath: '',
  defaultWorkspace: '',
  recentWorkspaces: [],
  theme: 'dark',
  requireDigestBoundApply: true,
  requireMutationConfirmation: true,
  allowIntegrationProbes: false,
  demoMode: false
}
