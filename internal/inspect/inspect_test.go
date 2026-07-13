package inspect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type fakeRunner struct {
	missing map[string]bool
	results map[string]CommandResult
	calls   []runnerCall
}

type runnerCall struct {
	dir  string
	name string
	args []string
}

func (f *fakeRunner) LookPath(name string) (string, error) {
	if f.missing[name] {
		return "", errors.New("missing")
	}
	return "/bin/" + name, nil
}

func (f *fakeRunner) Run(_ context.Context, dir, name string, args ...string) CommandResult {
	f.calls = append(f.calls, runnerCall{dir: dir, name: name, args: append([]string(nil), args...)})
	return f.results[name]
}

func TestRunIsOfflineByDefault(t *testing.T) {
	t.Parallel()
	root := testWorkspace(t)
	runner := &fakeRunner{}
	report, err := Run(context.Background(), root, Options{}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("default inspection ran specialist commands: %#v", runner.calls)
	}
	if report.Repository.State != "drifted" || !report.Repository.Ready {
		t.Fatalf("unexpected repository state: %#v", report.Repository)
	}
	for _, integration := range report.Integrations {
		if !integration.Selected || integration.Probe.State != ProbeNotRequested {
			t.Fatalf("unexpected offline integration: %#v", integration)
		}
	}
	if !report.Degraded {
		t.Fatal("unprobed selected integrations should be reported as degraded")
	}
}

func TestRunNormalizesExplicitCodemapAndVecgrepProbes(t *testing.T) {
	t.Parallel()
	root := testWorkspace(t)
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{results: map[string]CommandResult{
		"codemap": {Stdout: []byte(`{
  "root": "` + canonical + `", "registered": true, "files": 12,
  "nodes": 30, "edges": 44, "vectors": 30, "precise_edges": 18,
  "project_key": "abc", "stale": {"changed": 0, "new": 0, "deleted": 0}
}`)},
		"vecgrep": {Stdout: []byte(`{
  "project_root": "` + canonical + `", "provider": "ollama",
  "embedding_model": "nomic-embed-text", "dimensions": 768,
  "profile_status": "ok", "profile_matches": true, "index_fresh": false,
  "stats": {"files": 9, "chunks": 20, "embeddings": 20},
  "pending_changes": {"new_files": 1, "modified_files": 2, "deleted_files": 0, "total_pending": 3}
}`)},
	}}
	report, err := Run(context.Background(), root, Options{ProbeIntegrations: true, ProbeTimeout: time.Second}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %#v", runner.calls)
	}
	if got, want := runner.calls[0].args, []string{"--json", "--path", canonical, "status"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("codemap args = %#v, want %#v", got, want)
	}
	if got, want := runner.calls[1].args, []string{"status", "--format", "json"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("vecgrep args = %#v, want %#v", got, want)
	}
	if report.Integrations[0].Index.State != IndexFresh || report.Integrations[1].Index.State != IndexStale {
		t.Fatalf("unexpected normalized indexes: %#v", report.Integrations)
	}
	if report.Integrations[1].Profile.ProviderHealth != "not_reported" {
		t.Fatalf("provider health was inferred: %#v", report.Integrations[1].Profile)
	}
	if !report.Degraded {
		t.Fatal("stale Vecgrep index should degrade inspection")
	}
}

func TestRunPreservesPeerStatusWhenProbeFails(t *testing.T) {
	t.Parallel()
	root := testWorkspace(t)
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{results: map[string]CommandResult{
		"codemap": {Stdout: []byte(`{"root":"` + canonical + `","registered":false}`)},
		"vecgrep": {Err: errors.New("exit 1"), Stderr: []byte("database unavailable\nusage text")},
	}}
	report, err := Run(context.Background(), root, Options{ProbeIntegrations: true}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if report.Integrations[0].Probe.State != ProbeComplete || report.Integrations[0].Index.State != IndexNotIndexed {
		t.Fatalf("Codemap result was lost: %#v", report.Integrations[0])
	}
	if report.Integrations[1].Probe.State != ProbeFailed || report.Integrations[1].Probe.Detail != "database unavailable" {
		t.Fatalf("unexpected Vecgrep failure: %#v", report.Integrations[1])
	}
}

func TestRunRejectsWrongProjectAndMultipleJSONDocuments(t *testing.T) {
	t.Parallel()
	root := testWorkspace(t)
	runner := &fakeRunner{results: map[string]CommandResult{
		"codemap": {Stdout: []byte(`{"root":"` + t.TempDir() + `","registered":true,"files":1,"nodes":1}`)},
		"vecgrep": {Stdout: []byte(`{"project_root":"` + root + `"} {}`)},
	}}
	report, err := Run(context.Background(), root, Options{ProbeIntegrations: true}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if report.Integrations[0].Probe.State != ProbeWrongProject {
		t.Fatalf("wrong Codemap project accepted: %#v", report.Integrations[0])
	}
	if report.Integrations[1].Probe.State != ProbeInvalid {
		t.Fatalf("multiple Vecgrep documents accepted: %#v", report.Integrations[1])
	}
}

func TestRunDoesNotProbeUnselectedIntegrations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	m.Integrations.CodeStructure = "none"
	m.Integrations.SemanticSearch = "none"
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	runner := &fakeRunner{}
	report, err := Run(context.Background(), root, Options{ProbeIntegrations: true}, runner)
	if err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("unselected integrations were probed: %#v", runner.calls)
	}
	if report.Integrations[0].Probe.State != ProbeNotSelected || report.Integrations[1].Probe.State != ProbeNotSelected {
		t.Fatalf("unexpected unselected states: %#v", report.Integrations)
	}
}

func testWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRunMissingWorkspaceIsAnError(t *testing.T) {
	t.Parallel()
	_, err := Run(context.Background(), filepath.Join(t.TempDir(), "missing"), Options{}, &fakeRunner{})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("error = %v", err)
	}
}
