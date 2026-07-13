## Outcome

Describe the user-visible result.

## Verification

- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `go vet ./...`
- [ ] `git diff --check`

## Safety

- [ ] Plan/apply ownership rules remain explicit.
- [ ] Tests use temporary paths and do not touch real user state.
- [ ] Public docs and wire formats are updated where needed.
