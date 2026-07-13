# Bob specification

Status: v0.2 draft

## Purpose

Bob turns a versioned product manifest into a public-ready repository and keeps
the files it owns aligned as the manifest or recipe evolves.

```text
bob.yaml + recipe + observed files + bob.lock
                    |
                    v
             deterministic plan
                    |
              explicit apply
                    |
                    v
          repository + new bob.lock
```

## Current scope

Bob supports one recipe, `go-agent-tool`, currently at recipe version 3. It
creates a Go command-line project with a thin Cobra entrypoint, JSON output,
version and doctor commands, tests, public and agent documentation, community
templates for GitHub modules, non-mutating verification, vulnerability scans,
CI, optional GoReleaser configuration, and an optional Glyphrun behavior spec.

Release archives target macOS and Linux. Windows publication is deferred until
Bob has a tested atomic file-replacement implementation for that platform.

The manifest schema, lock schema, CLI JSON output envelope, user settings, MCP
tool output, and local telemetry event formats use independently versioned
schemas starting at 1.

## Ownership rules

The manifest is human-owned. The lock records the SHA-256 digest of each
Bob-owned file after the last successful apply.

For every desired artifact, planning returns exactly one state:

- `create`: the path does not exist;
- `adopt`: an unmanaged regular file already matches exactly;
- `unchanged`: a managed file matches the desired content;
- `update`: the current file still matches Bob's previous digest and the recipe
  now wants different content;
- `conflict`: ownership is absent or stale, or the destination is unsafe.

Any conflict blocks the complete apply. Bob does not delete files.

## Recipe upgrades

A lock from an older positive version of the same recipe is a safe upgrade
input because it records the exact hash of every previously managed whole file.
Planning uses those hashes for ownership decisions, renders the current recipe,
and proposes the new lock version. Untouched managed files may update, new files
may be created, and human-modified files remain conflicts.

A lock whose recipe version is newer than the running Bob binary is rejected.
Bob never changes the content of a published recipe version in place.

## Filesystem safety

Desired paths are normalized relative paths inside the selected workspace.
Absolute paths, parent traversal, `.git`, `bob.yaml`, and `bob.lock` are reserved.
Pre-existing symlinks, directories, devices, sockets, and named pipes are conflicts.
Each file is published through a temporary sibling and atomic rename; the lock
is published last. Multi-file publication is not globally transactional, so a
process crash can leave a partially applied tree. A subsequent plan reports the
exact state and may safely adopt already-published matching files.

Path publication is pathname-based. A concurrent same-user process that
swaps a checked parent directory is an OS-containment boundary; callers must not
run `apply` inside an untrusted, concurrently mutated directory tree.

## Command behavior

- `new` and `init` preview by default; `--write` authorizes creation.
- `plan` computes desired changes without writes.
- `plan --content` adds bounded desired-content previews; JSON plans include the
  same bounded preview field for create and update actions.
- `apply` is the explicit repository mutation command.
- `check` exits non-zero when the repository or lock would change.
- `doctor` probes declared dependencies with bounded direct commands.
- `inspect` summarizes Bob state and binary availability without launching
  specialist tools by default. `--probe-integrations` explicitly authorizes
  bounded Codemap and Vecgrep status calls.
- `config show` reports the effective user settings and resolved XDG paths.
- `config init` previews by default and creates a private settings file only
  with `--write`; `--telemetry` opts into local recording.
- `stats` returns aggregates only and never returns individual events.
- `studio` is an interactive repository-read-only Overview, Plan, and Stats
  projection. It refuses non-interactive terminals and does not support JSON.
- `mcp serve` exposes six repository-read-only tools over newline-delimited
  stdio.
- `explain` describes Bob's capability and ecosystem boundary.
- `recipe list|show` describes the embedded recipe catalog.
- `version` reports build metadata.

Normal non-interactive agent-oriented commands support `--json`. Studio is
interactive-only. JSON stdout uses a versioned envelope and contains no
progress logging or ANSI escapes; MCP reserves stdout for JSON-RPC.

## User configuration and local telemetry

Bob resolves per-user configuration, data, state, and cache locations with the
XDG Base Directory conventions. Bob-specific absolute overrides take
precedence, and `BOB_CONFIG` may select the exact settings file. Relative XDG or
Bob override values are rejected. Resolving paths and loading default settings
does not create directories.

The schema-v1 YAML settings file controls local telemetry retention and daily
event limits. A missing file means telemetry is disabled with 30-day retention
and a 1,000-event daily cap if later enabled. `BOB_TELEMETRY` may explicitly
override the enabled setting.

Telemetry is disabled by default and has no network transport. When enabled,
Bob records schema-v1 events beneath its XDG state directory using private
directories and files. Events use a machine-local HMAC pseudonym for a
workspace and a closed vocabulary for surface, operation, outcome, reason,
recipe, duration, and aggregate action counts. The durable schema cannot
represent raw paths, argv, filenames, file content, manifest content,
free-form labels, or raw errors. Recording is best-effort and cannot change the
result of a product operation. Retention and daily caps are enforced locally.

`stats` and `bob_stats` return bounded aggregates by operation. They do not
return individual events. Studio may read a 30-day workspace aggregate but
does not record its own use.

## MCP behavior

The MCP surface is a thin typed projection over manifest, recipe, engine,
inspection, and aggregate telemetry packages. It does not invoke Cobra or parse
CLI JSON. All six tools declare read-only, non-destructive, idempotent,
closed-world annotations for their target repository effects:

- `bob_inspect` returns Bob state and offline binary availability without
  running integration probes;
- `bob_plan` returns a bounded action projection, truncation metadata, and a
  deterministic complete-plan digest without desired-content previews;
- `bob_check` returns compact convergence, conflict, and lock-drift state with
  the same plan digest;
- `bob_validate_manifest` strictly validates either an authorized workspace
  manifest or bounded inline YAML;
- `bob_recipe_describe` reports the embedded recipe and supported choices;
- `bob_stats` returns local aggregate usage for one authorized workspace or all
  retained pseudonymous workspaces.

A computed conflict is a successful plan result that contains no mutation
authority. If telemetry is opted in, MCP operations other than `bob_stats` may
append a local schema-bounded event; no MCP tool mutates a repository.

At server startup, the canonical default workspace is the exact allowlist.
Repeatable `--allow-workspace` values add exact existing workspaces.
`--allow-any-workspace` is the explicit broad mode and allows reading any
existing workspace accessible to the Bob process; it is not a sandbox.

MCP mutation is outside v0.2. Agents apply through the normal approved
`bob apply <workspace>` command and then plan again.

## Non-goals

Bob does not implement model inference, autonomous task planning, behavioral
acceptance truth, secrets, arbitrary plugins, background execution, remote
repository creation, commits, pushes, tags, package publication, or hosted
deployment in v0.2. It has no remote telemetry service. Repository mutation
over MCP is also outside v0.2.
