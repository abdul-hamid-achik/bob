// Package mcp exposes Bob's read-only repository inventory and planning surface
// over stdio. It never invokes Cobra, parses CLI output, or writes to stdout
// outside the MCP transport.
package mcp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/version"
	"github.com/abdul-hamid-achik/bob/internal/workspace"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const instructions = `Bob is a deterministic repository factory. Use bob_inspect first to read
Bob-managed drift and offline integration availability, then bob_plan to review every proposed
file action. These MCP tools never mutate. To apply a conflict-free plan, use the normal approved
shell path with "bob apply <workspace>", then call bob_plan again. Codemap and Vecgrep search,
impact analysis, indexing, and verification remain owned by those tools and Cortex.`

type Server struct {
	defaultWorkspace string
	runner           inspectpkg.Runner
	srv              *sdkmcp.Server
}

type WorkspaceInput struct {
	Workspace string `json:"workspace,omitempty" jsonschema:"existing repository directory; defaults to the server startup workspace"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InspectOutput struct {
	SchemaVersion int                `json:"schema_version"`
	OK            bool               `json:"ok"`
	Report        *inspectpkg.Report `json:"report,omitempty"`
	Error         *ErrorInfo         `json:"error,omitempty"`
}

type PlanAction struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	CurrentSHA256 string `json:"current_sha256,omitempty"`
	DesiredSHA256 string `json:"desired_sha256,omitempty"`
	LockedSHA256  string `json:"locked_sha256,omitempty"`
	CurrentMode   string `json:"current_mode,omitempty"`
	DesiredMode   string `json:"desired_mode,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

type PlanOutput struct {
	SchemaVersion int                        `json:"schema_version"`
	OK            bool                       `json:"ok"`
	Workspace     string                     `json:"workspace"`
	Clean         bool                       `json:"clean"`
	LockChanged   bool                       `json:"lock_changed"`
	ConflictCount int                        `json:"conflict_count"`
	Counts        inspectpkg.ActionCounts    `json:"counts"`
	Actions       []PlanAction               `json:"actions"`
	Warnings      []string                   `json:"warnings"`
	NextActions   []inspectpkg.CommandAction `json:"next_actions"`
	Error         *ErrorInfo                 `json:"error,omitempty"`
}

// NewServer constructs the compact read-only MCP surface.
func NewServer(defaultWorkspace string, runner inspectpkg.Runner) (*Server, error) {
	if defaultWorkspace == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve startup workspace: %w", err)
		}
		defaultWorkspace = cwd
	}
	canonical, err := workspace.Resolve(defaultWorkspace, true)
	if err != nil {
		return nil, fmt.Errorf("resolve startup workspace: %w", err)
	}
	if runner == nil {
		runner = inspectpkg.ExecRunner{}
	}
	s := &Server{defaultWorkspace: canonical, runner: runner}
	s.srv = sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "bob", Version: version.Version},
		&sdkmcp.ServerOptions{Instructions: instructions},
	)
	s.register()
	return s, nil
}

// Run serves newline-delimited MCP JSON-RPC over stdio until cancellation.
func (s *Server) Run(ctx context.Context) error {
	return s.serve(ctx, &sdkmcp.StdioTransport{})
}

func (s *Server) serve(ctx context.Context, transport sdkmcp.Transport) error {
	return s.srv.Run(ctx, transport)
}

func (s *Server) register() {
	sdkmcp.AddTool(s.srv, readOnlyTool(
		"bob_inspect", "Inspect a Bob workspace",
		"Summarize Bob manifest and drift state plus offline availability of selected Codemap and Vecgrep binaries. Does not run specialist status commands, search, index, verify, or mutate.",
	), s.handleInspect)
	sdkmcp.AddTool(s.srv, readOnlyTool(
		"bob_plan", "Plan repository construction",
		"Return a compact deterministic action list for the current bob.yaml and bob.lock. Conflicts are useful plan results and block the separate approved CLI apply path.",
	), s.handlePlan)
}

func readOnlyTool(name, title, description string) *sdkmcp.Tool {
	destructive := false
	openWorld := false
	return &sdkmcp.Tool{
		Name: name, Title: title, Description: description,
		Annotations: &sdkmcp.ToolAnnotations{
			Title: title, ReadOnlyHint: true, DestructiveHint: &destructive,
			IdempotentHint: true, OpenWorldHint: &openWorld,
		},
	}
}

