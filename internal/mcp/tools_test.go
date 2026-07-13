package mcp

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPlanFilteringDigestCheckAndReadOnlyContract(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	applyWorkspace(t, root)
	before := fileSnapshot(t, root)
	runner := &offlineRunner{}
	session := connect(t, root, runner)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_plan", Arguments: map[string]any{},
	})
	if err != nil || result.IsError {
		t.Fatalf("default plan: result=%#v err=%v", result, err)
	}
	var compact PlanOutput
	decodeStructured(t, result, &compact)
	if !compact.Clean || compact.PlanDigest == "" || len(compact.Actions) != 0 {
		t.Fatalf("default plan should filter converged actions: %#v", compact)
	}
	if compact.Truncation.FilteredUnchanged != compact.Truncation.TotalActions || compact.Truncation.Truncated {
		t.Fatalf("filtered actions were reported as truncation: %#v", compact.Truncation)
	}

	result, err = session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_plan", Arguments: map[string]any{"include_unchanged": true, "max_actions": 1},
	})
	if err != nil || result.IsError {
		t.Fatalf("expanded plan: result=%#v err=%v", result, err)
	}
	var expanded PlanOutput
	decodeStructured(t, result, &expanded)
	if expanded.PlanDigest != compact.PlanDigest {
		t.Fatalf("projection changed complete-plan digest: compact=%s expanded=%s", compact.PlanDigest, expanded.PlanDigest)
	}
	if len(expanded.Actions) != 1 || !expanded.Truncation.Truncated || expanded.Truncation.OmittedActions == 0 {
		t.Fatalf("max_actions projection was not explicit: %#v", expanded.Truncation)
	}

	result, err = session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_check", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("check: result=%#v err=%v", result, err)
	}
	var check CheckOutput
	decodeStructured(t, result, &check)
	if !check.Clean || check.PlanDigest != compact.PlanDigest || check.Counts != compact.Counts {
		t.Fatalf("check and plan disagree: check=%#v plan=%#v", check, compact)
	}

	result, err = session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_plan", Arguments: map[string]any{"max_actions": maximumMaxActions + 1},
	})
	if err != nil || !result.IsError {
		t.Fatalf("out-of-range max_actions should fail: result=%#v err=%v", result, err)
	}
	decodeStructured(t, result, &compact)
	if compact.Error == nil || compact.Error.Code != "input_invalid" {
		t.Fatalf("unexpected max_actions failure: %#v", compact)
	}

	assertSnapshotEqual(t, before, fileSnapshot(t, root))
	if runner.calls() != 0 {
		t.Fatalf("plan/check invoked subprocess runner %d time(s)", runner.calls())
	}
}

func TestValidateManifestStrictXORAndBound(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	before := fileSnapshot(t, root)
	runner := &offlineRunner{}
	session := connect(t, root, runner)
	m, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := manifest.Encode(m)
	if err != nil {
		t.Fatal(err)
	}

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_validate_manifest", Arguments: map[string]any{"manifest_yaml": string(encoded)},
	})
	if err != nil || result.IsError {
		t.Fatalf("inline validation: result=%#v err=%v", result, err)
	}
	var output ValidateManifestOutput
	decodeStructured(t, result, &output)
	if !output.OK || output.Source != "inline" || output.Manifest == nil || output.Manifest.Product.Name != m.Product.Name || output.Recipe == nil {
		t.Fatalf("unexpected normalized manifest: %#v", output)
	}

	result, err = session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_validate_manifest", Arguments: map[string]any{"workspace": "."},
	})
	if err != nil || result.IsError {
		t.Fatalf("workspace validation: result=%#v err=%v", result, err)
	}
	decodeStructured(t, result, &output)
	if output.Source != "workspace" || output.Workspace != root || output.Authority.SelectedWorkspace != root {
		t.Fatalf("workspace authority missing from validation: %#v", output)
	}

	cases := []struct {
		name string
		args map[string]any
		code string
	}{
		{name: "neither", args: map[string]any{}, code: "input_invalid"},
		{name: "both", args: map[string]any{"workspace": ".", "manifest_yaml": string(encoded)}, code: "input_invalid"},
		{name: "unknown field", args: map[string]any{"manifest_yaml": string(encoded) + "\nunknown: true\n"}, code: "manifest_invalid"},
		{name: "multiple documents", args: map[string]any{"manifest_yaml": string(encoded) + "\n---\n{}\n"}, code: "manifest_invalid"},
		{name: "oversized", args: map[string]any{"manifest_yaml": strings.Repeat("x", maxInlineManifestSize+1)}, code: "manifest_too_large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_validate_manifest", Arguments: tc.args})
			if err != nil || !result.IsError {
				t.Fatalf("expected structured failure: result=%#v err=%v", result, err)
			}
			decodeStructured(t, result, &output)
			if output.OK || output.Error == nil || output.Error.Code != tc.code {
				t.Fatalf("failure = %#v, want %s", output, tc.code)
			}
		})
	}

	assertSnapshotEqual(t, before, fileSnapshot(t, root))
	if runner.calls() != 0 {
		t.Fatalf("manifest validation invoked subprocess runner %d time(s)", runner.calls())
	}
}

