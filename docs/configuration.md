# Configuration & local telemetry

Bob keeps repository intent in `bob.yaml` and machine-local user preferences in
an XDG-style settings file. A missing user settings file is valid and leaves
telemetry disabled.

## Inspect the effective configuration

```bash
bob config show
bob config show --json
```

The result includes the exact settings file and Bob data, state, and cache
directories. Path resolution itself does not create them.

On a typical Unix account with no overrides, Bob resolves:

| Purpose | Default |
|---|---|
| Settings | `~/.config/bob/config.yaml` |
| Data | `~/.local/share/bob` |
| State | `~/.local/state/bob` |
| Cache | `~/.cache/bob` |

Bob follows this precedence:

| Purpose | Bob override | XDG base | Fallback |
|---|---|---|---|
| Settings directory | `BOB_CONFIG_DIR` | `XDG_CONFIG_HOME` | `~/.config` |
| Data directory | `BOB_DATA_DIR` | `XDG_DATA_HOME` | `~/.local/share` |
| State directory | `BOB_STATE_DIR` | `XDG_STATE_HOME` | `~/.local/state` |
| Cache directory | `BOB_CACHE_DIR` | `XDG_CACHE_HOME` | `~/.cache` |

Bob-specific directory overrides name the Bob directory itself. XDG variables
name a base, so Bob appends `bob`. `BOB_CONFIG` has highest precedence for the
exact settings filename. Every supplied path must be absolute; Bob rejects a
relative override rather than guessing.

## Initialize settings

Previewing is the default:

```bash
bob config init
```

Create the file explicitly:

```bash
bob config init --write
```

The initializer creates a private directory and mode-0600 file without
replacing an existing settings path. The schema-v1 document is strict:

```yaml
schema_version: 1
telemetry:
  enabled: false
  retention_days: 30
  max_events_per_day: 1000
```

Unknown fields, multiple YAML documents, unsupported schema versions, symlinked
settings files, and invalid limits are errors.

## Opt into local telemetry

Telemetry is disabled by default. To enable it while initializing a new file:

```bash
bob config init --telemetry --write
```

For an existing settings file, edit `telemetry.enabled` deliberately. The
boolean `BOB_TELEMETRY` environment variable overrides that field for the
current process and is useful for isolated automation:

```bash
BOB_TELEMETRY=true bob plan .
BOB_TELEMETRY=false bob plan .
```

Enabling telemetry does not enable a service or network request. Bob has no
telemetry transport. It writes one bounded JSON event per recorded operation
under `<state_dir>/telemetry/v1/YYYY-MM-DD/`, with private directories and
files. Recording is best-effort: a full, unavailable, or damaged telemetry
store cannot change the command's product result.

The durable event schema can contain only:

- CLI or MCP surface and a closed operation name;
- closed outcome and failure-reason categories;
- duration and aggregate create/update/adopt/unchanged/conflict counts;
- the selected recipe and version;
- a machine-local HMAC workspace pseudonym.

It has no field for a raw path, argument, filename, file or manifest content,
free-form label, model input/output, secret, or raw error. The canonical path is
used only as HMAC input and is not persisted. Pseudonyms are comparable only
within the local store that owns the random HMAC key.

Bob records `new`, `init`, `plan`, `apply`, `check`, `doctor`, and `inspect` on
the CLI. MCP records its corresponding inspect, plan, check, manifest
validation, and recipe-description operations. Configuration commands,
`stats`, `bob_stats`, Studio, version, and help do not record events.

Retention is a UTC calendar-day window and pruning occurs during enabled
recording. Disabling telemetry stops new events but does not silently delete
existing state. To remove it, stop Bob/MCP processes and delete the
`telemetry` directory beneath the exact `state_dir` reported by
`bob config show`.

## Read aggregate stats

```bash
bob stats .                 # current workspace, seven days
bob stats . --since 30d
bob stats --all --since all
bob stats . --json
```

A workspace and `--all` are mutually exclusive. Durations accept a positive Go
duration such as `24h`, a day count from `1d` through `365d`, or `all`. Output
contains totals and per-operation aggregates, never individual events. When
telemetry is disabled, the command returns an honest empty/disabled result.

The same aggregate is available to agents through `bob_stats`, subject to the
MCP server's workspace authority. See [MCPHub & local-agent](./guides/mcphub-local-agent.md).
