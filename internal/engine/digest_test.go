package engine

import (
	"io/fs"
	"testing"
)

func digestFixture() PlanResult {
	return PlanResult{
		SchemaVersion: PlanSchemaVersion,
		Recipe:        LockRecipe{ID: "go-agent-tool", Version: 4},
		Actions: []Action{{
			Path: "README.md", Kind: ActionUpdate, Code: CodeContentUpdate,
			CurrentSHA256: "current", DesiredSHA256: "desired", LockedSHA256: "locked",
			CurrentMode: 0o600, DesiredMode: 0o644, Reason: "managed content changed",
			CurrentPreview: "current preview", DesiredPreview: "desired preview",
		}},
		ConflictCount: 0,
		LockChanged:   true,
		DesiredLock: LockFile{
			SchemaVersion: LockSchemaVersion,
			Recipe:        LockRecipe{ID: "go-agent-tool", Version: 4},
			Files:         []LockEntry{{Path: "README.md", SHA256: "desired"}},
		},
		canonicalRoot: "/first/workspace",
	}
}

func cloneDigestFixture() PlanResult {
	plan := digestFixture()
	plan.Actions = append([]Action(nil), plan.Actions...)
	plan.DesiredLock.Files = append([]LockEntry(nil), plan.DesiredLock.Files...)
	return plan
}

func TestDigestPlanVersionOneGolden(t *testing.T) {
	t.Parallel()
	got := DigestPlan(digestFixture())
	const want = "92889e7ef042361f086f0a654dc21e7bd64989b1dc6db60f28494c1f485c533d"
	if got.Version != PlanDigestVersion || got.SHA256 != want {
		t.Fatalf("digest = v%d:%s, want v%d:%s", got.Version, got.SHA256, PlanDigestVersion, want)
	}
}

func TestDigestPlanChangesForEveryVersionOneIdentityField(t *testing.T) {
	t.Parallel()
	base := DigestPlan(digestFixture()).SHA256
	tests := []struct {
		name   string
		mutate func(*PlanResult)
	}{
		{name: "plan schema", mutate: func(p *PlanResult) { p.SchemaVersion++ }},
		{name: "recipe id", mutate: func(p *PlanResult) { p.Recipe.ID = "files" }},
		{name: "recipe version", mutate: func(p *PlanResult) { p.Recipe.Version++ }},
		{name: "lock changed", mutate: func(p *PlanResult) { p.LockChanged = !p.LockChanged }},
		{name: "desired lock schema", mutate: func(p *PlanResult) { p.DesiredLock.SchemaVersion++ }},
		{name: "desired lock recipe id", mutate: func(p *PlanResult) { p.DesiredLock.Recipe.ID = "files" }},
		{name: "desired lock recipe version", mutate: func(p *PlanResult) { p.DesiredLock.Recipe.Version++ }},
		{name: "desired lock path", mutate: func(p *PlanResult) { p.DesiredLock.Files[0].Path = "OTHER.md" }},
		{name: "desired lock hash", mutate: func(p *PlanResult) { p.DesiredLock.Files[0].SHA256 = "other" }},
		{name: "action path", mutate: func(p *PlanResult) { p.Actions[0].Path = "OTHER.md" }},
		{name: "action kind", mutate: func(p *PlanResult) { p.Actions[0].Kind = ActionConflict }},
		{name: "action code", mutate: func(p *PlanResult) { p.Actions[0].Code = CodeManagedHashMismatch }},
		{name: "current hash", mutate: func(p *PlanResult) { p.Actions[0].CurrentSHA256 = "other" }},
		{name: "desired hash", mutate: func(p *PlanResult) { p.Actions[0].DesiredSHA256 = "other" }},
		{name: "locked hash", mutate: func(p *PlanResult) { p.Actions[0].LockedSHA256 = "other" }},
		{name: "current mode", mutate: func(p *PlanResult) { p.Actions[0].CurrentMode = fs.FileMode(0o640) }},
		{name: "desired mode", mutate: func(p *PlanResult) { p.Actions[0].DesiredMode = fs.FileMode(0o600) }},
		{name: "reason", mutate: func(p *PlanResult) { p.Actions[0].Reason = "other reason" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := cloneDigestFixture()
			tc.mutate(&plan)
			if got := DigestPlan(plan).SHA256; got == base {
				t.Fatalf("semantic mutation did not change digest %s", got)
			}
		})
	}
}

func TestDigestPlanIgnoresPresentationCountsAndWorkspaceRelocation(t *testing.T) {
	t.Parallel()
	base := DigestPlan(digestFixture()).SHA256
	plan := cloneDigestFixture()
	plan.ConflictCount = 99
	plan.Actions[0].CurrentPreview = "different current preview"
	plan.Actions[0].DesiredPreview = "different desired preview"
	plan.canonicalRoot = "/relocated/workspace"
	plan.lockExists = true
	plan.lockSHA256 = "private-observation"
	plan.observedRecipe = LockRecipe{ID: "files", Version: 1}
	plan.observedLock = LockFile{SchemaVersion: 99}
	if got := DigestPlan(plan).SHA256; got != base {
		t.Fatalf("presentation or private observation changed digest: got %s want %s", got, base)
	}
}
