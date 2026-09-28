# Bob Console

A desktop companion for [Bob](https://bobcli.dev), the deterministic repository
factory. Bob Console does not reimplement Bob: every panel drives the real `bob`
binary through its versioned JSON envelope, or the real `bob mcp serve`
projection, and renders exactly what came back.

The complete feature inventory lives in [`FEATURES.md`](FEATURES.md): 53
surfaces — 26 CLI commands and subcommands, the nine MCP tools, and 18 console
capabilities built on Bob's contracts.

## What it is for

- **Review** a workspace before touching it: convergence verdict, classified
  plan actions, conflict families, the lock ledger, path ownership, and the
  bounded workspace context.
- **Drive** the lifecycle deliberately: apply bound to the plan digest you
  actually reviewed, recipe upgrades, removal previews, manifest edits with
  strict validation, and closed playbooks resolved with typed inputs.
- **Audit** everything: an append-only local ledger records argv, cwd, exit
  code, duration, and the observed plan digest for every invocation.
- **Work on Bob itself**: the development gates from the repository Taskfile
  stream live, the published docs render in place, and read-only git state is
  one panel away.

## Running it

```bash
cd desktop
npm install          # installs Electron and the renderer toolchain
node node_modules/electron/install.js   # npm may gate postinstall scripts
npm run dev          # Electron with HMR
```

Other entry points:

```bash
npm run build        # bundle main, preload, and renderer into out/
npm start            # run the built app
npm run web          # renderer-only browser preview (demo mode, port 5199)
npm test             # vitest: shared contracts, runner, MCP client, safety gates
npm run typecheck    # strict tsc for the node and web projects
npm run smoke        # boot the real app hidden and probe every surface
npm run dist         # electron-builder --mac --dir into release/
```

`npm run smoke` is the end-to-end gate: it starts Electron hidden, drives the
renderer bridge through every Bob surface against a workspace you choose, and
writes a JSON report:

```bash
node scripts/smoke.mjs /absolute/path/to/a/bob/workspace
```

## How it stays safe

- The main process only ever spawns the binary resolved in Settings (or the
  checkout's `./bin/bob`), plus `git` and `task` for their own read-mostly
  panels. The renderer cannot name a binary.
- Mutating invocations are refused unless the panel carries an explicit
  confirmation. Whether an invocation mutates is decided from the argv, not the
  feature: `--write` turns a preview into a write and `--dry-run` turns a
  mutation into a preview.
- Apply and upgrade pass `--expect-plan-digest` from the reviewed plan when the
  safety default is on, so a workspace that changed underneath is refused with
  exit 5 instead of being overwritten.
- `--probe-integrations` and MCP `--allow-any-workspace` sit behind a separate
  Settings opt-in that is off by default, and each use still confirms.
- Filesystem reads are confined to known workspaces, the resolved Bob checkout,
  and the console's own data directory; traversal and absolute paths are
  refused.
- The activity ledger is local, capped at 500 entries, and exportable. Nothing
  is sent anywhere.

## App icon

The icon is Bob's own brand mark — the brick-built lowercase `b` from
`docs/public/favicon.svg` (mutation-orange stem, blueprint-blue bowl on
graphite) — scaled to app-icon proportions with an Apple-style corner radius.

```bash
sh scripts/gen-icon.sh   # renders build/icon.svg to build/icon.icns + icon.png
```

The script needs macOS (`sips`, `iconutil`) and Chrome. electron-builder picks
`build/icon.icns` for macOS and `build/icon.png` elsewhere; unpackaged dev runs
set the same PNG on the Dock and the window.

## Layout

```text
src/shared    envelope types, feature registry, argv builders, diff, safety gates
src/main      window, IPC, process runner, MCP stdio client, docs, git, tasks
src/preload   the typed contextBridge surface (window.bob)
src/renderer  panels, design system, store; one panel per Bob surface
tests         vitest suite over the shared contracts and the main-process modules
scripts       demo-data generator and the end-to-end smoke runner
```

Demo mode replays envelopes captured from a real Bob checkout so every panel is
reviewable on a machine without Bob installed. It never writes files, starts
servers, or runs tasks, and it labels itself everywhere.
