# Bob Studio

Studio is Bob's interactive, repository-read-only operations board. It presents
one coherent offline snapshot from the same inspection and planning packages as
the CLI.

```bash
bob studio .
```

Studio requires interactive stdin and stdout and refuses `TERM=dumb`. It does
not support `--json`; use `bob inspect --json`, `bob plan --json`, or
`bob stats --json` for automation.

## Views

| View | Shows |
|---|---|
| Overview | Repository readiness, convergence, lock state, action totals, offline integration availability, warnings, and next actions. |
| Plan | Bob's exact plan actions and selected-action hashes, modes, reason, and bounded desired preview when present. |
| Stats | A 30-day aggregate for the selected workspace when local telemetry is enabled. |

The Stats view reads only aggregates. Studio does not record its own use. If
telemetry is disabled, the view reports no operation history. As with every Bob
command, enabled telemetry runtime initialization may create private store
metadata under the XDG state directory; it never writes a repository file.

## Keys

| Key | Action |
|---|---|
| `1`, `2`, `3` | Open Overview, Plan, or Stats. |
| `tab`, `shift+tab` | Move between views. |
| `↑`/`k`, `↓`/`j` | Select a plan action or scroll the active view. |
| `g`/`home`, `G`/`end` | Move to the first or last item. |
| `PgUp`/`PgDn`, `ctrl+u`/`ctrl+d` | Scroll by a page. |
| `a` | Toggle attention-only versus all plan actions. |
| `r` | Refresh one coherent offline snapshot. |
| `?` | Open help. |
| `esc` | Close help or return to Overview. |
| `q`, `ctrl+c` | Quit. |

Use `--single-pane` to force the compact accessible layout:

```bash
bob studio . --single-pane
```

Studio also switches to compact layouts for narrow or short terminals. A
failed refresh keeps the previous snapshot visible and marks it stale instead
of replacing useful state with an empty screen.

## Authority boundary

Studio performs repository reads only. It has no apply, shell, editor, index,
search, integration-probe, repair, or approval shortcut. Suggested next actions
remain text for a person or agent to review and invoke through the normal
approved command path.

Codemap and Vecgrep availability is discovered offline. Studio never runs their
status commands; use `bob inspect --probe-integrations` only when that extra
subprocess authority is intended.
