package recipe

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

const MetadataSchemaVersion = 1

type Metadata struct {
	SchemaVersion   int                        `json:"schema_version"`
	Recipe          MetadataRecipeRef          `json:"recipe"`
	Summary         string                     `json:"summary"`
	Capabilities    []CapabilityDefinition     `json:"capabilities"`
	Artifacts       []ArtifactDescriptor       `json:"artifacts"`
	Invariants      []InvariantDefinition      `json:"invariants"`
	ExtensionPoints []ExtensionPointDefinition `json:"extension_points"`
	Playbooks       []PlaybookDefinition       `json:"playbooks"`
}

type MetadataRecipeRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type CapabilityDefinition struct {
	ID             string   `json:"id"`
	Category       string   `json:"category"`
	Selection      string   `json:"selection"`
	Summary        string   `json:"summary"`
	ManifestFields []string `json:"manifest_fields"`
	ArtifactIDs    []string `json:"artifact_ids"`
	Binary         string   `json:"binary,omitempty"`
	Limitations    []string `json:"limitations"`
}

type ArtifactDescriptor struct {
	ID            string   `json:"id"`
	Path          string   `json:"path"`
	Roles         []string `json:"roles"`
	Ownership     string   `json:"ownership"`
	CapabilityIDs []string `json:"capability_ids"`
}

type InvariantDefinition struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
}

type ExtensionPointDefinition struct {
	ID             string   `json:"id"`
	Purpose        string   `json:"purpose"`
	Ownership      string   `json:"ownership"`
	CreatePatterns []string `json:"create_patterns"`
	ForbiddenPaths []string `json:"forbidden_paths"`
	CapabilityIDs  []string `json:"capability_ids"`
	PlaybookIDs    []string `json:"playbook_ids"`
}

type PlaybookSummary struct {
	ID             string   `json:"id"`
	Title          string   `json:"title,omitempty"`
	Applicable     bool     `json:"applicable"`
	Available      bool     `json:"available"`
	BlockedBy      []string `json:"blocked_by"`
	RequiredInputs []string `json:"required_inputs"`
	ScopeClass     string   `json:"scope_class"`
	Risk           string   `json:"risk"`
}

type PlaybookDefinition struct {
	ID                string                    `json:"id"`
	Title             string                    `json:"title"`
	Purpose           string                    `json:"purpose"`
	Applicable        bool                      `json:"applicable"`
	Available         bool                      `json:"available"`
	BlockedBy         []string                  `json:"blocked_by"`
	ScopeClass        string                    `json:"scope_class"`
	Risk              string                    `json:"risk"`
	Inputs            []PlaybookInputDefinition `json:"inputs"`
	Preconditions     []string                  `json:"preconditions"`
	Boundary          PlaybookBoundary          `json:"boundary"`
	Steps             []PlaybookStep            `json:"steps"`
	VerificationHints []string                  `json:"verification_hints"`
	FailureModes      []string                  `json:"failure_modes"`
	CapabilityIDs     []string                  `json:"capability_ids"`
	ExtensionPointIDs []string                  `json:"extension_point_ids"`
}

type PlaybookInputDefinition struct {
	Name       string   `json:"name"`
	Required   bool     `json:"required"`
	Type       string   `json:"type"`
	Validation string   `json:"validation"`
	Enum       []string `json:"enum,omitempty"`
	Forbidden  []string `json:"forbidden,omitempty"`
}

type PlaybookBoundary struct {
	Create    []string `json:"create"`
	Modify    []string `json:"modify"`
	Forbidden []string `json:"forbidden"`
}

type PlaybookStep struct {
	ID                        string   `json:"id"`
	Kind                      string   `json:"kind"`
	Effect                    string   `json:"effect"`
	Summary                   string   `json:"summary"`
	Paths                     []string `json:"paths"`
	Argv                      []string `json:"argv"`
	DependsOn                 []string `json:"depends_on"`
	RequiresExplicitAuthority bool     `json:"requires_explicit_authority"`
	SuccessCondition          string   `json:"success_condition"`
	BlockedBy                 []string `json:"blocked_by"`
}

