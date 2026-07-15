package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

func reviewedDigest(t *testing.T, root string, artifacts []recipe.Artifact) string {
	t.Helper()
	plan, err := Plan(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	return DigestPlan(plan).Qualified()
}

func assertNoApplyDebris(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() == ApplyLockFilename || strings.HasPrefix(entry.Name(), ".bob-stage-") || strings.HasPrefix(entry.Name(), ".bob-atomic-") {
			t.Fatalf("apply left temporary state %q", entry.Name())
		}
	}
}

func TestValidateExpectedPlanDigestRequiresExactQualifiedLowercaseForm(t *testing.T) {
	t.Parallel()
	valid := "sha256:" + strings.Repeat("a", 64)
	if err := ValidateExpectedPlanDigest(valid); err != nil {
		t.Fatalf("valid digest rejected: %v", err)
	}
	for _, value := range []string{
		"", strings.Repeat("a", 64), "SHA256:" + strings.Repeat("a", 64),
		"sha256:" + strings.Repeat("A", 64), "sha256:" + strings.Repeat("a", 63),
		" sha256:" + strings.Repeat("a", 64), "sha256:" + strings.Repeat("a", 64) + "\n",
	} {
		if err := ValidateExpectedPlanDigest(value); !errors.Is(err, ErrInvalidPlanDigest) {
			t.Errorf("ValidateExpectedPlanDigest(%q) error = %v, want ErrInvalidPlanDigest", value, err)
		}
	}
}

func TestDigestGatedApplyAcceptsExactFreshPlanAndReturnsReceipt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{
		artifact("cmd/acme/main.go", "package main\n"),
		artifact("README.md", "hello\n"),
	}
	expected := reviewedDigest(t, root, artifacts)
	result, err := ApplyWithOptions(root, testManifest(), artifacts, ApplyOptions{ExpectedPlanDigest: expected})
	if err != nil {
		t.Fatal(err)
	}
	wantWritten := []string{"README.md", "cmd/acme/main.go"}
	if !reflect.DeepEqual(result.Written, wantWritten) {
		t.Fatalf("written = %v, want %v", result.Written, wantWritten)
	}
	receipt := result.Receipt
	if receipt.SchemaVersion != ApplyReceiptSchemaVersion || receipt.PlanDigestVersion != PlanDigestVersion {
		t.Fatalf("receipt versions = %#v", receipt)
	}
	if receipt.ExpectedPlanDigest != expected || receipt.AppliedPlanDigest != expected {
		t.Fatalf("receipt digests = %#v, want %s", receipt, expected)
	}
	if !reflect.DeepEqual(receipt.Written, wantWritten) || len(receipt.Adopted) != 0 || len(receipt.Unchanged) != 0 {
		t.Fatalf("receipt changes = %#v", receipt)
	}
	if !receipt.LockWritten || !receipt.ConvergedAfterApply {
		t.Fatalf("receipt convergence = %#v", receipt)
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	wantNext := []string{"bob", "check", canonicalRoot, "--json"}
	if !reflect.DeepEqual(receipt.NextCheck.Argv, wantNext) {
		t.Fatalf("next check = %v, want %v", receipt.NextCheck.Argv, wantNext)
	}
	assertNoApplyDebris(t, root)
}

func TestDigestMismatchRefusesBeforeAnyRepositoryWrite(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	reviewed := []recipe.Artifact{artifact("README.md", "reviewed\n")}
	expected := reviewedDigest(t, root, reviewed)
	current := []recipe.Artifact{
		artifact("README.md", "changed after review\n"),
		artifact("nested/new.txt", "must not be staged\n"),
	}
	result, err := ApplyWithOptions(root, testManifest(), current, ApplyOptions{ExpectedPlanDigest: expected})
	if !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("apply error = %v, want ErrPlanDigestMismatch", err)
	}
	var mismatch *PlanDigestMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("apply error type = %T, want *PlanDigestMismatchError", err)
	}
	if mismatch.ExpectedPlanDigest != expected || mismatch.ActualPlanDigest == expected {
		t.Fatalf("mismatch details = %#v", mismatch)
	}
	if result.Receipt.SchemaVersion != 0 {
		t.Fatalf("mismatch returned a mutation receipt: %#v", result.Receipt)
	}
	if len(result.Written) != 0 || result.LockWritten {
		t.Fatalf("mismatched apply mutated: %#v", result)
	}
	for _, path := range []string{"README.md", "nested", LockFilename} {
		if _, statErr := os.Lstat(filepath.Join(root, path)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("mismatched apply created %s: %v", path, statErr)
		}
	}
	assertNoApplyDebris(t, root)
}

