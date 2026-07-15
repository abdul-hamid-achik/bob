package recipe

import "github.com/abdul-hamid-achik/bob/internal/manifest"

func PlaybookSummaries(definitions []PlaybookDefinition) []PlaybookSummary {
	summaries := make([]PlaybookSummary, 0, len(definitions))
	for _, definition := range definitions {
		required := []string{}
		for _, input := range definition.Inputs {
			if input.Required {
				required = append(required, input.Name)
			}
		}
		summaries = append(summaries, PlaybookSummary{
			ID: definition.ID, Title: definition.Title, Applicable: definition.Applicable,
			Available: definition.Available, BlockedBy: append([]string{}, definition.BlockedBy...),
			RequiredInputs: required, ScopeClass: definition.ScopeClass, Risk: definition.Risk,
		})
	}
	return summaries
}

func goAgentPlaybooks(m manifest.Manifest) []PlaybookDefinition {
	githubPaths := []string{".github/workflows/ci.yml"}
	if m.Distribution.GoReleaser {
		githubPaths = append(githubPaths, ".github/workflows/release.yml")
	}
	goreleaserPaths := []string{".goreleaser.yaml"}
	if m.Distribution.GitHubActions {
		goreleaserPaths = append(goreleaserPaths, ".github/workflows/release.yml")
	}
	terminalVerification := togglePlaybook("enable-terminal-verification", "Enable terminal verification", "integrations.terminal_verification", m.Integrations.TerminalVerification == "glyphrun", []string{"glyphrun.config.yml", "specs/help.yml"})
	terminalVerification.Purpose = "Select integrations.terminal_verification=glyphrun and reconcile its declared config and help spec."
	terminalVerification.Preconditions = []string{"integrations.terminal_verification accepts the exact value glyphrun"}
	terminalVerification.VerificationHints = []string{
		"the resolved guide reports glyph binary availability without running it",
		"binary availability does not imply that a Glyphrun spec passed",
		"behavior verification remains not_assessed in Bob",
		"bob check --json proves repository convergence only",
	}
	definitions := []PlaybookDefinition{
		{
			ID: "add-cli-command", Title: "Add a CLI command", Purpose: "Add a Cobra command through a recipe-supported human extension point.",
			Applicable: true, Available: true, BlockedBy: []string{}, ScopeClass: "small", Risk: "medium",
			Inputs:            []PlaybookInputDefinition{{Name: "command_name", Required: true, Type: "identifier", Validation: "lowercase-kebab", Forbidden: []string{"help"}}},
			Boundary:          PlaybookBoundary{Create: []string{"internal/cli/<command_name>.go", "internal/cli/<command_name>_test.go"}, Modify: []string{}, Forbidden: []string{"bob.lock", "internal/cli/registry.go", "internal/cli/registry_test.go", "internal/cli/root.go", "internal/cli/root_test.go"}},
			Steps:             addCLICommandSteps(),
			VerificationHints: addCLICommandVerificationHints(m), FailureModes: []string{"duplicate registration IDs or command names fail command-tree construction", "editing a Bob-owned composition file causes managed_hash_mismatch"},
			ExtensionPointIDs: []string{"cli.command_files"},
		},
		togglePlaybook("enable-github-actions", "Enable GitHub Actions", "distribution.github_actions", m.Distribution.GitHubActions, githubPaths),
		togglePlaybook("enable-goreleaser", "Enable GoReleaser", "distribution.goreleaser", m.Distribution.GoReleaser, goreleaserPaths),
		{
			ID: "enable-homebrew", Title: "Enable Homebrew", Purpose: "Select Homebrew packaging after its explicit public-release prerequisites are satisfied.",
			Applicable: true, Available: !m.Distribution.Homebrew, BlockedBy: homebrewBlockers(m), ScopeClass: "multi_surface", Risk: "high",
			Inputs: []PlaybookInputDefinition{}, Preconditions: []string{"product.visibility=public", "distribution.goreleaser=true", "distribution.github_actions=true", "product.module uses a GitHub host"},
			Boundary:          PlaybookBoundary{Create: []string{}, Modify: []string{"bob.yaml", ".goreleaser.yaml", ".github/workflows/release.yml"}, Forbidden: []string{"bob.lock"}},
			Steps:             manifestToggleSteps("distribution.homebrew", []string{".goreleaser.yaml", ".github/workflows/release.yml"}),
			VerificationHints: []string{"bob check --json proves repository convergence only"}, FailureModes: []string{"disabled prerequisites require a human decision"},
		},
		terminalVerification,
		conflictPlaybook(),
		upgradePlaybook(),
	}
	for i := range definitions {
		switch definitions[i].ID {
		case "add-cli-command":
			definitions[i].CapabilityIDs = []string{"surface.cli", "surface.json"}
		case "enable-github-actions":
			definitions[i].CapabilityIDs = []string{"distribution.github_actions"}
		case "enable-goreleaser":
			definitions[i].CapabilityIDs = []string{"distribution.goreleaser"}
			if !m.Distribution.GitHubActions {
				definitions[i].Preconditions = append(definitions[i].Preconditions, "distribution.github_actions=true is required for a release workflow")
			}
		case "enable-homebrew":
			definitions[i].CapabilityIDs = []string{"distribution.homebrew"}
		case "enable-terminal-verification":
			definitions[i].CapabilityIDs = []string{"integration.glyphrun"}
		case "upgrade-recipe":
			definitions[i].CapabilityIDs = []string{"repository.whole_file_ownership"}
		}
		if definitions[i].ID == "enable-homebrew" && len(definitions[i].BlockedBy) > 0 {
			definitions[i].Available = false
			decisionID := ""
			prerequisiteBlockers := withoutString(definitions[i].BlockedBy, "capability_already_enabled")
			if len(prerequisiteBlockers) > 0 {
				decisionID = "resolve_prerequisites"
				definitions[i].Steps = append([]PlaybookStep{decisionStep(decisionID, "Choose which disabled prerequisite to change explicitly", prerequisiteBlockers)}, definitions[i].Steps...)
			}
			blockMutationSteps(&definitions[i], definitions[i].BlockedBy, decisionID)
		}
	}
	return definitions
}

