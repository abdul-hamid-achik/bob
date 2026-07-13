# Existing Repository

Use `init` when the target directory already exists. Bob writes only the
human-owned manifest first, then lets you review ownership conflicts.

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
wants that path, so Bob reports `conflict`: it cannot prove ownership and the
content differs.

Choose deliberately:

1. Keep the custom file and do not adopt this recipe in the directory; Bob will
   continue to refuse apply while the recipe targets it.
2. Move the custom file to a reviewed backup, rerun `bob plan`, and let Bob
   create its desired README.
3. If you intentionally make the file exactly match the desired content and
   mode, Bob can classify it as `adopt`.

Never delete or overwrite a conflict merely to make the command green. Decide
which system should own the path.

## Apply and check

When the complete plan is conflict-free:

```bash
bob apply
bob check
git diff --stat
```

Review the resulting repository before committing. Future recipe upgrades can
use the hashes in `bob.lock` to update untouched managed files and leave human
changes alone.
