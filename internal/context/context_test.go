package context

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/guidance"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

func TestContextCleanDriftedAndConflicted(t *testing.T) {
	t.Parallel()
	lookPath := func(name string) (string, error) { return "/bin/" + name, nil }
	cleanRoot := contextWorkspace(t, maximalManifest(), true)
	clean, err := Load(cleanRoot, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	if !clean.Repository.Clean || clean.Repository.State != "clean" || len(clean.Actions) != 0 {
		t.Fatalf("clean = %#v", clean.Repository)
	}
	for _, capability := range clean.Capabilities {
		if capability.Verification != "not_assessed" {
			t.Fatalf("verification = %q", capability.Verification)
		}
	}

	driftRoot := contextWorkspace(t, maximalManifest(), false)
	drift, err := Load(driftRoot, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	if drift.Repository.State != "drifted" || drift.Repository.ConflictCount != 0 || len(drift.Actions) != 1 || drift.Actions[0].Effect != "read_only" {
		t.Fatalf("drift = %#v actions=%#v", drift.Repository, drift.Actions)
	}

	conflictRoot := contextWorkspace(t, maximalManifest(), true)
	if err := os.WriteFile(filepath.Join(conflictRoot, "internal/cli/root.go"), []byte("package cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflicted, err := Load(conflictRoot, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	if conflicted.Repository.State != "conflicted" || conflicted.Repository.ConflictCount == 0 || conflicted.Actions[0].ReasonCode != "ownership_conflict" {
		t.Fatalf("conflicted = %#v actions=%#v", conflicted.Repository, conflicted.Actions)
	}
}

func TestContextMissingAndInvalidManifestFailClosed(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if _, err := Load(root, Options{}); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, manifest.Filename), []byte("schema_version: nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, Options{}); err == nil || !strings.Contains(err.Error(), "decode manifest") {
		t.Fatalf("invalid error = %v", err)
	}
}

func TestContextIsReadOnlyAndOnlyPerformsOfflineLookup(t *testing.T) {
	t.Parallel()
	root := contextWorkspace(t, maximalManifest(), true)
	before := repositorySnapshot(t, root)
	var lookedUp []string
	_, err := Load(root, Options{Profile: ProfileFull, LookPath: func(name string) (string, error) { lookedUp = append(lookedUp, name); return "", errors.New("missing") }})
	if err != nil {
		t.Fatal(err)
	}
	after := repositorySnapshot(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("context mutated repository:\nbefore=%#v\nafter=%#v", before, after)
	}
	sort.Strings(lookedUp)
	want := []string{"cairn", "codemap", "fcheap", "glyph", "goreleaser", "tvault", "vecgrep"}
	if !reflect.DeepEqual(lookedUp, want) {
		t.Fatalf("offline lookups = %#v, want %#v", lookedUp, want)
	}
}

func TestContextProfilesShareDigestsAndCompactFitsBudget(t *testing.T) {
	t.Parallel()
	root := contextWorkspace(t, maximalManifest(), true)
	lookPath := func(name string) (string, error) { return "/bin/" + name, nil }
	results := map[Profile]Result{}
	maximum := 0
	for _, profile := range []Profile{ProfileCompact, ProfileStandard, ProfileFull} {
		result, err := Load(root, Options{Profile: profile, LookPath: lookPath})
		if err != nil {
			t.Fatal(err)
		}
		results[profile] = result
		if result.ContractDigest != results[ProfileCompact].ContractDigest || result.ContextDigest != results[ProfileCompact].ContextDigest {
			t.Fatalf("profile %s changed digests", profile)
		}
		data, err := json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		if profile == ProfileCompact {
			maximum = len(data)
			if maximum > 6144 {
				t.Fatalf("compact data = %d bytes", maximum)
			}
			if result.Truncation.Truncated {
				t.Fatalf("maximum current recipe fixture required compact truncation: %#v", result.Truncation)
			}
		}
	}
	t.Logf("maximum compact context data: %d bytes", maximum)
	if len(results[ProfileFull].Artifacts) == 0 || len(results[ProfileCompact].Artifacts) != 0 {
		t.Fatal("profile artifact projection is incorrect")
	}
}

func TestContextDigestsExcludeWorkspaceAndProfiles(t *testing.T) {
	t.Parallel()
	m := maximalManifest()
	left := contextWorkspace(t, m, true)
	right := contextWorkspace(t, m, true)
	lookPath := func(name string) (string, error) { return "", errors.New("missing") }
	a, err := Load(left, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(right, Options{Profile: ProfileFull, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	if a.ContractDigest != b.ContractDigest {
		t.Fatalf("contract digest depends on workspace: %s != %s", a.ContractDigest, b.ContractDigest)
	}
	if a.ContextDigest != b.ContextDigest {
		t.Fatalf("context digest depends on workspace/profile: %s != %s", a.ContextDigest, b.ContextDigest)
	}
}

func TestContractDigestIgnoresWorkspaceMaterialization(t *testing.T) {
	t.Parallel()
	m := maximalManifest()
	driftedRoot := contextWorkspace(t, m, false)
	cleanRoot := contextWorkspace(t, m, true)
	lookPath := func(name string) (string, error) { return "/bin/" + name, nil }
	drifted, err := Load(driftedRoot, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	clean, err := Load(cleanRoot, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	if drifted.Repository.State != "drifted" || clean.Repository.State != "clean" {
		t.Fatalf("fixture states = %q and %q", drifted.Repository.State, clean.Repository.State)
	}
	if drifted.ContractDigest != clean.ContractDigest {
		t.Fatalf("materialization changed contract digest: %s != %s", drifted.ContractDigest, clean.ContractDigest)
	}
	if drifted.ContextDigest == clean.ContextDigest {
		t.Fatal("materialization did not change context digest")
	}
}

func TestContractDigestExcludesAvailability(t *testing.T) {
	t.Parallel()
	root := contextWorkspace(t, maximalManifest(), true)
	available, err := Load(root, Options{Profile: ProfileCompact, LookPath: func(name string) (string, error) { return "/bin/" + name, nil }})
	if err != nil {
		t.Fatal(err)
	}
	unavailable, err := Load(root, Options{Profile: ProfileCompact, LookPath: func(string) (string, error) { return "", errors.New("missing") }})
	if err != nil {
		t.Fatal(err)
	}
	if available.ContractDigest != unavailable.ContractDigest {
		t.Fatal("contract digest includes environment availability")
	}
	if available.ContextDigest == unavailable.ContextDigest {
		t.Fatal("context digest omitted projected environment availability")
	}
}

func TestContextDigestBindsContractDigest(t *testing.T) {
	t.Parallel()
	leftManifest := manifest.Manifest{
		SchemaVersion: 1, Recipe: manifest.RecipeFiles,
		Product: manifest.Product{Name: "files", Description: "First contract"},
		Files:   []manifest.FileDecl{{Path: "same.txt", Content: "same\n"}},
	}
	rightManifest := leftManifest
	rightManifest.Product.Description = "Second contract"
	lookPath := func(string) (string, error) { return "", errors.New("unexpected") }
	left, err := Load(contextWorkspace(t, leftManifest, true), Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	right, err := Load(contextWorkspace(t, rightManifest, true), Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	if left.ContractDigest == right.ContractDigest {
		t.Fatal("fixture did not change contract digest")
	}
	if left.ContextDigest == right.ContextDigest {
		t.Fatal("context digest did not bind the changed contract digest")
	}
}

func TestContractDigestNormalizesFilesDeclarationOrderPathsAndDefaultMode(t *testing.T) {
	t.Parallel()
	leftManifest := manifest.Manifest{
		SchemaVersion: 1, Recipe: manifest.RecipeFiles,
		Product: manifest.Product{Name: "files", Description: "Equivalent tree"},
		Files: []manifest.FileDecl{
			{Path: "./nested/../b.txt", Content: "b\n"},
			{Path: "a.txt", Mode: "0644", Content: "a\n"},
		},
	}
	rightManifest := leftManifest
	rightManifest.Files = []manifest.FileDecl{
		{Path: "a.txt", Mode: "644", Content: "a\n"},
		{Path: "b.txt", Mode: "0644", Content: "b\n"},
	}
	lookPath := func(string) (string, error) { return "", errors.New("unexpected") }
	left, err := Load(contextWorkspace(t, leftManifest, true), Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	right, err := Load(contextWorkspace(t, rightManifest, true), Options{Profile: ProfileFull, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	if left.ContractDigest != right.ContractDigest || left.ContextDigest != right.ContextDigest {
		t.Fatalf("equivalent file trees changed digests: contract %s/%s context %s/%s", left.ContractDigest, right.ContractDigest, left.ContextDigest, right.ContextDigest)
	}
}

func TestContextTruncationIsDeterministic(t *testing.T) {
	t.Parallel()
	m := manifest.Manifest{SchemaVersion: 1, Recipe: manifest.RecipeFiles, Product: manifest.Product{Name: "many-files", Description: "Many files"}}
	for i := 0; i < 600; i++ {
		m.Files = append(m.Files, manifest.FileDecl{Path: filepath.ToSlash(filepath.Join("generated", strings.Repeat("segment", 3), fmtName(i)+".txt")), Content: "value\n"})
	}
	root := contextWorkspace(t, m, false)
	a, err := Load(root, Options{Profile: ProfileFull, LookPath: func(string) (string, error) { return "", errors.New("unexpected") }})
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(root, Options{Profile: ProfileFull, LookPath: func(string) (string, error) { return "", errors.New("unexpected") }})
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	if string(left) != string(right) {
		t.Fatal("truncation is not deterministic")
	}
	if !a.Truncation.Truncated || len(left) > 64<<10 || a.Truncation.Omitted["artifacts"] == 0 {
		t.Fatalf("truncation = %#v bytes=%d", a.Truncation, len(left))
	}
}

func TestContextFailsClosedWhenIdentityExceedsProfileBound(t *testing.T) {
	t.Parallel()
	m := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Recipe:        manifest.RecipeFiles,
		Product: manifest.Product{
			Name:        strings.Repeat("a", 70<<10),
			Description: "Oversized bounded-output fixture",
		},
		Files: []manifest.FileDecl{{Path: "small.txt", Content: "small\n"}},
	}
	root := contextWorkspace(t, m, false)
	for _, profile := range []Profile{ProfileCompact, ProfileStandard, ProfileFull} {
		_, err := Load(root, Options{Profile: profile, LookPath: func(string) (string, error) {
			return "", errors.New("unexpected lookup")
		}})
		if err == nil {
			t.Fatalf("profile %s returned an oversized result", profile)
		}
		if code, ok := guidance.ErrorCode(err); !ok || code != "context_failed" {
			t.Fatalf("profile %s error code = %q, %t; error=%v", profile, code, ok, err)
		}
		if !strings.Contains(err.Error(), fmt.Sprintf("exceeds %d-byte bound", mustProfileLimit(t, profile))) {
			t.Fatalf("profile %s error = %v", profile, err)
		}
	}
}

func TestFilesContextDoesNotInferApplicationSemantics(t *testing.T) {
	t.Parallel()
	m := manifest.Manifest{SchemaVersion: 1, Recipe: manifest.RecipeFiles, Product: manifest.Product{Name: "service", Description: "Service"}, Files: []manifest.FileDecl{{Path: "cmd/server/main.go", Content: "package main\n"}}}
	result, err := Load(contextWorkspace(t, m, false), Options{Profile: ProfileFull, LookPath: func(string) (string, error) { return "", errors.New("unexpected") }})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.EntryPoints) != 0 {
		t.Fatalf("files recipe inferred entry point: %#v", result.EntryPoints)
	}
	for _, artifact := range result.Artifacts {
		if !reflect.DeepEqual(artifact.Roles, []string{"declared_file"}) {
			t.Fatalf("roles = %#v", artifact.Roles)
		}
	}
}

type snapshotEntry struct {
	Data    string
	Mode    os.FileMode
	ModTime time.Time
}

func repositorySnapshot(t *testing.T, root string) map[string]snapshotEntry {
	t.Helper()
	result := map[string]snapshotEntry{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		result[filepath.ToSlash(rel)] = snapshotEntry{string(data), info.Mode(), info.ModTime()}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func contextWorkspace(t *testing.T, m manifest.Manifest, apply bool) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	if apply {
		artifacts, err := recipe.Render(m)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Apply(root, m, artifacts); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func maximalManifest() manifest.Manifest {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	m.Integrations.BrowserVerification = "cairntrace"
	m.Integrations.Secrets = "tinyvault"
	m.Integrations.Artifacts = "fcheap"
	m.Distribution.Homebrew = true
	return m
}

func fmtName(value int) string { return fmt.Sprintf("file-%04d", value) }

func mustProfileLimit(t *testing.T, profile Profile) int {
	t.Helper()
	limit, err := profileLimit(profile)
	if err != nil {
		t.Fatal(err)
	}
	return limit
}
