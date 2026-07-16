package cli

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// TestNewRecipeSelection covers the multi-recipe surface of bob new: explicit
// --recipe selection, stack auto-detection for targets that already have
// content, the greenfield go-agent-tool default, and the module-flag
// contract (required by go-agent-tool, rejected by every other recipe).
func TestNewRecipeSelection(t *testing.T) {
	t.Parallel()
	tsSeed := map[string]string{
		"package.json":  `{"workspaces":["apps/*"]}`,
		"tsconfig.json": "{}",
		"bun.lock":      "",
		"turbo.json":    "{}",
	}
	cases := []struct {
		name string
		// seed files materialized under the target before running; nil keeps
		// the target missing (greenfield).
		seed map[string]string
		// args appended after: new demo --dir <target>
		args []string
		// wantErr non-empty means the command must fail with exit code 4
		// (input_invalid) and an error containing this text.
		wantErr string
		// wantStdout substrings each preview must print.
		wantStdout []string
		// wantStderr substrings a successful run must print to stderr.
		wantStderr []string
		// wantManifest substrings the written bob.yaml must contain.
		wantManifest []string
		// wantFiles paths that must exist under the target after --write.
		wantFiles []string
	}{
		{
			name:         "explicit ts-app scaffolds seed hygiene into a fresh target",
			args:         []string{"--recipe", "ts-app", "--write"},
			wantManifest: []string{"recipe: ts-app"},
			wantFiles:    []string{"bob.yaml", "AGENTS.md", "SECURITY.md", ".gitignore", ".github/workflows/ci.yml"},
		},
		{
			name:       "auto-detection previews ts-app for a seeded Bun+tsconfig target",
			seed:       tsSeed,
			args:       nil,
			wantStdout: []string{"recipe: ts-app", "kind: monorepo"},
		},
		{
			name:         "auto-detection writes ts-app seeds into the non-empty target",
			seed:         tsSeed,
			args:         []string{"--write"},
			wantManifest: []string{"recipe: ts-app", "kind: monorepo"},
			wantFiles:    []string{"bob.yaml", "AGENTS.md", ".gitignore", ".github/workflows/ci.yml"},
		},
		{
			name:       "greenfield target with no --recipe keeps the go-agent-tool default",
			args:       []string{"--module", "github.com/acme/demo"},
			wantStdout: []string{"recipe: go-agent-tool"},
		},
		{
			name:    "defaulted go-agent-tool still requires --module",
			args:    nil,
			wantErr: "--module is required for recipe go-agent-tool",
		},
		{
			name:    "explicit go-agent-tool still requires --module",
			args:    []string{"--recipe", "go-agent-tool"},
			wantErr: "--module is required for recipe go-agent-tool",
		},
		{
			name:    "--module is rejected for a non-Go recipe",
			args:    []string{"--recipe", "ts-app", "--module", "github.com/acme/demo"},
			wantErr: "--module is only used by recipe go-agent-tool",
		},
		{
			name:    "mismatched explicit stack recipe write is blocked",
			seed:    map[string]string{"pyproject.toml": "[project]\nname = \"demo\"\n"},
			args:    []string{"--recipe", "ts-app", "--write"},
			wantErr: "looks like python",
		},
		{
			name:       "mismatched explicit stack recipe preview warns",
			seed:       map[string]string{"pyproject.toml": "[project]\nname = \"demo\"\n"},
			args:       []string{"--recipe", "ts-app"},
			wantStdout: []string{"recipe: ts-app"},
			wantStderr: []string{"looks like python", "--recipe python-app"},
		},
		{
			name: "write into an already bob-managed target is refused",
			seed: map[string]string{
				"package.json":  `{"workspaces":["apps/*"]}`,
				"tsconfig.json": "{}",
				"bun.lock":      "",
				"bob.yaml":      "schema_version: 1\nrecipe: ts-app\nproduct:\n    name: demo\n",
			},
			args:    []string{"--write"},
			wantErr: "already has bob.yaml",
		},
		{
			name:    "unknown recipe suggests a close id",
			args:    []string{"--recipe", "ts-apps"},
			wantErr: "did you mean",
		},
		{
			name:         "explicit files recipe scaffolds a starter manifest and README",
			args:         []string{"--recipe", "files", "--write"},
			wantManifest: []string{"recipe: files", "README.md"},
			wantFiles:    []string{"bob.yaml", "README.md"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			target := filepath.Join(t.TempDir(), "demo")
			for path, content := range tc.seed {
				full := filepath.Join(target, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			args := append([]string{"new", "demo", "--dir", target}, tc.args...)
			stdout, stderr, err := executeForTest(args...)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected failure containing %q, got success:\n%s", tc.wantErr, stdout)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tc.wantErr)
				}
				if ExitCode(err) != ExitInvalidInput {
					t.Fatalf("expected exit code %d, got %d for %v", ExitInvalidInput, ExitCode(err), err)
				}
				if _, seeded := tc.seed["bob.yaml"]; !seeded {
					if _, statErr := os.Stat(filepath.Join(target, "bob.yaml")); !os.IsNotExist(statErr) {
						t.Fatalf("failed new must not write bob.yaml: %v", statErr)
					}
				}
				for path, content := range tc.seed {
					current, readErr := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
					if readErr != nil || string(current) != content {
						t.Fatalf("seeded %s must never be touched by a failed new: %q err=%v", path, current, readErr)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("new: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
			}
			for _, want := range tc.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Fatalf("stdout missing %q:\n%s", want, stdout)
				}
			}
			for _, want := range tc.wantStderr {
				if !strings.Contains(stderr, want) {
					t.Fatalf("stderr missing %q:\n%s", want, stderr)
				}
			}
			if !slices.Contains(tc.args, "--write") {
				if _, statErr := os.Stat(filepath.Join(target, "bob.yaml")); !os.IsNotExist(statErr) {
					t.Fatalf("preview must not write bob.yaml: %v", statErr)
				}
				return
			}
			for _, path := range tc.wantFiles {
				if _, statErr := os.Stat(filepath.Join(target, filepath.FromSlash(path))); statErr != nil {
					t.Fatalf("expected %s: %v", path, statErr)
				}
			}
			data, readErr := os.ReadFile(filepath.Join(target, "bob.yaml"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			for _, want := range tc.wantManifest {
				if !strings.Contains(string(data), want) {
					t.Fatalf("bob.yaml missing %q:\n%s", want, data)
				}
			}
			for path, content := range tc.seed {
				current, readErr := os.ReadFile(filepath.Join(target, filepath.FromSlash(path)))
				if readErr != nil || string(current) != content {
					t.Fatalf("seeded %s must never be touched: %q err=%v", path, current, readErr)
				}
			}
		})
	}
}

// TestNewAutoDetectedScaffoldConverges proves the auto-detected stack
// scaffold immediately satisfies the drift gate, so bob new on an existing
// repository hands off cleanly to plan/apply/check.
func TestNewAutoDetectedScaffoldConverges(t *testing.T) {
	t.Parallel()
	target := filepath.Join(t.TempDir(), "demo")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	for path, content := range map[string]string{
		"package.json":  `{"workspaces":["apps/*"]}`,
		"tsconfig.json": "{}",
		"bun.lock":      "",
	} {
		if err := os.WriteFile(filepath.Join(target, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := executeForTest("new", "demo", "--dir", target, "--write"); err != nil {
		t.Fatalf("new --write: %v", err)
	}
	stdout, _, err := executeForTest("check", target)
	if err != nil || !strings.Contains(stdout, "clean:") {
		t.Fatalf("check after new: err=%v output=%s", err, stdout)
	}
}
