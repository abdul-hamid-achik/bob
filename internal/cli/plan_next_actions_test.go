package cli

import (
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
)

func TestPlanNextActionsCarryReviewedDigest(t *testing.T) {
	t.Parallel()
	pending := engine.PlanResult{
		Actions: []engine.Action{
			{Path: "LICENSE", Kind: engine.ActionCreate, Code: engine.CodeMissing},
		},
	}
	want := engine.DigestPlan(pending).Qualified()
	if !strings.HasPrefix(want, "sha256:") {
		t.Fatalf("qualified digest = %q", want)
	}
	got := planNextActions(pending, "/tmp/ws")
	if len(got) != 2 || got[0] != "review the plan" {
		t.Fatalf("pending next actions = %#v", got)
	}
	if got[1] != "run bob apply /tmp/ws --expect-plan-digest "+want {
		t.Fatalf("pending apply action = %q", got[1])
	}

	lockOnly := engine.PlanResult{LockChanged: true}
	got = planNextActions(lockOnly, "")
	if len(got) != 2 || got[0] != "review the lock update" {
		t.Fatalf("lock-only next actions = %#v", got)
	}
	if !strings.Contains(got[1], "--expect-plan-digest sha256:") {
		t.Fatalf("lock-only apply action lacks digest: %q", got[1])
	}
}

func TestPlanNextActionsConflictAndConvergedUnchanged(t *testing.T) {
	t.Parallel()
	conflict := engine.PlanResult{
		Actions: []engine.Action{
			{Path: "README.md", Kind: engine.ActionConflict, Code: engine.CodeManagedHashMismatch},
		},
		ConflictCount: 1,
	}
	got := planNextActions(conflict, "/tmp/ws")
	if len(got) != 2 || got[0] != "resolve unmanaged or modified-file conflicts" || got[1] != "rerun bob plan /tmp/ws" {
		t.Fatalf("conflict next actions = %#v", got)
	}
	for _, action := range got {
		if strings.Contains(action, "expect-plan-digest") {
			t.Fatalf("conflict next actions must not carry a digest: %#v", got)
		}
	}
	got = planNextActions(engine.PlanResult{}, "/tmp/ws")
	if len(got) != 1 || got[0] != "repository is converged" {
		t.Fatalf("converged next actions = %#v", got)
	}
}
