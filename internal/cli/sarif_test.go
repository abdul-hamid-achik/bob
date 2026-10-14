package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/version"
)

// sarifResultJSON mirrors the SARIF result shape so tests decode the emitted
// document independently of the internal structs.
type sarifResultJSON struct {
	RuleID  string `json:"ruleId"`
	Level   string `json:"level"`
	Message struct {
		Text string `json:"text"`
	} `json:"message"`
	Locations []struct {
		PhysicalLocation struct {
			ArtifactLocation struct {
				URI string `json:"uri"`
			} `json:"artifactLocation"`
		} `json:"physicalLocation"`
	} `json:"locations"`
}

type sarifRuleJSON struct {
	ID string `json:"id"`
}

type sarifDriverJSON struct {
	Name           string          `json:"name"`
	InformationURI string          `json:"informationUri"`
	Version        string          `json:"version"`
	Rules          []sarifRuleJSON `json:"rules"`
}

type sarifDocumentJSON struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []struct {
		Tool struct {
			Driver sarifDriverJSON `json:"driver"`
		} `json:"tool"`
		Results []sarifResultJSON `json:"results"`
	} `json:"runs"`
}

func TestSarifDocumentShapeAndLevels(t *testing.T) {
	t.Parallel()
	plan := engine.PlanResult{
		Actions: []engine.Action{
			{Path: "conflict.txt", Kind: engine.ActionConflict, Code: engine.CodeUnmanagedDiffers, Reason: "unmanaged file differs from the desired content"},
			{Path: "new.txt", Kind: engine.ActionCreate, Code: engine.CodeMissing, Reason: "destination does not exist"},
			{Path: "updated.txt", Kind: engine.ActionUpdate, Code: engine.CodeContentUpdate, Reason: "managed file still matches bob.lock and may be updated safely"},
			{Path: "internal/retired.txt", Kind: engine.ActionRetire, Code: engine.CodeRetiredUnchanged, Reason: "bob.lock owns this file but the recipe no longer declares it"},
			{Path: "same.txt", Kind: engine.ActionUnchanged, Code: engine.CodeInSync, Reason: "managed file already matches the desired content and mode"},
			{Path: "adopted.txt", Kind: engine.ActionAdopt, Code: engine.CodeIdenticalContent, Reason: "unmanaged file has identical content and can be adopted"},
		},
		ConflictCount: 1,
	}

	doc := buildSARIF(plan)

	if doc.Schema != "https://json.schemastore.org/sarif-2.1.0.json" {
		t.Fatalf("schema = %q", doc.Schema)
	}
	if doc.Version != "2.1.0" {
		t.Fatalf("version = %q", doc.Version)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("runs length = %d", len(doc.Runs))
	}
	driver := doc.Runs[0].Tool.Driver
	if driver.Name != "bob" {
		t.Fatalf("tool name = %q", driver.Name)
	}
	if driver.InformationURI != "https://bobcli.dev" {
		t.Fatalf("informationUri = %q", driver.InformationURI)
	}
	if driver.Version != version.Version {
		t.Fatalf("tool version = %q, want %q", driver.Version, version.Version)
	}

	// Rules must contain exactly one entry per reported code, in sorted order,
	// and omit the skipped unchanged/adopt codes.
	wantRules := []string{"content_update", "missing", "retired_unchanged", "unmanaged_differs"}
	if len(driver.Rules) != len(wantRules) {
		t.Fatalf("rules = %#v, want %v", driver.Rules, wantRules)
	}
	for i, want := range wantRules {
		if driver.Rules[i].ID != want {
			t.Fatalf("rule[%d] = %q, want %q", i, driver.Rules[i].ID, want)
		}
	}

	// Results report only non-converged actions with the correct level and
	// repository-relative path, and skip unchanged/adopt entirely.
	if len(doc.Runs[0].Results) != 4 {
		t.Fatalf("results length = %d, want 4", len(doc.Runs[0].Results))
	}
	levels := map[string]string{
		"unmanaged_differs": "error",
		"missing":           "note",
		"content_update":    "warning",
		"retired_unchanged": "warning",
	}
	reasons := map[string]string{
		"unmanaged_differs": "unmanaged file differs from the desired content",
		"missing":           "destination does not exist",
		"content_update":    "managed file still matches bob.lock and may be updated safely",
		"retired_unchanged": "bob.lock owns this file but the recipe no longer declares it",
	}
	uris := map[string]string{
		"unmanaged_differs": "conflict.txt",
		"missing":           "new.txt",
		"content_update":    "updated.txt",
		"retired_unchanged": "internal/retired.txt",
	}
	for _, result := range doc.Runs[0].Results {
		if result.Level != levels[result.RuleID] {
			t.Fatalf("result %q level = %q, want %q", result.RuleID, result.Level, levels[result.RuleID])
		}
		if result.Message.Text != reasons[result.RuleID] {
			t.Fatalf("result %q message = %q, want %q", result.RuleID, result.Message.Text, reasons[result.RuleID])
		}
		if len(result.Locations) != 1 {
			t.Fatalf("result %q locations length = %d", result.RuleID, len(result.Locations))
		}
		if got := result.Locations[0].PhysicalLocation.ArtifactLocation.URI; got != uris[result.RuleID] {
			t.Fatalf("result %q uri = %q, want %q", result.RuleID, got, uris[result.RuleID])
		}
	}
}