func TestExactWorkspaceAuthorityAndExplicitEscapeHatches(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	other := mcpWorkspace(t)
	runner := &offlineRunner{}

	defaultSession := connect(t, root, runner)
	result, err := defaultSession.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_check", Arguments: map[string]any{"workspace": other},
	})
	if err != nil || !result.IsError {
		t.Fatalf("unlisted workspace should be denied: result=%#v err=%v", result, err)
	}
	var denied CheckOutput
	decodeStructured(t, result, &denied)
	if denied.Error == nil || denied.Error.Code != "workspace_unauthorized" || denied.Authority.Mode != "exact_allowlist" {
		t.Fatalf("unexpected authority denial: %#v", denied)
	}

	allowedSession := connectWithOptions(t, root, runner, ServerOptions{AllowedWorkspaces: []string{other}})
	result, err = allowedSession.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_check", Arguments: map[string]any{"workspace": other},
	})
	if err != nil || result.IsError {
		t.Fatalf("explicitly allowed workspace: result=%#v err=%v", result, err)
	}
	var allowed CheckOutput
	decodeStructured(t, result, &allowed)
	if allowed.Workspace != other || allowed.Authority.AllowedWorkspaceCount != 2 {
		t.Fatalf("unexpected allowed authority: %#v", allowed.Authority)
	}

	anySession := connectWithOptions(t, root, runner, ServerOptions{AllowAnyWorkspace: true})
	result, err = anySession.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_check", Arguments: map[string]any{"workspace": other},
	})
	if err != nil || result.IsError {
		t.Fatalf("allow-any workspace: result=%#v err=%v", result, err)
	}
	decodeStructured(t, result, &allowed)
	if allowed.Authority.Mode != "any_workspace" {
		t.Fatalf("allow-any authority not disclosed: %#v", allowed.Authority)
	}

	link := root + "-link"
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
	result, err = defaultSession.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_check", Arguments: map[string]any{"workspace": link},
	})
	if err != nil || !result.IsError {
		t.Fatalf("symlink boundary should be rejected: result=%#v err=%v", result, err)
	}
	decodeStructured(t, result, &denied)
	if denied.Error == nil || denied.Error.Code != "workspace_invalid" {
		t.Fatalf("unexpected symlink-boundary failure: %#v", denied)
	}
	if runner.calls() != 0 {
		t.Fatalf("authority checks invoked subprocess runner %d time(s)", runner.calls())
	}
}

func TestRecipeDescribeIsTypedAndClosedWorld(t *testing.T) {
	t.Parallel()
	root := mcpWorkspace(t)
	before := fileSnapshot(t, root)
	runner := &offlineRunner{}
	session := connect(t, root, runner)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_recipe_describe", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("describe recipe: result=%#v err=%v", result, err)
	}
	var output RecipeDescribeOutput
	decodeStructured(t, result, &output)
	if !output.OK || output.Recipe == nil || output.Recipe.ID != currentRecipeID || output.Recipe.Version == 0 || len(output.Recipe.SupportedChoices) == 0 {
		t.Fatalf("unexpected recipe description: %#v", output)
	}

	result, err = session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "bob_recipe_describe", Arguments: map[string]any{"recipe": "unknown"},
	})
	if err != nil || !result.IsError {
		t.Fatalf("unknown recipe should fail: result=%#v err=%v", result, err)
	}
	decodeStructured(t, result, &output)
	if output.Error == nil || output.Error.Code != "recipe_unknown" {
		t.Fatalf("unexpected recipe failure: %#v", output)
	}

	assertSnapshotEqual(t, before, fileSnapshot(t, root))
	if runner.calls() != 0 {
		t.Fatalf("recipe description invoked subprocess runner %d time(s)", runner.calls())
	}
}
