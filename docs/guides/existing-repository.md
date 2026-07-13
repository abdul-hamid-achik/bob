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
bob apply
bob check
git diff --stat
```

Review the resulting repository before committing. Future recipe upgrades can
use the hashes in `bob.lock` to update untouched managed files and leave human
changes alone. Bob remembers what it built. It does not remember what you
promise you'll clean up later.
