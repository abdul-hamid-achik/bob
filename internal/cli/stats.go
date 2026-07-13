package cli

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/settings"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
	"github.com/abdul-hamid-achik/bob/internal/workspace"
	"github.com/spf13/cobra"
)

func newStatsCommand(opts *options, store *telemetry.Store) *cobra.Command {
	var all bool
	var sinceValue string
	command := &cobra.Command{
		Use:   "stats [workspace]",
		Short: "Summarize privacy-bounded local Bob usage",
		Long: `Summarize opt-in local telemetry without returning individual events,
stored raw paths, arguments, filenames, manifests, or raw errors. Use --all for every
pseudonymous workspace retained on this machine.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all && len(args) != 0 {
				return fmt.Errorf("stats: workspace and --all are mutually exclusive")
			}
			since, err := parseStatsSince(sinceValue, time.Now().UTC())
			if err != nil {
				return fmt.Errorf("stats: %w", err)
			}
			value, settingsErr := settings.Load()
			if settingsErr != nil {
				return fmt.Errorf("stats: %w", settingsErr)
			}
			query := telemetry.Query{Since: since}
			selected := "all"
			if !all {
				root := argumentPath(args)
				canonical, err := workspace.Resolve(root, true)
				if err != nil {
					return fmt.Errorf("stats: %w", err)
				}
				selected = canonical
				if value.Telemetry.Enabled && store != nil {
					workspaceID, err := store.WorkspaceID(canonical)
					if err != nil {
						return fmt.Errorf("stats: identify workspace: %w", err)
					}
					query.WorkspaceID = workspaceID
				}
			}
			var summary telemetry.Stats
			if store != nil {
				summary, err = store.Aggregate(cmd.Context(), query)
				if err != nil {
					return fmt.Errorf("stats: %w", err)
				}
			} else {
				summary = telemetry.Stats{SchemaVersion: telemetry.SchemaVersion, Since: since, Until: time.Now().UTC()}
			}
			data := map[string]any{
				"enabled": value.Telemetry.Enabled, "local_only": true,
				"selection": selected, "stats": summary,
			}
			warnings := []string(nil)
			if !value.Telemetry.Enabled {
				warnings = []string{"local telemetry is disabled; enable it with bob config init --telemetry --write or BOB_TELEMETRY=true"}
			}
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "stats", data, warnings, nil)
			}
			return printStats(cmd.Context(), cmd, summary, value.Telemetry.Enabled, selected)
		},
	}
	command.Flags().BoolVar(&all, "all", false, "aggregate every retained pseudonymous workspace")
	command.Flags().StringVar(&sinceValue, "since", "7d", "lookback window such as 24h, 7d, or 30d")
	return command
}

func parseStatsSince(value string, now time.Time) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "all" {
		return time.Time{}, nil
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days < 1 || days > 365 {
			return time.Time{}, fmt.Errorf("--since days must be between 1d and 365d")
		}
		return now.AddDate(0, 0, -days), nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 || duration > 365*24*time.Hour {
		return time.Time{}, fmt.Errorf("--since must be all or a positive duration no greater than 365d")
	}
	return now.Add(-duration), nil
}

func printStats(_ context.Context, cmd *cobra.Command, stats telemetry.Stats, enabled bool, selected string) error {
	if !enabled {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "telemetry  disabled — no state is written; enable with: bob config init --telemetry --write")
		return err
	}
	mean := int64(0)
	if stats.Events > 0 {
		mean = stats.DurationMS / int64(stats.Events)
	}
	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "selection  %s\nwindow     %s → %s\nevents     %d (%d success, %d failure, %d conflict, %d drift)\nlatency    %d ms total, %d ms mean\nactions    %d create, %d update, %d adopt, %d unchanged, %d conflict\n",
		selected, formatStatsTime(stats.Since), formatStatsTime(stats.Until), stats.Events,
		stats.Successes, stats.Failures, stats.ConflictEvents, stats.DriftEvents,
		stats.DurationMS, mean, stats.Actions.Create, stats.Actions.Update,
		stats.Actions.Adopt, stats.Actions.Unchanged, stats.Actions.Conflict); err != nil {
		return err
	}
	for _, operation := range stats.ByOperation {
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%-10s %4d calls  %4d failures  %8d ms\n", operation.Operation, operation.Events, operation.Failures, operation.DurationMS); err != nil {
			return err
		}
	}
	if stats.Skipped > 0 {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "warning    %d unreadable event file(s) skipped\n", stats.Skipped)
		return err
	}
	return nil
}

func formatStatsTime(value time.Time) string {
	if value.IsZero() {
		return "retained start"
	}
	return value.UTC().Format(time.RFC3339)
}
