package recipe_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"go.yaml.in/yaml/v3"
)

type publishedLock struct {
	Recipe struct {
		Version int `yaml:"version"`
	} `yaml:"recipe"`
	Files []struct {
		Path   string `yaml:"path"`
		SHA256 string `yaml:"sha256"`
	} `yaml:"files"`
}

func TestRecipeV2DeltaPreservesEveryPublishedV1ManagedPath(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("testdata", "go-agent-tool-v1.lock"))
	if err != nil {
		t.Fatal(err)
	}
	var v1 publishedLock
	if err := yaml.Unmarshal(data, &v1); err != nil {
		t.Fatal(err)
	}
	if v1.Recipe.Version != 1 || len(v1.Files) == 0 {
		t.Fatalf("invalid published v1 lock fixture: %#v", v1)
	}

	m, err := manifest.LoadFile(filepath.Join("..", "..", "examples", "integrated", manifest.Filename))
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := recipe.Render(m)
	if err != nil {
		t.Fatal(err)
	}
	v2 := make(map[string]string, len(artifacts))
	for _, artifact := range artifacts {
		sum := sha256.Sum256(artifact.Content)
		v2[artifact.Path] = hex.EncodeToString(sum[:])
	}

	changed := 0
	v1Paths := make(map[string]struct{}, len(v1.Files))
	for _, entry := range v1.Files {
		v1Paths[entry.Path] = struct{}{}
		current, ok := v2[entry.Path]
		if !ok {
			t.Errorf("recipe v2 dropped published v1 path %s", entry.Path)
			continue
		}
		if current != entry.SHA256 {
			changed++
		}
	}
	added := 0
	for path := range v2 {
		if _, ok := v1Paths[path]; !ok {
			added++
		}
	}
	if changed == 0 || added == 0 {
		t.Fatalf("v1 to v2 delta has changed=%d added=%d, want both positive", changed, added)
	}
	for _, path := range []string{"CODE_OF_CONDUCT.md", ".github/dependabot.yml", ".github/pull_request_template.md"} {
		if _, ok := v2[path]; !ok {
			t.Errorf("recipe v2 missing expected new path %s", path)
		}
	}
}
