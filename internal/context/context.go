// Package context composes Bob's deterministic repository contract into a
// bounded, read-only workspace projection. It never runs specialist tools or
// mutates repository state.
package context

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/guidance"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	playbookpkg "github.com/abdul-hamid-achik/bob/internal/playbook"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/workspace"
)

const SchemaVersion = 1

type Profile string

const (
	ProfileCompact  Profile = "compact"
	ProfileStandard Profile = "standard"
	ProfileFull     Profile = "full"
)

type Options struct {
	Profile  Profile
	LookPath func(string) (string, error)
}

type Result struct {
	SchemaVersion   int                         `json:"schema_version"`
	Profile         Profile                     `json:"profile"`
	Workspace       string                      `json:"workspace"`
	ContractDigest  string                      `json:"contract_digest"`
	ContextDigest   string                      `json:"context_digest"`
	Recipe          recipe.MetadataRecipeRef    `json:"recipe"`
	Product         Product                     `json:"product"`
	Repository      Repository                  `json:"repository"`
	Capabilities    []Capability                `json:"capabilities"`
	EntryPoints     []EntryPoint                `json:"entry_points"`
	ExtensionPoints []ExtensionPoint            `json:"extension_points"`
	Invariants      []Invariant                 `json:"invariants"`
	Playbooks       []recipe.PlaybookSummary    `json:"playbooks"`
	Artifacts       []recipe.ArtifactDescriptor `json:"artifacts,omitempty"`
	Notices         []Notice                    `json:"notices"`
	Actions         []Action                    `json:"actions"`
	Truncation      Truncation                  `json:"truncation"`
}

