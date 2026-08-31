package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

// closedActionCodes mirrors the const block in engine.go. Every code in the
// closed vocabulary must map to a family; a new code without a family is a
// wire-contract gap this table makes visible.
var closedActionCodes = map[string]ActionFamily{
	CodeUnmanagedDiffers:     FamilyUnmanagedDivergence,
	CodeManagedHashMismatch:  FamilyContractDrift,
	CodeManagedMissing:       FamilyContractDrift,
	CodeUnmanagedModeDiffers: FamilyUnmanagedDivergence,
	CodeRetiredOwned:         FamilyContractDrift,
	CodeSymlink:              FamilyOwnershipHazard,
	CodeSpecialFile:          FamilyOwnershipHazard,
	CodeMissing:              FamilyScaffold,
	CodeModeDrift:            FamilyConvergence,
	CodeContentUpdate:        FamilyConvergence,
	CodeInSync:               FamilyConvergence,
	CodeIdenticalContent:     FamilyConvergence,
	CodeSeedExists:           FamilyConvergence,
}

// TestFamilyByCodeMatchesClosedVocabularyMirror keeps the production map and
// this file's mirror in lockstep. Go cannot enumerate consts, so a new code
// added to engine.go's const block is invisible to both tables; what this
// guard does catch is the half-update — a code added to familyByCode without
// the mirror (or vice versa) fails here instead of silently skipping
// ConflictFamilyCounts, misclassifying ConflictClass, and emitting a
// malformed "conflict_" reason code in context.
func TestFamilyByCodeMatchesClosedVocabularyMirror(t *testing.T) {
	t.Parallel()
	if len(familyByCode) != len(closedActionCodes) {
		t.Fatalf("familyByCode has %d entries but the test mirror has %d; both tables must list every closed code", len(familyByCode), len(closedActionCodes))
	}
	for code, want := range closedActionCodes {
		got, ok := familyByCode[code]
		if !ok {
			t.Errorf("code %q is missing from familyByCode; a code without a family silently breaks conflict classification", code)
			continue
		}
		if got != want {
			t.Errorf("familyByCode[%q] = %q, want %q", code, got, want)
		}
	}
	for code := range familyByCode {
		if _, ok := closedActionCodes[code]; !ok {
			t.Errorf("code %q is in familyByCode but missing from the closedActionCodes mirror; extend the mirror so totality stays provable", code)
		}
	}
}

func TestActionFamilyCoversClosedCodeVocabulary(t *testing.T) {
	t.Parallel()
	if len(closedActionCodes) != 13 {
		t.Fatalf("closed code table has %d entries; update it (and familyByCode, and the const block in engine.go) when the vocabulary changes", len(closedActionCodes))
	}
	for code, want := range closedActionCodes {
		if got := (Action{Kind: ActionConflict, Code: code}).Family(); got != want {
			t.Errorf("Family(%q) = %q, want %q", code, got, want)
		}
	}
	if got := (Action{Code: "not_a_real_code"}).Family(); got != "" {
		t.Errorf("Family of an unknown code = %q, want empty", got)
	}
}

func TestPlanConflictClass(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		setup    func(t *testing.T, root string)
		class    string
		dominant ActionFamily
	}{
		{
			name: "unmanaged divergence without lock",
			setup: func(t *testing.T, root string) {
				if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("human\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			class:    ConflictClassUnmanagedDivergence,
			dominant: FamilyUnmanagedDivergence,
		},
		{
			name: "contract drift on managed file",
			setup: func(t *testing.T, root string) {
				if _, err := Apply(root, testManifest(), []recipe.Artifact{artifact("README.md", "bob\n")}); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("edited\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			class:    ConflictClassContractDrift,
			dominant: FamilyContractDrift,
		},
		{
			name: "ownership hazard from symlink",
			setup: func(t *testing.T, root string) {
				if err := os.Symlink(filepath.Join(os.TempDir()), filepath.Join(root, "README.md")); err != nil {
					t.Fatal(err)
				}
			},
			class:    ConflictClassOwnershipHazard,
			dominant: FamilyOwnershipHazard,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			tt.setup(t, root)
			plan, err := Plan(root, testManifest(), []recipe.Artifact{artifact("README.md", "bob\n")})
			if err != nil {
				t.Fatal(err)
			}
			if !plan.HasConflicts() {
				t.Fatalf("fixture produced no conflicts: %#v", plan.Actions)
			}
			if got := plan.ConflictClass(); got != tt.class {
				t.Fatalf("ConflictClass() = %q, want %q", got, tt.class)
			}
			if got := plan.DominantConflictFamily(); got != tt.dominant {
				t.Fatalf("DominantConflictFamily() = %q, want %q", got, tt.dominant)
			}
		})
	}
}

func TestPlanConflictClassMixedPrefersHazard(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("human\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(os.TempDir()), filepath.Join(root, "LICENSE")); err != nil {
		t.Fatal(err)
	}
	plan, err := Plan(root, testManifest(), []recipe.Artifact{artifact("README.md", "bob\n"), artifact("LICENSE", "MIT\n")})
	if err != nil {
		t.Fatal(err)
	}
	if got := plan.ConflictClass(); got != ConflictClassMixed {
		t.Fatalf("ConflictClass() = %q, want %q", got, ConflictClassMixed)
	}
	if got := plan.DominantConflictFamily(); got != FamilyOwnershipHazard {
		t.Fatalf("DominantConflictFamily() = %q, want %q (hazard outranks divergence)", got, FamilyOwnershipHazard)
	}
}

func TestConflictFreePlanHasNoConflictClass(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	plan, err := Plan(root, testManifest(), []recipe.Artifact{artifact("README.md", "bob\n")})
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasConflicts() {
		t.Fatal("fixture unexpectedly conflicted")
	}
	if got := plan.ConflictClass(); got != ConflictClassNone {
		t.Fatalf("ConflictClass() = %q, want %q", got, ConflictClassNone)
	}
	if got := plan.DominantConflictFamily(); got != "" {
		t.Fatalf("DominantConflictFamily() = %q, want empty", got)
	}
}

func TestPlanActionCountsAndLockExists(t *testing.T) {
	t.Parallel()
	freshRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(freshRoot, "LICENSE"), []byte("MIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fresh, err := Plan(freshRoot, testManifest(), []recipe.Artifact{artifact("README.md", "bob\n"), artifact("LICENSE", "MIT\n")})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.LockExists() {
		t.Fatal("fresh workspace reported a lock")
	}
	counts := fresh.ActionCounts()
	if counts.Create != 1 || counts.Adopt != 1 || counts.Update != 0 || counts.Unchanged != 0 || counts.Conflict != 0 {
		t.Fatalf("fresh counts = %#v", counts)
	}

	appliedRoot := t.TempDir()
	if _, err := Apply(appliedRoot, testManifest(), []recipe.Artifact{artifact("README.md", "bob\n")}); err != nil {
		t.Fatal(err)
	}
	applied, err := Plan(appliedRoot, testManifest(), []recipe.Artifact{artifact("README.md", "bob\n")})
	if err != nil {
		t.Fatal(err)
	}
	if !applied.LockExists() {
		t.Fatal("applied workspace reported no lock")
	}
	if counts := applied.ActionCounts(); counts.Unchanged != 1 || counts.Create != 0 || counts.Conflict != 0 {
		t.Fatalf("applied counts = %#v", counts)
	}
}
