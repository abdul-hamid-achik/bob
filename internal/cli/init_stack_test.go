package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeStackFixture materializes marker files for detection tests inside a
// directory whose basename is a valid product name.
func writeStackFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		full := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func tsFixture(t *testing.T) string {
	t.Helper()
	return writeStackFixture(t, map[string]string{
		"package.json":  `{"workspaces":["apps/*"]}`,
		"tsconfig.json": "{}",
		"bun.lock":      "",
		"turbo.json":    "{}",
	})
}

func TestInitAutoSelectsStackRecipeWithoutModule(t *testing.T) {
	t.Parallel()
	root := tsFixture(t)
	stdout, stderr, err := executeForTest("init", root)
	if err != nil {
		t.Fatalf("init preview: %v\n%s", err, stderr)
	}
	if !strings.Contains(stdout, "recipe: ts-app") {
		t.Fatalf("expected ts-app selection, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "kind: monorepo") {
		t.Fatalf("expected detected monorepo kind, got:\n%s", stdout)
	}
	if strings.Contains(stderr, "warning:") {
		t.Fatalf("matching stack must not warn: %s", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "bob.yaml")); !os.IsNotExist(err) {
		t.Fatalf("preview must not write bob.yaml: %v", err)
	}
}

func TestInitJSONPreviewIncludesDetection(t *testing.T) {
	t.Parallel()
	root := writeStackFixture(t, map[string]string{
		"package.json": `{"dependencies":{"vue":"^3"}}`,
	})
	stdout, _, err := executeForTest("init", root, "--json")
	if err != nil {
		t.Fatalf("init --json: %v\n%s", err, stdout)
	}
	var envelope struct {
		OK   bool `json:"ok"`
		Data struct {
			Manifest struct {
				Recipe string `json:"recipe"`
			} `json:"manifest"`
			Detection struct {
				Primary string `json:"primary"`
			} `json:"detection"`
			Written bool `json:"written"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, stdout)
	}
	if !envelope.OK || envelope.Data.Written {
		t.Fatalf("unexpected envelope: %s", stdout)
	}
	if envelope.Data.Manifest.Recipe != "vue-app" || envelope.Data.Detection.Primary != "vue" {
		t.Fatalf("expected vue-app for a vue repository, got %s", stdout)
	}
}

func TestInitRefusesMismatchedRecipeWriteWithoutForce(t *testing.T) {
	t.Parallel()
	root := tsFixture(t)
	_, _, err := executeForTest("init", root, "--recipe", "go-agent-tool", "--module", "github.com/acme/demo", "--write")
	if err == nil {
		t.Fatal("expected mismatch refusal")
	}
	message := err.Error()
	if !strings.Contains(message, "looks like typescript") || !strings.Contains(message, "recipe go-agent-tool targets go") {
		t.Fatalf("refusal must name the detected stack and the recipe language: %s", message)
	}
	if !strings.Contains(message, "--force") || !strings.Contains(message, "--recipe ts-app") {
		t.Fatalf("refusal must offer --force and the matching recipe: %s", message)
	}
	if _, statErr := os.Stat(filepath.Join(root, "bob.yaml")); !os.IsNotExist(statErr) {
		t.Fatalf("refused write must not create bob.yaml: %v", statErr)
	}
}

func TestInitPreviewWarnsOnMismatchWithoutFailing(t *testing.T) {
	t.Parallel()
	root := tsFixture(t)
	stdout, stderr, err := executeForTest("init", root, "--recipe", "go-agent-tool", "--module", "github.com/acme/demo")
	if err != nil {
		t.Fatalf("mismatched preview must still render: %v", err)
	}
	if !strings.Contains(stderr, "warning:") || !strings.Contains(stderr, "looks like typescript") {
		t.Fatalf("expected prominent mismatch warning on stderr: %s", stderr)
	}
	if !strings.Contains(stdout, "recipe: go-agent-tool") {
		t.Fatalf("preview must show the explicitly chosen recipe:\n%s", stdout)
	}
}

func TestInitForceWritesMismatchedRecipe(t *testing.T) {
	t.Parallel()
	root := tsFixture(t)
	_, _, err := executeForTest("init", root, "--recipe", "go-agent-tool", "--module", "github.com/acme/demo", "--write", "--force")
	if err != nil {
		t.Fatalf("--force must override the guard: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "bob.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "recipe: go-agent-tool") {
		t.Fatalf("unexpected manifest: %s", data)
	}
}

func TestInitStackWriteLifecycleConverges(t *testing.T) {
	t.Parallel()
	root := tsFixture(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("existing readme\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeForTest("init", root, "--write"); err != nil {
		t.Fatalf("init --write: %v", err)
	}
	if _, _, err := executeForTest("apply", root); err != nil {
		t.Fatalf("apply: %v", err)
	}
	stdout, _, err := executeForTest("check", root)
	if err != nil || !strings.Contains(stdout, "clean:") {
		t.Fatalf("check after apply: err=%v output=%s", err, stdout)
	}
	content, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil || string(content) != "existing readme\n" {
		t.Fatalf("existing README must never be touched: %q err=%v", content, err)
	}
	for _, seeded := range []string{"AGENTS.md", "SECURITY.md", ".gitignore", ".github/workflows/ci.yml"} {
		if _, err := os.Stat(filepath.Join(root, seeded)); err != nil {
			t.Fatalf("expected seeded %s: %v", seeded, err)
		}
	}
}

func TestInitDefaultsToGoAgentToolForUnknownRepositories(t *testing.T) {
	t.Parallel()
	root := writeStackFixture(t, map[string]string{"notes.txt": "no stack markers"})
	_, _, err := executeForTest("init", root)
	if err == nil || !strings.Contains(err.Error(), "--module is required for recipe go-agent-tool") {
		t.Fatalf("expected module requirement for the historical default: err=%v", err)
	}
	stdout, _, err := executeForTest("init", root, "--module", "github.com/acme/demo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "recipe: go-agent-tool") {
		t.Fatalf("unknown repository must keep the historical default:\n%s", stdout)
	}
}

func TestInitRejectsFilesRecipeAndUnknownRecipe(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	_, _, err := executeForTest("init", root, "--recipe", "files")
	if err == nil || !strings.Contains(err.Error(), "write bob.yaml by hand") {
		t.Fatalf("files recipe must be rejected with guidance: err=%v", err)
	}
	_, _, err = executeForTest("init", root, "--recipe", "ts-apps")
	if err == nil || !strings.Contains(err.Error(), "did you mean") {
		t.Fatalf("unknown recipe must suggest a close id: err=%v", err)
	}
}

func TestInitStackRecipeAcceptsOptionalModule(t *testing.T) {
	t.Parallel()
	root := writeStackFixture(t, map[string]string{"pyproject.toml": "[project]\nname = \"demo\"\n"})
	stdout, _, err := executeForTest("init", root, "--module", "github.com/acme/demo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "recipe: python-app") || !strings.Contains(stdout, "module: github.com/acme/demo") {
		t.Fatalf("unexpected preview:\n%s", stdout)
	}
}