// ResolveMetadata deterministically describes the current built-in recipe
// contract for a validated manifest. It renders only to prove that every
// descriptor names a real artifact; it never observes the workspace.
func ResolveMetadata(m manifest.Manifest) (Metadata, error) {
	if err := m.Validate(); err != nil {
		return Metadata{}, fmt.Errorf("resolve metadata: %w", err)
	}
	artifacts, err := Render(m)
	if err != nil {
		return Metadata{}, fmt.Errorf("resolve metadata: %w", err)
	}
	var metadata Metadata
	switch {
	case m.Recipe == manifest.RecipeGoAgentTool:
		metadata = resolveGoAgentMetadata(m, artifacts)
	case m.Recipe == manifest.RecipeFiles:
		metadata = resolveFilesMetadata(m, artifacts)
	case manifest.IsStackRecipe(m.Recipe):
		metadata = resolveStackMetadata(m, artifacts)
	default:
		return Metadata{}, fmt.Errorf("resolve metadata: unsupported recipe %q", m.Recipe)
	}
	sortMetadata(&metadata)
	if err := ValidateMetadata(metadata, artifacts); err != nil {
		return Metadata{}, fmt.Errorf("resolve metadata: %w", err)
	}
	return metadata, nil
}

func resolveGoAgentMetadata(m manifest.Manifest, artifacts []Artifact) Metadata {
	metadata := Metadata{
		SchemaVersion: MetadataSchemaVersion,
		Recipe:        MetadataRecipeRef{ID: m.Recipe, Version: goAgentToolRecipeVersion},
		Summary:       "Go and Cobra CLI repository with whole-file ownership, deterministic human command extensions, documentation, CI, release, and optional local-tool seams.",
		Playbooks:     goAgentPlaybooks(m),
	}
	definitions := []CapabilityDefinition{
		capability("distribution.github_actions", "distribution", selectedBool(m.Distribution.GitHubActions), "GitHub Actions workflow generation", []string{"distribution.github_actions"}, ""),
		capability("distribution.goreleaser", "distribution", selectedBool(m.Distribution.GoReleaser), "GoReleaser configuration and release workflow generation", []string{"distribution.goreleaser"}, "goreleaser"),
		capability("distribution.homebrew", "distribution", selectedBool(m.Distribution.Homebrew), "Homebrew release packaging in the GoReleaser contract", []string{"distribution.homebrew"}, ""),
		capability("docs.markdown", "documentation", selectedChoice(m.Distribution.Docs, "markdown"), "Generated Markdown documentation", []string{"distribution.docs"}, ""),
		capability("integration.cairntrace", "integration", selectedChoice(m.Integrations.BrowserVerification, "cairntrace"), "Declared browser-verification seam", []string{"integrations.browser_verification"}, "cairn"),
		capability("integration.codemap", "integration", selectedChoice(m.Integrations.CodeStructure, "codemap"), "Declared structural-code seam", []string{"integrations.code_structure"}, "codemap"),
		capability("integration.fcheap", "integration", selectedChoice(m.Integrations.Artifacts, "fcheap"), "Declared artifact-store seam", []string{"integrations.artifacts"}, "fcheap"),
		capability("integration.glyphrun", "integration", selectedChoice(m.Integrations.TerminalVerification, "glyphrun"), "Declared terminal-verification seam and optional specs", []string{"integrations.terminal_verification"}, "glyph"),
		capability("integration.tinyvault", "integration", selectedChoice(m.Integrations.Secrets, "tinyvault"), "Declared secret-broker seam", []string{"integrations.secrets"}, "tvault"),
		capability("integration.vecgrep", "integration", selectedChoice(m.Integrations.SemanticSearch, "vecgrep"), "Declared semantic-search seam", []string{"integrations.semantic_search"}, "vecgrep"),
		capability("repository.public_hygiene", "repository", "required", "Public repository policy and contribution files", []string{"product.visibility", "product.license"}, ""),
		capability("repository.whole_file_ownership", "repository", "required", "Content-hash ownership of complete recipe artifacts", []string{"recipe"}, ""),
		capability("surface.cli", "surface", "required", "Cobra command-line entry point", []string{"surfaces.cli"}, ""),
		capability("surface.json", "surface", "required", "Machine-readable command output", []string{"surfaces.json"}, ""),
	}
	byID := make(map[string]*CapabilityDefinition, len(definitions))
	for i := range definitions {
		byID[definitions[i].ID] = &definitions[i]
	}
	for _, artifact := range artifacts {
		descriptor := describeGoAgentArtifact(artifact.Path, m)
		metadata.Artifacts = append(metadata.Artifacts, descriptor)
		for _, capabilityID := range descriptor.CapabilityIDs {
			definition := byID[capabilityID]
			definition.ArtifactIDs = append(definition.ArtifactIDs, descriptor.ID)
		}
	}
	metadata.Capabilities = definitions
	metadata.Invariants = []InvariantDefinition{
		{ID: "cli.stdout.machine_clean", Statement: "Machine-readable stdout remains valid JSON; diagnostics belong on stderr."},
		{ID: "cli.command_extensions_v4", Statement: "go-agent-tool@4 commands register from human-owned package files without editing Bob-owned root or registry files."},
		{ID: "repository.no_unmanaged_overwrite", Statement: "Bob never overwrites an unmanaged differing file."},
		{ID: "repository.whole_file_updates", Statement: "A managed file updates only while its current hash matches bob.lock."},
		{ID: "verification.not_implied", Statement: "Selection, materialization, and binary availability do not prove application behavior."},
	}
	metadata.ExtensionPoints = []ExtensionPointDefinition{
		{
			ID: "cli.command_files", Purpose: "Add Cobra commands through deterministic registration without modifying Bob-owned composition files", Ownership: "human",
			CreatePatterns: []string{"internal/cli/<command>.go", "internal/cli/<command>_test.go"},
			ForbiddenPaths: []string{"internal/cli/registry.go", "internal/cli/registry_test.go", "internal/cli/root.go", "internal/cli/root_test.go"},
			CapabilityIDs:  []string{"surface.cli", "surface.json"}, PlaybookIDs: []string{"add-cli-command"},
		},
		{
			ID: "domain.packages", Purpose: "Add human-owned domain behavior outside Bob-owned composition files", Ownership: "human",
			CreatePatterns: []string{"internal/<package>/<file>.go", "internal/<package>/<file>_test.go"},
			ForbiddenPaths: []string{"internal/cli/registry.go", "internal/cli/registry_test.go", "internal/cli/root.go", "internal/cli/root_test.go"}, CapabilityIDs: []string{"surface.cli"}, PlaybookIDs: []string{},
		},
	}
	if m.Integrations.TerminalVerification == "glyphrun" {
		metadata.ExtensionPoints = append(metadata.ExtensionPoints, ExtensionPointDefinition{
			ID: "terminal.additional_specs", Purpose: "Add human-owned terminal behavior specs beside the Bob-owned help spec", Ownership: "human",
			CreatePatterns: []string{"specs/<spec>.yml"}, ForbiddenPaths: []string{"specs/help.yml"},
			CapabilityIDs: []string{"integration.glyphrun"}, PlaybookIDs: []string{},
		})
	}
	return metadata
}

