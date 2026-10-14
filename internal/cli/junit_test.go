package cli

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
)

// junitDocForTest mirrors the emitted JUnit document so tests decode it
// independently of the internal structs.
type junitDocForTest struct {
	XMLName    xml.Name `xml:"testsuites"`
	TestSuites []struct {
		Name      string `xml:"name,attr"`
		Tests     int    `xml:"tests,attr"`
		Failures  int    `xml:"failures,attr"`
		TestCases []struct {
			Name      string `xml:"name,attr"`
			ClassName string `xml:"classname,attr"`
			Time      string `xml:"time,attr"`
			Failure   *struct {
				Type    string `xml:"type,attr"`
				Message string `xml:"message,attr"`
			} `xml:"failure"`
		} `xml:"testcase"`
	} `xml:"testsuite"`
}

func decodeJUnit(t *testing.T, raw string) junitDocForTest {
	t.Helper()
	var doc junitDocForTest
	if err := xml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatalf("decode JUnit XML: %v\n%s", err, raw)
	}
	return doc
}

func TestJunitDocumentShapeAndCounts(t *testing.T) {
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

	doc := buildJUnit("plan", plan)

	if len(doc.TestSuites) != 1 {
		t.Fatalf("testsuites length = %d", len(doc.TestSuites))
	}
	suite := doc.TestSuites[0]
	if suite.Name != "bob plan" {
		t.Fatalf("suite name = %q", suite.Name)
	}
	if suite.Tests != 6 {
		t.Fatalf("tests = %d, want 6", suite.Tests)
	}
	if suite.Failures != 4 {
		t.Fatalf("failures = %d, want 4", suite.Failures)
	}
	if len(suite.TestCases) != 6 {
		t.Fatalf("testcases length = %d", len(suite.TestCases))
	}

	// Non-converged actions carry a failure with the stable code and reason;
	// unchanged/adopt are passing testcases with no failure element.
	failures := map[string]string{
		"conflict.txt":         "unmanaged_differs",
		"new.txt":              "missing",
		"updated.txt":          "content_update",
		"internal/retired.txt": "retired_unchanged",
	}
	passing := map[string]bool{"same.txt": true, "adopted.txt": true}
	for _, tc := range suite.TestCases {
		wantCode, shouldFail := failures[tc.Name]
		if shouldFail {
			if tc.Failure == nil {
				t.Fatalf("testcase %q should have a failure element", tc.Name)
			}
			if tc.Failure.Type != wantCode {
				t.Fatalf("testcase %q failure type = %q, want %q", tc.Name, tc.Failure.Type, wantCode)
			}
			if tc.ClassName != string(kindForPath(plan, tc.Name)) {
				t.Fatalf("testcase %q classname = %q", tc.Name, tc.ClassName)
			}
		} else if passing[tc.Name] {
			if tc.Failure != nil {
				t.Fatalf("testcase %q should pass, got failure %#v", tc.Name, tc.Failure)
			}
		}
		if tc.Time != "0" {
			t.Fatalf("testcase %q time = %q, want 0", tc.Name, tc.Time)
		}
	}
}

// kindForPath returns the expected classname for a path in the fixture plan.
func kindForPath(plan engine.PlanResult, path string) engine.ActionKind {
	for _, a := range plan.Actions {
		if a.Path == path {
			return a.Kind
		}
	}
	return ""
}

func TestJunitEscapesXMLSpecialCharacters(t *testing.T) {
	t.Parallel()
	plan := engine.PlanResult{
		Actions: []engine.Action{
			{Path: "a&b<c\".txt", Kind: engine.ActionConflict, Code: engine.CodeUnmanagedDiffers, Reason: "contains & and < and \" quotes"},
			{Path: "new<&.txt", Kind: engine.ActionCreate, Code: engine.CodeMissing, Reason: "destination <does> & not exist"},
		},
		ConflictCount: 1,
	}

	var sb strings.Builder
	if err := emitJUnit(&sb, "check", plan); err != nil {
		t.Fatalf("emitJUnit: %v", err)
	}
	doc := decodeJUnit(t, sb.String())
	if len(doc.TestSuites) != 1 {
		t.Fatalf("testsuites length = %d", len(doc.TestSuites))
	}
	cases := doc.TestSuites[0].TestCases
	if len(cases) != 2 {
		t.Fatalf("testcases length = %d", len(cases))
	}
	if cases[0].Name != `a&b<c".txt` {
		t.Fatalf("testcase name round-trip = %q", cases[0].Name)
	}
	if cases[0].Failure == nil || cases[0].Failure.Message != `contains & and < and " quotes` {
		t.Fatalf("testcase failure message round-trip = %#v", cases[0].Failure)
	}
	if cases[1].Name != `new<&.txt` {
		t.Fatalf("testcase name round-trip = %q", cases[1].Name)
	}
	if cases[1].Failure == nil || cases[1].Failure.Message != `destination <does> & not exist` {
		t.Fatalf("testcase failure message round-trip = %#v", cases[1].Failure)
	}
}

