package cli

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	bobpaths "github.com/abdul-hamid-achik/bob/internal/paths"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/settings"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
)

// goAgentToolTelemetryVersion reports the go-agent-tool recipe version.
// telemetry.Recipe is a closed enum that does not yet represent other
// recipes, so callers only set the recipe flag when the manifest's recipe is
// actually go-agent-tool; recipe.Version is looked up for that fixed id.
func goAgentToolTelemetryVersion() int {
	version, err := recipe.Version("go-agent-tool")
	if err != nil {
		// go-agent-tool is always a known recipe id; this is unreachable.
		return 0
	}
	return version
}

type commandMetrics struct {
	workspace string
	actions   telemetry.ActionCounts
	recipe    bool
}

func loadTelemetryRuntime() (*telemetry.Store, telemetry.Recorder, string) {
	layout, err := bobpaths.Resolve()
	if err != nil {
		return nil, telemetry.Noop{}, "local telemetry disabled: " + err.Error()
	}
	value, err := settings.LoadFile(layout.ConfigFile)
	if err != nil {
		return nil, telemetry.Noop{}, "local telemetry disabled: " + err.Error()
	}
	store, err := telemetry.Open(telemetry.Config{
		StateDir:        layout.StateDir,
		Enabled:         value.Telemetry.Enabled,
		RetentionDays:   value.Telemetry.RetentionDays,
		MaxEventsPerDay: value.Telemetry.MaxEventsPerDay,
	})
	if err != nil {
		return nil, telemetry.Noop{}, "local telemetry disabled: " + err.Error()
	}
	return store, telemetry.BestEffort(store), ""
}

func recordCLI(ctx context.Context, deps Dependencies, args []string, duration time.Duration, commandErr error) {
	operation, ok := recordedOperation(args)
	if !ok || deps.Recorder == nil {
		return
	}
	event := telemetry.Event{
		Surface: telemetry.SurfaceCLI, Operation: operation,
		Outcome: telemetry.OutcomeOK, DurationMS: duration.Milliseconds(),
	}
	if deps.metrics != nil {
		event.Actions = deps.metrics.actions
		if deps.metrics.recipe {
			event.Recipe = telemetry.RecipeGoAgentTool
			event.RecipeVersion = goAgentToolTelemetryVersion()
		}
		if deps.Telemetry != nil && deps.metrics.workspace != "" {
			if workspaceID, err := deps.Telemetry.WorkspaceID(deps.metrics.workspace); err == nil {
				event.WorkspaceID = workspaceID
			}
		}
	}
	if commandErr != nil {
		event.Outcome, event.Reason = classifyTelemetryFailure(operation, commandErr)
	}
	_ = deps.Recorder.Record(ctx, event)
}

func recordedOperation(args []string) (telemetry.Operation, bool) {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return "", false
		}
	}
	command := ""
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		command = arg
		break
	}
	switch command {
	case "new":
		return telemetry.OperationNew, true
	case "init":
		return telemetry.OperationInit, true
	case "plan":
		return telemetry.OperationPlan, true
	case "apply":
		return telemetry.OperationApply, true
	case "remove":
		return telemetry.OperationRemove, true
	case "check":
		return telemetry.OperationCheck, true
	case "doctor":
		return telemetry.OperationDoctor, true
	case "inspect":
		return telemetry.OperationInspect, true
	default:
		return "", false
	}
}

func classifyTelemetryFailure(operation telemetry.Operation, err error) (telemetry.Outcome, telemetry.Reason) {
	message := strings.ToLower(err.Error())
	switch {
	case errors.Is(err, context.Canceled):
		return telemetry.OutcomeCancelled, telemetry.ReasonInternal
	case operation == telemetry.OperationCheck && strings.Contains(message, "drift"):
		return telemetry.OutcomeDrift, telemetry.ReasonOwnershipConflict
	case strings.Contains(message, "conflict"):
		return telemetry.OutcomeConflict, telemetry.ReasonOwnershipConflict
	case strings.Contains(message, "manifest") && strings.Contains(message, "missing"):
		return telemetry.OutcomeError, telemetry.ReasonMissingManifest
	case strings.Contains(message, "manifest"):
		return telemetry.OutcomeError, telemetry.ReasonInvalidManifest
	case strings.Contains(message, "unsafe") || strings.Contains(message, "symlink") || strings.Contains(message, "traversal"):
		return telemetry.OutcomeError, telemetry.ReasonUnsafePath
	case strings.Contains(message, "required") || strings.Contains(message, "invalid"):
		return telemetry.OutcomeError, telemetry.ReasonInvalidInput
	case strings.Contains(message, "timeout") || strings.Contains(message, "deadline"):
		return telemetry.OutcomeError, telemetry.ReasonTimeout
	default:
		return telemetry.OutcomeError, telemetry.ReasonInternal
	}
}

func capturePlanMetrics(opts *options, workspace string, plan engine.PlanResult) {
	if opts == nil || opts.metrics == nil {
		return
	}
	opts.metrics.workspace = workspace
	opts.metrics.recipe = plan.Recipe.ID == "go-agent-tool"
	opts.metrics.actions = telemetry.ActionCounts{}
	for _, action := range plan.Actions {
		switch action.Kind {
		case engine.ActionCreate:
			opts.metrics.actions.Create++
		case engine.ActionUpdate:
			opts.metrics.actions.Update++
		case engine.ActionAdopt:
			opts.metrics.actions.Adopt++
		case engine.ActionUnchanged:
			opts.metrics.actions.Unchanged++
		case engine.ActionConflict:
			opts.metrics.actions.Conflict++
		}
	}
}

func captureInspectMetrics(opts *options, report inspectpkg.Report) {
	if opts == nil || opts.metrics == nil {
		return
	}
	opts.metrics.workspace = report.Workspace
	opts.metrics.recipe = report.Repository.Recipe == "go-agent-tool"
	opts.metrics.actions = telemetry.ActionCounts{
		Create: report.Repository.Actions.Create, Update: report.Repository.Actions.Update,
		Adopt: report.Repository.Actions.Adopt, Unchanged: report.Repository.Actions.Unchanged,
		Conflict: report.Repository.Actions.Conflict,
	}
}

func captureWorkspaceMetrics(opts *options, workspace string, recipeSelected bool) {
	if opts == nil || opts.metrics == nil {
		return
	}
	opts.metrics.workspace = workspace
	opts.metrics.recipe = recipeSelected
}