type Product struct {
	Name       string `json:"name"`
	Module     string `json:"module,omitempty"`
	Runtime    string `json:"runtime,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Visibility string `json:"visibility,omitempty"`
}

type Repository struct {
	State             string `json:"state"`
	Clean             bool   `json:"clean"`
	LockChanged       bool   `json:"lock_changed"`
	ConflictCount     int    `json:"conflict_count"`
	ManagedFiles      int    `json:"managed_files"`
	PlanDigestVersion int    `json:"plan_digest_version"`
	PlanDigest        string `json:"plan_digest"`
}

type Capability struct {
	ID              string              `json:"id"`
	Category        string              `json:"category,omitempty"`
	Selection       string              `json:"selection"`
	Materialization string              `json:"materialization"`
	Availability    string              `json:"availability"`
	Verification    string              `json:"verification"`
	Summary         string              `json:"summary,omitempty"`
	Evidence        *CapabilityEvidence `json:"evidence,omitempty"`
	Limitations     []string            `json:"limitations,omitempty"`
}

type CapabilityEvidence struct {
	ManifestFields []string `json:"manifest_fields,omitempty"`
	ArtifactIDs    []string `json:"artifact_ids,omitempty"`
	Paths          []string `json:"paths,omitempty"`
	Binary         string   `json:"binary,omitempty"`
}

type EntryPoint struct {
	ID            string   `json:"id"`
	Path          string   `json:"path"`
	Roles         []string `json:"roles,omitempty"`
	Ownership     string   `json:"ownership"`
	CapabilityIDs []string `json:"capability_ids,omitempty"`
}

type ExtensionPoint struct {
	ID             string   `json:"id"`
	Purpose        string   `json:"purpose,omitempty"`
	Ownership      string   `json:"ownership"`
	CreatePatterns []string `json:"create_patterns"`
	ForbiddenPaths []string `json:"forbidden_paths,omitempty"`
	CapabilityIDs  []string `json:"capability_ids,omitempty"`
	PlaybookIDs    []string `json:"playbook_ids,omitempty"`
}

type Invariant struct {
	ID        string `json:"id"`
	Statement string `json:"statement"`
}

type Notice = guidance.Notice
type Action = guidance.Action

type Truncation struct {
	Profile   Profile        `json:"profile"`
	ByteLimit int            `json:"byte_limit"`
	Truncated bool           `json:"truncated"`
	Omitted   map[string]int `json:"omitted"`
}

// Load returns one coherent workspace plan and metadata projection.
func Load(root string, options Options) (Result, error) {
	profile := options.Profile
	if profile == "" {
		profile = ProfileCompact
	}
	limit, err := profileLimit(profile)
	if err != nil {
		return Result{}, guidance.WithErrorCode(guidance.ErrorInputInvalid, err)
	}
	canonical, err := workspace.Resolve(root, true)
	if err != nil {
		return Result{}, guidance.WithErrorCode(guidance.ErrorWorkspaceInvalid, fmt.Errorf("resolve workspace: %w", err))
	}
	m, err := manifest.Load(canonical)
	if err != nil {
		return Result{}, guidance.WithErrorCode(guidance.ErrorManifestInvalid, err)
	}
	artifacts, err := recipe.Render(m)
	if err != nil {
		return Result{}, guidance.WithErrorCode("context_failed", fmt.Errorf("render recipe: %w", err))
	}
	plan, err := engine.Plan(canonical, m, artifacts)
	if err != nil {
		return Result{}, guidance.WithErrorCode("context_failed", fmt.Errorf("plan workspace: %w", err))
	}
	metadata, err := recipe.ResolveMetadata(m)
	if err != nil {
		return Result{}, guidance.WithErrorCode("context_failed", err)
	}
	lookPath := options.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}

	full := compose(canonical, m, plan, metadata, lookPath)
	full.ContractDigest = digestContract(m, metadata)
	full.ContextDigest = digestContext(full)
	projected := project(full, profile, limit)
	if size := jsonSize(projected); size > limit {
		return Result{}, guidance.WithErrorCode("context_failed", fmt.Errorf("context %s result exceeds %d-byte bound", profile, limit))
	}
	return projected, nil
}

func compose(root string, m manifest.Manifest, plan engine.PlanResult, metadata recipe.Metadata, lookPath func(string) (string, error)) Result {
	clean := !plan.LockChanged
	for _, action := range plan.Actions {
		if action.Kind != engine.ActionUnchanged {
			clean = false
			break
		}
	}
	state := "drifted"
	if plan.HasConflicts() {
		state = "conflicted"
	} else if clean {
		state = "clean"
	}
	digest := engine.DigestPlan(plan)
	result := Result{
		SchemaVersion: SchemaVersion, Profile: ProfileFull, Workspace: root, Recipe: metadata.Recipe,
		Product: Product{Name: m.Product.Name, Module: m.Product.Module, Runtime: m.Runtime.Language, Kind: m.Runtime.Kind, Visibility: m.Product.Visibility},
		Repository: Repository{
			State: state, Clean: clean, LockChanged: plan.LockChanged, ConflictCount: plan.ConflictCount,
			ManagedFiles: len(plan.DesiredLock.Files), PlanDigestVersion: digest.Version, PlanDigest: prefixed(digest.SHA256),
		},
		Capabilities: []Capability{}, EntryPoints: []EntryPoint{}, ExtensionPoints: []ExtensionPoint{},
		Invariants: []Invariant{}, Playbooks: playbookpkg.Summaries(metadata, plan),
		Artifacts: append([]recipe.ArtifactDescriptor(nil), metadata.Artifacts...), Notices: []Notice{}, Actions: []Action{},
		Truncation: Truncation{Profile: ProfileFull, ByteLimit: 64 << 10, Omitted: map[string]int{}},
	}
	actionsByPath := make(map[string]engine.Action, len(plan.Actions))
	for _, action := range plan.Actions {
		actionsByPath[action.Path] = action
	}
	artifactsByID := make(map[string]recipe.ArtifactDescriptor, len(metadata.Artifacts))
	for _, artifact := range metadata.Artifacts {
		artifactsByID[artifact.ID] = artifact
	}
	for _, definition := range metadata.Capabilities {
		capability := Capability{
			ID: definition.ID, Category: definition.Category, Selection: definition.Selection,
			Materialization: materialization(definition, artifactsByID, actionsByPath),
			Availability:    availability(definition, lookPath), Verification: "not_assessed", Summary: definition.Summary,
			Evidence: &CapabilityEvidence{
				ManifestFields: append([]string(nil), definition.ManifestFields...), ArtifactIDs: append([]string(nil), definition.ArtifactIDs...),
				Paths: capabilityPaths(definition.ArtifactIDs, artifactsByID), Binary: definition.Binary,
			},
			Limitations: append([]string(nil), definition.Limitations...),
		}
		result.Capabilities = append(result.Capabilities, capability)
		if definition.Selection != "disabled" && definition.Binary != "" && capability.Availability == "unavailable" {
			result.Notices = append(result.Notices, Notice{
				ID: "binary_unavailable:" + definition.ID, Severity: "warning", Code: "binary_unavailable",
				Message: "selected capability binary is unavailable; Bob did not run or verify it", CapabilityID: definition.ID, Paths: []string{},
			})
		}
	}
	for _, artifact := range metadata.Artifacts {
		if hasRole(artifact.Roles, "entrypoint") || hasRole(artifact.Roles, "composition_root") {
			result.EntryPoints = append(result.EntryPoints, EntryPoint{
				ID: artifact.ID, Path: artifact.Path, Roles: artifact.Roles, Ownership: artifact.Ownership, CapabilityIDs: artifact.CapabilityIDs,
			})
		}
	}
	for _, extension := range metadata.ExtensionPoints {
		result.ExtensionPoints = append(result.ExtensionPoints, ExtensionPoint(extension))
	}
	for _, invariant := range metadata.Invariants {
		result.Invariants = append(result.Invariants, Invariant(invariant))
	}
	switch state {
	case "conflicted":
		result.Actions = append(result.Actions, commandAction(root, "review_plan", "ownership_conflict"))
	case "drifted":
		result.Actions = append(result.Actions, commandAction(root, "review_plan", "repository_drift"))
	}
	sort.Slice(result.Notices, func(i, j int) bool { return result.Notices[i].ID < result.Notices[j].ID })
	return result
}

func commandAction(root, id, reason string) Action {
	return Action{ID: id, Kind: "command", Effect: "read_only", CWD: root,
		Argv: []string{"bob", "plan", root, "--json"}, ReasonCode: reason,
		RequiresExplicitAuthority: false, BlockedBy: []string{}}
}

func materialization(definition recipe.CapabilityDefinition, artifacts map[string]recipe.ArtifactDescriptor, actions map[string]engine.Action) string {
	if definition.Selection == "disabled" || definition.Selection == "not_applicable" {
		return "not_applicable"
	}
	if len(definition.ArtifactIDs) == 0 {
		return "not_applicable"
	}
	state := "in_sync"
	for _, id := range definition.ArtifactIDs {
		artifact, ok := artifacts[id]
		if !ok {
			return "unknown"
		}
		action, ok := actions[artifact.Path]
		if !ok {
			return "unknown"
		}
		switch action.Kind {
		case engine.ActionConflict:
			return "conflicted"
		case engine.ActionCreate:
			if state == "in_sync" {
				state = "missing"
			}
		case engine.ActionUpdate, engine.ActionAdopt:
			if state == "in_sync" {
				state = "drifted"
			}
		case engine.ActionUnchanged:
		default:
			return "unknown"
		}
	}
	return state
}

func availability(definition recipe.CapabilityDefinition, lookPath func(string) (string, error)) string {
	if definition.Binary == "" {
		return "not_applicable"
	}
	if definition.Selection == "disabled" || definition.Selection == "not_applicable" {
		return "not_checked"
	}
	if _, err := lookPath(definition.Binary); err != nil {
		return "unavailable"
	}
	return "available"
}

func capabilityPaths(ids []string, artifacts map[string]recipe.ArtifactDescriptor) []string {
	paths := make([]string, 0, len(ids))
	for _, id := range ids {
		if artifact, ok := artifacts[id]; ok {
			paths = append(paths, artifact.Path)
		}
	}
	sort.Strings(paths)
	return paths
}

func hasRole(roles []string, want string) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}

func profileLimit(profile Profile) (int, error) {
	switch profile {
	case ProfileCompact:
		return 6144, nil
	case ProfileStandard:
		return 24 << 10, nil
	case ProfileFull:
		return 64 << 10, nil
	default:
		return 0, fmt.Errorf("profile must be one of compact, standard, full (got %q)", profile)
	}
}

func project(full Result, profile Profile, limit int) Result {
	result := clone(full)
	result.Profile = profile
	result.Truncation = Truncation{Profile: profile, ByteLimit: limit, Omitted: map[string]int{}}
	switch profile {
	case ProfileCompact:
		result.Artifacts = nil
		for i := range result.Capabilities {
			result.Capabilities[i].Category = ""
			result.Capabilities[i].Summary = ""
			result.Capabilities[i].Limitations = nil
			result.Capabilities[i].Evidence = nil
		}
		for i := range result.Playbooks {
			result.Playbooks[i].Title = ""
		}
		for i := range result.EntryPoints {
			result.EntryPoints[i].Roles = nil
			result.EntryPoints[i].CapabilityIDs = nil
		}
		for i := range result.ExtensionPoints {
			result.ExtensionPoints[i].Purpose = ""
			result.ExtensionPoints[i].ForbiddenPaths = nil
			result.ExtensionPoints[i].CapabilityIDs = nil
			result.ExtensionPoints[i].PlaybookIDs = nil
		}
	case ProfileStandard:
		result.Artifacts = nil
	}
	truncate(&result, limit)
	return result
}

func truncate(result *Result, limit int) {
	for jsonSize(*result) > limit {
		switch {
		case len(result.Artifacts) > 0:
			truncateArtifactPrefix(result, limit)
		case stripCapabilityDetail(result):
		case len(result.Notices) > 0:
			result.Notices = result.Notices[:len(result.Notices)-1]
			omit(result, "notices", 1)
		case len(result.Invariants) > 1:
			result.Invariants = result.Invariants[:len(result.Invariants)-1]
			omit(result, "invariants", 1)
		case len(result.ExtensionPoints) > 1:
			result.ExtensionPoints = result.ExtensionPoints[:len(result.ExtensionPoints)-1]
			omit(result, "extension_points", 1)
		case len(result.EntryPoints) > 1:
			result.EntryPoints = result.EntryPoints[:len(result.EntryPoints)-1]
			omit(result, "entry_points", 1)
		default:
			return
		}
	}
}

func truncateArtifactPrefix(result *Result, limit int) {
	original := len(result.Artifacts)
	best := -1
	low, high := 0, original-1
	for low <= high {
		mid := low + (high-low)/2
		candidate := *result
		candidate.Artifacts = result.Artifacts[:mid]
		candidate.Truncation.Omitted = make(map[string]int, len(result.Truncation.Omitted)+1)
		for key, value := range result.Truncation.Omitted {
			candidate.Truncation.Omitted[key] = value
		}
		omit(&candidate, "artifacts", original-mid)
		if jsonSize(candidate) <= limit {
			best = mid
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
	if best < 0 {
		best = 0
	}
	result.Artifacts = result.Artifacts[:best]
	omit(result, "artifacts", original-best)
}

func stripCapabilityDetail(result *Result) bool {
	for i := len(result.Capabilities) - 1; i >= 0; i-- {
		capability := &result.Capabilities[i]
		switch {
		case len(capability.Limitations) > 0:
			omit(result, "capability_limitations", len(capability.Limitations))
			capability.Limitations = nil
			return true
		case capability.Evidence != nil && len(capability.Evidence.Paths) > 0:
			omit(result, "capability_evidence_paths", len(capability.Evidence.Paths))
			capability.Evidence.Paths = nil
			return true
		case capability.Evidence != nil && len(capability.Evidence.ArtifactIDs) > 0:
			omit(result, "capability_evidence_artifact_ids", len(capability.Evidence.ArtifactIDs))
			capability.Evidence.ArtifactIDs = nil
			return true
		case capability.Summary != "":
			omit(result, "capability_summaries", 1)
			capability.Summary = ""
			return true
		}
	}
	return false
}

func omit(result *Result, field string, count int) {
	result.Truncation.Truncated = true
	result.Truncation.Omitted[field] += count
}

func clone(value Result) Result {
	data, _ := json.Marshal(value)
	var cloned Result
	_ = json.Unmarshal(data, &cloned)
	return cloned
}

func jsonSize(value Result) int { data, _ := json.Marshal(value); return len(data) }

func digestContract(m manifest.Manifest, metadata recipe.Metadata) string {
	identity := struct {
		Manifest              manifest.Manifest        `json:"manifest"`
		MetadataSchemaVersion int                      `json:"metadata_schema_version"`
		Recipe                recipe.MetadataRecipeRef `json:"recipe"`
		Capabilities          []struct {
			ID, Category, Selection, Binary string
			ManifestFields, ArtifactIDs     []string
		} `json:"capabilities"`
		Artifacts       []recipe.ArtifactDescriptor `json:"artifacts"`
		InvariantIDs    []string                    `json:"invariant_ids"`
		ExtensionPoints []struct {
			ID, Ownership                                              string
			CreatePatterns, ForbiddenPaths, CapabilityIDs, PlaybookIDs []string
		} `json:"extension_points"`
		Playbooks []struct {
			ID, ScopeClass, Risk                        string
			Applicable, Available                       bool
			BlockedBy, CapabilityIDs, ExtensionPointIDs []string
			Inputs                                      []recipe.PlaybookInputDefinition
			Boundary                                    recipe.PlaybookBoundary
			Steps                                       []struct {
				ID, Kind, Effect                  string
				Paths, Argv, DependsOn, BlockedBy []string
				RequiresExplicitAuthority         bool
			}
		} `json:"playbooks"`
	}{Manifest: normalizedManifestForDigest(m), MetadataSchemaVersion: metadata.SchemaVersion, Recipe: metadata.Recipe, Artifacts: metadata.Artifacts}
	for _, capability := range metadata.Capabilities {
		identity.Capabilities = append(identity.Capabilities, struct {
			ID, Category, Selection, Binary string
			ManifestFields, ArtifactIDs     []string
		}{
			capability.ID, capability.Category, capability.Selection, capability.Binary, capability.ManifestFields, capability.ArtifactIDs,
		})
	}
	for _, invariant := range metadata.Invariants {
		identity.InvariantIDs = append(identity.InvariantIDs, invariant.ID)
	}
	for _, extension := range metadata.ExtensionPoints {
		identity.ExtensionPoints = append(identity.ExtensionPoints, struct {
			ID, Ownership                                              string
			CreatePatterns, ForbiddenPaths, CapabilityIDs, PlaybookIDs []string
		}{
			extension.ID, extension.Ownership, extension.CreatePatterns, extension.ForbiddenPaths, extension.CapabilityIDs, extension.PlaybookIDs,
		})
	}
	for _, playbook := range metadata.Playbooks {
		item := struct {
			ID, ScopeClass, Risk                        string
			Applicable, Available                       bool
			BlockedBy, CapabilityIDs, ExtensionPointIDs []string
			Inputs                                      []recipe.PlaybookInputDefinition
			Boundary                                    recipe.PlaybookBoundary
			Steps                                       []struct {
				ID, Kind, Effect                  string
				Paths, Argv, DependsOn, BlockedBy []string
				RequiresExplicitAuthority         bool
			}
		}{
			ID: playbook.ID, ScopeClass: playbook.ScopeClass, Risk: playbook.Risk,
			Applicable: playbook.Applicable, Available: playbook.Available, BlockedBy: playbook.BlockedBy,
			CapabilityIDs: playbook.CapabilityIDs, ExtensionPointIDs: playbook.ExtensionPointIDs,
			Inputs: playbook.Inputs, Boundary: playbook.Boundary,
		}
		for _, step := range playbook.Steps {
			item.Steps = append(item.Steps, struct {
				ID, Kind, Effect                  string
				Paths, Argv, DependsOn, BlockedBy []string
				RequiresExplicitAuthority         bool
			}{step.ID, step.Kind, step.Effect, step.Paths, step.Argv, step.DependsOn, step.BlockedBy, step.RequiresExplicitAuthority})
		}
		identity.Playbooks = append(identity.Playbooks, item)
	}
	return hashJSON(identity)
}

func normalizedManifestForDigest(m manifest.Manifest) manifest.Manifest {
	if m.Recipe != manifest.RecipeFiles {
		return m
	}
	normalized := m
	normalized.Files = append([]manifest.FileDecl(nil), m.Files...)
	for i := range normalized.Files {
		normalized.Files[i].Path = filepath.ToSlash(filepath.Clean(normalized.Files[i].Path))
		mode, err := manifest.ParseFileMode(normalized.Files[i].Mode)
		if err == nil {
			normalized.Files[i].Mode = fmt.Sprintf("%04o", mode.Perm())
		}
	}
	sort.Slice(normalized.Files, func(i, j int) bool { return normalized.Files[i].Path < normalized.Files[j].Path })
	return normalized
}

func digestContext(value Result) string {
	identity := clone(value)
	workspacePath := identity.Workspace
	identity.Profile = ""
	identity.Workspace = ""
	identity.ContextDigest = ""
	identity.Truncation = Truncation{}
	for i := range identity.Capabilities {
		identity.Capabilities[i].Summary = ""
		identity.Capabilities[i].Limitations = nil
	}
	for i := range identity.ExtensionPoints {
		identity.ExtensionPoints[i].Purpose = ""
	}
	for i := range identity.Invariants {
		identity.Invariants[i].Statement = ""
	}
	for i := range identity.Playbooks {
		identity.Playbooks[i].Title = ""
	}
	for i := range identity.Notices {
		identity.Notices[i].Message = ""
	}
	for i := range identity.Actions {
		identity.Actions[i].CWD = ""
		for j := range identity.Actions[i].Argv {
			if identity.Actions[i].Argv[j] == workspacePath {
				identity.Actions[i].Argv[j] = "<workspace>"
			}
		}
	}
	return hashJSON(identity)
}

func hashJSON(value any) string {
	data, _ := json.Marshal(value)
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func prefixed(value string) string { return "sha256:" + value }
