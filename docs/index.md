---
layout: home
titleTemplate: false

hero:
  name: Bob
  text: Deterministic repository construction
  tagline: Turn a small product contract into a reviewable, public-ready repository without handing ownership to an agent.
  actions:
    - theme: brand
      text: Get started
      link: /getting-started
    - theme: alt
      text: Understand safety
      link: /ownership-and-safety

features:
  - title: Plan before mutation
    details: Preview every create, update, adoption, and conflict before Bob writes a file.
  - title: Ownership you can audit
    details: bob.lock records content hashes for whole files Bob may safely reconcile.
  - title: Agent-native, model-free
    details: Use stable JSON and read-only MCP tools from local-agent, MCPHub, or another runtime.
---

Bob compiles `bob.yaml` through a versioned recipe, compares the result with the
working tree and `bob.lock`, and applies only changes whose ownership it can
prove. It does not infer application behavior, run a model, or declare that a
generated tool works.

```text
bob.yaml + recipe + bob.lock + working tree
                         |
                         v
               create / update / conflict
                         |
                    explicit apply
                         |
                         v
                 converged repository
```

## Choose a workflow

| Goal | Start here |
|---|---|
| Create a new public Go tool | [Getting Started](./getting-started.md) |
| Bring Bob into an existing directory | [Existing Repository](./guides/existing-repository.md) |
| Understand a conflict or lock decision | [Ownership & Safety](./ownership-and-safety.md) |
| Connect Bob to an agent | [MCPHub & local-agent](./guides/mcphub-local-agent.md) |

> Bob is early alpha. There is no tagged release yet. Build from a checkout,
> review every plan, and inspect the resulting repository diff before publishing.

The normative product contract remains the root
[`SPEC.md`](https://github.com/abdul-hamid-achik/bob/blob/main/SPEC.md). Release
history remains in
[`CHANGELOG.md`](https://github.com/abdul-hamid-achik/bob/blob/main/CHANGELOG.md).
