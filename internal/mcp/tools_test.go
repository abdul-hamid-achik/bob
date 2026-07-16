package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
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
	completePlan, _, err := buildPlan(root)
	if err != nil {
		t.Fatal(err)
	}
	sharedDigest := engine.DigestPlan(completePlan)
	if compact.PlanDigest != sharedDigest.SHA256 {
		t.Fatalf("MCP digest differs from engine: mcp=%s engine=%d:%s", compact.PlanDigest, sharedDigest.Version, sharedDigest.SHA256)
	}
	if compact.PlanDigestQualified != sharedDigest.Qualified() {
		t.Fatalf("qualified MCP digest differs from engine: mcp=%s engine=%s", compact.PlanDigestQualified, sharedDigest.Qualified())
	}
	if legacyDigest(completePlan) != sharedDigest.SHA256 {
		t.Fatalf("engine digest changed legacy v1 identity: legacy=%s engine=%s", legacyDigest(completePlan), sharedDigest.SHA256)
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
	if expanded.Actions[0].Code == "" {
		t.Fatalf("plan action omitted its machine-readable code: %#v", expanded.Actions[0])
	}

	result, err = session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_check", Arguments: map[string]any{}})
	if err != nil || result.IsError {
		t.Fatalf("check: result=%#v err=%v", result, err)
	}
	var check CheckOutput
	decodeStructured(t, result, &check)
	if !check.Clean || check.PlanDigest != compact.PlanDigest || check.PlanDigestQualified != compact.PlanDigestQualified || check.Counts != compact.Counts {
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

// legacyDigest is the exact pre-Phase-1 MCP implementation retained in the
// compatibility test so moving the function cannot silently change v1.
func legacyDigest(plan engine.PlanResult) string {
	actions := make([]PlanAction, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		actions = append(actions, projectAction(action))
	}
	identity := struct {
		SchemaVersion int               `json:"schema_version"`
		Recipe        engine.LockRecipe `json:"recipe"`
		LockChanged   bool              `json:"lock_changed"`
		DesiredLock   engine.LockFile   `json:"desired_lock"`
		Actions       []PlanAction      `json:"actions"`
	}{plan.SchemaVersion, plan.Recipe, plan.LockChanged, plan.DesiredLock, actions}
	data, _ := json.Marshal(identity)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
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
		Name: "bob_recipe_describe", Arguments: map[string]any{"recipe": "ts-app"},
	})
	if err != nil || result.IsError {
		t.Fatalf("describe stack recipe: result=%#v err=%v", result, err)
	}
	var stackOutput RecipeDescribeOutput
	decodeStructured(t, result, &stackOutput)
	if !stackOutput.OK || stackOutput.Recipe == nil || stackOutput.Recipe.ID != "ts-app" || stackOutput.Recipe.Version != 1 {
		t.Fatalf("unexpected stack recipe description: %#v", stackOutput)
	}
	if !strings.Contains(stackOutput.Recipe.Description, "seed") {
		t.Fatalf("stack recipe description must state seed-once semantics: %q", stackOutput.Recipe.Description)
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

func filesMCPWorkspace(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	m := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Recipe:        manifest.RecipeFiles,
		Product:       manifest.Product{Name: "demo-app", Description: "A demo app"},
		Files:         []manifest.FileDecl{{Path: "a.txt", Content: "hello\n"}},
	}
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func TestFilesRecipeWorkspacePlanCheckValidateAndDescribe(t *testing.T) {
	t.Parallel()
	root := filesMCPWorkspace(t)
	before := fileSnapshot(t, root)
	runner := &offlineRunner{}
	session := connect(t, root, runner)

	planResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_plan", Arguments: map[string]any{"include_unchanged": true}})
	if err != nil || planResult.IsError {
		t.Fatalf("plan: result=%#v err=%v", planResult, err)
	}
	var plan PlanOutput
	decodeStructured(t, planResult, &plan)
	if plan.Clean || len(plan.Actions) != 1 || plan.Actions[0].Kind != "create" {
		t.Fatalf("unexpected files-recipe plan: %#v", plan)
	}

	checkResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_check", Arguments: map[string]any{}})
	if err != nil || checkResult.IsError {
		t.Fatalf("check: result=%#v err=%v", checkResult, err)
	}
	var check CheckOutput
	decodeStructured(t, checkResult, &check)
	if check.Clean {
		t.Fatalf("unapplied files-recipe workspace reported clean: %#v", check)
	}

	validateResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_validate_manifest", Arguments: map[string]any{"workspace": "."}})
	if err != nil || validateResult.IsError {
		t.Fatalf("validate: result=%#v err=%v", validateResult, err)
	}
	var validate ValidateManifestOutput
	decodeStructured(t, validateResult, &validate)
	if validate.Recipe == nil || validate.Recipe.ID != "files" || validate.Recipe.Version != 1 {
		t.Fatalf("files manifest reported wrong recipe stamp: %#v", validate.Recipe)
	}

	describeResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_recipe_describe", Arguments: map[string]any{"recipe": "files"}})
	if err != nil || describeResult.IsError {
		t.Fatalf("describe files: result=%#v err=%v", describeResult, err)
	}
	var describe RecipeDescribeOutput
	decodeStructured(t, describeResult, &describe)
	if describe.Recipe == nil || describe.Recipe.ID != "files" || describe.Recipe.Version != 1 {
		t.Fatalf("unexpected files recipe description: %#v", describe)
	}

	contextResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_context", Arguments: map[string]any{}})
	if err != nil || contextResult.IsError {
		t.Fatalf("context: result=%#v err=%v", contextResult, err)
	}
	var contract ContextOutput
	decodeStructured(t, contextResult, &contract)
	if contract.Context == nil || contract.Context.Recipe.ID != "files" || len(contract.Context.EntryPoints) != 0 {
		t.Fatalf("files context inferred application semantics: %#v", contract.Context)
	}
	for _, capability := range contract.Context.Capabilities {
		if capability.Category != "" && capability.Category != "repository" {
			t.Fatalf("files context inferred capability %#v", capability)
		}
	}

	pathResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_path", Arguments: map[string]any{"path": "a.txt"}})
	if err != nil || pathResult.IsError {
		t.Fatalf("path: result=%#v err=%v", pathResult, err)
	}
	var path PathOutput
	decodeStructured(t, pathResult, &path)
	if path.Path == nil || path.Path.Artifact == nil || !reflect.DeepEqual(path.Path.Artifact.Roles, []string{"declared_file"}) {
		t.Fatalf("files path semantics = %#v", path.Path)
	}

	playbookResult, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "bob_playbook", Arguments: map[string]any{"operation": "list"}})
	if err != nil || playbookResult.IsError {
		t.Fatalf("playbook: result=%#v err=%v", playbookResult, err)
	}
	var catalog PlaybookOutput
	decodeStructured(t, playbookResult, &catalog)
	if catalog.List == nil || len(catalog.List.Playbooks) != 2 || catalog.List.Playbooks[0].ID != "resolve-ownership-conflict" || catalog.List.Playbooks[1].ID != "upgrade-recipe" {
		t.Fatalf("files playbooks = %#v", catalog.List)
	}

	assertSnapshotEqual(t, before, fileSnapshot(t, root))
	if runner.calls() != 0 {
		t.Fatalf("files-recipe workspace tools invoked subprocess runner %d time(s)", runner.calls())
	}
}
