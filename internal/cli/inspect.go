package cli

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/spf13/cobra"
)

func newInspectCommand(opts *options, runner inspectpkg.Runner) *cobra.Command {
	var probeIntegrations bool
	cmd := &cobra.Command{
		Use:   "inspect [path]",
		Short: "Summarize Bob state and selected integration readiness",
		Long: `Summarize Bob-managed repository drift and offline binary availability.
By default no specialist process runs. --probe-integrations explicitly calls
Codemap and Vecgrep status commands; those commands may open their tool-owned
stores, and Vecgrep may contact its configured embedding provider. Bob never
searches, indexes, repairs, or verifies the repository.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := inspectpkg.Run(cmd.Context(), argumentPath(args), inspectpkg.Options{ProbeIntegrations: probeIntegrations}, runner)
			if err != nil {
				return fmt.Errorf("inspect: %w", err)
			}
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "inspect", report, report.Warnings, actionReasons(report.NextActions))
			}
			return printInspection(cmd.OutOrStdout(), report)
		},
	}
	cmd.Flags().BoolVar(&probeIntegrations, "probe-integrations", false, "explicitly run selected Codemap and Vecgrep status commands")
	return cmd
}

func printInspection(w io.Writer, report inspectpkg.Report) error {
	if _, err := fmt.Fprintf(w, "workspace  %s\nBob        %s", report.Workspace, report.Repository.State); err != nil {
		return err
	}
	if report.Repository.Error != "" {
		if _, err := fmt.Fprintf(w, " — %s", report.Repository.Error); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	for _, integration := range report.Integrations {
		status := integration.Probe.State
		if integration.Probe.State == inspectpkg.ProbeComplete {
			status = integration.Index.State
		}
		if _, err := fmt.Fprintf(w, "%-10s %s\n", integration.Name, status); err != nil {
			return err
		}
	}
	for _, warning := range report.Warnings {
		if _, err := fmt.Fprintf(w, "warning    %s\n", warning); err != nil {
			return err
		}
	}
	for _, action := range report.NextActions {
		if _, err := fmt.Fprintf(w, "next       %s  # %s\n", displayArgv(action.Argv), action.Reason); err != nil {
			return err
		}
	}
	return nil
}

func actionReasons(actions []inspectpkg.CommandAction) []string {
	result := make([]string, 0, len(actions))
	for _, action := range actions {
		result = append(result, action.Reason)
	}
	return result
}

func displayArgv(argv []string) string {
	parts := make([]string, 0, len(argv))
	for _, arg := range argv {
		if strings.ContainsAny(arg, " \t\n\"'") {
			parts = append(parts, strconv.Quote(arg))
		} else {
			parts = append(parts, arg)
		}
	}
	return strings.Join(parts, " ")
}
