package cli

import (
	"os"
	"os/signal"
	"syscall"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/mcp"
	"github.com/spf13/cobra"
)

func newMCPCommand(runner inspectpkg.Runner) *cobra.Command {
	var selectedWorkspace string
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
			server, err := mcp.NewServer(root, runner)
			if err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return server.Run(ctx)
		},
	}
	serve.Flags().StringVar(&selectedWorkspace, "workspace", "", "default existing workspace (defaults to startup cwd)")
	mcpCommand.AddCommand(serve)
	return mcpCommand
}
