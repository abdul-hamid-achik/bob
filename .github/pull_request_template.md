## Outcome

Describe the user-visible result.

## Verification

- [ ] `task verify`
- [ ] `task docs-build` when documentation or navigation changed
- [ ] `task specs` when CLI or MCP behavior changed
- [ ] `task release-check` when packaging changed
- [ ] `git diff --check`

## Safety

- [ ] Plan/apply ownership rules remain explicit.
- [ ] Tests use temporary paths and do not touch real user state.
- [ ] Public docs and wire formats are updated where needed.
- [ ] Compatibility and recipe-version impact are described.
- [ ] Security, subprocess, and filesystem effects are described.

## Verification evidence

List the exact commands and user-visible behavior you verified.