func TestPlanJunitOutput(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: junit
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

	stdout, stderr, err := executeForTest("plan", target, "--junit")
	if err != nil {
		t.Fatalf("plan --junit: %v\n%s", err, stdout)
	}
	if stderr != "" {
		t.Fatalf("plan --junit stderr = %q", stderr)
	}
	doc := decodeJUnit(t, stdout)
	if len(doc.TestSuites) != 1 {
		t.Fatalf("testsuites length = %d", len(doc.TestSuites))
	}
	suite := doc.TestSuites[0]
	if suite.Name != "bob plan" {
		t.Fatalf("suite name = %q", suite.Name)
	}
	if suite.Tests != 2 || suite.Failures != 2 {
		t.Fatalf("tests/failures = %d/%d, want 2/2:\n%s", suite.Tests, suite.Failures, stdout)
	}
	byName := map[string]string{}
	for _, tc := range suite.TestCases {
		byName[tc.Name] = tc.Failure.Type
	}
	if byName["conflict.txt"] != "unmanaged_differs" || byName["new.txt"] != "missing" {
		t.Fatalf("failure types = %#v", byName)
	}
}

func TestCheckJunitPreservesExitCode(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: junit-check
  description: test
files:
  - path: conflict.txt
    content: "desired\n"
`)
	if err := os.WriteFile(filepath.Join(target, "conflict.txt"), []byte("actual\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := executeForTest("check", target, "--junit")
	if err == nil {
		t.Fatalf("check --junit on conflict should exit non-zero, stdout=%s", stdout)
	}
	if ExitCode(err) != ExitConflicts {
		t.Fatalf("check --junit exit code = %d, want %d", ExitCode(err), ExitConflicts)
	}
	doc := decodeJUnit(t, stdout)
	if len(doc.TestSuites) != 1 || doc.TestSuites[0].Name != "bob check" {
		t.Fatalf("suite = %#v", doc.TestSuites)
	}
	if doc.TestSuites[0].Failures != 1 {
		t.Fatalf("failures = %d, want 1", doc.TestSuites[0].Failures)
	}
	if len(doc.TestSuites[0].TestCases) != 1 || doc.TestSuites[0].TestCases[0].Failure == nil {
		t.Fatalf("testcases = %#v", doc.TestSuites[0].TestCases)
	}
}

func TestJunitMutualExclusion(t *testing.T) {
	t.Parallel()
	target := t.TempDir()
	writeManifest(t, target, `schema_version: 1
recipe: files
product:
  name: junit-mutex
  description: test
files:
  - path: a.txt
    content: "a\n"
`)
	for _, args := range [][]string{
		{"plan", target, "--junit", "--json"},
		{"check", target, "--junit", "--json"},
		{"plan", target, "--junit", "--sarif"},
		{"check", target, "--junit", "--sarif"},
	} {
		stdout, _, err := executeForTest(args...)
		if err == nil {
			t.Fatalf("%v should reject --junit with --json/--sarif", args)
		}
		if ExitCode(err) != ExitInvalidInput {
			t.Fatalf("%v exit code = %d, want %d (%v)", args, ExitCode(err), ExitInvalidInput, err)
		}
		if strings.Contains(stdout, "testsuites") || strings.Contains(stdout, "schema_version") {
			t.Fatalf("%v wrote a report to stdout on invalid input:\n%s", args, stdout)
		}
	}
}
