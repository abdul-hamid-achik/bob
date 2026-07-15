---
description: The files recipe — declare any file tree in bob.yaml and get plan/apply/lock safety without a Go/Cobra scaffold.
---

# Build any repository

`go-agent-tool` has opinions: Go, Cobra, a CLI shape. Most of the time that's
the point. Sometimes it isn't — you need a web service's config bundle, a set
of scripts, a non-Go project, or just three files in the right place with the
right permissions. `recipe: files` is Bob's answer: declare the tree, get the
same plan/apply/lock ownership safety, own none of the Go scaffolding you
didn't ask for.

## When to reach for `files` instead of `go-agent-tool`

- You're not building a Go/Cobra CLI. `go-agent-tool` isn't going to bend
  into a shape it wasn't designed for, and `files` doesn't care what language
  or shape you're building.
- You want Bob's ownership guarantees — safe create, safe update, refuse on
  conflict — applied to a tree you already know the contents of.
- You want a *repeatable* materialization of a small file set (config,
  scripts, boilerplate) with variable substitution, not a code generator.

Reach for `go-agent-tool` instead when you actually want the thing it builds:
a public-ready Go CLI with CI, release plumbing, and the ecosystem seams. It
still only comes from `bob new`/`bob init`. `files` manifests are never
scaffolded for you — you write them, or your agent does. That's the deal:
`bob recipe show files` prints a copyable example so nobody has to derive the
schema from source.

## Write the manifest by hand

There is no `bob new --recipe files`. Start from the example:

```bash
bob recipe show files
```

```text
files@1
  declare any file tree inline; bob materializes it with plan/apply safety

  Manifest schema:
    vars: map[string]string; keys must match ^[a-z][a-z0-9_]*$; declared-but-unused vars are fine
    files: list of {path, mode, content}; path must resolve inside the workspace; mode is an optional 3-4 digit octal permission string like "0644" (default "0644"; setuid, setgid, and sticky bits are rejected); content is written verbatim after substitution
  ...
```

Copy the example into a fresh `bob.yaml`:

```yaml
schema_version: 1
recipe: files
product:
  name: my-app
  description: A generated web service
vars:
  project_name: my-app
  port: "8080"
files:
  - path: package.json
    content: |
      {"name": "${vars.project_name}"}
  - path: scripts/run.sh
    mode: "0755"
    content: |
      #!/usr/bin/env bash
      echo "listening on ${vars.port}"
```

## Plan, apply, converge

```bash
bob plan .
```

```text
create     package.json
create     scripts/run.sh
lock       bob.lock

2 create, 0 update, 0 adopt, 0 unchanged, 0 conflict
```

Nothing is written yet. Apply it:

```bash
bob apply .
```

```text
applied: 2 written, 0 adopted, 0 unchanged; lock written: true
```

`scripts/run.sh` lands with mode `0755`, exactly as declared — check it
yourself, Bob isn't asking you to trust it:

```bash
stat -f "%Lp" scripts/run.sh   # 755
```

Run apply again. This is the whole pitch:

```bash
bob apply .
```

```text
applied: 0 written, 0 adopted, 2 unchanged; lock written: false
```

Nothing moved. `bob check .` agrees and exits `0`.

## Edit content, watch the plan notice

Change the port in `bob.yaml` from `"8080"` to `"9090"` and plan again:

```bash
bob plan .
```

```text
unchanged  package.json
update     scripts/run.sh
lock       bob.lock

0 create, 1 update, 0 adopt, 1 unchanged, 0 conflict
```

`plan --content --json` shows both sides of that update, bounded to 2048
bytes each:

```json
{
  "path": "scripts/run.sh",
  "kind": "update",
  "code": "content_update",
  "desired_preview": "#!/usr/bin/env bash\necho \"listening on 9090\"\n",
  "current_preview": "#!/usr/bin/env bash\necho \"listening on 8080\"\n",
  "reason": "managed file still matches bob.lock and may be updated safely"
}
```