func resolveFilesMetadata(m manifest.Manifest, artifacts []Artifact) Metadata {
	metadata := Metadata{
		SchemaVersion: MetadataSchemaVersion,
		Recipe:        MetadataRecipeRef{ID: m.Recipe, Version: FilesRecipeVersion},
		Summary:       "Manifest-declared file tree with deterministic substitution and whole-file ownership.",
		Capabilities: []CapabilityDefinition{
			capability("repository.declared_file_tree", "repository", "required", "Manifest-declared files", []string{"files", "vars"}, ""),
			capability("repository.whole_file_ownership", "repository", "required", "Content-hash ownership of complete recipe artifacts", []string{"recipe"}, ""),
		},
		Invariants: []InvariantDefinition{
			{ID: "files.content_semantics_unknown", Statement: "Bob owns declared bytes and does not infer application semantics from paths or content."},
			{ID: "repository.no_unmanaged_overwrite", Statement: "Bob never overwrites an unmanaged differing file."},
			{ID: "repository.whole_file_updates", Statement: "A managed file updates only while its current hash matches bob.lock."},
		},
		ExtensionPoints: []ExtensionPointDefinition{}, Playbooks: genericPlaybooks(),
	}
	for _, artifact := range artifacts {
		id := "files:" + artifact.Path
		metadata.Artifacts = append(metadata.Artifacts, ArtifactDescriptor{
			ID: id, Path: artifact.Path, Roles: []string{"declared_file"}, Ownership: "bob_whole_file",
			CapabilityIDs: []string{"repository.declared_file_tree", "repository.whole_file_ownership"},
		})
		for i := range metadata.Capabilities {
			metadata.Capabilities[i].ArtifactIDs = append(metadata.Capabilities[i].ArtifactIDs, id)
		}
	}
	return metadata
}

