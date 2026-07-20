// Package telemetry records privacy-bounded product events to the local
// filesystem. It has no network transport.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"
)

const SchemaVersion = 1

const (
	maxCount      = 1_000_000
	maxDurationMS = int64((24 * time.Hour) / time.Millisecond)
)

var (
	ErrDailyCap      = errors.New("telemetry daily event cap reached")
	ErrNewerSchema   = errors.New("telemetry data uses a newer schema")
	workspacePattern = regexp.MustCompile(`^w_[0-9a-f]{32}$`)
	eventIDPattern   = regexp.MustCompile(`^evt_[0-9a-f]{32}$`)
)

// Surface identifies the closed v1 set of Bob entry points. Studio is
// intentionally unrecorded; adding it requires an explicit schema decision.
type Surface string

const (
	SurfaceCLI Surface = "cli"
	SurfaceMCP Surface = "mcp"
)

// Operation identifies a closed set of Bob product operations.
type Operation string

const (
	OperationNew              Operation = "new"
	OperationInit             Operation = "init"
	OperationPlan             Operation = "plan"
	OperationApply            Operation = "apply"
	OperationRemove           Operation = "remove"
	OperationUpgrade          Operation = "upgrade"
	OperationCheck            Operation = "check"
	OperationDoctor           Operation = "doctor"
	OperationInspect          Operation = "inspect"
	OperationValidateManifest Operation = "validate_manifest"
	OperationRecipeDescribe   Operation = "recipe_describe"
)

// Outcome is a closed, non-sensitive result category.
type Outcome string

const (
	OutcomeOK        Outcome = "ok"
	OutcomeError     Outcome = "error"
	OutcomeConflict  Outcome = "conflict"
	OutcomeDrift     Outcome = "drift"
	OutcomeCancelled Outcome = "cancelled"
)

// Reason is a closed error category. Raw errors must never enter an Event.
type Reason string

const (
	ReasonInvalidInput           Reason = "invalid_input"
	ReasonMissingManifest        Reason = "missing_manifest"
	ReasonInvalidManifest        Reason = "invalid_manifest"
	ReasonUnsafePath             Reason = "unsafe_path"
	ReasonOwnershipConflict      Reason = "ownership_conflict"
	ReasonIntegrationUnavailable Reason = "integration_unavailable"
	ReasonTimeout                Reason = "timeout"
	ReasonInternal               Reason = "internal"
)

// Recipe identifies a closed set of scaffold recipes.
type Recipe string

const RecipeGoAgentTool Recipe = "go-agent-tool"

// ActionCounts contains aggregate action counts only; filenames and paths are
// intentionally not representable.
type ActionCounts struct {
	Create    int `json:"create,omitempty"`
	Update    int `json:"update,omitempty"`
	Adopt     int `json:"adopt,omitempty"`
	Unchanged int `json:"unchanged,omitempty"`
	Conflict  int `json:"conflict,omitempty"`
}

// Event is the complete durable event schema. IDs and RecordedAt are assigned
// by Store.Record. There are deliberately no argv, path, content, free-form
// label, or raw error fields.
type Event struct {
	SchemaVersion int          `json:"schema_version"`
	EventID       string       `json:"event_id"`
	RecordedAt    time.Time    `json:"recorded_at"`
	Surface       Surface      `json:"surface"`
	Operation     Operation    `json:"operation"`
	Outcome       Outcome      `json:"outcome"`
	Reason        Reason       `json:"reason,omitempty"`
	DurationMS    int64        `json:"duration_ms,omitempty"`
	WorkspaceID   string       `json:"workspace_id,omitempty"`
	Recipe        Recipe       `json:"recipe,omitempty"`
	RecipeVersion int          `json:"recipe_version,omitempty"`
	Actions       ActionCounts `json:"actions,omitempty"`
}

// Recorder is the small integration boundary used by CLI and MCP surfaces.
type Recorder interface {
	Record(context.Context, Event) error
}

// Noop discards events. It is the appropriate recorder when telemetry is
// disabled.
type Noop struct{}

