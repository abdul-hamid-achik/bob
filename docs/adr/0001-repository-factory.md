# ADR 0001: Build a deterministic repository factory

- Status: Accepted
- Date: 2026-07-12

## Context

Public developer-tool repositories repeatedly need the same foundations: command
structure, machine-readable output, documentation, tests, CI, security checks,
and release configuration. Copying these foundations by hand causes drift, while
opaque code generation makes upgrades difficult and can overwrite user work.

Bob also needs a clear boundary with adjacent tools. Agent runtimes already own
reasoning, model execution, approvals, and durable goals. Specialist tools own
code intelligence, secrets, behavior testing, evidence, and observability. A new
application DSL would require Bob to own business semantics and make existing
code difficult to adopt honestly.

## Decision

Bob will be a standalone, local-first repository factory and lifecycle
reconciler.

Version 0.1 will:

- read a strict, human-owned `bob.yaml` product contract;
- use the embedded, versioned `go-agent-tool` recipe to render desired files;
- compare desired whole files with repository state and the root `bob.lock`;
- emit a complete, deterministic, reviewable plan;
- apply only a complete conflict-free plan calculated from fresh state;
- record SHA-256 ownership for complete files in `bob.lock`;
- detect managed drift with `bob check`;
- expose human-readable and versioned JSON output from one CLI.

The implemented command surface is `new`, `init`, `plan`, `apply`, `check`,
`doctor`, `explain`, `recipe list`, `recipe show`, and `version`.

Version 0.1 does not require an LLM, daemon, MCP server, Studio, plugin runtime,
or persistent receipt store.

## Alternatives considered

### Agentic goal-to-product builder

Rejected as the core because it duplicates agent-runtime responsibilities and
makes Bob depend on model quality, execution recovery, and sandboxing. Agents may
drive Bob through its deterministic CLI/JSON interface.

### Intelligence-stack bootstrapper

Rejected as the whole product because it is narrower than the repeated
repository lifecycle problem and overlaps existing configuration tools. It may
become an optional recipe capability.

### Application DSL or general code generator

Rejected because Bob would need to own application semantics and behavior
preservation across frameworks. Bob scaffolds repository infrastructure and
explicit seams; it does not infer business logic.

## Ownership and safety consequences

- `new` and `init` preview unless `--write` is explicit.
- `plan` and `check` are read-only.
- Version 0.1 ownership covers complete files only.
- The root `bob.lock` binds each managed path to an exact content digest.
- Unmanaged differing files and user-modified managed files are conflicts.
- An unmanaged identical file may be classified as an `adopt` plan action; there
  is no general `bob adopt` command.
- Any conflict blocks apply before publication begins.
- Creates and updates use atomic per-file publication, and the lock is written
  last.
- Multi-file apply is not globally transactional; a crash may leave a partially
  published tree that the next plan must reconcile honestly.
- Workspace path traversal, reserved destinations, symlinks, and special files
  are rejected.
- Bob does not store secret values or execute generated project commands while
  planning or applying.
- Commit, push, tag, publication, and hosted-resource creation are outside the
  version 0.1 command surface.

## Consequences

### Positive

- Bob is useful without an agent or hosted service.
- Generated changes are reviewable and reproducible.
- Existing tools can drive Bob through a small CLI/JSON contract.
- Recipe upgrades appear as ordinary plans with visible conflicts.
- Generated repositories do not depend on Bob at runtime.

### Negative

- Safe reconciliation requires a lock and content hashing beyond one-shot
  template expansion.
- Whole-file ownership cannot update part of a user-owned file.
- Bob refuses transformations a best-effort generator might attempt.
- A process crash can interrupt a multi-file apply before `bob.lock` is
  published.
- The initial single-recipe scope serves only Go CLI projects.

These costs are accepted because predictable ownership and honest failure are
core product requirements.

## Non-goals for version 0.1

- autonomous feature implementation;
- application business-logic generation or migration;
- behavior-preserving import of arbitrary existing repositories;
- managed-block ownership;
- a public third-party recipe/plugin API;
- MCP or Studio surfaces;
- standalone `inspect`, `adopt`, or `verify` commands;
- persistent plans, execution history, or verification receipts;
- file deletion;
- implicit Git, release, package, or deployment mutations;
- a background service.

## Future decisions

Separate ADRs are required before adding:

1. managed-block or other partial-file ownership;
2. deletion, migration, or rollback semantics;
3. persistent verification commands and receipt redaction;
4. MCP mutation idempotency and authorization;
5. remote repository or release publication.
