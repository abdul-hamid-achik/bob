package mcp

import (
	"context"
	"errors"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Closed bob_apply outcomes. Every outcome is a typed structured result.
const (
	applyOutcomeApplied            = "applied"
	applyOutcomePlanDigestMismatch = "plan_digest_mismatch"
	applyOutcomeConflicts          = "conflicts"
	applyOutcomeWorkspaceBusy      = "workspace_busy"
	applyOutcomeUnauthorized       = "apply_unauthorized"
	applyOutcomeWorkspaceInvalid   = "workspace_invalid"
	applyOutcomeInputInvalid       = "input_invalid"
	applyOutcomeManifestInvalid    = "manifest_invalid"
	applyOutcomeCancelled          = "cancelled"
	applyOutcomeFailed             = "apply_failed"
)

type ApplyInput struct {
	Workspace        string `json:"workspace,omitempty" jsonschema:"existing repository directory authorized for mutation with --allow-apply-workspace; defaults to the startup workspace"`
	ExpectPlanDigest string `json:"expect_plan_digest" jsonschema:"required exact reviewed plan digest: sha256: followed by 64 lowercase hexadecimal characters, copied from plan_digest_qualified of bob_plan or bob_check"`
}

// ApplyConflict is one blocked path from a refused apply.
type ApplyConflict struct {
	Path   string `json:"path"`
	Code   string `json:"code,omitempty"`
	Reason string `json:"reason,omitempty"`
}

type ApplyOutput struct {
	SchemaVersion  int                  `json:"schema_version"`
	OK             bool                 `json:"ok"`
	Outcome        string               `json:"outcome"`
	Workspace      string               `json:"workspace,omitempty"`
	Authority      AuthorityInfo        `json:"authority"`
	ApplyAuthority ApplyAuthorityInfo   `json:"apply_authority"`
	Receipt        *engine.ApplyReceipt `json:"receipt,omitempty"`
	// ExpectedPlanDigest, ActualPlanDigest, and Converged describe a
	// plan_digest_mismatch. Converged reports only that the fresh plan is
	// clean now; Bob keeps no apply history, so it never claims the reviewed
	// plan was "already applied".
	ExpectedPlanDigest string                     `json:"expected_plan_digest,omitempty"`
	ActualPlanDigest   string                     `json:"actual_plan_digest,omitempty"`
	Converged          *bool                      `json:"converged,omitempty"`
	Conflicts          []ApplyConflict            `json:"conflicts"`
	Retryable          bool                       `json:"retryable"`
	NextActions        []inspectpkg.CommandAction `json:"next_actions"`
	Error              *ErrorInfo                 `json:"error,omitempty"`
}

// handleApply is the only MCP handler that may mutate a repository. Ordering:
//
//  1. Authority: the workspace must be in the explicit apply allowlist, which
//     is empty by default. Unauthorized calls never reach the engine.
//  2. Input: the digest must be the exact sha256:<64 lowercase hex> form.
//  3. Cancellation is honored only here, before the engine is entered. The
//     engine stages and publishes without a context, so once it starts it
//     runs to completion; it never aborts mid-publication in a way that could
//     leave published files without an updated lock.
//  4. engine.ApplyWorkspaceWithOptions owns everything else: it takes the
//     workspace lock with a single exclusive create (contention fails
//     immediately with ErrWorkspaceBusy; this handler never waits or retries),
//     reloads bob.yaml under that lock, fresh-plans, and compares the digest
//     before any conflict handling, staging, or publication.
//
// Nothing on this path writes to stdout; results travel only as JSON-RPC.
func (s *Server) handleApply(ctx context.Context, _ *sdkmcp.CallToolRequest, in ApplyInput) (*sdkmcp.CallToolResult, *ApplyOutput, error) {
	started := time.Now()
	root := ""
	outcome, reason := telemetry.OutcomeError, telemetry.ReasonInternal
	var recordedCounts inspectpkg.ActionCounts
	recipeID := ""
	recipeVersion := 0
	defer func() {
		s.recordOperation(ctx, telemetry.OperationApply, root, outcome, reason, recordedCounts, recipeID, recipeVersion, started)
	}()

	var authErr *authorityError
	root, authErr = s.authority.resolveApply(s.apply, in.Workspace)
	if authErr != nil {
		reason = reasonFromToolCode(authErr.code)
		return s.applyFailure("", authErr.code, authErr.message)
	}
	if err := engine.ValidateExpectedPlanDigest(in.ExpectPlanDigest); err != nil {
		reason = telemetry.ReasonInvalidInput
		return s.applyFailure(root, applyOutcomeInputInvalid, "expect_plan_digest must be sha256: followed by 64 lowercase hexadecimal characters")
	}
	if err := ctx.Err(); err != nil {
		outcome = telemetry.OutcomeCancelled
		return s.applyFailure(root, applyOutcomeCancelled, "request was cancelled before apply started; nothing was written")
	}

	result, err := engine.ApplyWorkspaceWithOptions(root, engine.ApplyOptions{ExpectedPlanDigest: in.ExpectPlanDigest})
	recipeID, recipeVersion = result.Plan.Recipe.ID, result.Plan.Recipe.Version
	if result.Plan.Recipe.ID != "" {
		recordedCounts, _, _, _ = summarizePlan(root, result.Plan, engine.DigestPlan(result.Plan))
	}
	if err == nil {
		outcome, reason = telemetry.OutcomeOK, ""
		receipt := result.Receipt
		out := s.newApplyOutput(root, applyOutcomeApplied)
		out.OK = true
		out.Receipt = &receipt
		out.NextActions = append(out.NextActions, inspectpkg.CommandAction{
			Reason: "confirm convergence with bob_check or the CLI", CWD: root,
			Argv: receipt.NextCheck.Argv, RequiresExplicitAuthority: false,
		})
		return &sdkmcp.CallToolResult{}, out, nil
	}

	var mismatch *engine.PlanDigestMismatchError
	switch {
	case errors.As(err, &mismatch):
		outcome, reason = telemetry.OutcomeDrift, telemetry.ReasonOwnershipConflict
		converged := result.Plan.Clean()
		res, out, _ := s.applyFailure(root, applyOutcomePlanDigestMismatch, mismatch.Error())
		out.ExpectedPlanDigest = mismatch.ExpectedPlanDigest
		out.ActualPlanDigest = mismatch.ActualPlanDigest
		out.Converged = &converged
		if !converged {
			out.NextActions = append(out.NextActions, replanAction(root, "review the current plan with bob_plan before applying its new digest"))
		}
		return res, out, nil
	case errors.Is(err, engine.ErrPlanConflicts):
		outcome, reason = telemetry.OutcomeConflict, telemetry.ReasonOwnershipConflict
		res, out, _ := s.applyFailure(root, applyOutcomeConflicts, "plan contains conflicts; nothing was written")
		for _, action := range result.Plan.Actions {
			if action.Kind == engine.ActionConflict {
				out.Conflicts = append(out.Conflicts, ApplyConflict{Path: action.Path, Code: action.Code, Reason: action.Reason})
			}
		}
		out.NextActions = append(out.NextActions, replanAction(root, "resolve Bob ownership conflicts, then replan"))
		return res, out, nil
	case errors.Is(err, engine.ErrWorkspaceBusy):
		res, out, _ := s.applyFailure(root, applyOutcomeWorkspaceBusy, "another mutating Bob operation holds the workspace lock; nothing was written")
		out.Retryable = true
		return res, out, nil
	case errors.Is(err, engine.ErrInvalidPlanDigest):
		reason = telemetry.ReasonInvalidInput
		return s.applyFailure(root, applyOutcomeInputInvalid, err.Error())
	case errors.Is(err, engine.ErrWorkspaceContract):
		reason = telemetry.ReasonInvalidManifest
		return s.applyFailure(root, applyOutcomeManifestInvalid, err.Error())
	default:
		return s.applyFailure(root, applyOutcomeFailed, err.Error())
	}
}

func replanAction(root, reason string) inspectpkg.CommandAction {
	return inspectpkg.CommandAction{
		Reason: reason, CWD: root, Argv: []string{"bob", "plan", root}, RequiresExplicitAuthority: false,
	}
}

func (s *Server) newApplyOutput(root, outcome string) *ApplyOutput {
	return &ApplyOutput{
		SchemaVersion: toolSchemaVersion, Outcome: outcome, Workspace: root,
		Authority: s.authority.info(root), ApplyAuthority: s.apply.info(),
		Conflicts: []ApplyConflict{}, NextActions: []inspectpkg.CommandAction{},
	}
}

func (s *Server) applyFailure(root, code, message string) (*sdkmcp.CallToolResult, *ApplyOutput, error) {
	out := s.newApplyOutput(root, code)
	out.Error = &ErrorInfo{Code: code, Message: message}
	return &sdkmcp.CallToolResult{IsError: true}, out, nil
}
