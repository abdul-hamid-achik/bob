package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/studio"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
	"github.com/abdul-hamid-achik/bob/internal/workspace"
	"github.com/spf13/cobra"
)

// StudioRunner is the injectable process boundary for the interactive TUI.
type StudioRunner interface {
	Run(context.Context, string, studio.Options) error
}

type defaultStudioRunner struct{}

func (defaultStudioRunner) Run(ctx context.Context, root string, options studio.Options) error {
	return studio.Run(ctx, root, options)
}

func newStudioCommand(opts *options, runner StudioRunner, integrationRunner inspectpkg.Runner, store *telemetry.Store) *cobra.Command {
	var singlePane bool
	if runner == nil {
		runner = defaultStudioRunner{}
	}
	command := &cobra.Command{
		Use:   "studio [workspace]",
		Short: "Launch the read-only Bob repository and usage board",
		Long: `Launch an interactive Overview, Plan, and Stats board. Studio performs
one coherent offline repository read per refresh, never runs specialist probes,
and exposes no apply, shell, editor, indexing, or repair shortcut.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.json {
				return errors.New("studio: --json is not supported; use bob inspect --json, bob plan --json, or bob stats --json")
			}
			root, err := workspace.Resolve(argumentPath(args), true)
			if err != nil {
				return fmt.Errorf("studio: %w", err)
			}
			source := telemetryStudioSource{
				base: studio.NewRepositorySource(integrationRunner), store: store,
			}
			return runner.Run(cmd.Context(), root, studio.Options{SinglePane: singlePane, Source: source})
		},
	}
	command.Flags().BoolVar(&singlePane, "single-pane", false, "force the accessible compact layout")
	return command
}

type telemetryStudioSource struct {
	base  studio.Source
	store *telemetry.Store
}

func (source telemetryStudioSource) Load(ctx context.Context, root string) (studio.Snapshot, error) {
	snapshot, err := source.base.Load(ctx, root)
	if err != nil {
		return studio.Snapshot{}, err
	}
	if source.store == nil || !source.store.Enabled() {
		return snapshot, nil
	}
	workspaceID, err := source.store.WorkspaceID(snapshot.Report.Workspace)
	if err != nil {
		return snapshot, nil
	}
	stats, err := source.store.Aggregate(ctx, telemetry.Query{
		Since: time.Now().UTC().AddDate(0, 0, -30), WorkspaceID: workspaceID,
	})
	if err != nil {
		return snapshot, nil
	}
	projected := studio.Stats{
		Enabled: true, WindowDays: 30,
		Total: stats.Events, Success: stats.Successes, Errors: stats.Failures,
		Conflicts: stats.ConflictEvents, PerOperation: map[string]int{},
	}
	for _, operation := range stats.ByOperation {
		projected.PerOperation[string(operation.Operation)] = operation.Events
	}
	snapshot.Stats = projected
	return snapshot, nil
}
