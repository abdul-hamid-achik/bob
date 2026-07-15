package recipe_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"go.yaml.in/yaml/v3"
)

type publishedLock struct {
	SchemaVersion int `yaml:"schema_version"`
	Recipe        struct {
		ID      string `yaml:"id"`
		Version int    `yaml:"version"`
	} `yaml:"recipe"`
	Files []struct {
		Path   string `yaml:"path"`
		SHA256 string `yaml:"sha256"`
	} `yaml:"files"`
}

func TestPublishedRecipeV2PreservesEveryPublishedV1ManagedPath(t *testing.T) {
	t.Parallel()
	v1, _ := loadPublishedLock(t, 1)
	v2, _ := loadPublishedLock(t, 2)
	v2Files := publishedFiles(v2)

	changed := 0
	v1Paths := make(map[string]struct{}, len(v1.Files))
	for _, entry := range v1.Files {
		v1Paths[entry.Path] = struct{}{}
		current, ok := v2Files[entry.Path]
		if !ok {
			t.Errorf("published recipe v2 dropped published v1 path %s", entry.Path)
			continue
		}
		if current != entry.SHA256 {
			changed++
		}
	}
	added := 0
	for path := range v2Files {
		if _, ok := v1Paths[path]; !ok {
			added++
		}
	}
	if changed == 0 || added == 0 {
		t.Fatalf("v1 to v2 delta has changed=%d added=%d, want both positive", changed, added)
	}
	for _, path := range []string{"CODE_OF_CONDUCT.md", ".github/dependabot.yml", ".github/pull_request_template.md"} {
		if _, ok := v2Files[path]; !ok {
			t.Errorf("published recipe v2 missing expected new path %s", path)
		}
	}
}

func TestRecipeV3UpgradeRetainsV2PathsAndRaisesGoSecurityPatch(t *testing.T) {
	t.Parallel()
	v2, _ := loadPublishedLock(t, 2)
	v2Files := publishedFiles(v2)

	m, err := manifest.LoadFile(filepath.Join("..", "..", "examples", "integrated", manifest.Filename))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := recipe.RenderVersion(m, 3)
	if err != nil {
		t.Fatal(err)
	}
	v3Files := make(map[string]string, len(artifacts))
	v3Content := make(map[string][]byte, len(artifacts))
	for _, artifact := range artifacts {
		v3Files[artifact.Path] = contentHash(artifact.Content)
		v3Content[artifact.Path] = artifact.Content
	}

	var changed, added []string
	for _, entry := range v2.Files {
		current, ok := v3Files[entry.Path]
		if !ok {
			t.Errorf("recipe v3 dropped published v2 path %s", entry.Path)
			continue
		}
		if current != entry.SHA256 {
			changed = append(changed, entry.Path)
		}
	}
	for path := range v3Files {
		if _, ok := v2Files[path]; !ok {
			added = append(added, path)
		}
	}
	sort.Strings(changed)
	sort.Strings(added)
	if !reflect.DeepEqual(changed, []string{"go.mod"}) || len(added) != 0 {
		t.Fatalf("v2 to v3 delta changed=%v added=%v, want only go.mod changed", changed, added)
	}

	goMod := string(v3Content["go.mod"])
	if !strings.Contains(goMod, "go 1.26.5") || strings.Contains(goMod, "go 1.26.0") {
		t.Fatalf("recipe v3 does not carry the Go 1.26.5 security floor:\n%s", goMod)
	}
	v2GoMod := strings.Replace(goMod, "go 1.26.5", "go 1.26.0", 1)
	if got, want := contentHash([]byte(v2GoMod)), v2Files["go.mod"]; got != want {
		t.Fatalf("published v2 go.mod fixture hash = %s, reconstructed baseline = %s", want, got)
	}

}

