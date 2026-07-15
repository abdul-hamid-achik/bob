// Package playbook resolves Bob's closed, recipe-versioned procedural guides.
// It never executes a step or mutates the repository.
package playbook

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/guidance"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/strsim"
	"github.com/abdul-hamid-achik/bob/internal/workspace"
)

const SchemaVersion = 1

type ListResult struct {
	SchemaVersion int                      `json:"schema_version"`
	Workspace     string                   `json:"workspace"`
	Recipe        recipe.MetadataRecipeRef `json:"recipe"`
	Playbooks     []recipe.PlaybookSummary `json:"playbooks"`
	Truncation    guidance.Truncation      `json:"truncation"`
}

type ShowResult struct {
	SchemaVersion    int                       `json:"schema_version"`
	Workspace        string                    `json:"workspace"`
	Recipe           recipe.MetadataRecipeRef  `json:"recipe"`
	RecipeIdentities *RecipeIdentities         `json:"recipe_identities,omitempty"`
	Observations     []Observation             `json:"observations"`
	Playbook         recipe.PlaybookDefinition `json:"playbook"`
	Truncation       guidance.Truncation       `json:"truncation"`
}

type Guide struct {
	SchemaVersion    int                       `json:"schema_version"`
	Workspace        string                    `json:"workspace"`
	Recipe           recipe.MetadataRecipeRef  `json:"recipe"`
	RecipeIdentities *RecipeIdentities         `json:"recipe_identities,omitempty"`
	Observations     []Observation             `json:"observations"`
	Playbook         recipe.PlaybookDefinition `json:"playbook"`
	Values           map[string]string         `json:"values"`
	Truncation       guidance.Truncation       `json:"truncation"`
}

// RecipeIdentities distinguishes the lock Bob observed from the current
// built-in contract. Observed is nil when the workspace has no lock.
type RecipeIdentities struct {
	Observed *recipe.MetadataRecipeRef `json:"observed,omitempty"`
	Current  recipe.MetadataRecipeRef  `json:"current"`
}

// Observation is a stable, machine-readable fact that is useful while
// resolving a playbook but is intentionally not recipe metadata. In
// particular, offline binary availability belongs here, never in the
// deterministic recipe contract.
type Observation struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

type Options struct {
	LookPath func(string) (string, error)
}

type serviceState struct {
	root     string
	manifest manifest.Manifest
	metadata recipe.Metadata
	plan     engine.PlanResult
}

// Summaries projects the same dynamic availability used by list/show/plan
// from an already coherent metadata and plan snapshot. Context uses this to
// avoid re-planning or drifting from the playbook service's blocker rules.
func Summaries(metadata recipe.Metadata, plan engine.PlanResult) []recipe.PlaybookSummary {
	return recipe.PlaybookSummaries(adjustedDefinitions(serviceState{metadata: metadata, plan: plan}))
}

func List(root string) (ListResult, error) {
	state, err := load(root)
	if err != nil {
		return ListResult{}, err
	}
	result := ListResult{SchemaVersion: SchemaVersion, Workspace: state.root, Recipe: state.metadata.Recipe, Playbooks: Summaries(state.metadata, state.plan), Truncation: noTruncation("list", 8<<10)}
	if err := enforceBound(result, 8<<10); err != nil {
		return ListResult{}, guidance.WithErrorCode("playbook_failed", err)
	}
	return result, nil
}

func Show(root, id string) (ShowResult, error) {
	return ShowWithOptions(root, id, Options{})
}