func TestDigestMismatchPrecedesConflictAndPreservesReviewedWorkspace(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	initial := []recipe.Artifact{artifact("README.md", "owned\n")}
	if _, err := Apply(root, testManifest(), initial); err != nil {
		t.Fatal(err)
	}
	next := []recipe.Artifact{artifact("README.md", "generated update\n")}
	expected := reviewedDigest(t, root, next)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(filepath.Join(root, LockFilename))
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyWithOptions(root, testManifest(), next, ApplyOptions{ExpectedPlanDigest: expected})
	if !errors.Is(err, ErrPlanDigestMismatch) || errors.Is(err, ErrPlanConflicts) {
		t.Fatalf("apply error = %v, want digest mismatch before conflict refusal", err)
	}
	if result.Plan.ConflictCount != 1 || result.Plan.Actions[0].Code != CodeManagedHashMismatch {
		t.Fatalf("fresh plan = %#v", result.Plan)
	}
	content, _ := os.ReadFile(filepath.Join(root, "README.md"))
	lockAfter, _ := os.ReadFile(filepath.Join(root, LockFilename))
	if string(content) != "human edit\n" || !reflect.DeepEqual(lockAfter, lockBefore) {
		t.Fatalf("mismatch changed workspace: content=%q lock_equal=%t", content, reflect.DeepEqual(lockAfter, lockBefore))
	}
	assertNoApplyDebris(t, root)
}

func TestDigestMismatchAfterLockChangesLeavesLockUntouched(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{artifact("README.md", "owned\n")}
	if _, err := Apply(root, testManifest(), artifacts); err != nil {
		t.Fatal(err)
	}
	expected := reviewedDigest(t, root, artifacts)
	lockPath := filepath.Join(root, LockFilename)
	file, err := os.OpenFile(lockPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("# changed after review\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyWithOptions(root, testManifest(), artifacts, ApplyOptions{ExpectedPlanDigest: expected}); !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("apply error = %v, want ErrPlanDigestMismatch", err)
	}
	after, err := os.ReadFile(lockPath)
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("digest mismatch rewrote lock: %v", err)
	}
	assertNoApplyDebris(t, root)
}

func TestModeOnlyDriftInvalidatesReviewedPlanBeforeRepair(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{artifact("README.md", "owned\n")}
	if _, err := Apply(root, testManifest(), artifacts); err != nil {
		t.Fatal(err)
	}
	expected := reviewedDigest(t, root, artifacts)
	path := filepath.Join(root, "README.md")
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	beforeLock, err := os.ReadFile(filepath.Join(root, LockFilename))
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyWithOptions(root, testManifest(), artifacts, ApplyOptions{ExpectedPlanDigest: expected})
	if !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("apply error = %v, want ErrPlanDigestMismatch", err)
	}
	if result.Plan.Actions[0].Code != CodeModeDrift || result.Plan.Actions[0].Kind != ActionUpdate {
		t.Fatalf("mode-drift plan = %#v", result.Plan.Actions[0])
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mismatch repaired unreviewed mode: %v", info.Mode().Perm())
	}
	afterLock, err := os.ReadFile(filepath.Join(root, LockFilename))
	if err != nil || !reflect.DeepEqual(afterLock, beforeLock) {
		t.Fatalf("mode mismatch rewrote lock: %v", err)
	}
	assertNoApplyDebris(t, root)
}

func TestMatchingDigestDoesNotBypassOwnershipConflict(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte("human\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts := []recipe.Artifact{artifact("README.md", "generated\n")}
	expected := reviewedDigest(t, root, artifacts)
	result, err := ApplyWithOptions(root, testManifest(), artifacts, ApplyOptions{ExpectedPlanDigest: expected})
	if !errors.Is(err, ErrPlanConflicts) || errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("apply error = %v, want ErrPlanConflicts", err)
	}
	if result.Receipt.SchemaVersion != 0 || len(result.Written) != 0 || result.LockWritten {
		t.Fatalf("conflicted result = %#v", result)
	}
	content, _ := os.ReadFile(path)
	if string(content) != "human\n" {
		t.Fatalf("conflicted apply changed human file: %q", content)
	}
	assertNoApplyDebris(t, root)
}

func TestInvalidExpectedDigestIsRejectedBeforeWorkspaceAndLockResolution(t *testing.T) {
	t.Parallel()
	missingRoot := filepath.Join(t.TempDir(), "missing")
	_, err := ApplyWithOptions(missingRoot, testManifest(), nil, ApplyOptions{ExpectedPlanDigest: "not-a-digest"})
	if !errors.Is(err, ErrInvalidPlanDigest) || strings.Contains(err.Error(), "workspace") {
		t.Fatalf("apply error = %v, want digest validation before workspace resolution", err)
	}
	if _, statErr := os.Lstat(missingRoot); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("invalid digest created workspace state: %v", statErr)
	}
}