`bob apply .` writes the one changed file and updates the lock. `package.json`
is untouched — Bob only rewrites what actually changed.

## Substitution rules, precisely

`${vars.key}` is replaced by a single, deterministic, literal-replacement
regex pass. That's the entire rule set:

- The pattern is `\$\{vars\.([a-z][a-z0-9_]*)\}`. Anything else — including a
  shell script's own `${FOO}` or `$HOME` — does not match, so it passes
  through untouched. `files` is not a template engine wearing a disguise;
  there are no loops, no conditionals, no includes.
- Every declared var must match `^[a-z][a-z0-9_]*$`. A var you declare but
  never reference is fine — Bob doesn't audit your unused variables.

## The unresolved-var failure, shown honestly

Reference a var you never declared, and rendering fails loudly instead of
silently leaving a blank:

```yaml
files:
  - path: extra.txt
    content: "${vars.missing_one} and ${vars.missing_two}"
```

```bash
bob plan .
```

```text
bob: plan: render files: unresolved variable reference(s): extra.txt: ${vars.missing_one}; extra.txt: ${vars.missing_two}
next: fix the invalid argument or flag noted in the message
next: run: bob learn --json
```

Exit code `4`, error code `input_invalid`. Every unresolved reference across
every file is collected, sorted, and deduped into one message with its file
path attached — you get the whole list in one failed `plan`, not one
frustrating fix-and-rerun cycle per variable.

## The ownership trade-off, stated plainly

`go-agent-tool` ships an upstream template: bump the recipe version and
previously generated files can be carried forward, because Bob knows what
they're *for*. `files` has no such upstream. Bob owns existence, mode, and
byte-for-byte convergence for every declared path — the same safety net as
any other recipe — but it does not maintain or upgrade the *content* over
time, because there is no template to carry it forward from. You wrote
`{"name": "${vars.project_name}"}`; you decide what it becomes next release.
This isn't a missing feature bolted on later — it's the honest shape of
"you declared arbitrary content, so you own what it means."

## Path safety

Identical to every other recipe, because it's the same engine underneath:

- no absolute paths, no `..` escaping the workspace;
- no targeting `.git`, `bob.yaml`, or `bob.lock`;
- a pre-existing symlink or special file (device, socket, named pipe) at a
  destination path is reported as a per-path `conflict` — the rest of the
  plan still shows.

## For agents

1. Run `bob learn --json` once, at session start. It lists both recipes
   (`files@1`, `go-agent-tool@4`), the exit-code and error-code maps, and the
   action-code vocabulary — the brief you're about to need.
2. Run `bob context --json` for the active `files@1` contract. Its generic
   capability and artifact metadata deliberately does not infer language,
   framework, commands, or business meaning from declared content.
3. Plan before proposing anything: `bob plan --json`. Prefer
   `bob plan --content --json` when you need to show a human or another agent
   what would actually change.
4. In a polling or retry loop, use `bob check --json --conflicts-only`. It
   trims the response to the paths that would actually block an apply,
   instead of hauling the full unchanged-action list back every iteration.
5. Branch on the stable `code` field (`missing`, `content_update`,
   `mode_drift`, `in_sync`, `identical_content`, or a conflict code like
   `unmanaged_differs`), never on the prose `reason` — the reason string is
   for a human's screen, the code is the contract.
6. When authority is tied to a reviewed plan, call
   `bob apply --expect-plan-digest sha256:<digest> --json`; Bob recomputes the
   complete plan under its apply lock and refuses a stale digest with zero
   repository writes.
7. If `bob apply` refuses, its failure `data.conflicts` array already lists
   every blocked path and code — no need to replan just to find out why.

See [Bob for coding agents](../agents.md) for the full exit-code table,
error-code vocabulary, and the corresponding recovery playbook, and the
[Manifest Reference](../reference/manifest.md#the-files-recipe) for the
complete field-by-field schema.
