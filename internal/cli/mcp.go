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
	mcpCommand := &cobra.Command{
		Use:   "mcp",
		Short: "Expose Bob's compact read-only MCP surface",
	}
	serve := &cobra.Command{
		Use:   "serve",
		Short: "Run the Bob MCP server over stdio",
		Long: `Run the read-only Bob MCP server using newline-delimited JSON-RPC over
stdio. stdout is reserved for the protocol. Register with MCPHub using:

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
				AllowedWorkspaces: allowedWorkspaces,
				AllowAnyWorkspace: allowAnyWorkspace,
				Recorder:          recorder,
				Telemetry:         store,
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
	mcpCommand.AddCommand(serve)
	return mcpCommand
}
