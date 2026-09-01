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
	if drift.Repository.ConflictClass != "none" || drift.Repository.LockExists {
		t.Fatalf("drift classification = %#v", drift.Repository)
	}
	if drift.Repository.ActionCounts.Create == 0 || drift.Repository.ActionCounts.Conflict != 0 {
		t.Fatalf("drift action counts = %#v", drift.Repository.ActionCounts)
	}

	conflictRoot := contextWorkspace(t, maximalManifest(), true)
	if err := os.WriteFile(filepath.Join(conflictRoot, "internal/cli/root.go"), []byte("package cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflicted, err := Load(conflictRoot, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	// The fixture edits a lock-owned file, so its conflict family is
	// contract_drift and the review action's reason code carries the class.
	if conflicted.Repository.State != "conflicted" || conflicted.Repository.ConflictCount == 0 || conflicted.Actions[0].ReasonCode != "conflict_contract_drift" {
		t.Fatalf("conflicted = %#v actions=%#v", conflicted.Repository, conflicted.Actions)
	}
	if conflicted.Repository.ConflictClass != "contract_drift" || !conflicted.Repository.LockExists {
		t.Fatalf("conflicted classification = %#v", conflicted.Repository)
	}
	if len(conflicted.Repository.TopConflicts) == 0 || conflicted.Repository.TopConflicts[0].Path != "internal/cli/root.go" {
		t.Fatalf("top conflicts = %#v", conflicted.Repository.TopConflicts)
	}
}

// TestContextClassifiesUnmanagedDivergence reproduces the repository-evolved-
// past-the-recipe shape: no lock, human-owned files at recipe paths. The
// verdict stays conflicted (wire enum unchanged) while the additive fields
// say which kind of conflict it is.
func TestContextClassifiesUnmanagedDivergence(t *testing.T) {
	t.Parallel()
	lookPath := func(name string) (string, error) { return "/bin/" + name, nil }
	root := contextWorkspace(t, maximalManifest(), false)
	if err := os.WriteFile(filepath.Join(root, "Taskfile.yml"), []byte("human words\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Load(root, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	if result.Repository.State != "conflicted" {
		t.Fatalf("state = %q", result.Repository.State)
	}
	if result.Repository.ConflictClass != "unmanaged_divergence" {
		t.Fatalf("conflict class = %q", result.Repository.ConflictClass)
	}
	if result.Repository.LockExists || !result.Repository.LockChanged {
		t.Fatalf("lock projection = %#v", result.Repository)
	}
	if result.Repository.ConflictFamilyCounts["unmanaged_divergence"] != result.Repository.ConflictCount || result.Repository.ConflictFamilyCounts["contract_drift"] != 0 || result.Repository.ConflictFamilyCounts["ownership_hazard"] != 0 {
		t.Fatalf("conflict family counts = %#v", result.Repository.ConflictFamilyCounts)
	}
	if result.Actions[0].ReasonCode != "conflict_unmanaged_divergence" {
		t.Fatalf("reason code = %q", result.Actions[0].ReasonCode)
	}
}

// TestContextSurfaceEvidenceMismatchNotice proves the read-only surface
// probe: a disabled descriptive surface whose declared evidence exists in
// the repository produces a warning notice, while an enabled surface or a
// repository without evidence stays silent.
func TestContextSurfaceEvidenceMismatchNotice(t *testing.T) {
	t.Parallel()
	lookPath := func(name string) (string, error) { return "/bin/" + name, nil }

	withEvidence := contextWorkspace(t, maximalManifest(), false)
	if err := os.MkdirAll(filepath.Join(withEvidence, "internal", "mcp"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(withEvidence, "internal", "mcp", "server.go"), []byte("package mcp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := Load(withEvidence, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	notice := findNotice(t, result, "surface_evidence_mismatch")
	if notice.CapabilityID != "surface.mcp" || notice.Severity != "warning" {
		t.Fatalf("notice = %#v", notice)
	}
	if !strings.Contains(notice.Message, "flip surfaces.mcp to true in bob.yaml or remove the surface") {
		t.Fatalf("message = %q", notice.Message)
	}
	// The evidence paths name what was found, not what was looked for.
	if len(notice.Paths) == 0 || !containsStringIn(notice.Paths, "internal/mcp") {
		t.Fatalf("paths = %#v", notice.Paths)
	}

	// The same repository with the surface declared stays silent.
	enabledManifest := maximalManifest()
	enabledManifest.Surfaces.MCP = true
	enabledRoot := contextWorkspace(t, enabledManifest, false)
	if err := os.MkdirAll(filepath.Join(enabledRoot, "internal", "mcp"), 0o755); err != nil {
		t.Fatal(err)
	}
	enabled, err := Load(enabledRoot, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	for _, notice := range enabled.Notices {
		if notice.Code == "surface_evidence_mismatch" {
			t.Fatalf("enabled surface still warned: %#v", notice)
		}
	}

	// A repository without any declared evidence stays silent too.
	plain, err := Load(contextWorkspace(t, maximalManifest(), false), Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	for _, notice := range plain.Notices {
		if notice.Code == "surface_evidence_mismatch" {
			t.Fatalf("evidence-free repository warned: %#v", notice)
		}
	}
}

// TestContextEntrypointContainsEvidenceCatchesSubcommandMCP reproduces the
// teak shape: no internal/mcp package, but the recipe-known entrypoint
// mentions mcp. Only the bounded Contains rule fires.
func TestContextEntrypointContainsEvidenceCatchesSubcommandMCP(t *testing.T) {
	t.Parallel()
	lookPath := func(name string) (string, error) { return "/bin/" + name, nil }
	root := contextWorkspace(t, maximalManifest(), false)
	entrypoint := filepath.Join(root, "cmd", "acme", "main.go")
	if err := os.MkdirAll(filepath.Dir(entrypoint), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(entrypoint, []byte("package main // headless mcp subcommand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := repositorySnapshot(t, root)
	result, err := Load(root, Options{Profile: ProfileCompact, LookPath: lookPath})
	if err != nil {
		t.Fatal(err)
	}
	if after := repositorySnapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatalf("evidence evaluation mutated repository:\nbefore=%#v\nafter=%#v", before, after)
	}
	notice := findNotice(t, result, "surface_evidence_mismatch")
	if !containsStringIn(notice.Paths, "cmd/acme/main.go") {
		t.Fatalf("entrypoint evidence missing: %#v", notice.Paths)
	}
}

func findNotice(t *testing.T, result Result, code string) Notice {
	t.Helper()
	for _, notice := range result.Notices {
		if notice.Code == code {
			return notice
		}
	}
	t.Fatalf("missing notice with code %q: %#v", code, result.Notices)
	return Notice{}
}

func containsStringIn(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