func (s *Server) handleInspect(ctx context.Context, _ *sdkmcp.CallToolRequest, in WorkspaceInput) (*sdkmcp.CallToolResult, *InspectOutput, error) {
	root := s.resolveInput(in.Workspace)
	report, err := inspectpkg.Run(ctx, root, inspectpkg.Options{}, s.runner)
	if err != nil {
		out := &InspectOutput{SchemaVersion: 1, OK: false, Error: &ErrorInfo{Code: "workspace_invalid", Message: err.Error()}}
		return &sdkmcp.CallToolResult{IsError: true}, out, nil
	}
	out := &InspectOutput{SchemaVersion: 1, OK: true, Report: &report}
	return &sdkmcp.CallToolResult{}, out, nil
}

func (s *Server) handlePlan(_ context.Context, _ *sdkmcp.CallToolRequest, in WorkspaceInput) (*sdkmcp.CallToolResult, *PlanOutput, error) {
	root := s.resolveInput(in.Workspace)
	canonical, err := workspace.Resolve(root, true)
	if err != nil {
		return planFailure(root, "workspace_invalid", err)
	}
	m, err := manifest.Load(canonical)
	if err != nil {
		return planFailure(canonical, "manifest_invalid", err)
	}
	artifacts, err := recipe.Render(m)
	if err != nil {
		return planFailure(canonical, "recipe_invalid", err)
	}
	plan, err := engine.Plan(canonical, m, artifacts)
	if err != nil {
		return planFailure(canonical, "plan_failed", err)
	}
	out := projectPlan(canonical, plan)
	return &sdkmcp.CallToolResult{}, out, nil
}

func (s *Server) resolveInput(input string) string {
	if input == "" {
		return s.defaultWorkspace
	}
	if filepath.IsAbs(input) {
		return input
	}
	return filepath.Join(s.defaultWorkspace, input)
}

func projectPlan(root string, plan engine.PlanResult) *PlanOutput {
	out := &PlanOutput{
		SchemaVersion: 1, OK: true, Workspace: root, LockChanged: plan.LockChanged,
		ConflictCount: plan.ConflictCount, Actions: make([]PlanAction, 0, len(plan.Actions)),
		Warnings: []string{}, NextActions: []inspectpkg.CommandAction{},
	}
	for _, action := range plan.Actions {
		projected := PlanAction{
			Path: action.Path, Kind: string(action.Kind), CurrentSHA256: action.CurrentSHA256,
			DesiredSHA256: action.DesiredSHA256, LockedSHA256: action.LockedSHA256,
			DesiredMode: fmt.Sprintf("%04o", action.DesiredMode.Perm()), Reason: action.Reason,
		}
		if action.CurrentMode != 0 {
			projected.CurrentMode = fmt.Sprintf("%04o", action.CurrentMode.Perm())
		}
		out.Actions = append(out.Actions, projected)
		switch action.Kind {
		case engine.ActionCreate:
			out.Counts.Create++
		case engine.ActionUpdate:
			out.Counts.Update++
		case engine.ActionAdopt:
			out.Counts.Adopt++
		case engine.ActionUnchanged:
			out.Counts.Unchanged++
		case engine.ActionConflict:
			out.Counts.Conflict++
		}
	}
	out.Clean = !plan.LockChanged && out.Counts.Unchanged == len(plan.Actions)
	switch {
	case plan.HasConflicts():
		out.Warnings = append(out.Warnings, fmt.Sprintf("%d conflict(s) block apply", plan.ConflictCount))
		out.NextActions = append(out.NextActions, inspectpkg.CommandAction{
			Reason: "resolve Bob ownership conflicts, then replan", CWD: root,
			Argv: []string{"bob", "plan", root}, RequiresExplicitAuthority: false,
		})
	case out.Clean:
		// The empty continuation is intentional: the repository is converged.
	default:
		out.NextActions = append(out.NextActions, inspectpkg.CommandAction{
			Reason: "apply the reviewed conflict-free plan through the approved shell path", CWD: root,
			Argv: []string{"bob", "apply", root}, RequiresExplicitAuthority: true,
		})
	}
	return out
}

func planFailure(root, code string, err error) (*sdkmcp.CallToolResult, *PlanOutput, error) {
	out := &PlanOutput{
		SchemaVersion: 1, OK: false, Workspace: root, Actions: []PlanAction{},
		Warnings: []string{}, NextActions: []inspectpkg.CommandAction{},
		Error: &ErrorInfo{Code: code, Message: err.Error()},
	}
	return &sdkmcp.CallToolResult{IsError: true}, out, nil
}
