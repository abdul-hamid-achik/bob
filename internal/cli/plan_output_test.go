package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	contextpkg "github.com/abdul-hamid-achik/bob/internal/context"
	"github.com/abdul-hamid-achik/bob/internal/engine"
)

func TestPrintPlanRendersCodeAndFamily(t *testing.T) {
	t.Parallel()
	// planRow builds one expected plan row from the documented column
	// contract: kind left-aligned in 10 columns, path in 40, then the
	// bracketed code and family.
	planRow := func(kind, path, code, family string) string {
		return fmt.Sprintf("%-10s %-40s [%s] %s", kind, path, code, family)
	}
	tests := []struct {
		name    string
		plan    engine.PlanResult
		want    []string
		notWant []string
	}{
		{
			name: "conflict carries cause",
			plan: engine.PlanResult{
				Actions: []engine.Action{
					{Path: "README.md", Kind: engine.ActionConflict, Code: engine.CodeUnmanagedDiffers},
				},
				ConflictCount: 1,
			},
			want: []string{
				planRow("conflict", "README.md", "unmanaged_differs", "unmanaged_divergence"),
				"\n0 create, 0 update, 0 adopt, 0 unchanged, 1 conflict\n",
			},
		},
		{
			name: "create carries scaffold family",
			plan: engine.PlanResult{
				Actions: []engine.Action{
					{Path: "internal/cli/registry.go", Kind: engine.ActionCreate, Code: engine.CodeMissing},
				},
			},
			want: []string{
				planRow("create", "internal/cli/registry.go", "missing", "scaffold"),
			},
		},
		{
			name: "managed update carries convergence family",
			plan: engine.PlanResult{
				Actions: []engine.Action{
					{Path: "go.mod", Kind: engine.ActionUpdate, Code: engine.CodeContentUpdate},
				},
				LockChanged: true,
			},
			want: []string{
				planRow("update", "go.mod", "content_update", "convergence"),
				"lock       bob.lock",
			},
		},
		{
			name: "conflicts-only keeps cause and drops other rows",
			plan: engine.PlanResult{
				Actions: []engine.Action{
					{Path: "LICENSE", Kind: engine.ActionCreate, Code: engine.CodeMissing},
					{Path: "README.md", Kind: engine.ActionConflict, Code: engine.CodeSymlink},
					{Path: "go.mod", Kind: engine.ActionUnchanged, Code: engine.CodeInSync},
				},
				ConflictCount: 1,
			},
			want: []string{
				planRow("conflict", "README.md", "symlink", "ownership_hazard"),
			},
			notWant: []string{"[missing] scaffold", "[in_sync] convergence"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			conflictsOnly := strings.Contains(tt.name, "conflicts-only")
			if err := printPlan(&buf, tt.plan, false, conflictsOnly); err != nil {
				t.Fatal(err)
			}
			output := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(output, want) {
					t.Errorf("output missing %q:\n%s", want, output)
				}
			}
			for _, notWant := range tt.notWant {
				if strings.Contains(output, notWant) {
					t.Errorf("output unexpectedly contains %q:\n%s", notWant, output)
				}
			}
		})
	}
}

func TestPlanNextActionsClassAware(t *testing.T) {
	t.Parallel()
	unmanagedNoLock := engine.PlanResult{
		Actions: []engine.Action{
			{Path: "README.md", Kind: engine.ActionConflict, Code: engine.CodeUnmanagedDiffers},
			{Path: "LICENSE", Kind: engine.ActionCreate, Code: engine.CodeMissing},
		},
		ConflictCount: 1,
	}
	managedConflict := engine.PlanResult{
		Actions: []engine.Action{
			{Path: "README.md", Kind: engine.ActionConflict, Code: engine.CodeManagedHashMismatch},
		},
		ConflictCount: 1,
	}
	if got := planNextActions(unmanagedNoLock, ""); got[0] != "recipe was never applied; files at these paths are human-owned — apply will not overwrite them" {
		t.Fatalf("unmanaged divergence without lock next actions = %#v", got)
	}
	if got := planNextActions(managedConflict, ""); got[0] != "resolve unmanaged or modified-file conflicts" {
		t.Fatalf("managed conflict next actions = %#v", got)
	}
	if got := planNextActions(unmanagedNoLock, "/tmp/ws"); got[1] != "rerun bob plan /tmp/ws" {
		t.Fatalf("workspace-qualified rerun = %#v", got)
	}
	if got := planNextActions(engine.PlanResult{}, ""); got[0] != "repository is converged" {
		t.Fatalf("converged next actions = %#v", got)
	}
}

