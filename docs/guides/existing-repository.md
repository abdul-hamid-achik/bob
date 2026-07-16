---
description: Bring Bob into a repository that already exists, without it bulldozing your existing files.
---

# Existing Repository

Bob does not care that your directory already has a life. Use `init` and it
writes only the human-owned manifest first, then waits for you to review any
ownership conflicts before it touches a single generated file.

## Initialize the manifest

```bash
cd path/to/acme-tool
bob init . \
  --name acme-tool \
  --module github.com/acme/acme-tool \
  --description "Agent-ready Acme CLI" \
  --write
```

This writes `bob.yaml`. It does not generate infrastructure or create a lock.

`bob init` detects the repository's stack and defaults to the matching recipe.
This walkthrough assumes a Go repository, which selects `go-agent-tool` — the
one recipe that requires `--module`. A TypeScript, JavaScript, Vue, Python,
Ruby, Lua, Rust, or static-web repository selects the matching seed-once stack
hygiene recipe instead; those recipes never conflict with an existing file,
because any existing destination satisfies a seed. The conflict walkthrough
below applies to the lock-owned recipes, `go-agent-tool` and `files`. Passing
a recipe that does not match the detected stack makes `--write` refuse unless
you add `--force`.

## Review the plan

```bash
bob plan
bob plan --content
```

`--content` includes bounded desired-content previews for create and update
actions. It does not make the command writable.

## Resolve an unmanaged-file conflict

Suppose the directory already contains a custom `README.md`. The recipe also
wants that path. Bob reports `conflict`: it cannot prove ownership, the content
differs, and it is not going to gamble on your behalf.

Choose deliberately:

1. Keep the custom file and do not adopt this recipe in the directory; Bob
   keeps refusing apply while the recipe targets it.
2. Move the custom file to a reviewed backup, rerun `bob plan`, and let Bob
   create its desired README.
3. If you intentionally make the file exactly match the desired content and
   mode, Bob classifies it as `adopt` and takes ownership from there.

Never delete or overwrite a conflict merely to make the command green. Decide
which system owns the path, then let the plan reflect that decision.

## Apply and check

When the complete plan is conflict-free:

```bash
bob plan --json
bob apply --expect-plan-digest sha256:<64-lowercase-hex> --json
bob check
git diff --stat
```

The guarded apply fresh-plans while holding Bob's apply lock and writes nothing
if the reviewed identity is stale. Review the resulting repository before
committing. Future recipe upgrades can
use the hashes in `bob.lock` to update untouched managed files and leave human
changes alone. Bob remembers what it built. It does not remember what you
promise you'll clean up later.
