package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/guidance"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

// classifyErrorCode maps an error to Bob's closed JSON error-code vocabulary
// for a failure envelope's data.error.code field: missing_manifest,
// manifest_invalid, input_invalid, conflicts, workspace_invalid, or the
// command_failed fallback. It reuses ExitCode's classification (exitcode.go)
// as the primary signal and refines within the invalid-input bucket by
// message content, since exitError only carries a numeric exit code.
func classifyErrorCode(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if errors.Is(err, engine.ErrPlanDigestMismatch) {
		return "plan_digest_mismatch"
	}
	// Preserve precise transport-neutral guidance codes. Missing bob.yaml is
	// the one intentional refinement of manifest_invalid because it has a
	// distinct established CLI recovery branch.
	if errors.Is(err, os.ErrNotExist) && strings.Contains(msg, manifest.Filename) {
		return "missing_manifest"
	}
	if code, ok := guidance.ErrorCode(err); ok {
		switch code {
		case guidance.ErrorInputInvalid, guidance.ErrorWorkspaceInvalid, guidance.ErrorManifestInvalid:
			return code
		}
	}
	switch ExitCode(err) {
	case ExitConflicts:
		return "conflicts"
	case ExitInvalidInput:
		switch {
		case errors.Is(err, os.ErrNotExist) && strings.Contains(msg, manifest.Filename):
			return "missing_manifest"
		case strings.Contains(msg, "validate manifest"), strings.Contains(msg, "decode manifest"), strings.Contains(msg, "read manifest"):
			return "manifest_invalid"
		case strings.Contains(msg, "resolve workspace"), strings.Contains(msg, "workspace root"), strings.Contains(msg, "workspace path"):
			return "workspace_invalid"
		default:
			return "input_invalid"
		}
	default:
		if errors.Is(err, engine.ErrPlanConflicts) {
			return "conflicts"
		}
		if strings.Contains(msg, "workspace") {
			return "workspace_invalid"
		}
		return "command_failed"
	}
}

// withWorkspaceArg appends workspace to command as a trailing positional
// argument, unless workspace is unknown or is the implicit current
// directory, in which case command is returned unchanged.
func withWorkspaceArg(command, workspace string) string {
	if workspace == "" || workspace == "." {
		return command
	}
	return command + " " + workspace
}

// nextActionsForCode returns concrete, copy-pasteable corrective commands for
// one closed-vocabulary error code, threading workspace into the commands
// that take a path argument. Placeholders like <module> and <recipe-id> are
// intentionally left literal: Bob does not know those values, only the
// agent driving it does.
func nextActionsForCode(code, workspace string) []string {
	switch code {
	case "missing_manifest":
		return []string{
			"run: " + withWorkspaceArg("bob init", workspace) + " --module <module> --write",
			"run: bob learn --json",
		}
	case "manifest_invalid":
		return []string{
			"fix the problems listed in the message",
			"run: bob recipe show <recipe-id> for the schema",
		}
	case "conflicts":
		return []string{
			"run: " + withWorkspaceArg("bob plan", workspace) + " --json and inspect actions with kind=conflict",
			"resolve each conflict, then rerun " + withWorkspaceArg("bob apply", workspace),
		}
	case "workspace_invalid":
		return []string{
			"fix the workspace path problem noted in the message",
			"run: bob learn --json",
		}
	case "input_invalid":
		return []string{
			"fix the invalid argument or flag noted in the message",
			"run: bob learn --json",
		}
	case "plan_digest_mismatch":
		return []string{
			"run: " + withWorkspaceArg("bob plan", workspace) + " --json",
			"review the new plan before applying it",
		}
	default:
		return []string{"run: bob learn --json"}
	}
}

// nextActionsForFailure computes next_actions for err. It is the single
// source both the JSON failure envelope and the human-mode stderr "next:"
// lines draw from. Exit code ExitDrift (check found drift without an
// ownership conflict) is not itself part of the closed error-code
// vocabulary, but still deserves its own precise suggestion.
func nextActionsForFailure(err error, workspace string) []string {
	if ExitCode(err) == ExitDrift {
		return []string{"run: " + withWorkspaceArg("bob apply", workspace)}
	}
	return nextActionsForCode(classifyErrorCode(err), workspace)
}

// printHumanFailure writes the same "bob: <error>" line cmd/bob has always
// printed, followed by "next: ..." corrective steps so a weak model that
// only reads stderr text (no --json) can still self-recover.
func printHumanFailure(w io.Writer, err error, workspace string) {
	_, _ = fmt.Fprintln(w, "bob:", err)
	for _, step := range nextActionsForFailure(err, workspace) {
		_, _ = fmt.Fprintf(w, "next: %s\n", step)
	}
}