func addCLICommandSteps() []PlaybookStep {
	steps := []PlaybookStep{
		{ID: "create_command_file", Kind: "agent_edit", Effect: "repository_mutation", Summary: "Create the command implementation and register one stable command ID from the human-owned file", Paths: []string{"internal/cli/<command_name>.go"}, Argv: []string{}, DependsOn: []string{}, RequiresExplicitAuthority: true, SuccessCondition: "the package registers exactly one non-nil command with lowercase-kebab ID", BlockedBy: []string{}},
		{ID: "create_command_test", Kind: "agent_edit", Effect: "repository_mutation", Summary: "Create focused command tests including machine-readable output behavior", Paths: []string{"internal/cli/<command_name>_test.go"}, Argv: []string{}, DependsOn: []string{"create_command_file"}, RequiresExplicitAuthority: true, SuccessCondition: "tests cover command behavior and the global JSON contract where applicable", BlockedBy: []string{}},
		{ID: "run_repository_tests", Kind: "command", Effect: "subprocess_probe", Summary: "Run the repository test suite", Paths: []string{}, Argv: []string{"go", "test", "./..."}, DependsOn: []string{"create_command_test"}, RequiresExplicitAuthority: false, SuccessCondition: "the repository test suite passes", BlockedBy: []string{}},
	}
	steps = append(steps,
		PlaybookStep{ID: "review_bob_plan", Kind: "bob_plan", Effect: "read_only", Summary: "Confirm the human-owned extension did not introduce Bob ownership drift", Paths: []string{}, Argv: []string{"bob", "plan", "<workspace>", "--json"}, DependsOn: []string{"run_repository_tests"}, RequiresExplicitAuthority: false, SuccessCondition: "Bob reports no ownership conflict caused by the command files", BlockedBy: []string{}},
		PlaybookStep{ID: "check_bob_convergence", Kind: "bob_check", Effect: "read_only", Summary: "Check Bob repository convergence separately from command behavior", Paths: []string{}, Argv: []string{"bob", "check", "<workspace>", "--json"}, DependsOn: []string{"review_bob_plan"}, RequiresExplicitAuthority: false, SuccessCondition: "Bob reports clean repository state", BlockedBy: []string{}},
	)
	return steps
}

func addCLICommandVerificationHints(m manifest.Manifest) []string {
	hints := []string{"bob check --json proves repository convergence only", "go test ./... verifies repository tests but does not make Bob a behavioral evidence authority"}
	if m.Integrations.TerminalVerification == "glyphrun" {
		hints = append(hints, "Glyphrun is selected; add and run a command-specific terminal behavior spec")
	}
	return hints
}

func genericPlaybooks() []PlaybookDefinition {
	definitions := []PlaybookDefinition{conflictPlaybook(), upgradePlaybook()}
	definitions[1].CapabilityIDs = []string{"repository.whole_file_ownership"}
	return definitions
}