func capability(id, category, selection, summary string, fields []string, binary string) CapabilityDefinition {
	limitations := []string{}
	if category == "integration" {
		limitations = []string{"Bob does not initialize, query, or verify this specialist tool", "binary availability does not imply usable specialist state"}
	}
	return CapabilityDefinition{ID: id, Category: category, Selection: selection, Summary: summary, ManifestFields: fields, ArtifactIDs: []string{}, Binary: binary, Limitations: limitations}
}

func selectedBool(value bool) string {
	if value {
		return "enabled"
	}
	return "disabled"
}

func selectedChoice(value, enabled string) string {
	if value == enabled {
		return "enabled"
	}
	return "disabled"
}

func describeGoAgentArtifact(path string, m manifest.Manifest) ArtifactDescriptor {
	id := goAgentArtifactID(path)
	roles := []string{"repository_support"}
	capabilities := []string{"repository.whole_file_ownership"}
	switch path {
	case "AGENTS.md":
		roles = []string{"agent_guidance"}
		capabilities = append(capabilities, selectedIntegrationCapabilityIDs(m)...)
	case "README.md", "CHANGELOG.md", "CODE_OF_CONDUCT.md", "CONTRIBUTING.md", "LICENSE", "SECURITY.md", "CLAUDE.md":
		roles = []string{"public_hygiene"}
		capabilities = append(capabilities, "repository.public_hygiene")
	case "cmd/" + m.Product.Name + "/main.go":
		roles = []string{"entrypoint"}
		capabilities = append(capabilities, "surface.cli")
	case "internal/cli/root.go":
		roles = []string{"cli", "composition_root"}
		capabilities = append(capabilities, "surface.cli", "surface.json")
		capabilities = append(capabilities, selectedIntegrationCapabilityIDs(m)...)
	case "internal/cli/root_test.go":
		roles = []string{"cli_test"}
		capabilities = append(capabilities, "surface.cli", "surface.json")
	case "internal/cli/registry.go":
		roles = []string{"cli", "extension_registry"}
		capabilities = append(capabilities, "surface.cli", "surface.json")
	case "internal/cli/registry_test.go":
		roles = []string{"cli_test", "extension_registry_test"}
		capabilities = append(capabilities, "surface.cli", "surface.json")
	case ".github/workflows/ci.yml":
		roles = []string{"ci_workflow"}
		capabilities = append(capabilities, "distribution.github_actions")
	case ".github/workflows/release.yml":
		roles = []string{"release_workflow"}
		capabilities = append(capabilities, "distribution.github_actions", "distribution.goreleaser")
	case ".goreleaser.yaml":
		roles = []string{"release_config"}
		capabilities = append(capabilities, "distribution.goreleaser")
		if m.Distribution.Homebrew {
			capabilities = append(capabilities, "distribution.homebrew")
		}
	case "docs/index.md":
		roles = []string{"documentation"}
		capabilities = append(capabilities, "docs.markdown")
	case "glyphrun.config.yml":
		roles = []string{"terminal_config"}
		capabilities = append(capabilities, "integration.glyphrun")
	case "specs/help.yml":
		roles = []string{"terminal_spec"}
		capabilities = append(capabilities, "integration.glyphrun")
	}
	if strings.HasPrefix(path, ".github/") && !strings.HasPrefix(path, ".github/workflows/") {
		capabilities = append(capabilities, "repository.public_hygiene")
	}
	return ArtifactDescriptor{ID: id, Path: path, Roles: uniqueSorted(roles), Ownership: "bob_whole_file", CapabilityIDs: uniqueSorted(capabilities)}
}

