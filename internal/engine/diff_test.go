package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

func TestPlanDiffCreateShowsAllAdditions(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{artifact("hello.txt", "line one\nline two\nline three\n")}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := PlanDiff(root, &plan, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	d := diffs[0]
	if d.Path != "hello.txt" || d.Kind != "create" {
		t.Fatalf("unexpected diff header: path=%q kind=%q", d.Path, d.Kind)
	}
	if d.OldLines != nil {
		t.Fatalf("create diff should have nil OldLines, got %v", d.OldLines)
	}
	if len(d.NewLines) != 3 {
		t.Fatalf("expected 3 new lines, got %d: %v", len(d.NewLines), d.NewLines)
	}
	if !strings.Contains(d.Unified, "--- a/hello.txt") || !strings.Contains(d.Unified, "+++ b/hello.txt") {
		t.Fatalf("unified diff missing file headers:\n%s", d.Unified)
	}
	if !strings.Contains(d.Unified, "+line one") || !strings.Contains(d.Unified, "+line two") || !strings.Contains(d.Unified, "+line three") {
		t.Fatalf("unified diff missing addition lines:\n%s", d.Unified)
	}
	if strings.Contains(d.Unified, "-line") {
		t.Fatalf("create diff should not contain deletions:\n%s", d.Unified)
	}
}

func TestPlanDiffUpdateShowsChanges(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	original := []recipe.Artifact{artifact("config.yaml", "name: old\nversion: 1\ndebug: false\n")}
	if _, err := Apply(root, testManifest(), original); err != nil {
		t.Fatal(err)
	}
	updated := []recipe.Artifact{artifact("config.yaml", "name: new\nversion: 2\ndebug: false\n")}
	plan, err := Plan(root, testManifest(), updated)
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := PlanDiff(root, &plan, updated)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	d := diffs[0]
	if d.Path != "config.yaml" || d.Kind != "update" {
		t.Fatalf("unexpected diff header: path=%q kind=%q", d.Path, d.Kind)
	}
	if len(d.OldLines) != 3 || len(d.NewLines) != 3 {
		t.Fatalf("expected 3 old and 3 new lines, got %d old, %d new", len(d.OldLines), len(d.NewLines))
	}
	if !strings.Contains(d.Unified, "-name: old") || !strings.Contains(d.Unified, "+name: new") {
		t.Fatalf("unified diff missing name change:\n%s", d.Unified)
	}
	if !strings.Contains(d.Unified, "-version: 1") || !strings.Contains(d.Unified, "+version: 2") {
		t.Fatalf("unified diff missing version change:\n%s", d.Unified)
	}
	// The unchanged "debug: false" line should appear as context.
	if !strings.Contains(d.Unified, " debug: false") {
		t.Fatalf("unified diff missing context line:\n%s", d.Unified)
	}
}

func TestPlanDiffSkipsUnchangedAndConflict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// Create a managed file, then plan with identical content (unchanged)
	// and a conflicting unmanaged file.
	managed := []recipe.Artifact{artifact("managed.txt", "stable\n")}
	if _, err := Apply(root, testManifest(), managed); err != nil {
		t.Fatal(err)
	}
	// Write an unmanaged file that will conflict.
	if err := os.WriteFile(filepath.Join(root, "unmanaged.txt"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts := []recipe.Artifact{
		artifact("managed.txt", "stable\n"),
		artifact("unmanaged.txt", "desired\n"),
	}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	// Verify we have unchanged and conflict actions.
	kinds := actionKinds(plan)
	hasUnchanged := false
	hasConflict := false
	for _, k := range kinds {
		if k == ActionUnchanged {
			hasUnchanged = true
		}
		if k == ActionConflict {
			hasConflict = true
		}
	}
	if !hasUnchanged || !hasConflict {
		t.Fatalf("test setup: expected unchanged and conflict actions, got %v", kinds)
	}
	diffs, err := PlanDiff(root, &plan, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Fatalf("expected 0 diffs for unchanged/conflict actions, got %d", len(diffs))
	}
}

func TestPlanDiffEmptyFileCreation(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{artifact("empty.txt", "")}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := PlanDiff(root, &plan, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	d := diffs[0]
	if d.Kind != "create" {
		t.Fatalf("expected create, got %q", d.Kind)
	}
	if d.NewLines != nil {
		t.Fatalf("empty file should have nil NewLines, got %v", d.NewLines)
	}
	// An empty create produces an empty unified diff (no lines to show).
	if d.Unified != "" {
		t.Fatalf("expected empty unified diff for empty file, got:\n%s", d.Unified)
	}
}

func TestPlanDiffLongLines(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	longLine := strings.Repeat("x", 10000)
	content := longLine + "\nshort\n"
	artifacts := []recipe.Artifact{artifact("wide.txt", content)}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := PlanDiff(root, &plan, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	d := diffs[0]
	if d.Note != "" {
		t.Fatalf("long lines should not be skipped, got note: %s", d.Note)
	}
	if !strings.Contains(d.Unified, "+"+longLine) {
		t.Fatalf("unified diff missing long line:\n%.200s...", d.Unified)
	}
}

func TestPlanDiffSkipsOversizedContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	big := strings.Repeat("a\n", (diffByteLimit/2)+1)
	artifacts := []recipe.Artifact{artifact("big.txt", big)}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := PlanDiff(root, &plan, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if diffs[0].Note == "" {
		t.Fatal("expected a skip note for oversized content")
	}
	if diffs[0].Unified != "" {
		t.Fatalf("oversized content should have empty unified diff, got:\n%.200s...", diffs[0].Unified)
	}
}

func TestPlanDiffNoTrailingNewline(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	original := []recipe.Artifact{artifact("noeol.txt", "alpha\nbeta")}
	if _, err := Apply(root, testManifest(), original); err != nil {
		t.Fatal(err)
	}
	updated := []recipe.Artifact{artifact("noeol.txt", "alpha\ngamma")}
	plan, err := Plan(root, testManifest(), updated)
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := PlanDiff(root, &plan, updated)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(diffs))
	}
	if !strings.Contains(diffs[0].Unified, "\\ No newline at end of file") {
		t.Fatalf("expected no-newline marker:\n%s", diffs[0].Unified)
	}
}

func TestPlanDiffMultipleFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{
		artifact("a.txt", "aaa\n"),
		artifact("b.txt", "bbb\n"),
		artifact("c.txt", "ccc\n"),
	}
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	diffs, err := PlanDiff(root, &plan, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 3 {
		t.Fatalf("expected 3 diffs, got %d", len(diffs))
	}
	for i, want := range []string{"a.txt", "b.txt", "c.txt"} {
		if diffs[i].Path != want {
			t.Fatalf("diff[%d].Path = %q, want %q", i, diffs[i].Path, want)
		}
	}
}

func TestFormatUnifiedDiffHunkHeaders(t *testing.T) {
	t.Parallel()
	// Verify the @@ hunk header line numbers are correct.
	old := "one\ntwo\nthree\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
	new := "one\ntwo\nTHREE\nfour\nfive\nsix\nseven\neight\nnine\nten\n"
	got := formatUnifiedDiff("test.txt", old, new)
	if !strings.Contains(got, "@@ -1,") {
		t.Fatalf("expected hunk starting at line 1:\n%s", got)
	}
	if !strings.Contains(got, "-three") || !strings.Contains(got, "+THREE") {
		t.Fatalf("expected three->THREE change:\n%s", got)
	}
}

func TestFormatUnifiedDiffIdenticalContent(t *testing.T) {
	t.Parallel()
	got := formatUnifiedDiff("same.txt", "hello\n", "hello\n")
	if got != "" {
		t.Fatalf("identical content should produce empty diff, got:\n%s", got)
	}
}

func TestFormatUnifiedDiffEmptyToContent(t *testing.T) {
	t.Parallel()
	got := formatUnifiedDiff("new.txt", "", "hello\nworld\n")
	if !strings.Contains(got, "+hello") || !strings.Contains(got, "+world") {
		t.Fatalf("expected all-addition diff:\n%s", got)
	}
	if strings.Contains(got, "-") && !strings.Contains(got, "---") {
		t.Fatalf("create diff should not have deletions:\n%s", got)
	}
}
