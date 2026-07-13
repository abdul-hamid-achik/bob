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

Bob's MCP tools never mutate repositories. By default, they may read only the
canonical startup workspace. Repeatable `--allow-workspace` flags add exact
existing workspaces. `--allow-any-workspace` deliberately expands this to any
existing workspace path the hosting process can access; it is not a sandbox
boundary. MCP annotations describe intended repository effects; MCPHub and
agent runtimes must still enforce their own authorization policies.

Telemetry is disabled by default and has no network transport. When explicitly
enabled, CLI and MCP operations write bounded JSON events only beneath Bob's
resolved XDG state directory. The event schema has no field for paths,
arguments, filenames, file or manifest content, free-form labels, or raw error
messages. Workspace identity is a machine-local HMAC pseudonym whose secret key
is stored alongside the private telemetry metadata. Directories are mode 0700
and files are mode 0600. Recording is best-effort and cannot fail the requested
Bob operation.

`bob stats`, `bob_stats`, and Studio consume aggregates rather than exposing
individual events. Studio never records its own use or changes repository
files. With telemetry enabled, Bob runtime initialization may create private
store metadata in the XDG state directory even for a read-only repository
surface.

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

The settings and telemetry directories protect against symlinked terminal
directories and reject non-regular settings/event files. They do not protect
against another process already running as the same OS user. Treat access to
the Bob process and its XDG directories as access to local operational data.