func goAgentArtifactID(path string) string {
	known := map[string]string{
		".gitignore": "repo.gitignore", ".golangci.yml": "go.lint_config", ".goreleaser.yaml": "release.goreleaser",
		"AGENTS.md": "repo.agents", "CHANGELOG.md": "repo.changelog", "CLAUDE.md": "repo.claude",
		"CODE_OF_CONDUCT.md": "repo.code_of_conduct", "CONTRIBUTING.md": "repo.contributing", "LICENSE": "repo.license",
		"README.md": "repo.readme", "SECURITY.md": "repo.security", "Taskfile.yml": "repo.taskfile",
		"go.mod": "go.module", "go.sum": "go.sum", "internal/cli/root.go": "cli.root",
		"internal/cli/root_test.go": "cli.root_tests", "internal/cli/registry.go": "cli.registry",
		"internal/cli/registry_test.go": "cli.registry_tests", "internal/version/version.go": "version.package",
		".github/workflows/ci.yml": "ci.workflow", ".github/workflows/release.yml": "release.workflow",
		".github/ISSUE_TEMPLATE/bug.yml": "github.issue_bug", ".github/ISSUE_TEMPLATE/config.yml": "github.issue_config",
		".github/ISSUE_TEMPLATE/feature.yml": "github.issue_feature", ".github/dependabot.yml": "github.dependabot",
		".github/pull_request_template.md": "github.pull_request_template", "docs/index.md": "docs.index",
		"glyphrun.config.yml": "terminal.glyphrun_config", "specs/help.yml": "terminal.help_spec",
	}
	if id, ok := known[path]; ok {
		return id
	}
	if strings.HasPrefix(path, "cmd/") && strings.HasSuffix(path, "/main.go") {
		return "cli.entrypoint"
	}
	return "artifact:" + path
}

func selectedIntegrationCapabilityIDs(m manifest.Manifest) []string {
	var ids []string
	if m.Integrations.CodeStructure == "codemap" {
		ids = append(ids, "integration.codemap")
	}
	if m.Integrations.SemanticSearch == "vecgrep" {
		ids = append(ids, "integration.vecgrep")
	}
	if m.Integrations.TerminalVerification == "glyphrun" {
		ids = append(ids, "integration.glyphrun")
	}
	if m.Integrations.BrowserVerification == "cairntrace" {
		ids = append(ids, "integration.cairntrace")
	}
	if m.Integrations.Secrets == "tinyvault" {
		ids = append(ids, "integration.tinyvault")
	}
	if m.Integrations.Artifacts == "fcheap" {
		ids = append(ids, "integration.fcheap")
	}
	return ids
}

