# Bob manifest examples

These manifests are executable documentation. The test suite loads each file,
renders its recipe, and confirms that a fresh workspace produces a complete
conflict-free plan.

- [`minimal/bob.yaml`](minimal/bob.yaml) keeps every optional integration off
  while retaining the public repository foundation.
- [`integrated/bob.yaml`](integrated/bob.yaml) selects the local intelligence
  stack used by Bob's default manifest.

Preview either example in a temporary directory by copying it as `bob.yaml`,
then running `bob plan`. Review and replace the example module path before using
`bob apply` in a real repository.
