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
- **Whole-file ownership:** Bob updates only complete files tracked by
  content hash in the repository-root `bob.lock`.
- **Agent-native:** the CLI and its versioned JSON envelope are first-class
  interfaces.
- **Bounded guidance:** recipe metadata and workspace context reduce guessing
  without adding model inference, task memory, or behavioral verification.
- **Honest degradation:** optional tools may be absent without being reported as
  successful.
- **Public by default:** the initial recipe includes the foundations for
  documentation, contribution, testing, security, and release packaging.

## Implemented workflow

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

The version 0.1 baseline exposed these commands:

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

The current command surface also includes:

```text
bob inspect [path]      summarize Bob and integration readiness
bob config show|init    inspect or initialize XDG user settings
bob stats [path]        summarize opt-in local usage
bob studio [path]       open a read-only terminal operations board
bob mcp serve           expose nine typed repository-read-only MCP tools
bob context [path]      compile a bounded, read-only workspace contract
bob path <path>         classify one exact repository path through Bob ownership
bob playbook ...        resolve a closed typed procedure without executing it
```

`new` and `init` preview by default and require `--write` to create files.
`context`, `path`, `playbook`, `plan`, `check`, and plain `inspect` are read-only. Integration probes require
an explicit inspect flag. Normal CLI commands support the global `--json` flag;
MCP stdio reserves stdout for JSON-RPC.

Local telemetry is disabled by default and has no network transport. When
enabled, it provides privacy-bounded aggregates for people and agents without
storing repository paths, arguments, filenames, content, or raw errors. Studio
uses the deterministic engine and those aggregates as a projection; it does not
become a task runner.

## Current foundation

The `go-agent-tool` recipe generates a Go/Cobra
CLI with JSON output, version and doctor commands, tests, public documentation,
Task tasks, CI, optional GoReleaser configuration, and an optional Glyphrun
behavior spec.

The manifest may select optional integration seams and development tools. Bob
can render those files and probe selected tools, but it does not run external
verification workflows or persist verification receipts.

Bob uses whole-file ownership only. Planning classifies each desired file as
`create`, `adopt`, `unchanged`, `update`, or `conflict`. Here, `adopt` is a plan
action for an unmanaged regular file whose content already matches exactly; it
is not a standalone `bob adopt` command or a claim that an existing application
was behaviorally imported.

The current recipe version 4 keeps that ownership model and adds a deterministic
human-owned command-registration seam. New Cobra commands live in extension
files; the generated root and registry remain complete Bob-owned artifacts.
Version 4 also retains the public repository structure established in version
3: community templates, a Code of Conduct, Dependabot, non-mutating
verification, vulnerability scanning, pinned CI actions, release configuration,
and a security-patched Go baseline.

## Current boundaries

Bob currently does not:

- require an LLM or embedding model;
- infer or rewrite application business logic;
- overwrite an unmanaged differing file or a managed file changed by a person;
- own managed blocks inside otherwise user-owned files;
- delete generated files;
- expose Studio mutation or MCP mutation surfaces;
- provide standalone `adopt` or `verify` commands;
- persist plans, detailed execution histories, or verification receipts;
- infer natural-language intent or provide autonomous playbook routing;
- implement a general plugin system;
- create commits, push branches, tag releases, publish packages, or create
  hosted resources;
- expose a background daemon.

## Future directions

Possible later additions include digest-gated MCP apply, richer repository
inspection, an explicit existing-repository adoption workflow, and bounded
verification receipts. Those features must not weaken deterministic planning,
whole-file ownership, or explicit mutation.

## Success criteria

Bob is successful when a user can preview and create a small public Go tool,
review the complete plan, apply it, run `bob check`, and understand which files
the root `bob.lock` proves Bob owns.

## Naming and positioning

The repository and command use `bob`, but the public descriptor is
"deterministic repository factory." The project does not use a construction
character, borrowed logo, entertainment catchphrase, or the name "Bob the
Builder." Before a broader branded launch, the package/binary name should get a
separate availability and trademark review; renaming the distribution does not
change the manifest or engine design.
