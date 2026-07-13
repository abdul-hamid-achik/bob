# Bob specification

Status: v0.1 draft

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

## Version 0.1 scope

Version 0.1 supports one recipe, `go-agent-tool`. It creates a Go command-line
project with a thin Cobra entrypoint, JSON output, version and doctor commands,
tests, public documentation, Task tasks, CI, optional GoReleaser configuration,
and an optional Glyphrun behavior spec.

Release archives target macOS and Linux. Windows publication is deferred until
Bob has a tested atomic file-replacement implementation for that platform.

The manifest schema, lock schema, and JSON output envelope use independent
integer schema versions starting at 1.

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

Any conflict blocks the complete apply. Bob does not delete files in v0.1.

## Filesystem safety

Desired paths are normalized relative paths inside the selected workspace.
Absolute paths, parent traversal, `.git`, `bob.yaml`, and `bob.lock` are reserved.
Pre-existing symlinks, directories, devices, sockets, and named pipes are conflicts.
Each file is published through a temporary sibling and atomic rename; the lock
is published last. Multi-file publication is not globally transactional, so a
process crash can leave a partially applied tree. A subsequent plan reports the
exact state and may safely adopt already-published matching files.

Path publication is pathname-based in v0.1. A concurrent same-user process that
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
- `explain` describes Bob's capability and ecosystem boundary.
- `recipe list|show` describes the embedded recipe catalog.
- `version` reports build metadata.

All agent-oriented commands support `--json`. JSON stdout uses a versioned
envelope and contains no progress logging or ANSI escapes.

## Non-goals

Bob does not implement model inference, autonomous task planning, behavioral
acceptance truth, secrets, arbitrary plugins, background execution, remote
repository creation, commits, pushes, tags, package publication, or hosted
deployment in v0.1.
