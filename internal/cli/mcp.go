package cli

import (
	"os"
	"os/signal"
	"syscall"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/mcp"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
	"github.com/spf13/cobra"
)

func newMCPCommand(runner inspectpkg.Runner, recorder telemetry.Recorder, store *telemetry.Store) *cobra.Command {
	var selectedWorkspace string
	var allowedWorkspaces []string
	var allowAnyWorkspace bool
	var allowedApplyWorkspaces []string
	mcpCommand := &cobra.Command{
		Use:   "mcp",
		Short: "Expose Bob's compact MCP surface (read-only unless apply is explicitly authorized)",
	}
	serve := &cobra.Command{
		Use:   "serve",
		Short: "Run the Bob MCP server over stdio",
		Long: `Run the Bob MCP server using newline-delimited JSON-RPC over stdio.
stdout is reserved for the protocol. Nine tools are repository-read-only. The
tenth, bob_apply, is disabled unless --allow-apply-workspace names the exact
workspace; it then applies only a plan whose fresh digest matches the reviewed
expect_plan_digest. --allow-workspace and --allow-any-workspace never grant
apply authority. Register with MCPHub using:

  mcphub add bob /absolute/path/to/bob mcp serve`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := selectedWorkspace
			if root == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				root = cwd
			}
			server, err := mcp.NewServerWithOptions(root, runner, mcp.ServerOptions{
				AllowedWorkspaces:      allowedWorkspaces,
				AllowAnyWorkspace:      allowAnyWorkspace,
				AllowedApplyWorkspaces: allowedApplyWorkspaces,
				Recorder:               recorder,
				Telemetry:              store,
			})
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return server.Run(ctx)
		},
	}
	serve.Flags().StringVar(&selectedWorkspace, "workspace", "", "default existing workspace (defaults to startup cwd)")
	serve.Flags().StringArrayVar(&allowedWorkspaces, "allow-workspace", nil, "additional exact existing workspace allowed to MCP tools (repeatable)")
	serve.Flags().BoolVar(&allowAnyWorkspace, "allow-any-workspace", false, "allow MCP tools to read any existing workspace accessible to Bob")
	serve.Flags().StringArrayVar(&allowedApplyWorkspaces, "allow-apply-workspace", nil, "exact existing workspace bob_apply may mutate with a reviewed plan digest (repeatable; never implied by other flags)")
	mcpCommand.AddCommand(serve)
	return mcpCommand
}
