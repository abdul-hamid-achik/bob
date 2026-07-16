package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

func stackManifestForTest(t *testing.T) manifest.Manifest {
	t.Helper()
	m, err := manifest.DefaultStack(manifest.RecipeTSApp, "demo", "", "A demo repository.", "")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestPlanSeedArtifactIsSatisfiedByAnyExistingContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("human words\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	m := stackManifestForTest(t)
	artifacts := []recipe.Artifact{
		{Path: "README.md", Seed: true, Content: []byte("seeded readme\n")},
		{Path: "SECURITY.md", Seed: true, Content: []byte("seeded policy\n")},
	}
	plan, err := Plan(root, m, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasConflicts() {
		t.Fatalf("seed artifacts must never conflict: %#v", plan.Actions)
	}
	byPath := map[string]Action{}
	for _, action := range plan.Actions {
		byPath[action.Path] = action
	}
	readme := byPath["README.md"]
	if readme.Kind != ActionUnchanged || readme.Code != CodeSeedExists {
		t.Fatalf("existing seed destination = %s/%s, want unchanged/%s", readme.Kind, readme.Code, CodeSeedExists)
	}
	security := byPath["SECURITY.md"]
	if security.Kind != ActionCreate || security.Code != CodeMissing {
		t.Fatalf("missing seed destination = %s/%s, want create/%s", security.Kind, security.Code, CodeMissing)
	}
	if len(plan.DesiredLock.Files) != 0 {
		t.Fatalf("seed artifacts must not be lock-owned: %#v", plan.DesiredLock.Files)
	}
}

func TestPlanSeedArtifactTreatsSymlinkAsSatisfied(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "target.md"), []byte("elsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("target.md", filepath.Join(root, "README.md")); err != nil {
		t.Fatal(err)
	}
	m := stackManifestForTest(t)
	plan, err := Plan(root, m, []recipe.Artifact{{Path: "README.md", Seed: true, Content: []byte("seed\n")}})
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasConflicts() {
		t.Fatalf("a symlink satisfies a seed artifact: %#v", plan.Actions)
	}
	if plan.Actions[0].Kind != ActionUnchanged || plan.Actions[0].Code != CodeSeedExists {
		t.Fatalf("unexpected action: %#v", plan.Actions[0])
	}
}

func TestApplySeedsOnceAndNeverUpdates(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := stackManifestForTest(t)
	artifacts := []recipe.Artifact{
		{Path: "README.md", Seed: true, Content: []byte("seeded readme\n")},
	}
	result, err := Apply(root, m, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Written) != 1 || result.Written[0] != "README.md" {
		t.Fatalf("unexpected written set: %#v", result.Written)
	}
	content, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || string(content) != "seeded readme\n" {
		t.Fatalf("seed content = %q, err=%v", content, err)
	}

	// A later human rewrite must stay unchanged and never conflict or revert.
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("rewritten by human\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := Apply(root, m, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Written) != 0 || len(second.Unchanged) != 1 {
		t.Fatalf("second apply must be a no-op: %#v", second)
	}
	content, err = os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || string(content) != "rewritten by human\n" {
		t.Fatalf("human content must survive apply: %q err=%v", content, err)
	}
}

func TestSeedAndOwnedArtifactsCoexist(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := stackManifestForTest(t)
	artifacts := []recipe.Artifact{
		{Path: "README.md", Seed: true, Content: []byte("seed\n")},
		{Path: "owned.txt", Content: []byte("owned bytes\n")},
	}
	plan, err := Plan(root, m, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.DesiredLock.Files) != 1 || plan.DesiredLock.Files[0].Path != "owned.txt" {
		t.Fatalf("only the non-seed artifact may be lock-owned: %#v", plan.DesiredLock.Files)
	}
	if _, err := Apply(root, m, artifacts); err != nil {
		t.Fatal(err)
	}
	// The owned artifact keeps whole-file conflict semantics.
	if err := os.WriteFile(filepath.Join(root, "owned.txt"), []byte("hand edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	drifted, err := Plan(root, m, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if !drifted.HasConflicts() {
		t.Fatalf("owned artifact edit must conflict: %#v", drifted.Actions)
	}
}
