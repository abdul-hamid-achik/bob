# Security policy

## Supported versions

Before the first tagged release, security fixes are made on `main`. After the
first release, the latest release and `main` are supported. Forks and older
development snapshots are not maintained by this project.

## Reporting a vulnerability

Please do not open a public issue for a vulnerability that could put users or
their repositories at risk. Use GitHub's private vulnerability reporting for
this repository instead:

https://github.com/abdul-hamid-achik/bob/security/advisories/new

Include the affected version, reproduction steps, impact, and any suggested
mitigation. You should receive an acknowledgment within seven days.

## Security boundary

Bob generates and reconciles repository files. It does not sandbox commands,
manage secrets, or make agent authorization decisions. It does not run generated
project commands during `plan` or `apply`.

Bob's MCP tools are read-only, but they may read any existing workspace path the
hosting process can access. The server's `--workspace` is a default, not a
sandbox boundary. MCP annotations describe intended effects; MCPHub and agent
runtimes must still enforce their own authorization policies.

Plain `inspect` does not launch specialist tools. The explicit
`--probe-integrations` flag runs bounded Codemap and Vecgrep status commands;
those tools may open their own state, and Vecgrep may contact its configured
provider. Bob does not sandbox them.

Bob refuses path traversal, symlink destinations, and unmanaged-file
replacement. A repository owner must still review every plan before applying
it and review the resulting diff before publishing.

Bob uses pathname-based atomic publication. A concurrent same-user
process that replaces an already-checked parent directory is outside Bob's OS
containment boundary. Run `apply` only in a trusted workspace whose parent
directories are not being mutated concurrently.
