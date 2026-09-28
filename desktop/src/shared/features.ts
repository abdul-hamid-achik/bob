/**
 * Bob Console feature registry.
 *
 * One entry per Bob surface the console integrates. The registry drives the
 * navigation, the command palette, the feature-coverage matrix, argv building,
 * and mutation/authority gating, so it is the single source of truth for
 * "which Bob features exist and where they live in the app".
 */

export type FeatureGroup =
  | 'workspace'
  | 'contract'
  | 'lifecycle'
  | 'guidance'
  | 'environment'
  | 'agent'
  | 'operator'

export type PanelId =
  | 'overview'
  | 'plan'
  | 'apply'
  | 'check'
  | 'upgrade'
  | 'remove'
  | 'path'
  | 'context'
  | 'manifest'
  | 'recipes'
  | 'lock'
  | 'scaffold'
  | 'playbooks'
  | 'doctor'
  | 'inspect'
  | 'settings'
  | 'stats'
  | 'learn'
  | 'explain'
  | 'mcp'
  | 'studio'
  | 'console'
  | 'docs'
  | 'activity'
  | 'features'
  | 'files'
  | 'tasks'
  | 'repository'
  | 'app-settings'

export type Surface = 'cli' | 'mcp' | 'tui' | 'app'

export interface FlagSpec {
  name: string
  kind: 'boolean' | 'string' | 'stringArray' | 'enum'
  label: string
  help: string
  values?: string[]
  placeholder?: string
}

export interface Feature {
  id: string
  group: FeatureGroup
  title: string
  command: string
  surface: Surface
  summary: string
  detail: string
  panel: PanelId
  mutates: boolean
  /** Requires the explicit subprocess authority the operator must grant. */
  authority?: boolean
  /** Long-running / streaming command. */
  streaming?: boolean
  argvTemplate?: string[]
  flags?: FlagSpec[]
  docs?: string[]
  invariants?: string[]
}

export const GROUP_LABEL: Record<FeatureGroup, string> = {
  workspace: 'Workspace',
  contract: 'Contract & recipes',
  lifecycle: 'Ownership lifecycle',
  guidance: 'Guidance',
  environment: 'Environment & telemetry',
  agent: 'Agent surface',
  operator: 'Console'
}

const WS = '{workspace}'