func TestGoAgentToolV3LockUpgradesSafelyToV4(t *testing.T) {
	t.Parallel()
	m := manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	root, _, v4 := installGoAgentV3Workspace(t, m)
	humanPath := filepath.Join(root, "internal", "cli", "hello.go")
	humanContent := []byte("package cli\n")
	if err := os.WriteFile(humanPath, humanContent, 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := engine.Plan(root, m, v4)
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasConflicts() || !plan.LockChanged || plan.Recipe.Version != 4 {
		t.Fatalf("v3 to v4 plan = %#v", plan)
	}
	actions := make(map[string]engine.Action, len(plan.Actions))
	for _, action := range plan.Actions {
		actions[action.Path] = action
	}
	if actions["internal/cli/root.go"].Kind != engine.ActionUpdate || actions["internal/cli/registry.go"].Kind != engine.ActionCreate || actions["internal/cli/registry_test.go"].Kind != engine.ActionCreate {
		t.Fatalf("v4 composition actions = root:%#v registry:%#v registry_test:%#v", actions["internal/cli/root.go"], actions["internal/cli/registry.go"], actions["internal/cli/registry_test.go"])
	}
	if _, err := engine.Apply(root, m, v4); err != nil {
		t.Fatal(err)
	}
	lock, err := engine.LoadLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if lock.Recipe.Version != 4 {
		t.Fatalf("upgraded lock version = %d, want 4", lock.Recipe.Version)
	}
	preserved, err := os.ReadFile(humanPath)
	if err != nil || !reflect.DeepEqual(preserved, humanContent) {
		t.Fatalf("v4 upgrade changed unmanaged command file: %q, %v", preserved, err)
	}
	converged, err := engine.Plan(root, m, v4)
	if err != nil {
		t.Fatal(err)
	}
	if converged.HasConflicts() || converged.LockChanged {
		t.Fatalf("second v4 plan is not converged: %#v", converged)
	}
	for _, action := range converged.Actions {
		if action.Kind != engine.ActionUnchanged {
			t.Fatalf("second plan action = %#v", action)
		}
	}

	withExtension, err := engine.Plan(root, m, v4)
	if err != nil {
		t.Fatal(err)
	}
	if withExtension.HasConflicts() || withExtension.LockChanged {
		t.Fatalf("human extension dirtied Bob state: %#v", withExtension)
	}
	for _, action := range withExtension.Actions {
		if action.Path == "internal/cli/hello.go" {
			t.Fatalf("human extension was adopted into Bob ownership: %#v", action)
		}
	}
}

func TestGoAgentToolV3ModifiedRootBlocksV4WithoutWrites(t *testing.T) {
	t.Parallel()
	m := manifest.Default("acme-tool", "github.com/acme/acme-tool", "Build useful things.")
	root, _, v4 := installGoAgentV3Workspace(t, m)
	rootPath := filepath.Join(root, "internal", "cli", "root.go")
	before, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	human := append(append([]byte(nil), before...), []byte("\n// human command registration\n")...)
	if err := os.WriteFile(rootPath, human, 0o644); err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(filepath.Join(root, engine.LockFilename))
	if err != nil {
		t.Fatal(err)
	}

	plan, err := engine.Plan(root, m, v4)
	if err != nil {
		t.Fatal(err)
	}
	var rootAction engine.Action
	for _, action := range plan.Actions {
		if action.Path == "internal/cli/root.go" {
			rootAction = action
			break
		}
	}
	if rootAction.Kind != engine.ActionConflict || rootAction.Code != engine.CodeManagedHashMismatch {
		t.Fatalf("modified v3 root action = %#v", rootAction)
	}
	if _, err := engine.Apply(root, m, v4); !errors.Is(err, engine.ErrPlanConflicts) {
		t.Fatalf("conflicted upgrade error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "internal", "cli", "registry.go")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("conflicted upgrade created registry: %v", err)
	}
	rootAfter, err := os.ReadFile(rootPath)
	if err != nil || !reflect.DeepEqual(rootAfter, human) {
		t.Fatalf("modified root changed: %v", err)
	}
	lockAfter, err := os.ReadFile(filepath.Join(root, engine.LockFilename))
	if err != nil || !reflect.DeepEqual(lockAfter, lockBefore) {
		t.Fatalf("conflicted upgrade changed lock: %v", err)
	}
}

func installGoAgentV3Workspace(t *testing.T, m manifest.Manifest) (string, []recipe.Artifact, []recipe.Artifact) {
	t.Helper()
	root := t.TempDir()
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	v3, err := recipe.RenderVersion(m, 3)
	if err != nil {
		t.Fatal(err)
	}
	v4, err := recipe.Render(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(root, m, v3); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, engine.LockFilename)
	lock, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(lock), "  version: 4\n", "  version: 3\n", 1)
	if updated == string(lock) {
		t.Fatal("temporary v3 workspace lock did not contain current recipe version")
	}
	if err := os.WriteFile(lockPath, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, v3, v4
}

func loadPublishedLock(t *testing.T, version int) (publishedLock, []byte) {
	t.Helper()
	path := filepath.Join("testdata", "go-agent-tool-v"+strconv.Itoa(version)+".lock")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lock publishedLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		t.Fatal(err)
	}
	if lock.SchemaVersion != 1 || lock.Recipe.ID != "go-agent-tool" || lock.Recipe.Version != version || len(lock.Files) == 0 {
		t.Fatalf("invalid published v%d lock fixture: %#v", version, lock)
	}
	return lock, data
}

func publishedFiles(lock publishedLock) map[string]string {
	files := make(map[string]string, len(lock.Files))
	for _, entry := range lock.Files {
		files[entry.Path] = entry.SHA256
	}
	return files
}

func contentHash(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
