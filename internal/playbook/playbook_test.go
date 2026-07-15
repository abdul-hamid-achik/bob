package playbook

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/guidance"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
)

func TestListIsSortedBoundedAndHonestForV4(t *testing.T) {
	root := playbookWorkspace(t, manifest.Default("acme", "github.com/acme/acme", "Acme"), true)
	result, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"add-cli-command", "enable-github-actions", "enable-goreleaser", "enable-homebrew", "enable-terminal-verification", "resolve-ownership-conflict", "upgrade-recipe"}
	ids := []string{}
	for _, summary := range result.Playbooks {
		ids = append(ids, summary.ID)
	}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %#v", ids)
	}
	add := result.Playbooks[0]
	if !add.Available || len(add.BlockedBy) != 0 {
		t.Fatalf("add-cli-command = %#v", add)
	}
	if result.Playbooks[len(result.Playbooks)-1].Available || !reflect.DeepEqual(result.Playbooks[len(result.Playbooks)-1].BlockedBy, []string{"already_current_recipe"}) {
		t.Fatalf("upgrade = %#v", result.Playbooks[len(result.Playbooks)-1])
	}
	data, _ := json.Marshal(result)
	if len(data) > 8<<10 {
		t.Fatalf("list result = %d bytes", len(data))
	}
	if strings.Contains(string(data), `"blocked_by":null`) || strings.Contains(string(data), `"required_inputs":null`) {
		t.Fatalf("list result contains nullable list fields: %s", data)
	}
}

func TestPlaybookInputValidationAndSuggestion(t *testing.T) {
	root := playbookWorkspace(t, manifest.Default("acme", "github.com/acme/acme", "Acme"), false)
	if _, err := Show(root, "enable-goreleasr"); err == nil || !strings.Contains(err.Error(), "did you mean \"enable-goreleaser\"") {
		t.Fatalf("unknown id error = %v", err)
	}
	if _, err := Plan(root, "add-cli-command", nil); err == nil || !strings.Contains(err.Error(), "missing required inputs: command_name") {
		t.Fatalf("missing error = %v", err)
	}
	if _, err := Plan(root, "add-cli-command", map[string]string{"extra": "x"}); err == nil || !strings.Contains(err.Error(), "unknown inputs: extra") || !strings.Contains(err.Error(), "missing required inputs: command_name") {
		t.Fatalf("combined input error = %v", err)
	}
	for _, unsafe := range []string{"hello;rm", "Hello", "two words", "../escape"} {
		if _, err := Plan(root, "add-cli-command", map[string]string{"command_name": unsafe}); err == nil {
			t.Fatalf("unsafe command_name %q accepted", unsafe)
		}
	}
	if _, err := Plan(root, "add-cli-command", map[string]string{"command_name": strings.Repeat("a", 4097)}); err == nil || !strings.Contains(err.Error(), "exceed") {
		t.Fatalf("oversized input error = %v", err)
	}
	if _, err := Plan(root, "add-cli-command", map[string]string{"command_name": "help"}); err == nil {
		t.Fatal("reserved Cobra help command was accepted")
	} else {
		code, ok := guidance.ErrorCode(err)
		if !ok || code != guidance.ErrorInputInvalid || !strings.Contains(err.Error(), `reserved value "help"`) {
			t.Fatalf("reserved command error code=%q ok=%t err=%v", code, ok, err)
		}
	}
}

func TestDynamicAvailabilityDoesNotMutateRecipeMetadata(t *testing.T) {
	root := playbookWorkspace(t, manifest.Default("acme", "github.com/acme/acme", "Acme"), false)
	state, err := load(root)
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(state.metadata.Playbooks)
	if err != nil {
		t.Fatal(err)
	}
	_ = adjustedDefinitions(state)
	after, err := json.Marshal(state.metadata.Playbooks)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("dynamic availability mutated recipe metadata:\nbefore=%s\nafter=%s", before, after)
	}
}

