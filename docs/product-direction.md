# Product direction

Bob is a local-first repository factory for public, agent-native developer
tools. It turns a strict `bob.yaml` product contract into a reviewable plan,
applies Bob-owned files safely, and detects drift.

Bob removes repeated repository plumbing without becoming another coding agent,
package manager, or application framework.

## Product options considered

### 1. Repository factory and lifecycle reconciler

Bob creates a repository from a declared product contract and keeps Bob-owned
infrastructure aligned as that contract evolves. Typical infrastructure includes
a CLI, structured output, documentation, behavior specs, CI, and release
configuration.

This is the selected direction. It is deterministic, independently useful, and
fits alongside agents and specialist tools instead of replacing them.

### 2. Agentic goal-to-product builder

Bob could accept a product goal and autonomously implement it. This offers a
compelling experience, but it duplicates the responsibilities of agent runtimes:
planning, approvals, model routing, durable execution, and recovery.

An agent may drive Bob's stable CLI/JSON contract. Model execution is not part
of Bob's core.

### 3. Intelligence-stack bootstrapper

Bob could install and configure code search, evidence, secrets, behavior testing,
and MCP tools for an existing project. This is useful but too narrow for the
whole product, and specialist tools already manage their own setup.

Stack setup may become an optional recipe capability after the repository
factory is established.

### 4. Application DSL or general code generator

Bob could define a language that compiles into applications. That would require
Bob to own application semantics and behavior-preserving migrations across many
frameworks. It would also make existing-project adoption prone to false claims
about preserved behavior.

Bob generates repository infrastructure and explicit integration seams. It does
not invent or reconstruct application business logic.

## Product principles

- **Local-first:** planning and generation work without a hosted service.
- **Deterministic:** the same manifest, recipe version, and observed files
  produce the same plan.
- **Plan before mutation:** users and agents can inspect proposed changes.
- **Whole-file ownership:** Bob 0.1 updates only complete files tracked by
  content hash in the repository-root `bob.lock`.
- **Agent-native:** the CLI and its versioned JSON envelope are first-class
  interfaces.
- **Honest degradation:** optional tools may be absent without being reported as
  successful.
- **Public by default:** the initial recipe includes the foundations for
  documentation, contribution, testing, security, and release packaging.

## Implemented version 0.1 workflow

```text
bob.yaml + embedded recipe + observed files + bob.lock
                         |
                         v
                    bob plan
                         |
                 review every action
                         |
                         v
                    bob apply
                         |
                         v
                    bob check
```

Version 0.1 exposes these commands:

```text
bob new <name>          preview or create a new repository
bob init [path]         preview or write bob.yaml in a repository
bob plan [path]         calculate changes without writing
bob apply [path]        apply one conflict-free, freshly calculated plan
bob check [path]        fail when managed state would change
bob doctor [path]       probe required and selected optional tools
bob explain             describe Bob's contract and boundaries
bob recipe list         list embedded recipes
bob recipe show <id>    describe an embedded recipe
bob version             print build metadata
```

`new` and `init` preview by default and require `--write` to create files.
`plan` and `check` are read-only. Every command supports the global `--json`
flag.

## Implemented version 0.1 scope

Version 0.1 proves one embedded recipe: `go-agent-tool`. It generates a Go/Cobra
CLI with JSON output, version and doctor commands, tests, public documentation,
Task tasks, CI, optional GoReleaser configuration, and an optional Glyphrun
behavior spec.

The manifest may select optional integration seams and development tools. Bob
can render those files and probe selected tools, but it does not run external
verification workflows or persist verification receipts.

Bob 0.1 uses whole-file ownership only. Planning classifies each desired file as
`create`, `adopt`, `unchanged`, `update`, or `conflict`. Here, `adopt` is a plan
action for an unmanaged regular file whose content already matches exactly; it
is not a standalone `bob adopt` command or a claim that an existing application
was behaviorally imported.

## Version 0.1 boundaries

Bob 0.1 does not:

- require an LLM or embedding model;
- infer or rewrite application business logic;
- overwrite an unmanaged differing file or a managed file changed by a person;
- own managed blocks inside otherwise user-owned files;
- delete generated files;
- expose MCP or Studio surfaces;
- provide standalone `inspect`, `adopt`, or `verify` commands;
- persist plans, execution histories, or verification receipts;
- implement a general plugin system;
- create commits, push branches, tag releases, publish packages, or create
  hosted resources;
- expose a background daemon.

## Future directions

Possible later additions include compact MCP and Studio projections over the
same core, richer repository inspection, an explicit existing-repository
adoption workflow, and bounded verification receipts. Those features must not
weaken deterministic planning, whole-file ownership, or explicit mutation.

## Success criteria

Bob 0.1 is successful when a user can preview and create a small public Go tool,
review the complete plan, apply it, run `bob check`, and understand which files
the root `bob.lock` proves Bob owns.

## Naming and positioning

The repository and command use `bob`, but the public descriptor is
"deterministic repository factory." The project does not use a construction
character, borrowed logo, entertainment catchphrase, or the name "Bob the
Builder." Before a broader branded launch, the package/binary name should get a
separate availability and trademark review; renaming the distribution does not
change the manifest or engine design.
