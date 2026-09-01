package mcp

import (
	"context"
	"time"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
)

func (s *Server) recordOperation(ctx context.Context, operation telemetry.Operation, root string, outcome telemetry.Outcome, reason telemetry.Reason, counts inspectpkg.ActionCounts, recipeID string, recipeVersion int, started time.Time) {
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
	if recipeID != "" {
		if recipeVersion < 1 {
			if version, err := recipe.Version(recipeID); err == nil {
				recipeVersion = version
			}
		}
		if recipeVersion >= 1 {
			event.Recipe = telemetry.Recipe(recipeID)
			event.RecipeVersion = recipeVersion
		}
	}
	if root != "" && s.telemetry != nil && s.telemetry.Enabled() {
		if workspaceID, err := s.telemetry.WorkspaceID(root); err == nil {
			event.WorkspaceID = workspaceID
		}
	}
	_ = s.recorder.Record(ctx, event)
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