func TestConflictWarningsClassAware(t *testing.T) {
	t.Parallel()
	unmanagedNoLock := engine.PlanResult{
		Actions: []engine.Action{
			{Path: "README.md", Kind: engine.ActionConflict, Code: engine.CodeUnmanagedDiffers},
		},
		ConflictCount: 1,
	}
	managedConflict := engine.PlanResult{
		Actions: []engine.Action{
			{Path: "README.md", Kind: engine.ActionConflict, Code: engine.CodeManagedHashMismatch},
		},
		ConflictCount: 1,
	}
	got := conflictWarnings(unmanagedNoLock)
	if len(got) != 2 || got[0] != "1 conflict(s) block apply" || got[1] != "recipe was never applied; files at these paths are human-owned — apply will not overwrite them" {
		t.Fatalf("unmanaged divergence warnings = %#v", got)
	}
	if got := conflictWarnings(managedConflict); len(got) != 1 || got[0] != "1 conflict(s) block apply" {
		t.Fatalf("managed conflict warnings = %#v", got)
	}
	if got := conflictWarnings(engine.PlanResult{}); got != nil {
		t.Fatalf("conflict-free warnings = %#v", got)
	}
}

func TestPrintContextHumanVerdictLine(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		repo contextpkg.Repository
		want string
	}{
		{
			name: "clean",
			repo: contextpkg.Repository{State: "clean", Clean: true, LockExists: true, ConflictClass: "none",
				ConflictFamilyCounts: map[string]int{"ownership_hazard": 0, "contract_drift": 0, "unmanaged_divergence": 0},
				ActionCounts:         engine.ActionCounts{Unchanged: 30}, ManagedFiles: 30},
			want: "repository: clean; managed: 30; conflicts: 0; lock changed: false; creates: 0",
		},
		{
			name: "unmanaged divergence without lock",
			repo: contextpkg.Repository{State: "conflicted", LockChanged: true, ConflictCount: 12, ConflictClass: "unmanaged_divergence",
				ConflictFamilyCounts: map[string]int{"ownership_hazard": 0, "contract_drift": 0, "unmanaged_divergence": 12},
				ActionCounts:         engine.ActionCounts{Create: 17}, ManagedFiles: 30},
			want: "repository: conflicted (unmanaged_divergence; no lock — recipe never applied); managed: 30; conflicts: 12 (unmanaged 12, managed 0, hazard 0); lock changed: true; creates: 17",
		},
		{
			name: "contract drift with lock",
			repo: contextpkg.Repository{State: "conflicted", LockChanged: false, LockExists: true, ConflictCount: 1, ConflictClass: "contract_drift",
				ConflictFamilyCounts: map[string]int{"ownership_hazard": 0, "contract_drift": 1, "unmanaged_divergence": 0},
				ActionCounts:         engine.ActionCounts{Unchanged: 29}, ManagedFiles: 30},
			want: "repository: conflicted (contract_drift); managed: 30; conflicts: 1 (unmanaged 0, managed 1, hazard 0); lock changed: false; creates: 0",
		},
		{
			name: "drifted without conflicts",
			repo: contextpkg.Repository{State: "drifted", LockChanged: true, ConflictClass: "none",
				ConflictFamilyCounts: map[string]int{"ownership_hazard": 0, "contract_drift": 0, "unmanaged_divergence": 0},
				ActionCounts:         engine.ActionCounts{Create: 17}, ManagedFiles: 30},
			want: "repository: drifted; managed: 30; conflicts: 0; lock changed: true; creates: 17",
		},
		{
			// The only class produced by the "present > 1" branch, and the one
			// where the per-family breakdown formatting regresses most easily.
			name: "mixed conflict families",
			repo: contextpkg.Repository{State: "conflicted", LockChanged: true, LockExists: true, ConflictCount: 2, ConflictClass: "mixed",
				ConflictFamilyCounts: map[string]int{"ownership_hazard": 0, "contract_drift": 1, "unmanaged_divergence": 1},
				ActionCounts:         engine.ActionCounts{Unchanged: 29}, ManagedFiles: 30},
			want: "repository: conflicted (mixed); managed: 30; conflicts: 2 (unmanaged 1, managed 1, hazard 0); lock changed: true; creates: 0",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := contextpkg.Result{Repository: tt.repo}
			var buf bytes.Buffer
			if err := printContext(&buf, result); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(buf.String(), tt.want) {
				t.Fatalf("verdict line missing %q:\n%s", tt.want, buf.String())
			}
		})
	}
}

// TestPlanHumanOutputShowsCauses drives the full plan command so the
// renderer's contract is proven end to end, not just in isolation.
func TestPlanHumanOutputShowsCauses(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "acme")
	if _, _, err := executeForTest("new", "acme", "--module", "github.com/acme/acme", "--dir", target, "--write"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "README.md"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("plan", target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, fmt.Sprintf("conflict   README.md")) || !strings.Contains(stdout, "[managed_hash_mismatch] contract_drift") {
		t.Fatalf("plan human output lacks cause classification:\n%s", stdout)
	}
	if !strings.Contains(stdout, "[missing] scaffold") && !strings.Contains(stdout, "[in_sync] convergence") {
		t.Fatalf("plan human output lacks non-conflict families:\n%s", stdout)
	}
}