func togglePlaybook(id, title, manifestField string, enabled bool, generated []string) PlaybookDefinition {
	blocked := []string{}
	if enabled {
		blocked = []string{"capability_already_enabled"}
	}
	definition := PlaybookDefinition{
		ID: id, Title: title, Purpose: "Change one human-owned manifest selection and reconcile its declared artifacts.",
		Applicable: true, Available: !enabled, BlockedBy: blocked, ScopeClass: generatedScope(generated), Risk: "medium",
		Inputs: []PlaybookInputDefinition{}, Boundary: PlaybookBoundary{Create: append([]string(nil), generated...), Modify: []string{"bob.yaml"}, Forbidden: []string{"bob.lock"}},
		Steps: manifestToggleSteps(manifestField, generated), VerificationHints: []string{"bob check --json proves repository convergence only"},
		FailureModes: []string{"ownership conflicts block apply without overwriting human content"},
	}
	if len(blocked) > 0 {
		blockMutationSteps(&definition, blocked, "")
	}
	return definition
}

func generatedScope(paths []string) string {
	if len(paths) > 1 {
		return "multi_surface"
	}
	if len(paths) == 1 {
		return "small"
	}
	return "metadata_only"
}

// blockMutationSteps keeps an unavailable or currently blocked playbook from
// presenting repository mutation as immediately actionable. The read-only
// inspection steps remain useful guidance, but callers must re-resolve the
// playbook after its stable blockers have been cleared.
func blockMutationSteps(definition *PlaybookDefinition, blockers []string, decisionID string) {
	for i := range definition.Steps {
		step := &definition.Steps[i]
		if step.Effect != "repository_mutation" && step.Effect != "user_configuration_mutation" {
			continue
		}
		step.BlockedBy = uniqueSorted(append(step.BlockedBy, blockers...))
		if decisionID != "" && step.ID != decisionID {
			step.DependsOn = uniqueSorted(append(step.DependsOn, decisionID))
		}
	}
}

func withoutString(values []string, excluded string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != excluded {
			result = append(result, value)
		}
	}
	return result
}

func manifestToggleSteps(field string, generated []string) []PlaybookStep {
	return []PlaybookStep{
		{ID: "edit_manifest", Kind: "manifest_edit", Effect: "repository_mutation", Summary: "Change only " + field + " in bob.yaml", Paths: []string{"bob.yaml"}, Argv: []string{}, DependsOn: []string{}, RequiresExplicitAuthority: true, SuccessCondition: "the strict manifest validates", BlockedBy: []string{}},
		{ID: "review_plan", Kind: "bob_plan", Effect: "read_only", Summary: "Review the complete reconciliation plan", Paths: append([]string(nil), generated...), Argv: []string{"bob", "plan", "<workspace>", "--json"}, DependsOn: []string{"edit_manifest"}, RequiresExplicitAuthority: false, SuccessCondition: "the complete plan is understood and has no conflicts", BlockedBy: []string{}},
		{ID: "apply_plan", Kind: "bob_apply", Effect: "repository_mutation", Summary: "Apply only the exact reviewed Bob plan with explicit authority", Paths: append([]string(nil), generated...), Argv: []string{"bob", "apply", "<workspace>", "--expect-plan-digest", "<reviewed_plan_digest>", "--json"}, DependsOn: []string{"review_plan"}, RequiresExplicitAuthority: true, SuccessCondition: "Bob applies the reviewed digest and returns a reconciliation receipt", BlockedBy: []string{}},
		{ID: "check_convergence", Kind: "bob_check", Effect: "read_only", Summary: "Check repository convergence", Paths: []string{}, Argv: []string{"bob", "check", "<workspace>", "--json"}, DependsOn: []string{"apply_plan"}, RequiresExplicitAuthority: false, SuccessCondition: "Bob reports clean repository state", BlockedBy: []string{}},
	}
}

