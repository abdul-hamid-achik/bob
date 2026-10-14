package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// planActionCountsJSON mirrors the projected action_counts shape so tests can
// decode it independently of the internal struct.
type planActionCountsJSON struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Adopt     int `json:"adopt"`
	Unchanged int `json:"unchanged"`
	Retire    int `json:"retire"`
	Conflict  int `json:"conflict"`
}

func writeManifest(t *testing.T, target, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(target, "bob.yaml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPlanJSONActionCounts proves bob plan --json exposes a full, typed
// action_counts object covering create, update, unchanged, and conflict kinds
// from the same plan that produced the digest. It also proves --conflicts-only
// trims the listed actions while the counts still report the full totals.
func TestPlanJSONActionCounts(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	// Seed a lock with two owned files, then evolve the manifest so a single
	// plan yields one unchanged, one update, one create, and one conflict.
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: counts
  description: test
files:
  - path: keep.txt
    content: "keep\n"
  - path: update.txt
    content: "v1\n"
`)
	if _, _, err := executeForTest("apply", target); err != nil {
		t.Fatalf("apply seed: %v", err)
	}
	// conflict.txt exists on disk but is not lock-owned, so the newly declared
	// path with different content is an unmanaged-divergence conflict.
	if err := os.WriteFile(filepath.Join(target, "conflict.txt"), []byte("human edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: counts
  description: test
files:
  - path: keep.txt
    content: "keep\n"
  - path: update.txt
    content: "v2\n"
  - path: new.txt
    content: "new\n"
  - path: conflict.txt
    content: "wanted\n"
`)

	stdout, _, err := executeForTest("--json", "plan", target)
	if err != nil {
		t.Fatalf("plan --json: %v\n%s", err, stdout)
	}
	var got struct {
		OK   bool `json:"ok"`
		Data struct {
			ActionCounts planActionCountsJSON `json:"action_counts"`
			Actions      []struct {
				Kind string `json:"kind"`
			} `json:"actions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	if !got.OK {
		t.Fatalf("plan reported ok=false: %s", stdout)
	}
	want := planActionCountsJSON{Create: 1, Update: 1, Unchanged: 1, Conflict: 1}
	if got.Data.ActionCounts != want {
		t.Fatalf("action_counts = %#v, want %#v\n%s", got.Data.ActionCounts, want, stdout)
	}
	if len(got.Data.Actions) != 4 {
		t.Fatalf("expected 4 actions, got %d:\n%s", len(got.Data.Actions), stdout)
	}

	// --conflicts-only trims the actions but must keep the full counts.
	trimmed, _, err := executeForTest("--json", "plan", target, "--conflicts-only")
	if err != nil {
		t.Fatalf("plan --json --conflicts-only: %v\n%s", err, trimmed)
	}
	var trimmedGot struct {
		Data struct {
			ActionCounts planActionCountsJSON `json:"action_counts"`
			Actions      []struct {
				Kind string `json:"kind"`
			} `json:"actions"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(trimmed), &trimmedGot); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, trimmed)
	}
	if trimmedGot.Data.ActionCounts != want {
		t.Fatalf("conflicts-only action_counts = %#v, want full %#v\n%s", trimmedGot.Data.ActionCounts, want, trimmed)
	}
	if len(trimmedGot.Data.Actions) != 1 || trimmedGot.Data.Actions[0].Kind != "conflict" {
		t.Fatalf("conflicts-only should list exactly one conflict action, got %#v\n%s", trimmedGot.Data.Actions, trimmed)
	}
}

// TestPlanJSONActionCountsRetire proves --allow-retire reclassifies a dropped
// lock-owned path as a retire action and that the projected action_counts
// reports it, with the retired path no longer counted as a conflict.
func TestPlanJSONActionCountsRetire(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: counts
  description: test
files:
  - path: keep.txt
    content: "keep\n"
  - path: retire.txt
    content: "retire\n"
`)
	if _, _, err := executeForTest("apply", target); err != nil {
		t.Fatalf("apply seed: %v", err)
	}
	// Drop retire.txt from the manifest; with --allow-retire it becomes a
	// retire action instead of a retired_owned conflict.
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: counts
  description: test
files:
  - path: keep.txt
    content: "keep\n"
`)

	stdout, _, err := executeForTest("--json", "plan", target, "--allow-retire")
	if err != nil {
		t.Fatalf("plan --json --allow-retire: %v\n%s", err, stdout)
	}
	var got struct {
		Data struct {
			ActionCounts planActionCountsJSON `json:"action_counts"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	want := planActionCountsJSON{Unchanged: 1, Retire: 1}
	if got.Data.ActionCounts != want {
		t.Fatalf("action_counts = %#v, want %#v\n%s", got.Data.ActionCounts, want, stdout)
	}
}

// TestCheckJSONActionCounts proves bob check --json nests the same
// action_counts object under data.plan.
func TestCheckJSONActionCounts(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: counts
  description: test
files:
  - path: a.txt
    content: "a\n"
  - path: b.txt
    content: "b\n"
`)
	stdout, _, err := executeForTest("--json", "check", target)
	if err == nil {
		t.Fatalf("check of an unapplied workspace should report drift, got nil error")
	}
	var got struct {
		OK   bool `json:"ok"`
		Data struct {
			Plan struct {
				ActionCounts planActionCountsJSON `json:"action_counts"`
			} `json:"plan"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout)
	}
	if got.Data.Plan.ActionCounts.Create != 2 {
		t.Fatalf("check action_counts.create = %d, want 2\n%s", got.Data.Plan.ActionCounts.Create, stdout)
	}
}
