# Bob manifest examples

These manifests are executable documentation. The test suite loads each file,
renders its recipe, and confirms that a fresh workspace produces a complete
conflict-free plan.

- [`minimal/bob.yaml`](minimal/bob.yaml) keeps every optional integration off
  while retaining the public repository foundation.
- [`integrated/bob.yaml`](integrated/bob.yaml) selects the local intelligence
  stack used by Bob's default manifest.
- [`files-minimal/bob.yaml`](files-minimal/bob.yaml) uses the `files` recipe
  to declare a tiny inline file tree, including a `0755` script and
  `${vars.*}` substitution.

Preview any example in a temporary directory by copying it as `bob.yaml`, then
running `bob plan`. Review and replace the example module path before using
`bob apply` on a `go-agent-tool` example in a real repository.
