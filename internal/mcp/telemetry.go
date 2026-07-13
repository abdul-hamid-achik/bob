package mcp

import (
	"context"
	"time"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
)

func (s *Server) recordOperation(ctx context.Context, operation telemetry.Operation, root string, outcome telemetry.Outcome, reason telemetry.Reason, counts inspectpkg.ActionCounts, recipe bool, started time.Time) {
	if s == nil || s.recorder == nil {
		return
	}
	event := telemetry.Event{
		Surface: telemetry.SurfaceMCP, Operation: operation, Outcome: outcome,
		Reason: reason, DurationMS: time.Since(started).Milliseconds(),
		Actions: telemetry.ActionCounts{
			Create: counts.Create, Update: counts.Update, Adopt: counts.Adopt,
			Unchanged: counts.Unchanged, Conflict: counts.Conflict,
		},
	}
	if recipe {
		event.Recipe = telemetry.RecipeGoAgentTool
		event.RecipeVersion = currentRecipeVersion()
	}
	if root != "" && s.telemetry != nil && s.telemetry.Enabled() {
		if workspaceID, err := s.telemetry.WorkspaceID(root); err == nil {
			event.WorkspaceID = workspaceID
		}
	}
	_ = s.recorder.Record(ctx, event)
}

// currentRecipeVersion reports the go-agent-tool recipe version. Callers only
// set the recipe flag when the manifest's recipe is actually go-agent-tool
// (telemetry.Recipe is a closed enum that does not yet represent other
// recipes), so recipe.Version is looked up for that fixed id rather than a
// manifest that may not be in hand at the telemetry call site.
func currentRecipeVersion() int {
	version, err := recipe.Version(string(telemetry.RecipeGoAgentTool))
	if err != nil {
		// go-agent-tool is always a known recipe id; this is unreachable.
		return 0
	}
	return version
}

func reasonFromToolCode(code string) telemetry.Reason {
	switch code {
	case "input_invalid", "manifest_too_large", "recipe_unknown":
		return telemetry.ReasonInvalidInput
	case "manifest_invalid":
		return telemetry.ReasonInvalidManifest
	case "workspace_invalid", "workspace_unauthorized":
		return telemetry.ReasonUnsafePath
	default:
		return telemetry.ReasonInternal
	}
}