export const FEATURES: Feature[] = [
  /* ------------------------------------------------------------ workspace */
  {
    id: 'workspaces',
    group: 'workspace',
    title: 'Workspace manager',
    command: 'app',
    surface: 'app',
    summary: 'Open, track, and switch between Bob workspaces.',
    detail:
      'Keeps a recent-workspace list, sniffs bob.yaml and bob.lock for each entry, and resolves the recipe identity so every other panel knows which workspace it drives.',
    panel: 'overview',
    mutates: false
  },
  {
    id: 'version',
    group: 'workspace',
    title: 'Version & build metadata',
    command: 'bob version',
    surface: 'cli',
    summary: 'Report the resolved Bob binary version, commit, and build date.',
    detail:
      'Also proves which binary the console resolved: the configured path, the repository ./bin/bob, or the first bob on PATH.',
    panel: 'overview',
    mutates: false,
    argvTemplate: ['version'],
    docs: ['docs/reference/cli.md']
  },
  {
    id: 'explain',
    group: 'workspace',
    title: 'Product contract',
    command: 'bob explain',
    surface: 'cli',
    summary: 'State what Bob owns, what it does not own, and the recipe catalog.',
    detail:
      'Renders the owns / does_not_own boundary lists so the console never implies Bob is an LLM runtime, package manager, or verification authority.',
    panel: 'explain',
    mutates: false,
    argvTemplate: ['explain'],
    docs: ['docs/product-direction.md', 'docs/reference/cli.md'],
    invariants: ['Bob is not an LLM runtime, evidence authority, or secret manager']
  },

  /* ------------------------------------------------------------- contract */
  {
    id: 'recipe-list',
    group: 'contract',
    title: 'Recipe catalog',
    command: 'bob recipe list',
    surface: 'cli',
    summary: 'List every embedded recipe with version, stacks, surfaces, and seeded paths.',
    detail:
      'Fourteen recipes: go-agent-tool, files, and twelve seed-once stack hygiene recipes. The catalog drives recipe pickers in init and new.',
    panel: 'recipes',
    mutates: false,
    argvTemplate: ['recipe', 'list'],
    docs: ['docs/reference/manifest.md'],
    invariants: ['Never change a published recipe version in place']
  },
  {
    id: 'recipe-show',
    group: 'contract',
    title: 'Recipe detail',
    command: 'bob recipe show <id>',
    surface: 'cli',
    summary: 'Inspect one recipe: language, ownership note, surfaces, seeded paths.',
    detail:
      'Shows the exact whole-file set a recipe seeds so ownership can be reviewed before any apply.',
    panel: 'recipes',
    mutates: false,
    argvTemplate: ['recipe', 'show', '{recipe}']
  },
  {
    id: 'manifest-view',
    group: 'contract',
    title: 'Manifest reader',
    command: 'bob.yaml',
    surface: 'app',
    summary: 'Read the human-owned bob.yaml contract with typed sections.',
    detail:
      'Parses product, runtime, surfaces, integrations, distribution, and ownership into a readable form next to the raw YAML.',
    panel: 'manifest',
    mutates: false,
    docs: ['docs/reference/manifest.md']
  },
  {
    id: 'manifest-edit',
    group: 'contract',
    title: 'Manifest editor',
    command: 'bob.yaml (edit)',
    surface: 'app',
    summary: 'Edit bob.yaml and strictly validate before saving.',
    detail:
      'Validation runs through the read-only MCP tool bob_validate_manifest on the inline buffer, so a malformed contract is caught before it touches disk. Saving writes the file and re-plans.',
    panel: 'manifest',
    mutates: true,
    docs: ['docs/reference/manifest.md'],
    invariants: ['bob.yaml is human-owned; Bob never rewrites it']
  },
  {
    id: 'validate-manifest',
    group: 'contract',
    title: 'Strict manifest validation',
    command: 'bob_validate_manifest',
    surface: 'mcp',
    summary: 'Validate a workspace manifest or bounded inline YAML without writing it.',
    detail:
      'Returns the normalized typed manifest plus strict validation errors. Inline YAML is capped at 65536 bytes and is mutually exclusive with workspace.',
    panel: 'manifest',
    mutates: false
  },
  {
    id: 'lock-inspect',
    group: 'contract',
    title: 'Lock ledger',
    command: 'bob.lock',
    surface: 'app',
    summary: 'Read the ownership ledger: recipe version and every whole-file digest.',
    detail:
      'Parses bob.lock, joins it against the current plan, and flags entries whose locked digest no longer matches desired or current content.',
    panel: 'lock',
    mutates: false,
    docs: ['docs/ownership-and-safety.md'],
    invariants: ['A managed file may update only if its current hash matches the prior lock']
  },
  {
    id: 'init',
    group: 'contract',
    title: 'Initialize manifest',
    command: 'bob init',
    surface: 'cli',
    summary: 'Detect the stack, preview the manifest, and optionally write bob.yaml.',
    detail:
      'Preview is the default. Detection reports stacks, markers, and the primary stack; a recipe that does not match the detected stack warns in preview and refuses --write without --force.',
    panel: 'scaffold',
    mutates: true,
    argvTemplate: ['init', WS],
    flags: [
      { name: '--name', kind: 'string', label: 'Project name', help: 'Defaults to the directory name.', placeholder: 'acme-tool' },
      { name: '--module', kind: 'string', label: 'Module path', help: 'Required by go-agent-tool; optional identity for stack recipes.', placeholder: 'github.com/acme/acme-tool' },
      { name: '--description', kind: 'string', label: 'Description', help: 'One-line product description.', placeholder: 'Agent-ready Acme CLI' },
      { name: '--recipe', kind: 'string', label: 'Recipe', help: 'Defaults to the recipe matching the detected stack.', placeholder: 'ts-app' },
      { name: '--write', kind: 'boolean', label: 'Write bob.yaml', help: 'Without it Bob only previews.' },
      { name: '--force', kind: 'boolean', label: 'Force mismatched recipe', help: 'Write even when the recipe does not match the detected stack.' }
    ],
    docs: ['docs/getting-started.md', 'docs/reference/manifest.md']
  },
  {
    id: 'new',
    group: 'contract',
    title: 'Create repository',
    command: 'bob new',
    surface: 'cli',
    summary: 'Preview or create a whole repository from a built-in recipe.',
    detail:
      'Preview lists every artifact the recipe would render. Seed-once stack recipes refuse to scaffold application source and tell you to initialize the application first.',
    panel: 'scaffold',
    mutates: true,
    argvTemplate: ['new', '{name}'],
    flags: [
      { name: '--dir', kind: 'string', label: 'Target directory', help: 'Defaults to the project name.', placeholder: '/absolute/path' },
      { name: '--module', kind: 'string', label: 'Go module path', help: 'Required by go-agent-tool; rejected by every other recipe.', placeholder: 'github.com/acme/acme-tool' },
      { name: '--description', kind: 'string', label: 'Description', help: 'One-line product description.' },
      { name: '--recipe', kind: 'string', label: 'Recipe', help: 'Defaults to the detected stack recipe, else go-agent-tool.' },
      { name: '--write', kind: 'boolean', label: 'Create files', help: 'Without it Bob only previews the artifact list.' }
    ],
    docs: ['docs/getting-started.md']
  },

  /* ------------------------------------------------------------ lifecycle */
  {
    id: 'plan',
    group: 'lifecycle',
    title: 'Plan',
    command: 'bob plan',
    surface: 'cli',
    summary: 'Compare the recipe with the repository and classify every path.',
    detail:
      'Actions are create, adopt, unchanged, update, or conflict. The plan carries a deterministic digest that apply and upgrade can be bound to. Plan never writes and always exits 0.',
    panel: 'plan',
    mutates: false,
    argvTemplate: ['plan', WS],
    flags: [
      { name: '--content', kind: 'boolean', label: 'Content previews', help: 'Bounded desired-content previews for create/update/conflict, plus current content for conflicts.' },
      { name: '--diff', kind: 'boolean', label: 'Unified diffs', help: 'Unified content diffs for create and update actions.' },
      { name: '--conflicts-only', kind: 'boolean', label: 'Conflicts only', help: 'Compact output for capped agent harnesses.' },
      { name: '--watch', kind: 'boolean', label: 'Watch bob.yaml', help: 'Re-plan on change. Mutually exclusive with --json, so the console streams the human output.' }
    ],
    docs: ['docs/reference/cli.md', 'docs/ownership-and-safety.md'],
    invariants: ['plan does not mutate repositories', 'Repeated apply converges to a no-op']
  },
  {
    id: 'apply',
    group: 'lifecycle',
    title: 'Apply',
    command: 'bob apply',
    surface: 'cli',
    summary: 'Write one complete conflict-free plan, bound to the reviewed digest.',
    detail:
      'Preflights the complete plan and writes nothing when any conflict exists. The console always offers --expect-plan-digest from the plan you actually reviewed, so a workspace that changed under you is refused with exit 5.',
    panel: 'apply',
    mutates: true,
    argvTemplate: ['apply', WS],
    flags: [
      { name: '--expect-plan-digest', kind: 'string', label: 'Expected plan digest', help: 'Apply only when a fresh plan matches this exact sha256:<64-lowercase-hex> digest.', placeholder: 'sha256:…' }
    ],
    docs: ['docs/ownership-and-safety.md'],
    invariants: [
      'apply preflights the complete plan and writes nothing when any conflict exists',
      'Bob never overwrites an unmanaged differing file'
    ]
  },
  {
    id: 'check',
    group: 'lifecycle',
    title: 'Convergence check',
    command: 'bob check',
    surface: 'cli',
    summary: 'Fail when managed repository state would change.',
    detail:
      'Exit 3 means drift without an ownership conflict; exit 2 means conflicts block apply. The console shows the same plan digest the planner produced so CI and desktop agree.',
    panel: 'check',
    mutates: false,
    argvTemplate: ['check', WS],
    flags: [{ name: '--conflicts-only', kind: 'boolean', label: 'Conflicts only', help: 'Show only conflicting actions.' }],
    docs: ['docs/reference/cli.md'],
    invariants: ['check does not mutate repositories']
  },
  {
    id: 'upgrade',
    group: 'lifecycle',
    title: 'Recipe upgrade',
    command: 'bob upgrade',
    surface: 'cli',
    summary: 'Migrate bob.lock to the current recipe version.',
    detail:
      'Dry run reports from_version, to_version, and the action count. A real upgrade honours the same digest guard as apply.',
    panel: 'upgrade',
    mutates: true,
    argvTemplate: ['upgrade', WS],
    flags: [
      { name: '--dry-run', kind: 'boolean', label: 'Dry run', help: 'Show what would change without applying.' },
      { name: '--expect-plan-digest', kind: 'string', label: 'Expected plan digest', help: 'Upgrade only when a fresh plan matches this digest.', placeholder: 'sha256:…' }
    ],
    docs: ['docs/ownership-and-safety.md']
  },
  {
    id: 'remove',
    group: 'lifecycle',
    title: 'Remove management',
    command: 'bob remove',
    surface: 'cli',
    summary: 'Stop Bob managing a workspace, deleting only lock-owned files.',
    detail:
      'Dry run lists removed, skipped, and conflicting paths. Seed-once and unmanaged files stay, bob.yaml is preserved, and bob.lock is removed last. --force is required for drifted managed files.',
    panel: 'remove',
    mutates: true,
    argvTemplate: ['remove', WS],
    flags: [
      { name: '--dry-run', kind: 'boolean', label: 'Dry run', help: 'Show what would be removed without removing anything.' },
      { name: '--force', kind: 'boolean', label: 'Force drifted files', help: 'Remove managed files even when their content drifted from bob.lock.' }
    ],
    docs: ['docs/ownership-and-safety.md']
  },
  {
    id: 'path',
    group: 'lifecycle',
    title: 'Path classification',
    command: 'bob path <path>',
    surface: 'cli',
    summary: 'Explain Bob\'s exact relationship to one repository path.',
    detail:
      'Returns classification, state, the effect a human edit would have, ownership hashes, the artifact identity, extension points, related playbooks, notices, and typed next actions.',
    panel: 'path',
    mutates: false,
    argvTemplate: ['path', '--workspace', WS, '--', '{path}'],
    docs: ['docs/reference/path.md'],
    invariants: ['path does not mutate repositories']
  },
  {
    id: 'path-batch',
    group: 'lifecycle',
    title: 'Batch path classification',
    command: 'bob path --batch',
    surface: 'cli',
    summary: 'Classify up to seven paths against one workspace plan.',
    detail:
      'One plan is computed and reused for every path, so a batch answer is internally consistent and cheaper than seven calls.',
    panel: 'path',
    mutates: false,
    argvTemplate: ['path', '--batch', '--workspace', WS, '--', '{paths}'],
    docs: ['docs/reference/path.md']
  },
  {
    id: 'context',
    group: 'lifecycle',
    title: 'Workspace context',
    command: 'bob context',
    surface: 'cli',
    summary: 'Bounded offline contract for a workspace: recipe, capabilities, invariants, plan digest.',
    detail:
      'Three profiles — compact, standard, full — with explicit truncation reporting. Capabilities expose selection, materialization, availability, and verification facets without running any specialist tool.',
    panel: 'context',
    mutates: false,
    argvTemplate: ['context', WS],
    flags: [
      { name: '--profile', kind: 'enum', label: 'Profile', help: 'compact, standard, or full.', values: ['compact', 'standard', 'full'] }
    ],
    docs: ['docs/reference/context.md'],
    invariants: ['context does not mutate repositories', 'Bounded offline composition']
  },
  {
    id: 'watch',
    group: 'lifecycle',
    title: 'Live plan watch',
    command: 'bob plan --watch',
    surface: 'cli',
    summary: 'Re-plan whenever bob.yaml changes and stream the result.',
    detail:
      'Because --watch and --json are mutually exclusive, the console streams Bob\'s human watch output and separately re-runs a typed plan on each change so the table stays machine-readable.',
    panel: 'plan',
    mutates: false,
    streaming: true,
    argvTemplate: ['plan', WS, '--watch']
  },

  /* ------------------------------------------------------------- guidance */
  {
    id: 'playbook-list',
    group: 'guidance',
    title: 'Playbook index',
    command: 'bob playbook list',
    surface: 'cli',
    summary: 'List the closed procedures available for the active recipe.',
    detail:
      'Each entry reports applicability, availability, blockers, required inputs, scope class, and risk.',
    panel: 'playbooks',
    mutates: false,
    argvTemplate: ['playbook', 'list', WS],
    docs: ['docs/reference/playbooks.md'],
    invariants: ['playbook does not mutate repositories']
  },
  {
    id: 'playbook-show',
    group: 'guidance',
    title: 'Playbook detail',
    command: 'bob playbook show <id>',
    surface: 'cli',
    summary: 'Show purpose, typed inputs, boundary, steps, verification hints, and failure modes.',
    detail:
      'Steps carry argv, effect, dependencies, success conditions, and whether they need explicit authority. The console renders them as an ordered procedure, never as an automatic execution.',
    panel: 'playbooks',
    mutates: false,
    argvTemplate: ['playbook', 'show', '{playbook}', WS],
    docs: ['docs/reference/playbooks.md']
  },
  {
    id: 'playbook-plan',
    group: 'guidance',
    title: 'Playbook resolution',
    command: 'bob playbook plan <id> --set k=v',
    surface: 'cli',
    summary: 'Resolve a procedure with typed inputs into concrete argv-shaped steps.',
    detail:
      'Inputs are typed and closed: repository_path values are validated as safe relative paths and enums are checked against the current planner action. Resolution still executes nothing.',
    panel: 'playbooks',
    mutates: false,
    argvTemplate: ['playbook', 'plan', '{playbook}', WS],
    flags: [{ name: '--set', kind: 'stringArray', label: 'Typed input', help: 'key=value, repeatable.', placeholder: 'path=README.md' }],
    docs: ['docs/reference/playbooks.md']
  },
  {
    id: 'next-actions',
    group: 'guidance',
    title: 'Actionable next steps',
    command: 'next_actions',
    surface: 'app',
    summary: 'Turn every envelope\'s guidance into runnable commands.',
    detail:
      'Bob returns next_actions on success and failure. The console parses the ones that are commands, shows the exact argv, and runs the read-only ones with one click; mutating ones stay behind confirmation.',
    panel: 'overview',
    mutates: false,
    docs: ['docs/agents.md']
  },
  {
    id: 'learn',
    group: 'guidance',
    title: 'Agent onboarding brief',
    command: 'bob learn',
    surface: 'cli',
    summary: 'One-shot brief: commands, exit codes, error codes, invariants, lifecycle, MCP tools.',
    detail:
      'The console renders the brief as a navigable reference and keeps the raw JSON copyable for pasting into an agent runtime.',
    panel: 'learn',
    mutates: false,
    argvTemplate: ['learn'],
    docs: ['docs/agents.md'],
    invariants: ['learn does not mutate repositories']
  },

  /* ---------------------------------------------------------- environment */
  {
    id: 'doctor',
    group: 'environment',
    title: 'Tool probes',
    command: 'bob doctor',
    surface: 'cli',
    summary: 'Probe required and selected optional development tools honestly.',
    detail:
      'Reports found, usable, resolved path, version, and a note per tool. ready=false or degraded=true is surfaced, never smoothed over.',
    panel: 'doctor',
    mutates: false,
    argvTemplate: ['doctor', WS],
    docs: ['docs/getting-started.md']
  },
  {
    id: 'inspect',
    group: 'environment',
    title: 'Workspace inventory',
    command: 'bob inspect',
    surface: 'cli',
    summary: 'Summarize Bob state and offline specialist-binary availability.',
    detail:
      'Reports repository readiness, convergence, action counts, and whether Codemap and Vecgrep binaries exist — without running them.',
    panel: 'inspect',
    mutates: false,
    argvTemplate: ['inspect', WS],
    docs: ['docs/reference/cli.md'],
    invariants: ['plain inspect does not mutate repositories', 'No specialist process runs by default']
  },
  {
    id: 'inspect-probe',
    group: 'environment',
    title: 'Explicit integration probe',
    command: 'bob inspect --probe-integrations',
    surface: 'cli',
    summary: 'Call the public Codemap and Vecgrep status commands, only on explicit authority.',
    detail:
      'Those commands may open tool-owned stores and Vecgrep may contact its configured embedding provider. The console keeps this behind a separate opt-in that is off by default and shows the warning before every run.',
    panel: 'inspect',
    mutates: false,
    authority: true,
    argvTemplate: ['inspect', WS, '--probe-integrations'],
    docs: ['docs/guides/mcphub-local-agent.md'],
    invariants: ['inspect --probe-integrations is explicit subprocess authority', 'It never initializes, indexes, resets, searches, or repairs a specialist tool']
  },
  {
    id: 'config-show',
    group: 'environment',
    title: 'Effective settings',
    command: 'bob config show',
    surface: 'cli',
    summary: 'Show resolved XDG paths and effective per-user settings.',
    detail:
      'Reports config file, cache, data, and state directories plus the telemetry settings and the local-only telemetry destination.',
    panel: 'settings',
    mutates: false,
    argvTemplate: ['config', 'show'],
    docs: ['docs/configuration.md']
  },
  {
    id: 'config-init',
    group: 'environment',
    title: 'Initialize settings',
    command: 'bob config init',
    surface: 'cli',
    summary: 'Preview or create private XDG user settings, optionally enabling telemetry.',
    detail:
      'Telemetry is disabled by default, has no network transport, and never stores paths, arguments, filenames, manifest content, or raw errors. The console shows that statement next to the toggle.',
    panel: 'settings',
    mutates: true,
    argvTemplate: ['config', 'init'],
    flags: [
      { name: '--telemetry', kind: 'boolean', label: 'Enable telemetry', help: 'Privacy-bounded local telemetry.' },
      { name: '--write', kind: 'boolean', label: 'Write settings file', help: 'Without it Bob only previews.' }
    ],
    docs: ['docs/configuration.md'],
    invariants: ['Telemetry is disabled by default and has no network transport']
  },
  {
    id: 'stats',
    group: 'environment',
    title: 'Local usage stats',
    command: 'bob stats',
    surface: 'cli',
    summary: 'Aggregate opt-in local usage without exposing individual events.',
    detail:
      'Events, successes, failures, conflict and drift events, duration, and a per-operation breakdown for a lookback window. Empty while telemetry is disabled.',
    panel: 'stats',
    mutates: false,
    argvTemplate: ['stats', WS],
    flags: [
      { name: '--since', kind: 'string', label: 'Lookback window', help: 'Such as 24h, 7d, or 30d.', placeholder: '7d' },
      { name: '--all', kind: 'boolean', label: 'All workspaces', help: 'Aggregate every retained pseudonymous workspace. Mutually exclusive with a workspace argument.' }
    ],
    docs: ['docs/configuration.md', 'docs/studio.md'],
    invariants: ['stats never records events', 'Never stores paths, arguments, filenames, or raw errors']
  },
  {
    id: 'studio',
    group: 'environment',
    title: 'Studio board',
    command: 'bob studio',
    surface: 'tui',
    summary: 'Launch the read-only Bubble Tea Overview, Plan, and Stats board in a terminal.',
    detail:
      'The console reproduces the same three projections natively and can also hand off to the real TUI in Terminal or iTerm. Studio exposes no apply, shell, editor, indexing, probing, or repair action.',
    panel: 'studio',
    mutates: false,
    streaming: true,
    argvTemplate: ['studio', WS],
    flags: [{ name: '--single-pane', kind: 'boolean', label: 'Single pane', help: 'Force the accessible compact layout.' }],
    docs: ['docs/studio.md'],
    invariants: ['studio does not mutate repositories', 'Studio exposes no apply, shell, editor, indexing, probing, or repair action']
  },

  /* ---------------------------------------------------------------- agent */
  {
    id: 'mcp-serve',
    group: 'agent',
    title: 'MCP server',
    command: 'bob mcp serve',
    surface: 'mcp',
    summary: 'Run the read-only stdio MCP projection and drive its nine typed tools.',
    detail:
      'The console starts the server with an exact workspace allowlist, lists tools with their input schemas, calls any tool interactively, and shows the server instructions. stdout stays JSON-RPC-only.',
    panel: 'mcp',
    mutates: false,
    streaming: true,
    argvTemplate: ['mcp', 'serve', '--workspace', WS],
    flags: [
      { name: '--allow-workspace', kind: 'stringArray', label: 'Additional workspace', help: 'Exact existing workspace allowed to MCP tools, repeatable.', placeholder: '/absolute/path' },
      { name: '--allow-any-workspace', kind: 'boolean', label: 'Allow any workspace', help: 'Expands read authority to any workspace Bob can reach. Must be an explicit choice.' }
    ],
    docs: ['docs/guides/mcphub-local-agent.md', 'docs/agents.md'],
    invariants: ['The nine MCP tools never mutate repositories or run specialist probes', 'MCP stdout is JSON-RPC-only']
  },
  {
    id: 'mcp-bob_context',
    group: 'agent',
    title: 'bob_context',
    command: 'MCP tool',
    surface: 'mcp',
    summary: 'Bounded workspace contract and current plan identity.',
    detail: 'Defaults to the compact profile; never runs specialist tools or mutates.',
    panel: 'mcp',
    mutates: false
  },
  {
    id: 'mcp-bob_path',
    group: 'agent',
    title: 'bob_path',
    command: 'MCP tool',
    surface: 'mcp',
    summary: 'Classify one exact repository-relative path without returning file bodies.',
    detail: 'Path is capped at 4096 bytes.',
    panel: 'mcp',
    mutates: false
  },
  {
    id: 'mcp-bob_playbook',
    group: 'agent',
    title: 'bob_playbook',
    command: 'MCP tool',
    surface: 'mcp',
    summary: 'List, show, or resolve a closed procedure without executing it.',
    detail: 'Typed values are bounded: at most 32 entries, 128-byte keys, 4096-byte values.',
    panel: 'mcp',
    mutates: false
  },
  {
    id: 'mcp-bob_inspect',
    group: 'agent',
    title: 'bob_inspect',
    command: 'MCP tool',
    surface: 'mcp',
    summary: 'Bob drift plus offline Codemap and Vecgrep availability.',
    detail: 'Never runs specialist status commands, searches, indexes, verifies, or mutates.',
    panel: 'mcp',
    mutates: false
  },
  {
    id: 'mcp-bob_plan',
    group: 'agent',
    title: 'bob_plan',
    command: 'MCP tool',
    surface: 'mcp',
    summary: 'Bounded action list and deterministic digest.',
    detail: 'Unchanged actions are excluded by default; max_actions is 1–500 with a default of 100.',
    panel: 'mcp',
    mutates: false
  },
  {
    id: 'mcp-bob_check',
    group: 'agent',
    title: 'bob_check',
    command: 'MCP tool',
    surface: 'mcp',
    summary: 'Compact convergence, conflict, and lock-drift result.',
    detail: 'Carries the same complete-plan digest as bob_plan.',
    panel: 'mcp',
    mutates: false
  },
  {
    id: 'mcp-bob_validate_manifest',
    group: 'agent',
    title: 'bob_validate_manifest',
    command: 'MCP tool',
    surface: 'mcp',
    summary: 'Strictly validate one workspace manifest or bounded inline YAML.',
    detail: 'Returns the normalized typed manifest and never writes it.',
    panel: 'mcp',
    mutates: false
  },
  {
    id: 'mcp-bob_recipe_describe',
    group: 'agent',
    title: 'bob_recipe_describe',
    command: 'MCP tool',
    surface: 'mcp',
    summary: 'Describe the embedded recipe contract without reading a workspace.',
    detail: 'Reports supported choices, schema version, and generated surfaces.',
    panel: 'mcp',
    mutates: false
  },
  {
    id: 'mcp-bob_stats',
    group: 'agent',
    title: 'bob_stats',
    command: 'MCP tool',
    surface: 'mcp',
    summary: 'Aggregate opt-in local usage without individual events.',
    detail: 'since_days defaults to 7 and caps at 365; all and workspace are mutually exclusive.',
    panel: 'mcp',
    mutates: false
  },

  /* ------------------------------------------------------------- operator */
  {
    id: 'overview',
    group: 'operator',
    title: 'Convergence overview',
    command: 'app',
    surface: 'app',
    summary: 'One screen: workspace state, action counts, conflicts, digest, and the next move.',
    detail:
      'Composes inspect, check, and context into a single anchor so the operator can see whether the repository is converged before choosing an action.',
    panel: 'overview',
    mutates: false
  },
  {
    id: 'console',
    group: 'operator',
    title: 'Command console',
    command: 'bob <anything>',
    surface: 'app',
    summary: 'Run any Bob command with an argv builder and inspect the raw envelope.',
    detail:
      'Shows argv, cwd, exit code with its documented meaning, duration, stdout, stderr, and the parsed envelope side by side. Mutating commands still require confirmation.',
    panel: 'console',
    mutates: false
  },
  {
    id: 'activity',
    group: 'operator',
    title: 'Activity ledger',
    command: 'app',
    surface: 'app',
    summary: 'Append-only local log of every invocation the console made.',
    detail:
      'Records argv, cwd, exit code, duration, plan digest, and timestamp for audit. It never leaves the machine and can be cleared or exported.',
    panel: 'activity',
    mutates: false
  },
  {
    id: 'diff',
    group: 'operator',
    title: 'Content diff viewer',
    command: 'app',
    surface: 'app',
    summary: 'Read bounded desired/current previews and unified diffs per action.',
    detail:
      'Line-level diff highlighting for create, update, and conflict actions, with hash triples (current, locked, desired) beside the text.',
    panel: 'plan',
    mutates: false
  },
  {
    id: 'files',
    group: 'operator',
    title: 'Managed file browser',
    command: 'app',
    surface: 'app',
    summary: 'Browse owned, seeded, and unmanaged paths and open them in your editor.',
    detail:
      'Joins the lock ledger with the plan and the working tree so ownership is visible per path, and hands off to the configured editor or Finder without editing anything itself.',
    panel: 'files',
    mutates: false
  },
  {
    id: 'repository',
    group: 'operator',
    title: 'Repository state',
    command: 'git status / log / diff',
    surface: 'app',
    summary: 'Read-only git state for the workspace: branch, dirty paths, recent commits.',
    detail:
      'Helps review what an apply actually changed. The console never stages, commits, resets, or pushes.',
    panel: 'repository',
    mutates: false
  },
  {
    id: 'tasks',
    group: 'operator',
    title: 'Development gates',
    command: 'task <name>',
    surface: 'app',
    summary: 'Run the repository Taskfile gates and stream their output.',
    detail:
      'build, test, race, lint, verify, specs, docs-build, ship, and agent-bootstrap for working on Bob itself. Output streams live and exit codes are reported verbatim.',
    panel: 'tasks',
    mutates: true,
    streaming: true,
    docs: ['CONTRIBUTING.md', 'AGENTS.md']
  },
  {
    id: 'docs',
    group: 'operator',
    title: 'Documentation browser',
    command: 'app',
    surface: 'app',
    summary: 'Read the published reference pages and guides without leaving the console.',
    detail:
      'Renders docs/*.md from the Bob checkout, including the CLI, manifest, context, path, and playbook references plus ownership-and-safety.',
    panel: 'docs',
    mutates: false
  },
  {
    id: 'features',
    group: 'operator',
    title: 'Feature coverage matrix',
    command: 'app',
    surface: 'app',
    summary: 'Every Bob feature, where it lives, and a live probe that proves it runs.',
    detail:
      'Lists each registry entry with its group, surface, mutation flag, and panel, and can execute the underlying read-only command to record real evidence of integration.',
    panel: 'features',
    mutates: false
  },
  {
    id: 'app-settings',
    group: 'operator',
    title: 'Console settings',
    command: 'app',
    surface: 'app',
    summary: 'Bob binary path, default workspace, theme, and safety defaults.',
    detail:
      'Safety defaults: bind apply to the reviewed plan digest, require confirmation before any mutation, and keep specialist probes off.',
    panel: 'app-settings',
    mutates: false
  },
  {
    id: 'demo-mode',
    group: 'operator',
    title: 'Demo mode',
    command: 'app',
    surface: 'app',
    summary: 'Review every panel against real captured Bob output without a binary.',
    detail:
      'Replays envelopes captured from a Bob checkout so the interface is reviewable on a machine where Bob is not installed. Clearly labelled; never mixed with live results.',
    panel: 'app-settings',
    mutates: false
  }
]

export const FEATURE_BY_ID: ReadonlyMap<string, Feature> = new Map(FEATURES.map((f) => [f.id, f]))

export const MCP_TOOL_FEATURE_IDS = FEATURES.filter((f) => f.surface === 'mcp' && f.id.startsWith('mcp-bob_')).map(
  (f) => f.id.slice('mcp-'.length)
)

export function featureById(id: string): Feature | undefined {
  return FEATURE_BY_ID.get(id)
}

export function featuresByGroup(): { group: FeatureGroup; label: string; features: Feature[] }[] {
  const order: FeatureGroup[] = ['workspace', 'contract', 'lifecycle', 'guidance', 'environment', 'agent', 'operator']
  return order.map((group) => ({
    group,
    label: GROUP_LABEL[group],
    features: FEATURES.filter((f) => f.group === group)
  }))
}

export const MUTATING_FEATURE_IDS = FEATURES.filter((f) => f.mutates).map((f) => f.id)
export const AUTHORITY_FEATURE_IDS = FEATURES.filter((f) => f.authority).map((f) => f.id)
