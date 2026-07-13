package recipe_test

import (
	"path/filepath"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

func TestDocumentedManifestExamplesProduceConflictFreePlans(t *testing.T) {
	examples, err := filepath.Glob(filepath.Join("..", "..", "examples", "*", manifest.Filename))
	if err != nil {
		t.Fatal(err)
	}
	if len(examples) == 0 {
		t.Fatal("no documented manifest examples found")
	}
	for _, path := range examples {
		path := path
		t.Run(filepath.Base(filepath.Dir(path)), func(t *testing.T) {
			t.Parallel()
			m, err := manifest.LoadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			artifacts, err := recipe.Render(m)
			if err != nil {
				t.Fatal(err)
			}
			root := t.TempDir()
			plan, err := engine.Plan(root, m, artifacts)
			if err != nil {
				t.Fatal(err)
			}
			if plan.HasConflicts() || len(plan.Actions) == 0 {
				t.Fatalf("example plan is not a useful fresh plan: %#v", plan)
			}
			for _, action := range plan.Actions {
				if action.Kind != engine.ActionCreate {
					t.Fatalf("fresh example action = %#v, want create", action)
				}
			}
		})
	}
}