func sortMetadata(metadata *Metadata) {
	sort.Slice(metadata.Capabilities, func(i, j int) bool { return metadata.Capabilities[i].ID < metadata.Capabilities[j].ID })
	for i := range metadata.Capabilities {
		metadata.Capabilities[i].ManifestFields = uniqueSorted(metadata.Capabilities[i].ManifestFields)
		metadata.Capabilities[i].ArtifactIDs = uniqueSorted(metadata.Capabilities[i].ArtifactIDs)
		metadata.Capabilities[i].Limitations = uniqueSorted(metadata.Capabilities[i].Limitations)
	}
	sort.Slice(metadata.Artifacts, func(i, j int) bool { return metadata.Artifacts[i].ID < metadata.Artifacts[j].ID })
	for i := range metadata.Artifacts {
		metadata.Artifacts[i].Roles = uniqueSorted(metadata.Artifacts[i].Roles)
		metadata.Artifacts[i].CapabilityIDs = uniqueSorted(metadata.Artifacts[i].CapabilityIDs)
	}
	sort.Slice(metadata.Invariants, func(i, j int) bool { return metadata.Invariants[i].ID < metadata.Invariants[j].ID })
	sort.Slice(metadata.ExtensionPoints, func(i, j int) bool { return metadata.ExtensionPoints[i].ID < metadata.ExtensionPoints[j].ID })
	for i := range metadata.ExtensionPoints {
		metadata.ExtensionPoints[i].CreatePatterns = uniqueSorted(metadata.ExtensionPoints[i].CreatePatterns)
		metadata.ExtensionPoints[i].ForbiddenPaths = uniqueSorted(metadata.ExtensionPoints[i].ForbiddenPaths)
		metadata.ExtensionPoints[i].CapabilityIDs = uniqueSorted(metadata.ExtensionPoints[i].CapabilityIDs)
		metadata.ExtensionPoints[i].PlaybookIDs = uniqueSorted(metadata.ExtensionPoints[i].PlaybookIDs)
	}
	sort.Slice(metadata.Playbooks, func(i, j int) bool { return metadata.Playbooks[i].ID < metadata.Playbooks[j].ID })
	for i := range metadata.Playbooks {
		playbook := &metadata.Playbooks[i]
		playbook.BlockedBy = uniqueSorted(playbook.BlockedBy)
		playbook.Preconditions = uniqueSorted(playbook.Preconditions)
		playbook.Boundary.Create = uniqueSorted(playbook.Boundary.Create)
		playbook.Boundary.Modify = uniqueSorted(playbook.Boundary.Modify)
		playbook.Boundary.Forbidden = uniqueSorted(playbook.Boundary.Forbidden)
		playbook.VerificationHints = uniqueSorted(playbook.VerificationHints)
		playbook.FailureModes = uniqueSorted(playbook.FailureModes)
		playbook.CapabilityIDs = uniqueSorted(playbook.CapabilityIDs)
		playbook.ExtensionPointIDs = uniqueSorted(playbook.ExtensionPointIDs)
		for j := range playbook.Inputs {
			playbook.Inputs[j].Enum = uniqueSorted(playbook.Inputs[j].Enum)
			playbook.Inputs[j].Forbidden = uniqueSorted(playbook.Inputs[j].Forbidden)
		}
	}
}