func TestUnguardedApplyStillReturnsAnHonestReceipt(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	artifacts := []recipe.Artifact{artifact("README.md", "hello\n")}
	result, err := Apply(root, testManifest(), artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if result.Receipt.ExpectedPlanDigest != "" || result.Receipt.AppliedPlanDigest == "" || !result.Receipt.ConvergedAfterApply {
		t.Fatalf("unguarded receipt = %#v", result.Receipt)
	}
}

func TestWorkspaceApplyReloadsManifestUnderLockAndRejectsStaleReview(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	reviewedManifest := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Recipe:        manifest.RecipeFiles,
		Product:       manifest.Product{Name: "race-fixture", Description: "Manifest race fixture"},
		Files:         []manifest.FileDecl{{Path: "reviewed.txt", Content: "reviewed\n"}},
	}
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), reviewedManifest, false); err != nil {
		t.Fatal(err)
	}
	reviewedArtifacts, err := recipe.Render(reviewedManifest)
	if err != nil {
		t.Fatal(err)
	}
	reviewedPlan, err := Plan(root, reviewedManifest, reviewedArtifacts)
	if err != nil {
		t.Fatal(err)
	}
	expected := DigestPlan(reviewedPlan).Qualified()

	// This deterministic sequence models the old CLI race: intent was loaded
	// and reviewed, then bob.yaml changed before apply acquired its lock.
	currentManifest := reviewedManifest
	currentManifest.Files = []manifest.FileDecl{
		{Path: "current.txt", Content: "current\n"},
		{Path: "nested/must-not-stage.txt", Content: "no writes\n"},
	}
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), currentManifest, true); err != nil {
		t.Fatal(err)
	}

	result, err := ApplyWorkspaceWithOptions(root, ApplyOptions{ExpectedPlanDigest: expected})
	if !errors.Is(err, ErrPlanDigestMismatch) {
		t.Fatalf("apply error = %v, want ErrPlanDigestMismatch", err)
	}
	if result.Plan.Recipe.ID != manifest.RecipeFiles || DigestPlan(result.Plan).Qualified() == expected {
		t.Fatalf("fresh plan did not bind current manifest: %#v", result.Plan)
	}
	for _, path := range []string{"reviewed.txt", "current.txt", "nested", LockFilename} {
		if _, statErr := os.Lstat(filepath.Join(root, path)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("stale-review apply created %s: %v", path, statErr)
		}
	}
	loaded, loadErr := manifest.Load(root)
	if loadErr != nil || !reflect.DeepEqual(loaded, currentManifest) {
		t.Fatalf("apply changed current manifest: loaded=%#v err=%v", loaded, loadErr)
	}
	assertNoApplyDebris(t, root)
}

func TestFilesRecipeReceiptIsBoundedWithCompleteCounts(t *testing.T) {
	t.Parallel()
	const fileCount = 2400
	m := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Recipe:        manifest.RecipeFiles,
		Product:       manifest.Product{Name: "many-files", Description: "Many files receipt fixture"},
		Files:         make([]manifest.FileDecl, 0, fileCount),
	}
	for i := 0; i < fileCount; i++ {
		m.Files = append(m.Files, manifest.FileDecl{
			Path:    fmt.Sprintf("generated/group-%02d/file-%04d-with-a-stable-name.txt", i%20, i),
			Content: "x\n",
		})
	}
	artifacts, err := recipe.Render(m)
	if err != nil {
		t.Fatal(err)
	}
	result := ApplyResult{
		Receipt: newApplyReceipt(t.TempDir(), "sha256:"+strings.Repeat("a", 64), PlanDigest{Version: PlanDigestVersion, SHA256: strings.Repeat("a", 64)}),
	}
	for i, item := range artifacts {
		switch i % 3 {
		case 0:
			result.Written = append(result.Written, item.Path)
		case 1:
			result.Adopted = append(result.Adopted, item.Path)
		default:
			result.Unchanged = append(result.Unchanged, item.Path)
		}
	}
	result.finalizeReceipt(true)
	receipt := result.Receipt
	encoded, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > ApplyReceiptByteLimit {
		t.Fatalf("receipt bytes = %d, limit = %d", len(encoded), ApplyReceiptByteLimit)
	}
	if receipt.WrittenCount+receipt.AdoptedCount+receipt.UnchangedCount != fileCount {
		t.Fatalf("complete counts lost: %#v", receipt)
	}
	if !receipt.Truncation.Truncated || receipt.Truncation.ByteLimit != ApplyReceiptByteLimit {
		t.Fatalf("truncation = %#v", receipt.Truncation)
	}
	omitted := receipt.Truncation.Omitted["written"] + receipt.Truncation.Omitted["adopted"] + receipt.Truncation.Omitted["unchanged"]
	returned := len(receipt.Written) + len(receipt.Adopted) + len(receipt.Unchanged)
	if returned+omitted != fileCount {
		t.Fatalf("returned=%d omitted=%d total=%d", returned, omitted, fileCount)
	}

	// The same complete reconciliation always retains the same prefixes.
	second := ApplyResult{Written: result.Written, Adopted: result.Adopted, Unchanged: result.Unchanged, Receipt: newApplyReceipt(t.TempDir(), "sha256:"+strings.Repeat("a", 64), PlanDigest{Version: PlanDigestVersion, SHA256: strings.Repeat("a", 64)})}
	second.Receipt.NextCheck = receipt.NextCheck
	second.finalizeReceipt(true)
	secondEncoded, err := json.Marshal(second.Receipt)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(encoded, secondEncoded) {
		t.Fatal("receipt truncation is not deterministic")
	}
}
