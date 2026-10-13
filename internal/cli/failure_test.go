package cli

import (
	"errors"
	"reflect"
	"testing"
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
