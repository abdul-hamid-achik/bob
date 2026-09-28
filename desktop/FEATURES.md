# Bob Console features

Every Bob surface this console integrates, and where it lives. The registry in
[`src/shared/features.ts`](src/shared/features.ts) is the source of truth; a test
(`tests/features-doc-sync.test.ts`) fails when this table and the registry drift.

Bob Console is a desktop companion for
[Bob](https://bobcli.dev), the deterministic repository factory. It never
reimplements Bob: every panel drives the real `bob` binary through its versioned
JSON envelope, or the real `bob mcp serve` projection, and shows what came back.

- **Surfaces** — 53 features: 26 CLI commands and subcommands, 9 MCP tools, 18
  console capabilities built on Bob's contracts.
- **Safety** — mutating invocations require an explicit confirmation, apply and
  upgrade can be bound to the reviewed plan digest, and the specialist probe
  stays behind a separate opt-in.
- **Evidence** — the Features panel probes every read-only surface against the
  active workspace and records the real exit code.

## Workspace

| ID | Feature | Bob surface | Mutates | Panel |
| --- | --- | --- | --- | --- |
| `workspaces` | Workspace manager | app | no | Overview |
| `version` | Version & build metadata | `bob version` | no | Overview |
| `explain` | Product contract | `bob explain` | no | Boundaries |

## Contract & recipes

| ID | Feature | Bob surface | Mutates | Panel |
| --- | --- | --- | --- | --- |
| `recipe-list` | Recipe catalog | `bob recipe list` | no | Recipes |
| `recipe-show` | Recipe detail | `bob recipe show <id>` | no | Recipes |
| `manifest-view` | Manifest reader | `bob.yaml` | no | Manifest |
| `manifest-edit` | Manifest editor | `bob.yaml` (edit) | yes | Manifest |
| `validate-manifest` | Strict manifest validation | `bob_validate_manifest` | no | Manifest |
| `lock-inspect` | Lock ledger | `bob.lock` | no | Lock ledger |
| `init` | Initialize manifest | `bob init` | with `--write` | Init & new |
| `new` | Create repository | `bob new` | with `--write` | Init & new |

## Ownership lifecycle

| ID | Feature | Bob surface | Mutates | Panel |
| --- | --- | --- | --- | --- |
| `plan` | Plan | `bob plan` | no | Plan |
| `apply` | Apply | `bob apply` | yes | Apply |
| `check` | Convergence check | `bob check` | no | Check |
| `upgrade` | Recipe upgrade | `bob upgrade` | unless `--dry-run` | Upgrade |
| `remove` | Remove management | `bob remove` | unless `--dry-run` | Remove |
| `path` | Path classification | `bob path <path>` | no | Path |
| `path-batch` | Batch path classification | `bob path --batch` | no | Path |
| `context` | Workspace context | `bob context` | no | Context |
| `watch` | Live plan watch | `bob plan --watch` | no | Plan |

## Guidance

| ID | Feature | Bob surface | Mutates | Panel |
| --- | --- | --- | --- | --- |
| `playbook-list` | Playbook index | `bob playbook list` | no | Playbooks |
| `playbook-show` | Playbook detail | `bob playbook show <id>` | no | Playbooks |
| `playbook-plan` | Playbook resolution | `bob playbook plan <id> --set k=v` | no | Playbooks |
| `next-actions` | Actionable next steps | `next_actions` | no | Overview |
| `learn` | Agent onboarding brief | `bob learn` | no | Agent brief |

## Environment & telemetry

| ID | Feature | Bob surface | Mutates | Panel |
| --- | --- | --- | --- | --- |
| `doctor` | Tool probes | `bob doctor` | no | Doctor |
| `inspect` | Workspace inventory | `bob inspect` | no | Inspect |
| `inspect-probe` | Explicit integration probe | `bob inspect --probe-integrations` | no, needs authority | Inspect |
| `config-show` | Effective settings | `bob config show` | no | Bob config |
| `config-init` | Initialize settings | `bob config init` | with `--write` | Bob config |
| `stats` | Local usage stats | `bob stats` | no | Stats |
| `studio` | Studio board | `bob studio` | no | Studio |

## Agent surface (MCP)

| ID | Feature | Bob surface | Mutates | Panel |
| --- | --- | --- | --- | --- |
| `mcp-serve` | MCP server | `bob mcp serve` | no | MCP |
| `mcp-bob_context` | bob_context | MCP tool | no | MCP |
| `mcp-bob_path` | bob_path | MCP tool | no | MCP |
| `mcp-bob_playbook` | bob_playbook | MCP tool | no | MCP |
| `mcp-bob_inspect` | bob_inspect | MCP tool | no | MCP |
| `mcp-bob_plan` | bob_plan | MCP tool | no | MCP |
| `mcp-bob_check` | bob_check | MCP tool | no | MCP |
| `mcp-bob_validate_manifest` | bob_validate_manifest | MCP tool | no | MCP |
| `mcp-bob_recipe_describe` | bob_recipe_describe | MCP tool | no | MCP |
| `mcp-bob_stats` | bob_stats | MCP tool | no | MCP |

## Console capabilities

| ID | Feature | Bob surface | Mutates | Panel |
| --- | --- | --- | --- | --- |
| `overview` | Convergence overview | app | no | Overview |
| `console` | Command console | `bob <anything>` | no | Command console |
| `activity` | Activity ledger | app | no | Activity |
| `diff` | Content diff viewer | app | no | Plan |
| `files` | Managed file browser | app | no | Files |
| `repository` | Repository state | `git status / log / diff` | no | Repository |
| `tasks` | Development gates | `task <name>` | yes | Dev gates |
| `docs` | Documentation browser | app | no | Docs |
| `features` | Feature coverage matrix | app | no | Features |
| `app-settings` | Console settings | app | no | Settings |
| `demo-mode` | Demo mode | app | no | Settings |

## Bob invariants this console preserves

The console is a client of Bob, so Bob's own contract still holds:

- Read-only panels never mutate: `context`, `path`, `playbook`, `plan`, `check`,
  plain `inspect`, `stats`, `studio`, `explain`, and `learn`.
- `--probe-integrations` is the only specialist subprocess the console can start,
  and it needs both a Settings opt-in and a per-run confirmation.
- The nine MCP tools are repository read-only; the console starts the server with
  an exact workspace allowlist and treats `--allow-any-workspace` as an explicit,
  gated choice.
- Apply and upgrade can be bound to the plan digest actually reviewed, so a
  workspace that changed underneath is refused rather than overwritten.
- `bob.yaml` is human-owned: the editor writes only what the operator typed, and
  only after a confirmation.
- The activity ledger is local, capped, and exportable; it never leaves the
  machine.