func ShowWithOptions(root, id string, options Options) (ShowResult, error) {
	state, err := load(root)
	if err != nil {
		return ShowResult{}, err
	}
	definition, err := find(adjustedDefinitions(state), id)
	if err != nil {
		return ShowResult{}, guidance.WithErrorCode(guidance.ErrorInputInvalid, err)
	}
	result := ShowResult{
		SchemaVersion: SchemaVersion, Workspace: state.root, Recipe: state.metadata.Recipe,
		RecipeIdentities: recipeIdentityObservation(state, id), Observations: playbookObservations(state, id, options),
		Playbook: definition, Truncation: noTruncation("show", 24<<10),
	}
	if err := truncateShow(&result); err != nil {
		return ShowResult{}, guidance.WithErrorCode("playbook_failed", err)
	}
	return result, nil
}

func Plan(root, id string, values map[string]string) (Guide, error) {
	return PlanWithOptions(root, id, values, Options{})
}

func PlanWithOptions(root, id string, values map[string]string, options Options) (Guide, error) {
	state, err := load(root)
	if err != nil {
		return Guide{}, err
	}
	definition, err := find(adjustedDefinitions(state), id)
	if err != nil {
		return Guide{}, guidance.WithErrorCode(guidance.ErrorInputInvalid, err)
	}
	validated, err := validateInputs(definition, values)
	if err != nil {
		return Guide{}, guidance.WithErrorCode(guidance.ErrorInputInvalid, err)
	}
	if id == "resolve-ownership-conflict" {
		classification, err := engine.ClassifyPath(state.root, state.plan, validated["path"])
		if err != nil {
			return Guide{}, guidance.WithErrorCode("playbook_failed", err)
		}
		actual := ""
		if classification.PlanAction != nil {
			actual = classification.PlanAction.Code
		}
		if actual != validated["action_code"] {
			return Guide{}, guidance.WithErrorCode(guidance.ErrorInputInvalid, fmt.Errorf("playbook input action_code=%q does not match current path action %q", validated["action_code"], actual))
		}
		routeConflictDefinition(&definition, validated["action_code"])
	}
	resolveDefinition(&definition, validated, state.root)
	result := Guide{
		SchemaVersion: SchemaVersion, Workspace: state.root, Recipe: state.metadata.Recipe,
		RecipeIdentities: recipeIdentityObservation(state, id), Observations: playbookObservations(state, id, options),
		Playbook: definition, Values: validated, Truncation: noTruncation("plan", 24<<10),
	}
	if err := truncateGuide(&result); err != nil {
		return Guide{}, guidance.WithErrorCode("playbook_failed", err)
	}
	return result, nil
}

func routeConflictDefinition(definition *recipe.PlaybookDefinition, code string) {
	for i := range definition.Steps {
		if definition.Steps[i].ID != "choose_intent" {
			continue
		}
		switch code {
		case engine.CodeManagedHashMismatch, engine.CodeManagedMissing:
			definition.Steps[i].Summary = "Choose whether human content becomes the contract or the exact Bob-owned file is restored deliberately"
			definition.Steps[i].BlockedBy = []string{"managed_content_intent_unproven"}
		case engine.CodeRetiredOwned:
			definition.Steps[i].Summary = "Choose whether to remove the retired owned path manually or retain it through an explicit contract change"
			definition.Steps[i].BlockedBy = []string{"retired_path_intent_unproven"}
		case engine.CodeUnmanagedDiffers, engine.CodeUnmanagedModeDiffers:
			definition.Steps[i].Summary = "Choose whether to preserve the unmanaged file or replace it only after its intent is proven"
			definition.Steps[i].BlockedBy = []string{"unmanaged_content_intent_unproven"}
		case engine.CodeSymlink, engine.CodeSpecialFile:
			definition.Steps[i].Summary = "Choose whether to replace the unsafe filesystem target or change the repository contract"
			definition.Steps[i].BlockedBy = []string{"unsafe_target_intent_unproven"}
		}
	}
}