func (Noop) Record(context.Context, Event) error { return nil }

type bestEffort struct{ recorder Recorder }

func (recorder bestEffort) Record(ctx context.Context, event Event) error {
	_ = recorder.recorder.Record(ctx, event)
	return nil
}

// BestEffort converts recorder failures into successful no-ops so optional
// telemetry can never fail a product operation. A nil recorder becomes Noop.
func BestEffort(recorder Recorder) Recorder {
	if recorder == nil {
		return Noop{}
	}
	return bestEffort{recorder: recorder}
}

func validateEvent(event Event, stored bool) error {
	if stored {
		if event.SchemaVersion > SchemaVersion {
			return fmt.Errorf("%w: %d", ErrNewerSchema, event.SchemaVersion)
		}
		if event.SchemaVersion != SchemaVersion {
			return fmt.Errorf("unsupported telemetry schema_version %d", event.SchemaVersion)
		}
		if !eventIDPattern.MatchString(event.EventID) {
			return errors.New("invalid telemetry event_id")
		}
		if event.RecordedAt.IsZero() {
			return errors.New("telemetry recorded_at is required")
		}
	} else if event.SchemaVersion != 0 && event.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported telemetry schema_version %d", event.SchemaVersion)
	}
	if !validSurface(event.Surface) {
		return fmt.Errorf("invalid telemetry surface %q", event.Surface)
	}
	if !validOperation(event.Operation) {
		return fmt.Errorf("invalid telemetry operation %q", event.Operation)
	}
	if !validOutcome(event.Outcome) {
		return fmt.Errorf("invalid telemetry outcome %q", event.Outcome)
	}
	if event.Reason != "" && !validReason(event.Reason) {
		return fmt.Errorf("invalid telemetry reason %q", event.Reason)
	}
	if event.Outcome == OutcomeOK && event.Reason != "" {
		return errors.New("successful telemetry events cannot have a failure reason")
	}
	if event.DurationMS < 0 || event.DurationMS > maxDurationMS {
		return fmt.Errorf("telemetry duration_ms must be between 0 and %d", maxDurationMS)
	}
	if event.WorkspaceID != "" && !workspacePattern.MatchString(event.WorkspaceID) {
		return errors.New("invalid telemetry workspace_id")
	}
	if event.Recipe != "" && event.Recipe != RecipeGoAgentTool {
		return fmt.Errorf("invalid telemetry recipe %q", event.Recipe)
	}
	if event.Recipe == "" && event.RecipeVersion != 0 {
		return errors.New("telemetry recipe_version requires a recipe")
	}
	if event.Recipe != "" && event.RecipeVersion < 1 {
		return errors.New("telemetry recipe_version must be positive")
	}
	counts := []int{
		event.Actions.Create,
		event.Actions.Update,
		event.Actions.Adopt,
		event.Actions.Unchanged,
		event.Actions.Conflict,
	}
	for _, count := range counts {
		if count < 0 || count > maxCount {
			return fmt.Errorf("telemetry counts must be between 0 and %d", maxCount)
		}
	}
	return nil
}

func validSurface(value Surface) bool {
	return value == SurfaceCLI || value == SurfaceMCP
}

func validOperation(value Operation) bool {
	switch value {
	case OperationNew, OperationInit, OperationPlan, OperationApply, OperationRemove, OperationUpgrade, OperationCheck,
		OperationDoctor, OperationInspect, OperationValidateManifest, OperationRecipeDescribe:
		return true
	default:
		return false
	}
}

func validOutcome(value Outcome) bool {
	switch value {
	case OutcomeOK, OutcomeError, OutcomeConflict, OutcomeDrift, OutcomeCancelled:
		return true
	default:
		return false
	}
}

func validReason(value Reason) bool {
	switch value {
	case ReasonInvalidInput, ReasonMissingManifest, ReasonInvalidManifest, ReasonUnsafePath,
		ReasonOwnershipConflict, ReasonIntegrationUnavailable, ReasonTimeout, ReasonInternal:
		return true
	default:
		return false
	}
}
