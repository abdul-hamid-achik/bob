package pathinfo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/guidance"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

func TestLoadClassifiesManagedExtensionReservedAndUnmanagedPaths(t *testing.T) {
	root := pathWorkspace(t)
	tests := []struct {
		path, classification, state, effect string
	}{
		{"internal/cli/root.go", "managed", "managed_in_sync", "will_conflict"},
		{"internal/cli/registry.go", "managed", "managed_in_sync", "will_conflict"},
		{"internal/cli/hello.go", "extension_point", "extension_point", "outside_bob_ownership"},
		{"internal/domain/service.go", "extension_point", "extension_point", "outside_bob_ownership"},
		{"notes.txt", "missing", "unmanaged_missing", "outside_bob_ownership"},
		{"bob.yaml", "reserved", "reserved", "requires_manifest_change"},
		{"bob.lock", "reserved", "reserved", "reserved_for_bob"},
	}
	for _, tc := range tests {
		result, err := Load(root, tc.path)
		if err != nil {
			t.Fatal(err)
		}
		if result.Classification != tc.classification || result.State != tc.state || result.HumanEditEffect != tc.effect {
			t.Fatalf("%s = %#v", tc.path, result)
		}
	}
	rootResult, err := Load(root, "internal/cli/root.go")
	if err != nil {
		t.Fatal(err)
	}
	if rootResult.Artifact == nil || rootResult.Artifact.ID != "cli.root" || !reflect.DeepEqual(rootResult.RelatedPlaybooks, []string{"add-cli-command"}) {
		t.Fatalf("root result = %#v", rootResult)
	}
	extensionResult, err := Load(root, "internal/cli/hello.go")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(extensionResult.ExtensionPoints, []string{"cli.command_files"}) || !reflect.DeepEqual(extensionResult.RelatedPlaybooks, []string{"add-cli-command"}) {
		t.Fatalf("command extension result = %#v", extensionResult)
	}
	releaseResult, err := Load(root, ".goreleaser.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if releaseResult.Artifact == nil || !reflect.DeepEqual(releaseResult.RelatedPlaybooks, []string{"enable-goreleaser", "enable-homebrew"}) {
		t.Fatalf("metadata-derived release playbooks = %#v", releaseResult)
	}
	data, err := json.Marshal(rootResult)
	if err != nil || len(data) > 8<<10 {
		t.Fatalf("path output bytes=%d err=%v", len(data), err)
	}
}

func TestMetadataPlaybooksForPathDoesNotDependOnArtifactIDs(t *testing.T) {
	metadata := recipe.Metadata{
		ExtensionPoints: []recipe.ExtensionPointDefinition{{ForbiddenPaths: []string{"owned/root.go"}, PlaybookIDs: []string{"extend-root"}}},
		Playbooks: []recipe.PlaybookDefinition{
			{ID: "reconcile-owned", Boundary: recipe.PlaybookBoundary{Modify: []string{"owned/root.go"}}},
		},
	}
	if got := metadataPlaybooksForPath(metadata, "owned/root.go"); !reflect.DeepEqual(got, []string{"extend-root", "reconcile-owned"}) {
		t.Fatalf("related playbooks = %#v", got)
	}
}

func TestPathTruncationKeepsClosedListsAsArrays(t *testing.T) {
	roles := make([]string, 200)
	capabilities := make([]string, 200)
	for i := range roles {
		roles[i] = strings.Repeat("role", 32)
		capabilities[i] = strings.Repeat("capability", 16)
	}
	result := Result{
		Artifact:   &Artifact{ID: "large", Roles: roles, CapabilityIDs: capabilities},
		Notices:    []guidance.Notice{},
		Actions:    []guidance.Action{},
		Truncation: guidance.Truncation{Profile: "path", ByteLimit: 8 << 10, Omitted: map[string]int{}},
	}
	truncate(&result)
	if result.Artifact.Roles == nil || result.Artifact.CapabilityIDs == nil {
		t.Fatalf("truncation produced nullable lists: %#v", result.Artifact)
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"roles":null`) || strings.Contains(string(data), `"capability_ids":null`) {
		t.Fatalf("truncated path contract contains null lists: %s", data)
	}
}

func TestLoadIsReadOnlyIncludingMtimes(t *testing.T) {
	root := pathWorkspace(t)
	before := pathSnapshot(t, root)
	if _, err := Load(root, "internal/domain/service.go"); err != nil {
		t.Fatal(err)
	}
	after := pathSnapshot(t, root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("path changed workspace:\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestClassificationSemanticsAreStableAcrossWorkspaceRelocation(t *testing.T) {
	left := pathWorkspace(t)
	right := pathWorkspace(t)
	a, err := Load(left, "internal/cli/root.go")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(right, "internal/cli/root.go")
	if err != nil {
		t.Fatal(err)
	}
	leftCanonical, rightCanonical := a.Workspace, b.Workspace
	a.Workspace, b.Workspace = "", ""
	for i := range a.Actions {
		a.Actions[i].CWD = ""
		for j := range a.Actions[i].Argv {
			if a.Actions[i].Argv[j] == leftCanonical {
				a.Actions[i].Argv[j] = "<workspace>"
			}
		}
	}
	for i := range b.Actions {
		b.Actions[i].CWD = ""
		for j := range b.Actions[i].Argv {
			if b.Actions[i].Argv[j] == rightCanonical {
				b.Actions[i].Argv[j] = "<workspace>"
			}
		}
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("relocation changed semantic classification:\nleft=%#v\nright=%#v", a, b)
	}
}

func pathWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	artifacts, err := recipe.Render(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(root, m, artifacts); err != nil {
		t.Fatal(err)
	}
	return root
}

type pathSnapshotEntry struct {
	data  string
	mode  os.FileMode
	mtime time.Time
}

func pathSnapshot(t *testing.T, root string) map[string]pathSnapshotEntry {
	t.Helper()
	result := map[string]pathSnapshotEntry{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
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
		result[filepath.ToSlash(rel)] = pathSnapshotEntry{string(data), info.Mode(), info.ModTime()}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
