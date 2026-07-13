// Package studio provides Bob's read-only terminal projection over inspect and
// plan reports. It never applies a plan or runs specialist status probes.
package studio

import (
	"context"
	"fmt"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
)

// Stats is a deliberately small, source-supplied operation projection. The
// repository source leaves it empty; a telemetry-backed adapter can map its
// aggregate without coupling the TUI model to telemetry storage types.
type Stats struct {
	Enabled      bool           `json:"enabled"`
	WindowDays   int            `json:"window_days"`
	Total        int            `json:"total"`
	Success      int            `json:"success"`
	Errors       int            `json:"errors"`
	Conflicts    int            `json:"conflicts"`
	PerOperation map[string]int `json:"per_operation"`
}

// Snapshot is the complete immutable input for one Studio frame generation.
// Plan is nil when the workspace has no valid manifest or planning failed.
type Snapshot struct {
	Report     inspectpkg.Report
	Plan       *engine.PlanResult
	CapturedAt time.Time
	Stats      Stats
}

// Source loads one read-only workspace snapshot.
type Source interface {
	Load(context.Context, string) (Snapshot, error)
}

// RepositorySource adapts Bob's current inspect and planning packages. The
// runner is used only for PATH discovery because ProbeIntegrations remains
// false. Its Run method is therefore never called by Studio.
type RepositorySource struct {
	Runner inspectpkg.Runner
}

// NewRepositorySource constructs the production read-only Studio source.
func NewRepositorySource(runner inspectpkg.Runner) RepositorySource {
	return RepositorySource{Runner: runner}
}

// Load reads inspect and plan state without applying changes or launching
// specialist tools.
func (s RepositorySource) Load(ctx context.Context, workspace string) (Snapshot, error) {
	observed, err := inspectpkg.Load(ctx, workspace, inspectpkg.Options{}, s.Runner)
	if err != nil {
		return Snapshot{}, fmt.Errorf("inspect workspace: %w", err)
	}
	snapshot := Snapshot{
		Report:     observed.Report,
		Plan:       observed.Plan,
		CapturedAt: observed.CapturedAt,
		Stats:      emptyStats(),
	}
	return snapshot, nil
}

func emptyStats() Stats {
	return Stats{PerOperation: map[string]int{}}
}