func TestAddCLICommandResolvesOnlyHumanOwnedFiles(t *testing.T) {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	m.Integrations.TerminalVerification = "none"
	root := playbookWorkspace(t, m, true)
	guide, err := Plan(root, "add-cli-command", map[string]string{"command_name": "hello-world"})
	if err != nil {
		t.Fatal(err)
	}
	if !guide.Playbook.Available || !reflect.DeepEqual(guide.Playbook.Boundary.Create, []string{"internal/cli/hello-world.go", "internal/cli/hello-world_test.go"}) {
		t.Fatalf("resolved command playbook = %#v", guide.Playbook)
	}
	for _, forbidden := range []string{"bob.lock", "internal/cli/registry.go", "internal/cli/root.go"} {
		if !contains(guide.Playbook.Boundary.Forbidden, forbidden) {
			t.Fatalf("forbidden boundary omitted %s: %#v", forbidden, guide.Playbook.Boundary.Forbidden)
		}
	}
	for _, step := range guide.Playbook.Steps {
		for _, path := range step.Paths {
			if path == "internal/cli/root.go" || path == "internal/cli/registry.go" {
				t.Fatalf("playbook step edits Bob-owned path: %#v", step)
			}
		}
	}
	if got := guide.Playbook.Steps[len(guide.Playbook.Steps)-1].Kind; got != "bob_check" {
		t.Fatalf("last step kind = %q, want bob_check", got)
	}
}

func TestAddCLICommandWaitsForMaterializedV4ExtensionContract(t *testing.T) {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	for name, root := range map[string]string{
		"unmaterialized": playbookWorkspace(t, m, false),
		"v3 lock":        playbookV3Workspace(t, m),
	} {
		t.Run(name, func(t *testing.T) {
			shown, err := Show(root, "add-cli-command")
			if err != nil {
				t.Fatal(err)
			}
			if shown.Playbook.Available || !reflect.DeepEqual(shown.Playbook.BlockedBy, []string{"extension_contract_not_materialized"}) {
				t.Fatalf("availability = %#v", shown.Playbook)
			}
			for _, step := range shown.Playbook.Steps {
				if step.Effect == "repository_mutation" && !reflect.DeepEqual(step.BlockedBy, []string{"extension_contract_not_materialized"}) {
					t.Fatalf("mutation step is not blocked: %#v", step)
				}
			}
		})
	}
}

func TestResolveConflictChecksExactCurrentActionAndUsesArgv(t *testing.T) {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	root := playbookWorkspace(t, m, true)
	path := "internal/cli/root.go"
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(path)), []byte("package cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"path": path, "action_code": engine.CodeManagedHashMismatch}
	guide, err := Plan(root, "resolve-ownership-conflict", values)
	if err != nil {
		t.Fatal(err)
	}
	step := guide.Playbook.Steps[0]
	wantArgv := []string{"bob", "path", "--workspace", guide.Workspace, "--json", "--", path}
	if !reflect.DeepEqual(step.Paths, []string{path}) || !reflect.DeepEqual(step.Argv, wantArgv) {
		t.Fatalf("resolved step = %#v", step)
	}
	if _, err := Plan(root, "resolve-ownership-conflict", map[string]string{"path": path, "action_code": engine.CodeRetiredOwned}); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatch error = %v", err)
	}
}

func TestResolveConflictTreatsLeadingHyphenPathAsData(t *testing.T) {
	path := "--help"
	m := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersion,
		Recipe:        manifest.RecipeFiles,
		Product:       manifest.Product{Name: "files", Description: "Files"},
		Files:         []manifest.FileDecl{{Path: path, Content: "owned\n"}},
	}
	root := playbookWorkspace(t, m, true)
	if err := os.WriteFile(filepath.Join(root, path), []byte("human\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	guide, err := Plan(root, "resolve-ownership-conflict", map[string]string{
		"path":        path,
		"action_code": engine.CodeManagedHashMismatch,
	})
	if err != nil {
		t.Fatal(err)
	}
	step := guide.Playbook.Steps[0]
	wantArgv := []string{"bob", "path", "--workspace", guide.Workspace, "--json", "--", path}
	if !reflect.DeepEqual(step.Paths, []string{path}) || !reflect.DeepEqual(step.Argv, wantArgv) {
		t.Fatalf("resolved leading-hyphen path step = %#v, want argv %#v", step, wantArgv)
	}
}