func load(root string) (serviceState, error) {
	canonical, err := workspace.Resolve(root, true)
	if err != nil {
		return serviceState{}, guidance.WithErrorCode(guidance.ErrorWorkspaceInvalid, fmt.Errorf("resolve workspace: %w", err))
	}
	m, err := manifest.Load(canonical)
	if err != nil {
		return serviceState{}, guidance.WithErrorCode(guidance.ErrorManifestInvalid, err)
	}
	artifacts, err := recipe.Render(m)
	if err != nil {
		return serviceState{}, guidance.WithErrorCode("playbook_failed", fmt.Errorf("render recipe: %w", err))
	}
	plan, err := engine.Plan(canonical, m, artifacts)
	if err != nil {
		return serviceState{}, guidance.WithErrorCode("playbook_failed", fmt.Errorf("plan workspace: %w", err))
	}
	metadata, err := recipe.ResolveMetadata(m)
	if err != nil {
		return serviceState{}, guidance.WithErrorCode("playbook_failed", err)
	}
	return serviceState{root: canonical, manifest: m, metadata: metadata, plan: plan}, nil
}

func adjustedDefinitions(state serviceState) []recipe.PlaybookDefinition {
	definitions := cloneDefinitions(state.metadata.Playbooks)
	for i := range definitions {
		switch definitions[i].ID {
		case "add-cli-command":
			if !extensionContractMaterialized(state) {
				definitions[i].Available = false
				definitions[i].BlockedBy = []string{"extension_contract_not_materialized"}
			}
		case "upgrade-recipe":
			observed, exists := state.plan.ObservedRecipe()
			switch {
			case !exists:
				definitions[i].Available = false
				definitions[i].BlockedBy = []string{"no_locked_recipe"}
			case observed.ID == state.plan.Recipe.ID && observed.Version == state.plan.Recipe.Version:
				definitions[i].Available = false
				definitions[i].BlockedBy = []string{"already_current_recipe"}
			case state.plan.HasConflicts():
				definitions[i].Available = false
				definitions[i].BlockedBy = []string{"ownership_conflicts"}
			default:
				definitions[i].Available = true
				definitions[i].BlockedBy = []string{}
				removeStep(&definitions[i], "resolve_conflicts")
			}
		}
	}
	for i := range definitions {
		if definitions[i].Available && len(definitions[i].BlockedBy) == 0 {
			continue
		}
		blockDefinitionMutations(&definitions[i])
	}
	return definitions
}

// cloneDefinitions isolates dynamic workspace availability and blocker
// projection from immutable recipe metadata. A shallow copy is insufficient:
// definitions contain nested slices of steps, inputs, boundaries, and codes.
func cloneDefinitions(source []recipe.PlaybookDefinition) []recipe.PlaybookDefinition {
	definitions := make([]recipe.PlaybookDefinition, len(source))
	for i, definition := range source {
		cloned := definition
		cloned.BlockedBy = append([]string{}, definition.BlockedBy...)
		cloned.Inputs = make([]recipe.PlaybookInputDefinition, len(definition.Inputs))
		for j, input := range definition.Inputs {
			cloned.Inputs[j] = input
			cloned.Inputs[j].Enum = append([]string{}, input.Enum...)
			cloned.Inputs[j].Forbidden = append([]string{}, input.Forbidden...)
		}
		cloned.Preconditions = append([]string{}, definition.Preconditions...)
		cloned.Boundary = recipe.PlaybookBoundary{
			Create:    append([]string{}, definition.Boundary.Create...),
			Modify:    append([]string{}, definition.Boundary.Modify...),
			Forbidden: append([]string{}, definition.Boundary.Forbidden...),
		}
		cloned.Steps = make([]recipe.PlaybookStep, len(definition.Steps))
		for j, step := range definition.Steps {
			cloned.Steps[j] = step
			cloned.Steps[j].Paths = append([]string{}, step.Paths...)
			cloned.Steps[j].Argv = append([]string{}, step.Argv...)
			cloned.Steps[j].DependsOn = append([]string{}, step.DependsOn...)
			cloned.Steps[j].BlockedBy = append([]string{}, step.BlockedBy...)
		}
		cloned.VerificationHints = append([]string{}, definition.VerificationHints...)
		cloned.FailureModes = append([]string{}, definition.FailureModes...)
		cloned.CapabilityIDs = append([]string{}, definition.CapabilityIDs...)
		cloned.ExtensionPointIDs = append([]string{}, definition.ExtensionPointIDs...)
		definitions[i] = cloned
	}
	return definitions
}

