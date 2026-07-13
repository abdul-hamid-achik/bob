package studio

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type offlineRunner struct {
	runs int
}

func (r *offlineRunner) LookPath(name string) (string, error) {
	if name == "codemap" {
		return "/bin/codemap", nil
	}
	return "", fmt.Errorf("missing %s", name)
}

func (r *offlineRunner) Run(context.Context, string, string, ...string) inspectpkg.CommandResult {
	r.runs++
	panic("Studio must not run specialist commands")
}

func TestRepositorySourceLoadsPlanWithoutSpecialistProbe(t *testing.T) {
	root := t.TempDir()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme CLI")
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	before := workspaceFingerprint(t, root)
	runner := &offlineRunner{}
	snapshot, err := NewRepositorySource(runner).Load(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if runner.runs != 0 {
		t.Fatalf("specialist Run calls = %d, want 0", runner.runs)
	}
	if snapshot.Plan == nil || len(snapshot.Plan.Actions) == 0 {
		t.Fatalf("snapshot plan is not useful: %#v", snapshot.Plan)
	}
	if snapshot.Stats.Total != 0 || snapshot.Stats.Success != 0 || snapshot.Stats.Errors != 0 || snapshot.Stats.Conflicts != 0 || len(snapshot.Stats.PerOperation) != 0 {
		t.Fatalf("repository source invented telemetry stats: %#v", snapshot.Stats)
	}
	if _, err := os.Stat(filepath.Join(root, "bob.lock")); !os.IsNotExist(err) {
		t.Fatalf("read-only source wrote bob.lock: %v", err)
	}
	if after := workspaceFingerprint(t, root); after != before {
		t.Fatalf("read-only source changed workspace\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestRepositorySourceReturnsMissingManifestSnapshot(t *testing.T) {
	runner := &offlineRunner{}
	snapshot, err := NewRepositorySource(runner).Load(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Plan != nil || snapshot.Report.Repository.State != "missing_manifest" || snapshot.Stats.Errors != 0 {
		t.Fatalf("unexpected missing-manifest snapshot: %#v", snapshot)
	}
	if runner.runs != 0 {
		t.Fatalf("specialist Run calls = %d, want 0", runner.runs)
	}
}

func workspaceFingerprint(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		line := fmt.Sprintf("%s|%s", filepath.ToSlash(rel), info.Mode())
		if entry.Type().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			line += fmt.Sprintf("|%x", sha256.Sum256(data))
		}
		entries = append(entries, line)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(entries)
	return strings.Join(entries, "\n")
}