func TestAvailabilityTracksManifestSelectionsAndFilesMetadataStaysGeneric(t *testing.T) {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	m.Distribution.GitHubActions = false
	m.Distribution.GoReleaser = false
	m.Integrations.TerminalVerification = "none"
	root := playbookWorkspace(t, m, false)
	result, err := List(root)
	if err != nil {
		t.Fatal(err)
	}
	availability := map[string]bool{}
	for _, summary := range result.Playbooks {
		availability[summary.ID] = summary.Available
	}
	if !availability["enable-github-actions"] || !availability["enable-goreleaser"] || !availability["enable-terminal-verification"] {
		t.Fatalf("availability = %#v", availability)
	}
	files := manifest.Manifest{SchemaVersion: 1, Recipe: manifest.RecipeFiles, Product: manifest.Product{Name: "files", Description: "Files"}, Files: []manifest.FileDecl{{Path: "package.json", Content: "{}\n"}}}
	filesResult, err := List(playbookWorkspace(t, files, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(filesResult.Playbooks) != 2 || filesResult.Playbooks[0].ID != "resolve-ownership-conflict" || filesResult.Playbooks[1].ID != "upgrade-recipe" {
		t.Fatalf("files playbooks = %#v", filesResult.Playbooks)
	}
}

func TestUnavailablePlaybooksBlockEveryMutationStep(t *testing.T) {
	root := playbookWorkspace(t, manifest.Default("acme", "github.com/acme/acme", "Acme"), true)
	for _, id := range []string{"enable-github-actions", "enable-goreleaser", "enable-terminal-verification", "upgrade-recipe"} {
		shown, err := Show(root, id)
		if err != nil {
			t.Fatal(err)
		}
		if shown.Playbook.Available || len(shown.Playbook.BlockedBy) == 0 {
			t.Fatalf("%s availability = %#v", id, shown.Playbook)
		}
		for _, step := range shown.Playbook.Steps {
			if step.Effect == "repository_mutation" && len(step.BlockedBy) == 0 {
				t.Fatalf("%s exposes unblocked mutation step: %#v", id, step)
			}
		}
	}
}

func TestHomebrewBlockersGateMutationBehindHumanDecision(t *testing.T) {
	m := manifest.Default("acme", "example.com/acme/acme", "Acme")
	m.Product.Visibility = "private"
	m.Distribution.GitHubActions = false
	m.Distribution.GoReleaser = false
	m.Distribution.Homebrew = false
	shown, err := Show(playbookWorkspace(t, m, false), "enable-homebrew")
	if err != nil {
		t.Fatal(err)
	}
	wantBlockers := []string{"github_actions_required", "github_module_required", "goreleaser_required", "public_visibility_required"}
	if shown.Playbook.Available || !reflect.DeepEqual(shown.Playbook.BlockedBy, wantBlockers) {
		t.Fatalf("homebrew blockers = %#v", shown.Playbook)
	}
	if len(shown.Playbook.Steps) == 0 || shown.Playbook.Steps[0].ID != "resolve_prerequisites" || shown.Playbook.Steps[0].Kind != "human_decision" {
		t.Fatalf("homebrew decision step = %#v", shown.Playbook.Steps)
	}
	for _, step := range shown.Playbook.Steps {
		if step.Effect != "repository_mutation" {
			continue
		}
		if !contains(step.DependsOn, "resolve_prerequisites") || !reflect.DeepEqual(step.BlockedBy, wantBlockers) {
			t.Fatalf("homebrew mutation is not gated: %#v", step)
		}
	}
	if !reflect.DeepEqual(shown.Playbook.Boundary.Modify, []string{".github/workflows/release.yml", ".goreleaser.yaml", "bob.yaml"}) {
		t.Fatalf("homebrew boundary = %#v", shown.Playbook.Boundary)
	}
}

func TestToggleScopeAndBoundaryDescribeGeneratedArtifacts(t *testing.T) {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	m.Distribution.GitHubActions = false
	m.Distribution.GoReleaser = false
	m.Integrations.TerminalVerification = "none"
	root := playbookWorkspace(t, m, false)
	tests := []struct {
		id    string
		scope string
		paths []string
	}{
		{"enable-github-actions", "small", []string{".github/workflows/ci.yml"}},
		{"enable-goreleaser", "small", []string{".goreleaser.yaml"}},
		{"enable-terminal-verification", "multi_surface", []string{"glyphrun.config.yml", "specs/help.yml"}},
	}
	for _, tc := range tests {
		shown, err := Show(root, tc.id)
		if err != nil {
			t.Fatal(err)
		}
		if shown.Playbook.ScopeClass != tc.scope || !reflect.DeepEqual(shown.Playbook.Boundary.Create, tc.paths) {
			t.Fatalf("%s scope/boundary = %s %#v", tc.id, shown.Playbook.ScopeClass, shown.Playbook.Boundary)
		}
	}
}

func TestMutationPlaybooksBindApplyToReviewedPlanDigest(t *testing.T) {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	m.Distribution.GitHubActions = false
	m.Distribution.GoReleaser = false
	m.Integrations.TerminalVerification = "none"
	root := playbookWorkspace(t, m, false)
	for _, id := range []string{"enable-github-actions", "enable-goreleaser", "enable-terminal-verification"} {
		shown, err := Show(root, id)
		if err != nil {
			t.Fatal(err)
		}
		assertDigestGatedApplyStep(t, shown.Playbook)
	}
	upgrade := playbookV3Workspace(t, manifest.Default("upgrade", "github.com/acme/upgrade", "Upgrade"))
	shown, err := Show(upgrade, "upgrade-recipe")
	if err != nil {
		t.Fatal(err)
	}
	assertDigestGatedApplyStep(t, shown.Playbook)
}

func assertDigestGatedApplyStep(t *testing.T, definition recipe.PlaybookDefinition) {
	t.Helper()
	found := false
	for _, step := range definition.Steps {
		if step.Kind != "bob_apply" {
			continue
		}
		found = true
		joined := strings.Join(step.Argv, "\x00")
		if !strings.Contains(joined, "--expect-plan-digest\x00<reviewed_plan_digest>") {
			t.Fatalf("%s apply step is not digest-gated: %#v", definition.ID, step)
		}
	}
	if !found {
		t.Fatalf("%s omitted bob_apply step", definition.ID)
	}
}

func TestUpgradeGuideExposesObservedAndCurrentRecipeIdentities(t *testing.T) {
	root := playbookV3Workspace(t, manifest.Default("acme", "github.com/acme/acme", "Acme"))
	guide, err := Plan(root, "upgrade-recipe", nil)
	if err != nil {
		t.Fatal(err)
	}
	if guide.RecipeIdentities == nil || guide.RecipeIdentities.Observed == nil {
		t.Fatalf("upgrade identities = %#v", guide.RecipeIdentities)
	}
	if got := *guide.RecipeIdentities.Observed; got.ID != manifest.RecipeGoAgentTool || got.Version != 3 {
		t.Fatalf("observed recipe = %#v", got)
	}
	if got := guide.RecipeIdentities.Current; got.ID != manifest.RecipeGoAgentTool || got.Version != 4 {
		t.Fatalf("current recipe = %#v", got)
	}
	if !guide.Playbook.Available || len(guide.Playbook.BlockedBy) != 0 {
		t.Fatalf("upgrade playbook = %#v", guide.Playbook)
	}
	for _, step := range guide.Playbook.Steps {
		if step.ID == "resolve_conflicts" || contains(step.DependsOn, "resolve_conflicts") {
			t.Fatalf("clean upgrade retained conflict gate: %#v", guide.Playbook.Steps)
		}
	}
}

func TestConflictedUpgradeBlocksApplyMutation(t *testing.T) {
	root := playbookV3Workspace(t, manifest.Default("acme", "github.com/acme/acme", "Acme"))
	if err := os.WriteFile(filepath.Join(root, "internal", "cli", "root.go"), []byte("package cli\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	shown, err := Show(root, "upgrade-recipe")
	if err != nil {
		t.Fatal(err)
	}
	if shown.Playbook.Available || !reflect.DeepEqual(shown.Playbook.BlockedBy, []string{"ownership_conflicts"}) {
		t.Fatalf("conflicted upgrade = %#v", shown.Playbook)
	}
	for _, step := range shown.Playbook.Steps {
		if step.Effect == "repository_mutation" && !reflect.DeepEqual(step.BlockedBy, []string{"ownership_conflicts"}) {
			t.Fatalf("conflicted upgrade mutation = %#v", step)
		}
	}
}

func TestTerminalVerificationSeparatesManifestAvailabilityAndVerification(t *testing.T) {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	m.Integrations.TerminalVerification = "none"
	lookedUp := []string{}
	guide, err := PlanWithOptions(playbookWorkspace(t, m, false), "enable-terminal-verification", nil, Options{LookPath: func(name string) (string, error) {
		lookedUp = append(lookedUp, name)
		return "/tools/" + name, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lookedUp, []string{"glyph"}) {
		t.Fatalf("binary lookups = %#v", lookedUp)
	}
	want := []Observation{
		{ID: "binary.glyph.availability", Value: "available"},
		{ID: "manifest.integrations.terminal_verification", Value: "none"},
		{ID: "verification", Value: "not_assessed"},
	}
	if !reflect.DeepEqual(guide.Observations, want) {
		t.Fatalf("terminal observations = %#v", guide.Observations)
	}
	shown, err := ShowWithOptions(guide.Workspace, "enable-terminal-verification", Options{LookPath: func(string) (string, error) {
		return "", errors.New("missing")
	}})
	if err != nil {
		t.Fatal(err)
	}
	if shown.Observations[0].Value != "unavailable" || shown.Observations[2].Value != "not_assessed" {
		t.Fatalf("unavailable observations = %#v", shown.Observations)
	}
}

func TestShowAndPlanTruncationIsExplicitAndDeterministic(t *testing.T) {
	definition := func() recipe.PlaybookDefinition {
		return recipe.PlaybookDefinition{
			ID: "bounded", Title: "Bounded", Purpose: strings.Repeat("p", 30<<10), Applicable: true, Available: false,
			BlockedBy: []string{"human_decision_required"}, ScopeClass: "single_file", Risk: "high",
			Inputs: []recipe.PlaybookInputDefinition{}, Preconditions: []string{strings.Repeat("c", 4<<10)},
			Boundary:          recipe.PlaybookBoundary{Create: []string{"target.go"}, Modify: []string{}, Forbidden: []string{"bob.lock"}},
			Steps:             []recipe.PlaybookStep{{ID: "decide", Kind: "human_decision", Effect: "read_only", Summary: strings.Repeat("s", 4<<10), Paths: []string{"target.go"}, Argv: []string{}, DependsOn: []string{}, RequiresExplicitAuthority: true, SuccessCondition: strings.Repeat("x", 4<<10), BlockedBy: []string{"human_decision_required"}}},
			VerificationHints: []string{strings.Repeat("v", 4<<10)}, FailureModes: []string{strings.Repeat("f", 4<<10)},
		}
	}
	makeShow := func() ShowResult {
		return ShowResult{SchemaVersion: 1, Workspace: "/workspace", Recipe: recipe.MetadataRecipeRef{ID: "files", Version: 1}, Observations: []Observation{}, Playbook: definition(), Truncation: noTruncation("show", 24<<10)}
	}
	left, right := makeShow(), makeShow()
	if err := truncateShow(&left); err != nil {
		t.Fatal(err)
	}
	if err := truncateShow(&right); err != nil {
		t.Fatal(err)
	}
	if !left.Truncation.Truncated || encodedSize(left) > 24<<10 || !reflect.DeepEqual(left, right) {
		t.Fatalf("show truncation = bytes:%d left:%#v right:%#v", encodedSize(left), left.Truncation, right.Truncation)
	}
	if len(left.Playbook.Steps) != 1 || !reflect.DeepEqual(left.Playbook.Steps[0].BlockedBy, []string{"human_decision_required"}) {
		t.Fatalf("blocking decision was truncated: %#v", left.Playbook.Steps)
	}
	guide := Guide{SchemaVersion: 1, Workspace: "/workspace", Recipe: recipe.MetadataRecipeRef{ID: "files", Version: 1}, Observations: []Observation{}, Playbook: definition(), Values: map[string]string{"path": strings.Repeat("a", 4096)}, Truncation: noTruncation("plan", 24<<10)}
	if err := truncateGuide(&guide); err != nil {
		t.Fatal(err)
	}
	if !guide.Truncation.Truncated || encodedSize(guide) > 24<<10 {
		t.Fatalf("plan truncation = bytes:%d truncation:%#v", encodedSize(guide), guide.Truncation)
	}
}

func TestPlaybookServicesDoNotMutateWorkspace(t *testing.T) {
	root := playbookWorkspace(t, manifest.Default("acme", "github.com/acme/acme", "Acme"), true)
	before := snapshot(t, root)
	if _, err := List(root); err != nil {
		t.Fatal(err)
	}
	if _, err := Show(root, "enable-homebrew"); err != nil {
		t.Fatal(err)
	}
	if _, err := Plan(root, "enable-homebrew", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("playbook service mutated the workspace")
	}
}

func TestPlaceholderResolutionIsDeterministic(t *testing.T) {
	left := recipe.PlaybookDefinition{Boundary: recipe.PlaybookBoundary{Create: []string{"<first>/<second>.go"}}, Steps: []recipe.PlaybookStep{{Argv: []string{"tool", "<first>", "<second>"}}}}
	right := left
	right.Boundary.Create = append([]string(nil), left.Boundary.Create...)
	right.Steps = append([]recipe.PlaybookStep(nil), left.Steps...)
	right.Steps[0].Argv = append([]string(nil), left.Steps[0].Argv...)
	resolveDefinition(&left, map[string]string{"first": "one", "second": "two"}, "/workspace")
	values := map[string]string{}
	values["second"] = "two"
	values["first"] = "one"
	resolveDefinition(&right, values, "/workspace")
	if !reflect.DeepEqual(left, right) {
		t.Fatalf("resolution differs:\nleft=%#v\nright=%#v", left, right)
	}

	opaque := recipe.PlaybookDefinition{
		Boundary: recipe.PlaybookBoundary{Create: []string{"<workspace>/<path>/<command_name>.go"}},
	}
	resolveDefinition(&opaque, map[string]string{
		"path":         "literal-<command_name>",
		"command_name": "hello",
	}, "/tmp/<path>/<command_name>")
	want := "/tmp/<path>/<command_name>/literal-<command_name>/hello.go"
	if got := opaque.Boundary.Create[0]; got != want {
		t.Fatalf("replacement values were scanned as templates: got %q want %q", got, want)
	}
}

func playbookWorkspace(t *testing.T, m manifest.Manifest, apply bool) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	if apply {
		artifacts, err := recipe.Render(m)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Apply(root, m, artifacts); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func playbookV3Workspace(t *testing.T, m manifest.Manifest) string {
	t.Helper()
	root := t.TempDir()
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, false); err != nil {
		t.Fatal(err)
	}
	artifacts, err := recipe.RenderVersion(m, 3)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Apply(root, m, artifacts); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(root, engine.LockFilename)
	data, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	v3 := strings.Replace(string(data), "  version: 4\n", "  version: 3\n", 1)
	if v3 == string(data) {
		t.Fatal("temporary lock did not contain current recipe version")
	}
	if err := os.WriteFile(lockPath, []byte(v3), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		info, err := entry.Info()
		if err != nil {
			return err
		}
		result[filepath.ToSlash(rel)] = string(data) + info.ModTime().String()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