func extensionContractMaterialized(state serviceState) bool {
	observed, exists := state.plan.ObservedRecipe()
	if !exists || observed.ID != state.plan.Recipe.ID || observed.Version != state.plan.Recipe.Version {
		return false
	}
	requiredPaths := map[string]struct{}{}
	for _, artifact := range state.metadata.Artifacts {
		for _, role := range artifact.Roles {
			if role == "composition_root" || role == "extension_registry" {
				requiredPaths[artifact.Path] = struct{}{}
			}
		}
	}
	if len(requiredPaths) == 0 {
		return false
	}
	for _, action := range state.plan.Actions {
		if _, required := requiredPaths[action.Path]; !required {
			continue
		}
		if action.Kind != engine.ActionUnchanged {
			return false
		}
		delete(requiredPaths, action.Path)
	}
	return len(requiredPaths) == 0
}

func removeStep(definition *recipe.PlaybookDefinition, id string) {
	steps := definition.Steps[:0]
	for _, step := range definition.Steps {
		if step.ID == id {
			continue
		}
		dependencies := step.DependsOn[:0]
		for _, dependency := range step.DependsOn {
			if dependency != id {
				dependencies = append(dependencies, dependency)
			}
		}
		step.DependsOn = dependencies
		steps = append(steps, step)
	}
	definition.Steps = steps
}

func blockDefinitionMutations(definition *recipe.PlaybookDefinition) {
	blockers := append([]string(nil), definition.BlockedBy...)
	if len(blockers) == 0 {
		blockers = []string{"playbook_unavailable"}
	}
	for i := range definition.Steps {
		step := &definition.Steps[i]
		if step.Effect != "repository_mutation" && step.Effect != "user_configuration_mutation" {
			continue
		}
		step.BlockedBy = sortedUnique(append(step.BlockedBy, blockers...))
	}
}

func recipeIdentityObservation(state serviceState, id string) *RecipeIdentities {
	if id != "upgrade-recipe" {
		return nil
	}
	identities := &RecipeIdentities{Current: state.metadata.Recipe}
	if observed, ok := state.plan.ObservedRecipe(); ok {
		identities.Observed = &recipe.MetadataRecipeRef{ID: observed.ID, Version: observed.Version}
	}
	return identities
}

func playbookObservations(state serviceState, id string, options Options) []Observation {
	if id != "enable-terminal-verification" {
		return []Observation{}
	}
	lookPath := options.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	availability := "unavailable"
	if _, err := lookPath("glyph"); err == nil {
		availability = "available"
	}
	return []Observation{
		{ID: "binary.glyph.availability", Value: availability},
		{ID: "manifest.integrations.terminal_verification", Value: state.manifest.Integrations.TerminalVerification},
		{ID: "verification", Value: "not_assessed"},
	}
}

func find(definitions []recipe.PlaybookDefinition, id string) (recipe.PlaybookDefinition, error) {
	if len(id) == 0 || len(id) > 128 {
		return recipe.PlaybookDefinition{}, errors.New("playbook id must contain 1 to 128 bytes")
	}
	ids := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		ids = append(ids, definition.ID)
		if definition.ID == id {
			return definition, nil
		}
	}
	suffix := ""
	if suggestion, ok := strsim.Closest(id, ids, 2); ok {
		suffix = fmt.Sprintf("; did you mean %q?", suggestion)
	}
	return recipe.PlaybookDefinition{}, fmt.Errorf("unknown playbook %q%s", id, suffix)
}

var kebabPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
var resolvedPlaceholderPattern = regexp.MustCompile(`<([a-z][a-z0-9_]*)>`)

func validateInputs(definition recipe.PlaybookDefinition, values map[string]string) (map[string]string, error) {
	if values == nil {
		values = map[string]string{}
	}
	if len(values) > 32 {
		return nil, errors.New("playbook accepts at most 32 input values")
	}
	schema := make(map[string]recipe.PlaybookInputDefinition, len(definition.Inputs))
	for _, input := range definition.Inputs {
		schema[input.Name] = input
	}
	unknown := []string{}
	invalidBounds := 0
	for key := range values {
		if len(key) == 0 || len(key) > 128 || len(values[key]) > 4096 {
			invalidBounds++
			continue
		}
		if _, ok := schema[key]; !ok {
			unknown = append(unknown, key)
		}
	}
	missing := []string{}
	for _, input := range definition.Inputs {
		if input.Required && strings.TrimSpace(values[input.Name]) == "" {
			missing = append(missing, input.Name)
		}
	}
	sort.Strings(unknown)
	sort.Strings(missing)
	problems := []string{}
	if invalidBounds > 0 {
		problems = append(problems, fmt.Sprintf("%d input values exceed key or value bounds", invalidBounds))
	}
	if len(unknown) > 0 {
		problems = append(problems, "unknown inputs: "+strings.Join(unknown, ", "))
	}
	if len(missing) > 0 {
		problems = append(problems, "missing required inputs: "+strings.Join(missing, ", "))
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	validated := map[string]string{}
	for _, input := range definition.Inputs {
		value, present := values[input.Name]
		if !present {
			continue
		}
		switch input.Type {
		case "identifier":
			if !kebabPattern.MatchString(value) {
				problems = append(problems, fmt.Sprintf("input %s must use lowercase-kebab", input.Name))
			}
		case "repository_path":
			normalized, err := engine.NormalizeRepositoryPath(value)
			if err != nil {
				problems = append(problems, fmt.Sprintf("input %s: %v", input.Name, err))
			} else {
				value = normalized
			}
		case "enum":
			if !contains(input.Enum, value) {
				problems = append(problems, fmt.Sprintf("input %s must be one of %s", input.Name, strings.Join(input.Enum, ", ")))
			}
		default:
			problems = append(problems, fmt.Sprintf("input %s has unsupported type %q", input.Name, input.Type))
		}
		if contains(input.Forbidden, value) {
			problems = append(problems, fmt.Sprintf("input %s uses reserved value %q", input.Name, value))
		}
		validated[input.Name] = value
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	return validated, nil
}

func resolveDefinition(definition *recipe.PlaybookDefinition, values map[string]string, root string) {
	resolve := func(value string) string {
		// Replace tokens from the original template in one pass. Replacement
		// values are opaque data: a workspace or input containing text such as
		// "<path>" must never be scanned again as another placeholder.
		return resolvedPlaceholderPattern.ReplaceAllStringFunc(value, func(token string) string {
			name := token[1 : len(token)-1]
			if name == "workspace" {
				return root
			}
			if replacement, ok := values[name]; ok {
				return replacement
			}
			return token
		})
	}
	for i := range definition.Boundary.Create {
		definition.Boundary.Create[i] = resolve(definition.Boundary.Create[i])
	}
	for i := range definition.Boundary.Modify {
		definition.Boundary.Modify[i] = resolve(definition.Boundary.Modify[i])
	}
	for i := range definition.Boundary.Forbidden {
		definition.Boundary.Forbidden[i] = resolve(definition.Boundary.Forbidden[i])
	}
	for i := range definition.Steps {
		for j := range definition.Steps[i].Paths {
			definition.Steps[i].Paths[j] = resolve(definition.Steps[i].Paths[j])
		}
		for j := range definition.Steps[i].Argv {
			definition.Steps[i].Argv[j] = resolve(definition.Steps[i].Argv[j])
		}
	}
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func noTruncation(profile string, limit int) guidance.Truncation {
	return guidance.Truncation{Profile: profile, ByteLimit: limit, Truncated: false, Omitted: map[string]int{}}
}

func enforceBound(value any, limit int) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode playbook result: %w", err)
	}
	if len(data) > limit {
		return fmt.Errorf("playbook result exceeds %d-byte bound", limit)
	}
	return nil
}

func truncateShow(result *ShowResult) error {
	truncateDefinition(&result.Playbook, &result.Truncation, func() int { return encodedSize(*result) })
	if encodedSize(*result) > result.Truncation.ByteLimit {
		return fmt.Errorf("playbook show identity exceeds %d-byte bound", result.Truncation.ByteLimit)
	}
	return nil
}

func truncateGuide(result *Guide) error {
	truncateDefinition(&result.Playbook, &result.Truncation, func() int { return encodedSize(*result) })
	if encodedSize(*result) > result.Truncation.ByteLimit {
		return fmt.Errorf("playbook plan identity exceeds %d-byte bound", result.Truncation.ByteLimit)
	}
	return nil
}

func truncateDefinition(definition *recipe.PlaybookDefinition, truncation *guidance.Truncation, size func() int) {
	limit := truncation.ByteLimit
	for size() > limit {
		key, removed := omitDefinitionField(definition)
		if !removed {
			return
		}
		truncation.Truncated = true
		truncation.Omitted[key]++
	}
}

// omitDefinitionField implements a deterministic priority order. Identity,
// applicability, blockers, typed inputs, step effects, dependencies, argv,
// and blocked human decisions are never discarded.
func omitDefinitionField(definition *recipe.PlaybookDefinition) (string, bool) {
	switch {
	case len(definition.FailureModes) > 0:
		definition.FailureModes = definition.FailureModes[:len(definition.FailureModes)-1]
		return "failure_modes", true
	case len(definition.VerificationHints) > 0:
		definition.VerificationHints = definition.VerificationHints[:len(definition.VerificationHints)-1]
		return "verification_hints", true
	case len(definition.Preconditions) > 0:
		definition.Preconditions = definition.Preconditions[:len(definition.Preconditions)-1]
		return "preconditions", true
	case definition.Purpose != "":
		definition.Purpose = ""
		return "purpose", true
	}
	for i := len(definition.Steps) - 1; i >= 0; i-- {
		if definition.Steps[i].SuccessCondition != "" {
			definition.Steps[i].SuccessCondition = ""
			return "step_success_conditions", true
		}
	}
	for i := len(definition.Steps) - 1; i >= 0; i-- {
		if definition.Steps[i].Summary != "" {
			definition.Steps[i].Summary = ""
			return "step_summaries", true
		}
	}
	for i := len(definition.Steps) - 1; i >= 0; i-- {
		if len(definition.Steps[i].Paths) > 0 {
			definition.Steps[i].Paths = definition.Steps[i].Paths[:len(definition.Steps[i].Paths)-1]
			return "step_paths", true
		}
	}
	if len(definition.Boundary.Create) > 0 {
		definition.Boundary.Create = definition.Boundary.Create[:len(definition.Boundary.Create)-1]
		return "boundary_create", true
	}
	if len(definition.Boundary.Modify) > 0 {
		definition.Boundary.Modify = definition.Boundary.Modify[:len(definition.Boundary.Modify)-1]
		return "boundary_modify", true
	}
	if len(definition.Boundary.Forbidden) > 0 {
		definition.Boundary.Forbidden = definition.Boundary.Forbidden[:len(definition.Boundary.Forbidden)-1]
		return "boundary_forbidden", true
	}
	return "", false
}

func encodedSize(value any) int {
	data, _ := json.Marshal(value)
	return len(data)
}

func sortedUnique(values []string) []string {
	sort.Strings(values)
	out := values[:0]
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}
