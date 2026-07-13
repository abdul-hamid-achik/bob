# Security policy

## Reporting a vulnerability

Please do not open a public issue for a vulnerability that could put users or
their repositories at risk. Use GitHub's private vulnerability reporting for
this repository instead.

Include the affected version, reproduction steps, impact, and any suggested
mitigation. You should receive an acknowledgment within seven days.

## Security boundary

Bob generates and reconciles repository files. It does not sandbox commands,
manage secrets, or make agent authorization decisions. Version 0.1 does not run
generated project commands during `plan` or `apply`.

Bob refuses path traversal, symlink destinations, and unmanaged-file
replacement. A repository owner must still review every plan before applying
it and review the resulting diff before publishing.

Version 0.1 uses pathname-based atomic publication. A concurrent same-user
process that replaces an already-checked parent directory is outside Bob's OS
containment boundary. Run `apply` only in a trusted workspace whose parent
directories are not being mutated concurrently.
