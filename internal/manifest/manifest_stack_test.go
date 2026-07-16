package manifest

import (
	"strings"
	"testing"
)

func TestDefaultStackValidatesForEveryStackRecipe(t *testing.T) {
	t.Parallel()
	for _, id := range StackRecipeIDs() {
		m, err := DefaultStack(id, "demo", "", "", "")
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if err := m.Validate(); err != nil {
			t.Fatalf("%s: default manifest invalid: %v", id, err)
		}
		runtime, ok := StackRecipeRuntime(id)
		if !ok {
			t.Fatalf("%s: no runtime contract", id)
		}
		if m.Runtime.Language != runtime.Languages[0] || m.Runtime.Kind != runtime.Kinds[0] {
			t.Fatalf("%s: default runtime %#v does not use contract defaults %#v", id, m.Runtime, runtime)
		}
		if m.Product.Module != "" {
			t.Fatalf("%s: module must default to empty", id)
		}
	}
}

func TestDefaultStackAcceptsSupportedKindAndRejectsOthers(t *testing.T) {
	t.Parallel()
	m, err := DefaultStack(RecipeTSApp, "demo", "github.com/acme/demo", "A demo.", "monorepo")
	if err != nil {
		t.Fatal(err)
	}
	if m.Runtime.Kind != "monorepo" || m.Product.Module != "github.com/acme/demo" {
		t.Fatalf("unexpected manifest: %#v", m)
	}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultStack(RecipeTSApp, "demo", "", "", "gem"); err == nil {
		t.Fatal("ts-app must reject runtime.kind gem")
	}
	if _, err := DefaultStack("go-agent-tool", "demo", "", "", ""); err == nil {
		t.Fatal("DefaultStack must reject non-stack recipes")
	}
}

func TestValidateStackRecipeRejectsWrongRuntime(t *testing.T) {
	t.Parallel()
	m, err := DefaultStack(RecipeRustCLI, "demo", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	m.Runtime.Language = "go"
	err = m.Validate()
	if err == nil || !strings.Contains(err.Error(), "rust-cli requires runtime.language to be one of rust") {
		t.Fatalf("unexpected error: %v", err)
	}
	m.Runtime.Language = "rust"
	m.Runtime.Kind = "app"
	err = m.Validate()
	if err == nil || !strings.Contains(err.Error(), "rust-cli requires runtime.kind to be one of cli") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateStackRecipeModuleIsOptionalButShapeChecked(t *testing.T) {
	t.Parallel()
	m, err := DefaultStack(RecipePythonApp, "demo", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Validate(); err != nil {
		t.Fatalf("empty module must validate: %v", err)
	}
	m.Product.Module = "bad module"
	err = m.Validate()
	if err == nil || !strings.Contains(err.Error(), "product.module") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateStackRecipeRejectsUnsupportedSections(t *testing.T) {
	t.Parallel()
	base := func() Manifest {
		m, err := DefaultStack(RecipeVueApp, "demo", "", "", "")
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	tests := []struct {
		name   string
		mutate func(*Manifest)
		want   string
	}{
		{"surfaces", func(m *Manifest) { m.Surfaces.CLI = true }, "surfaces is not used by recipe vue-app"},
		{"integrations", func(m *Manifest) { m.Integrations.Secrets = "tinyvault" }, "integrations is not used by recipe vue-app"},
		{"goreleaser", func(m *Manifest) { m.Distribution.GoReleaser = true }, "distribution.goreleaser is not supported by recipe vue-app"},
		{"homebrew", func(m *Manifest) { m.Distribution.Homebrew = true }, "distribution.homebrew is not supported by recipe vue-app"},
		{"docs", func(m *Manifest) { m.Distribution.Docs = "markdown" }, "distribution.docs must be none for recipe vue-app"},
		{"vars", func(m *Manifest) { m.Vars = map[string]string{"x": "y"} }, "vars is only supported by recipe files"},
		{"files", func(m *Manifest) { m.Files = []FileDecl{{Path: "a", Content: "b"}} }, "files is only supported by recipe files"},
		{"visibility", func(m *Manifest) { m.Product.Visibility = "internal" }, "product.visibility must be public or private"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			m := base()
			test.mutate(&m)
			err := m.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestUnknownRecipeErrorListsStackRecipes(t *testing.T) {
	t.Parallel()
	m := Default("demo", "github.com/acme/demo", "")
	m.Recipe = "unknown"
	err := m.Validate()
	if err == nil {
		t.Fatal("expected validation failure")
	}
	for _, id := range []string{RecipeFiles, RecipeGoAgentTool, RecipeTSApp, RecipeStaticWeb} {
		if !strings.Contains(err.Error(), id) {
			t.Fatalf("error %q does not list recipe %s", err.Error(), id)
		}
	}
}