func TestSarifEmptyPlanProducesEmptyRuns(t *testing.T) {
	t.Parallel()
	doc := buildSARIF(engine.PlanResult{})
	if len(doc.Runs) != 1 {
		t.Fatalf("runs length = %d", len(doc.Runs))
	}
	if len(doc.Runs[0].Results) != 0 {
		t.Fatalf("results length = %d, want 0", len(doc.Runs[0].Results))
	}
	if len(doc.Runs[0].Tool.Driver.Rules) != 0 {
		t.Fatalf("rules length = %d, want 0", len(doc.Runs[0].Tool.Driver.Rules))
	}
	if doc.Version != "2.1.0" {
		t.Fatalf("version = %q", doc.Version)
	}
}

func TestPlanSarifOutput(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	// A files workspace with one conflicting existing file and one missing
	// file yields exactly two findings: unmanaged_differs (error) and
	// missing (note).
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: sarif
  description: test
files:
  - path: conflict.txt
    content: "desired\n"
  - path: new.txt
    content: "new\n"
`)
	if err := os.WriteFile(filepath.Join(target, "conflict.txt"), []byte("actual\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := executeForTest("plan", target, "--sarif")
	if err != nil {
		t.Fatalf("plan --sarif: %v\n%s", err, stdout)
	}
	if stderr != "" {
		t.Fatalf("plan --sarif stderr = %q", stderr)
	}
	var doc sarifDocumentJSON
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("decode SARIF: %v\n%s", err, stdout)
	}
	if doc.Version != "2.1.0" || doc.Schema != "https://json.schemastore.org/sarif-2.1.0.json" {
		t.Fatalf("SARIF envelope = version %q schema %q", doc.Version, doc.Schema)
	}
	if len(doc.Runs) != 1 {
		t.Fatalf("runs length = %d", len(doc.Runs))
	}
	if doc.Runs[0].Tool.Driver.Name != "bob" {
		t.Fatalf("tool name = %q", doc.Runs[0].Tool.Driver.Name)
	}
	if len(doc.Runs[0].Results) != 2 {
		t.Fatalf("results length = %d, want 2:\n%s", len(doc.Runs[0].Results), stdout)
	}
	if len(doc.Runs[0].Tool.Driver.Rules) != 2 {
		t.Fatalf("rules length = %d, want 2:\n%s", len(doc.Runs[0].Tool.Driver.Rules), stdout)
	}
	levels := map[string]string{}
	for _, result := range doc.Runs[0].Results {
		levels[result.RuleID] = result.Level
	}
	if levels["unmanaged_differs"] != "error" || levels["missing"] != "note" {
		t.Fatalf("levels = %#v", levels)
	}
}

func TestCheckSarifPreservesExitCode(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: sarif-check
  description: test
files:
  - path: conflict.txt
    content: "desired\n"
`)
	if err := os.WriteFile(filepath.Join(target, "conflict.txt"), []byte("actual\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("check", target, "--sarif")
	if err == nil {
		t.Fatalf("check --sarif on conflict should exit non-zero, stdout=%s", stdout)
	}
	if ExitCode(err) != ExitConflicts {
		t.Fatalf("check --sarif exit code = %d, want %d", ExitCode(err), ExitConflicts)
	}
	var doc sarifDocumentJSON
	if decodeErr := json.Unmarshal([]byte(stdout), &doc); decodeErr != nil {
		t.Fatalf("check --sarif stdout is not SARIF: %v\n%s", decodeErr, stdout)
	}
	if len(doc.Runs[0].Results) != 1 || doc.Runs[0].Results[0].RuleID != "unmanaged_differs" {
		t.Fatalf("check --sarif results = %#v", doc.Runs[0].Results)
	}
}

func TestSarifAndJSONAreMutuallyExclusive(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: sarif-mutex
  description: test
files:
  - path: a.txt
    content: "a\n"
`)
	for _, args := range [][]string{
		{"plan", target, "--sarif", "--json"},
		{"check", target, "--sarif", "--json"},
	} {
		stdout, _, err := executeForTest(args...)
		if err == nil {
			t.Fatalf("%v should reject --sarif --json", args)
		}
		if ExitCode(err) != ExitInvalidInput {
			t.Fatalf("%v exit code = %d, want %d (%v)", args, ExitCode(err), ExitInvalidInput, err)
		}
		if strings.Contains(stdout, "schema_version") {
			t.Fatalf("%v wrote a JSON envelope to stdout on invalid input:\n%s", args, stdout)
		}
	}
}