func conflictPlaybook() PlaybookDefinition {
	return PlaybookDefinition{
		ID: "resolve-ownership-conflict", Title: "Resolve an ownership conflict", Purpose: "Present bounded recovery choices for one exact planner conflict without performing them.",
		Applicable: true, Available: true, BlockedBy: []string{}, ScopeClass: "single_file", Risk: "high",
		Inputs: []PlaybookInputDefinition{
			{Name: "path", Required: true, Type: "repository_path", Validation: "safe-relative-path"},
			{Name: "action_code", Required: true, Type: "enum", Validation: "closed-enum", Enum: []string{"managed_hash_mismatch", "managed_missing", "retired_owned", "special_file", "symlink", "unmanaged_differs", "unmanaged_mode_differs"}},
		},
		Boundary: PlaybookBoundary{Create: []string{}, Modify: []string{"<path>"}, Forbidden: []string{"bob.lock"}},
		Steps: []PlaybookStep{
			{ID: "inspect_conflict", Kind: "inspect", Effect: "read_only", Summary: "Confirm the exact path and planner action code", Paths: []string{"<path>"}, Argv: []string{"bob", "path", "--workspace", "<workspace>", "--json", "--", "<path>"}, DependsOn: []string{}, RequiresExplicitAuthority: false, SuccessCondition: "the observed action code matches the resolved input", BlockedBy: []string{}},
			decisionStep("choose_intent", "Choose deliberately whether to keep human content, restore Bob-owned bytes, or change the manifest contract", []string{"human_intent_unproven"}),
		},
		VerificationHints: []string{"bob plan --json must be reviewed again after the decision", "bob check --json proves repository convergence only"},
		FailureModes:      []string{"Bob never deletes or overwrites the human version automatically"},
	}
}

func upgradePlaybook() PlaybookDefinition {
	return PlaybookDefinition{
		ID: "upgrade-recipe", Title: "Upgrade the active recipe", Purpose: "Review the difference between the locked and current built-in recipe without mutating first.",
		Applicable: true, Available: true, BlockedBy: []string{}, ScopeClass: "repository_wide", Risk: "high", Inputs: []PlaybookInputDefinition{},
		Boundary: PlaybookBoundary{Create: []string{}, Modify: []string{}, Forbidden: []string{"bob.lock"}},
		Steps: []PlaybookStep{
			{ID: "review_upgrade_plan", Kind: "bob_plan", Effect: "read_only", Summary: "Review new, changed, retired, and conflicting recipe artifacts", Paths: []string{}, Argv: []string{"bob", "plan", "<workspace>", "--json"}, DependsOn: []string{}, RequiresExplicitAuthority: false, SuccessCondition: "all recipe migration effects are understood", BlockedBy: []string{}},
			decisionStep("resolve_conflicts", "Resolve every ownership conflict before authorizing apply", []string{"ownership_conflicts"}),
			{ID: "apply_upgrade", Kind: "bob_apply", Effect: "repository_mutation", Summary: "Apply the exact reviewed conflict-free recipe upgrade with explicit authority", Paths: []string{}, Argv: []string{"bob", "apply", "<workspace>", "--expect-plan-digest", "<reviewed_plan_digest>", "--json"}, DependsOn: []string{"review_upgrade_plan", "resolve_conflicts"}, RequiresExplicitAuthority: true, SuccessCondition: "Bob applies the reviewed digest, publishes the recipe update, and writes the lock last", BlockedBy: []string{}},
			{ID: "check_convergence", Kind: "bob_check", Effect: "read_only", Summary: "Check repository convergence after the upgrade", Paths: []string{}, Argv: []string{"bob", "check", "<workspace>", "--json"}, DependsOn: []string{"apply_upgrade"}, RequiresExplicitAuthority: false, SuccessCondition: "Bob reports clean repository state", BlockedBy: []string{}},
		},
		VerificationHints: []string{"bob check --json proves repository convergence only"}, FailureModes: []string{"retired paths are never deleted automatically", "hand-edited managed files remain conflicts"},
	}
}

func decisionStep(id, summary string, blocked []string) PlaybookStep {
	return PlaybookStep{ID: id, Kind: "human_decision", Effect: "read_only", Summary: summary, Paths: []string{}, Argv: []string{}, DependsOn: []string{}, RequiresExplicitAuthority: true, SuccessCondition: "a human-owned intent is selected explicitly", BlockedBy: blocked}
}

func homebrewBlockers(m manifest.Manifest) []string {
	blockers := []string{}
	if m.Product.Visibility != "public" {
		blockers = append(blockers, "public_visibility_required")
	}
	if !m.Distribution.GoReleaser {
		blockers = append(blockers, "goreleaser_required")
	}
	if !m.Distribution.GitHubActions {
		blockers = append(blockers, "github_actions_required")
	}
	if len(m.Product.Module) < len("github.com/") || m.Product.Module[:len("github.com/")] != "github.com/" {
		blockers = append(blockers, "github_module_required")
	}
	if m.Distribution.Homebrew {
		blockers = append(blockers, "capability_already_enabled")
	}
	return blockers
}
