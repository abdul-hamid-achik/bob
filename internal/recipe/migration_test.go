package recipe_test

import (
	"crypto/sha256"
	"encoding/hex"
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
	if engine.RecipeVersion != 3 {
		t.Fatalf("recipe version = %d, want 3", engine.RecipeVersion)
	}
	v2, v2LockData := loadPublishedLock(t, 2)
	v2Files := publishedFiles(v2)

	m, err := manifest.LoadFile(filepath.Join("..", "..", "examples", "integrated", manifest.Filename))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := recipe.Render(m)
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

	root := t.TempDir()
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		content := artifact.Content
		if artifact.Path == "go.mod" {
			content = []byte(v2GoMod)
		}
		if got, want := contentHash(content), v2Files[artifact.Path]; got != want {
			t.Fatalf("reconstructed v2 %s hash = %s, fixture = %s", artifact.Path, got, want)
		}
		path := filepath.Join(root, filepath.FromSlash(artifact.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, content, artifact.Mode); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, engine.LockFilename), v2LockData, 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := engine.Plan(root, m, artifacts)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.LockChanged || plan.Recipe.Version != 3 || len(plan.Actions) != len(v2.Files) {
		t.Fatalf("unexpected v2 to v3 plan: %#v", plan)
	}
	for _, action := range plan.Actions {
		want := engine.ActionUnchanged
		if action.Path == "go.mod" {
			want = engine.ActionUpdate
			if action.CurrentSHA256 != v2Files[action.Path] || action.DesiredSHA256 != v3Files[action.Path] {
				t.Fatalf("go.mod security update hashes do not match fixture/current recipe: %#v", action)
			}
		}
		if action.Kind != want {
			t.Errorf("v2 to v3 action for %s = %s, want %s", action.Path, action.Kind, want)
		}
	}
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