func uniqueSorted(values []string) []string {
	if values == nil {
		return []string{}
	}
	// Preserve an explicit empty list in public metadata. A nil slice encodes
	// as JSON null, which would make callers branch on two representations for
	// the same closed-list state.
	out := append([]string{}, values...)
	sort.Strings(out)
	result := out[:0]
	for _, value := range out {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}

// ValidateMetadata proves IDs, paths, and cross-references against the exact
// rendered artifact set. It is exported so recipe contract tests can exercise
// malformed metadata without adding a second validation implementation.
func ValidateMetadata(metadata Metadata, rendered []Artifact) error {
	if metadata.SchemaVersion != MetadataSchemaVersion {
		return fmt.Errorf("unsupported metadata schema version %d", metadata.SchemaVersion)
	}
	if metadata.Recipe.ID == "" || metadata.Recipe.Version < 1 {
		return fmt.Errorf("metadata recipe identity is invalid")
	}
	artifactPaths := make(map[string]struct{}, len(rendered))
	for _, artifact := range rendered {
		artifactPaths[filepath.ToSlash(filepath.Clean(artifact.Path))] = struct{}{}
	}
	capabilities := map[string]struct{}{}
	allIDs := map[string]struct{}{}
	for _, item := range metadata.Capabilities {
		if err := addMetadataID(capabilities, item.ID, "capability"); err != nil {
			return err
		}
		if err := addMetadataID(allIDs, item.ID, "metadata"); err != nil {
			return err
		}
	}
	artifacts := map[string]struct{}{}
	paths := map[string]struct{}{}
	for _, item := range metadata.Artifacts {
		if err := addMetadataID(artifacts, item.ID, "artifact"); err != nil {
			return err
		}
		if err := addMetadataID(allIDs, item.ID, "metadata"); err != nil {
			return err
		}
		path, err := safePath(item.Path)
		if err != nil {
			return fmt.Errorf("artifact %q: %w", item.ID, err)
		}
		if _, duplicate := paths[path]; duplicate {
			return fmt.Errorf("duplicate metadata artifact path %q", path)
		}
		paths[path] = struct{}{}
		if _, ok := artifactPaths[path]; !ok {
			return fmt.Errorf("artifact %q references unrendered path %q", item.ID, path)
		}
		for _, id := range item.CapabilityIDs {
			if _, ok := capabilities[id]; !ok {
				return fmt.Errorf("artifact %q references unknown capability %q", item.ID, id)
			}
		}
	}
	if len(paths) != len(artifactPaths) {
		for path := range artifactPaths {
			if _, ok := paths[path]; !ok {
				return fmt.Errorf("rendered artifact %q has no metadata descriptor", path)
			}
		}
	}
	for _, item := range metadata.Capabilities {
		for _, id := range item.ArtifactIDs {
			if _, ok := artifacts[id]; !ok {
				return fmt.Errorf("capability %q references unknown artifact %q", item.ID, id)
			}
		}
	}
	invariants := map[string]struct{}{}
	for _, item := range metadata.Invariants {
		if err := addMetadataID(invariants, item.ID, "invariant"); err != nil {
			return err
		}
		if err := addMetadataID(allIDs, item.ID, "metadata"); err != nil {
			return err
		}
	}
	playbooks := map[string]struct{}{}
	for _, item := range metadata.Playbooks {
		if err := addMetadataID(playbooks, item.ID, "playbook"); err != nil {
			return err
		}
		if err := addMetadataID(allIDs, item.ID, "metadata"); err != nil {
			return err
		}
		inputIDs := map[string]struct{}{}
		for _, input := range item.Inputs {
			if err := addMetadataID(inputIDs, input.Name, "playbook input"); err != nil {
				return fmt.Errorf("playbook %q: %w", item.ID, err)
			}
		}
		for _, id := range item.CapabilityIDs {
			if _, ok := capabilities[id]; !ok {
				return fmt.Errorf("playbook %q references unknown capability %q", item.ID, id)
			}
		}
		if !containsString([]string{"metadata_only", "single_file", "small", "multi_surface", "repository_wide"}, item.ScopeClass) {
			return fmt.Errorf("playbook %q has unsupported scope_class %q", item.ID, item.ScopeClass)
		}
		if !containsString([]string{"low", "medium", "high"}, item.Risk) {
			return fmt.Errorf("playbook %q has unsupported risk %q", item.ID, item.Risk)
		}
		for _, path := range append(append(append([]string{}, item.Boundary.Create...), item.Boundary.Modify...), item.Boundary.Forbidden...) {
			if err := validateGuidancePathTemplate(path); err != nil {
				return fmt.Errorf("playbook %q: %w", item.ID, err)
			}
		}
		steps := map[string]struct{}{}
		for _, step := range item.Steps {
			if err := addMetadataID(steps, step.ID, "playbook step"); err != nil {
				return fmt.Errorf("playbook %q: %w", item.ID, err)
			}
			if !containsString([]string{"inspect", "agent_edit", "manifest_edit", "command", "bob_plan", "bob_apply", "bob_check", "human_decision"}, step.Kind) {
				return fmt.Errorf("playbook %q step %q has unsupported kind %q", item.ID, step.ID, step.Kind)
			}
			if !containsString([]string{"read_only", "subprocess_probe", "repository_mutation", "user_configuration_mutation"}, step.Effect) {
				return fmt.Errorf("playbook %q step %q has unsupported effect %q", item.ID, step.ID, step.Effect)
			}
			for _, path := range step.Paths {
				if err := validateGuidancePathTemplate(path); err != nil {
					return fmt.Errorf("playbook %q step %q: %w", item.ID, step.ID, err)
				}
			}
		}
		for _, step := range item.Steps {
			for _, dependency := range step.DependsOn {
				if _, ok := steps[dependency]; !ok {
					return fmt.Errorf("playbook %q step %q depends on unknown step %q", item.ID, step.ID, dependency)
				}
			}
		}
	}
	extensions := map[string]struct{}{}
	for _, item := range metadata.ExtensionPoints {
		if err := addMetadataID(extensions, item.ID, "extension point"); err != nil {
			return err
		}
		if err := addMetadataID(allIDs, item.ID, "metadata"); err != nil {
			return err
		}
		for _, pattern := range item.CreatePatterns {
			if err := validatePathTemplate(pattern); err != nil {
				return fmt.Errorf("extension point %q: %w", item.ID, err)
			}
		}
		for _, path := range item.ForbiddenPaths {
			if _, err := safePath(path); err != nil {
				return fmt.Errorf("extension point %q: %w", item.ID, err)
			}
		}
		for _, id := range item.CapabilityIDs {
			if _, ok := capabilities[id]; !ok {
				return fmt.Errorf("extension point %q references unknown capability %q", item.ID, id)
			}
		}
		for _, id := range item.PlaybookIDs {
			if _, ok := playbooks[id]; !ok {
				return fmt.Errorf("extension point %q references unknown playbook %q", item.ID, id)
			}
		}
	}
	for _, item := range metadata.Playbooks {
		for _, id := range item.ExtensionPointIDs {
			if _, ok := extensions[id]; !ok {
				return fmt.Errorf("playbook %q references unknown extension point %q", item.ID, id)
			}
		}
	}
	return nil
}

func addMetadataID(seen map[string]struct{}, id, kind string) error {
	if strings.TrimSpace(id) == "" {
		return fmt.Errorf("%s id is empty", kind)
	}
	if _, duplicate := seen[id]; duplicate {
		return fmt.Errorf("duplicate %s id %q", kind, id)
	}
	seen[id] = struct{}{}
	return nil
}

var pathTemplatePlaceholder = regexp.MustCompile(`<[^/<>]+>`)

func validatePathTemplate(pattern string) error {
	resolved := pathTemplatePlaceholder.ReplaceAllString(pattern, "placeholder")
	resolved = strings.ReplaceAll(resolved, "*", "file")
	if strings.ContainsAny(resolved, "<>") {
		return fmt.Errorf("invalid path template %q", pattern)
	}
	if _, err := safePath(resolved); err != nil {
		return fmt.Errorf("invalid path template %q: %w", pattern, err)
	}
	return nil
}

func validateGuidancePathTemplate(pattern string) error {
	resolved := pathTemplatePlaceholder.ReplaceAllString(pattern, "placeholder")
	if strings.ContainsAny(resolved, "<>\x00") || filepath.IsAbs(resolved) || filepath.VolumeName(resolved) != "" {
		return fmt.Errorf("invalid guidance path template %q", pattern)
	}
	clean := filepath.ToSlash(filepath.Clean(resolved))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || clean == ".git" || strings.HasPrefix(clean, ".git/") {
		return fmt.Errorf("invalid guidance path template %q", pattern)
	}
	return nil
}

func containsString(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
