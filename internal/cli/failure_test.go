package cli

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
)

func TestNextActionsForFailureDrift(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		workspace string
		want      []string
	}{
		{
			name:      "implicit current directory",
			workspace: ".",
			want: []string{
				"run: bob plan --json",
				"run: bob apply --expect-plan-digest <data.plan_digest>",
			},
		},
		{
			name:      "explicit workspace path",
			workspace: "/tmp/acme",
			want: []string{
				"run: bob plan /tmp/acme --json",
				"run: bob apply /tmp/acme --expect-plan-digest <data.plan_digest>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := newExitError(ExitDrift, errors.New("check: repository drift detected"))
			got := nextActionsForFailure(err, tt.workspace)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("next actions = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestClassifyErrorCodeWorkspaceBusy(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf("apply: acquire workspace lock: %w", engine.ErrWorkspaceBusy)
	if got := classifyErrorCode(err); got != "workspace_busy" {
		t.Fatalf("classifyErrorCode = %q, want workspace_busy", got)
	}
	if got := ExitCode(err); got != 1 {
		t.Fatalf("ExitCode = %d, want 1", got)
	}
	next := nextActionsForCode("workspace_busy", ".")
	if !strings.Contains(strings.Join(next, "\n"), engine.ApplyLockFilename) {
		t.Fatalf("workspace_busy next actions must mention %s: %#v", engine.ApplyLockFilename, next)
	}
	if got := nextActionsForFailure(err, "."); !reflect.DeepEqual(got, next) {
		t.Fatalf("nextActionsForFailure = %#v, want %#v", got, next)
	}
}

func TestNextActionsForFailureConflictsUnchanged(t *testing.T) {
	t.Parallel()

	err := newExitError(ExitConflicts, errors.New("check: repository conflicts detected"))
	got := nextActionsForFailure(err, ".")
	want := []string{
		"run: bob plan --json and inspect actions with kind=conflict",
		"resolve each conflict, then rerun bob apply",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("conflict next actions = %#v, want %#v", got, want)
	}
}
