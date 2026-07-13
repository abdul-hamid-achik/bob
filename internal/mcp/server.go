// Package mcp exposes Bob's repository-read-only inventory and planning surface
// over stdio. It never invokes Cobra, parses CLI output, or writes to stdout
// outside the MCP transport. When explicitly enabled, machine-local telemetry
// records privacy-bounded operation events under Bob's XDG state directory.
package mcp

import (
	"context"
	"fmt"
	"os"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
	"github.com/abdul-hamid-achik/bob/internal/version"
	"github.com/abdul-hamid-achik/bob/internal/workspace"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const instructions = `Bob is a deterministic repository factory. Use bob_inspect or bob_check to
read Bob-managed state, bob_validate_manifest to validate a workspace manifest or bounded inline
YAML, bob_recipe_describe to inspect the embedded recipe contract, bob_stats to summarize opt-in
local usage, and bob_plan to review proposed file actions. These MCP tools never mutate repositories; opt-in telemetry may update Bob's local XDG state. To apply a conflict-free plan, use the normal approved
shell path with "bob apply <workspace>", then call bob_check again. Codemap and Vecgrep search,
impact analysis, indexing, and verification remain owned by those tools and Cortex.`

type Server struct {
	authority workspaceAuthority
	runner    inspectpkg.Runner
	recorder  telemetry.Recorder
	telemetry *telemetry.Store
	srv       *sdkmcp.Server
}

// ServerOptions narrows the filesystem authority granted to the MCP server.
// The startup workspace is always included in the exact allowlist.
type ServerOptions struct {
	AllowedWorkspaces []string
	AllowAnyWorkspace bool
	Recorder          telemetry.Recorder
	Telemetry         *telemetry.Store
}

// NewServer constructs the compact read-only MCP surface.
func NewServer(defaultWorkspace string, runner inspectpkg.Runner) (*Server, error) {
	return NewServerWithOptions(defaultWorkspace, runner, ServerOptions{})
}

// NewServerWithOptions constructs the MCP surface with explicit workspace
// authority. Relative allowed workspaces are resolved from the startup root.
func NewServerWithOptions(defaultWorkspace string, runner inspectpkg.Runner, options ServerOptions) (*Server, error) {
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
	if options.Recorder == nil {
		options.Recorder = telemetry.Noop{}
	}
	authority, err := newWorkspaceAuthority(canonical, options.AllowedWorkspaces, options.AllowAnyWorkspace)
	if err != nil {
		return nil, err
	}
	s := &Server{
		authority: authority, runner: runner,
		recorder: telemetry.BestEffort(options.Recorder), telemetry: options.Telemetry,
	}
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
		"Return a bounded deterministic action list and digest for the current bob.yaml and bob.lock. Unchanged actions are excluded by default. Conflicts are useful plan results and block the separate approved CLI apply path.",
	), s.handlePlan)
	sdkmcp.AddTool(s.srv, readOnlyTool(
		"bob_check", "Check Bob-managed repository convergence",
		"Return a compact convergence, conflict, and lock-drift summary with the same complete-plan digest as bob_plan. Does not run specialist commands or mutate.",
	), s.handleCheck)
	sdkmcp.AddTool(s.srv, readOnlyTool(
		"bob_validate_manifest", "Validate a Bob manifest",
		"Strictly validate exactly one source: bob.yaml in an authorized workspace or bounded inline YAML. Returns the normalized typed manifest and never writes it.",
	), s.handleValidateManifest)
	sdkmcp.AddTool(s.srv, readOnlyTool(
		"bob_recipe_describe", "Describe an embedded Bob recipe",
		"Describe the deterministic built-in recipe contract, supported choices, schema version, and generated surfaces without reading a workspace.",
	), s.handleRecipeDescribe)
	sdkmcp.AddTool(s.srv, readOnlyTool(
		"bob_stats", "Summarize local Bob usage",
		"Return aggregate opt-in local telemetry for an authorized workspace or all pseudonymous workspaces. Never returns individual events or stored raw paths, arguments, filenames, manifests, or raw errors.",
	), s.handleStats)
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
