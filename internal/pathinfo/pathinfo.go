// Package pathinfo composes Bob's exact engine ownership classification with
// recipe metadata. It is read-only and does not inspect file contents beyond
// the planner's existing bounded observations.
package pathinfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/guidance"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/workspace"
)

const SchemaVersion = 1

type Result struct {
	SchemaVersion    int                 `json:"schema_version"`
	Workspace        string              `json:"workspace"`
	Path             string              `json:"path"`
	Exists           bool                `json:"exists"`
	Classification   string              `json:"classification"`
	State            string              `json:"state"`
	HumanEditEffect  string              `json:"human_edit_effect"`
	Ownership        Ownership           `json:"ownership"`
	PlanAction       *engine.PathAction  `json:"plan_action,omitempty"`
	Artifact         *Artifact           `json:"artifact,omitempty"`
	ExtensionPoints  []string            `json:"extension_points"`
	RelatedPlaybooks []string            `json:"related_playbooks"`
	Notices          []guidance.Notice   `json:"notices"`
	Actions          []guidance.Action   `json:"actions"`
	Truncation       guidance.Truncation `json:"truncation"`
}

type Ownership struct {
	Recipe        recipe.MetadataRecipeRef `json:"recipe"`
	LockedSHA256  string                   `json:"locked_sha256,omitempty"`
	CurrentSHA256 string                   `json:"current_sha256,omitempty"`
}

type Artifact struct {
	ID            string   `json:"id"`
	Roles         []string `json:"roles"`
	CapabilityIDs []string `json:"capability_ids"`
}

func Load(root, requested string) (Result, error) {
	if _, err := engine.NormalizeRepositoryPath(requested); err != nil {
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
		return Result{}, guidance.WithErrorCode("path_failed", fmt.Errorf("render recipe: %w", err))
	}
	plan, err := engine.Plan(canonical, m, artifacts)
	if err != nil {
		return Result{}, guidance.WithErrorCode("path_failed", fmt.Errorf("plan workspace: %w", err))
	}
	metadata, err := recipe.ResolveMetadata(m)
	if err != nil {
		return Result{}, guidance.WithErrorCode("path_failed", err)
	}
	classification, err := engine.ClassifyPath(canonical, plan, requested)
	if err != nil {
		return Result{}, guidance.WithErrorCode("path_failed", err)
	}
	result := Result{
		SchemaVersion: SchemaVersion, Workspace: canonical, Path: classification.Path, Exists: classification.Exists,
		Classification: classification.Classification, State: classification.State, HumanEditEffect: classification.HumanEditEffect,
		Ownership:  Ownership{Recipe: metadata.Recipe, LockedSHA256: classification.LockedSHA256, CurrentSHA256: classification.CurrentSHA256},
		PlanAction: classification.PlanAction, ExtensionPoints: []string{}, RelatedPlaybooks: []string{}, Notices: []guidance.Notice{}, Actions: []guidance.Action{},
		Truncation: guidance.Truncation{Profile: "path", ByteLimit: 8 << 10, Truncated: false, Omitted: map[string]int{}},
	}
	for _, descriptor := range metadata.Artifacts {
		if descriptor.Path == result.Path {
			result.Artifact = &Artifact{ID: descriptor.ID, Roles: append([]string(nil), descriptor.Roles...), CapabilityIDs: append([]string(nil), descriptor.CapabilityIDs...)}
			result.RelatedPlaybooks = append(result.RelatedPlaybooks, metadataPlaybooksForPath(metadata, result.Path)...)
			break
		}
	}
	if result.Classification == engine.PathClassificationUnmanaged || result.Classification == engine.PathClassificationMissing {
		for _, extension := range recipe.MatchExtensionPoints(metadata, result.Path) {
			result.ExtensionPoints = append(result.ExtensionPoints, extension.ID)
			result.RelatedPlaybooks = append(result.RelatedPlaybooks, extension.PlaybookIDs...)
		}
		if len(result.ExtensionPoints) > 0 {
			result.Classification = "extension_point"
			result.State = "extension_point"
		}
	}
	if result.PlanAction != nil && result.PlanAction.Kind == engine.ActionConflict {
		result.RelatedPlaybooks = append(result.RelatedPlaybooks, "resolve-ownership-conflict")
	}
	result.ExtensionPoints = unique(result.ExtensionPoints)
	result.RelatedPlaybooks = unique(result.RelatedPlaybooks)
	for _, id := range result.RelatedPlaybooks {
		result.Actions = append(result.Actions, guidance.Action{
			ID: "show_playbook:" + id, Kind: "command", Effect: "read_only", CWD: canonical,
			Argv: []string{"bob", "playbook", "show", id, canonical, "--json"}, ReasonCode: "related_playbook",
			RequiresExplicitAuthority: false, BlockedBy: []string{},
		})
	}
	truncate(&result)
	if jsonSize(result) > 8<<10 {
		return Result{}, guidance.WithErrorCode("path_failed", errors.New("path result exceeds 8192-byte bound"))
	}
	return result, nil
}

// metadataPlaybooksForPath derives related procedures from the recipe's
// declared boundaries and extension constraints. Keeping this projection
// metadata-driven avoids baking recipe-specific artifact IDs into the path
// service as new immutable recipe versions are published.
func metadataPlaybooksForPath(metadata recipe.Metadata, path string) []string {
	ids := []string{}
	for _, extension := range metadata.ExtensionPoints {
		if containsPath(extension.ForbiddenPaths, path) {
			ids = append(ids, extension.PlaybookIDs...)
		}
	}
	for _, playbook := range metadata.Playbooks {
		if containsPath(playbook.Boundary.Create, path) || containsPath(playbook.Boundary.Modify, path) || containsPath(playbook.Boundary.Forbidden, path) {
			ids = append(ids, playbook.ID)
		}
	}
	return unique(ids)
}

func containsPath(paths []string, path string) bool {
	for _, candidate := range paths {
		if candidate == path {
			return true
		}
	}
	return false
}

func truncate(result *Result) {
	const limit = 8 << 10
	for jsonSize(*result) > limit {
		switch {
		case result.Artifact != nil && len(result.Artifact.Roles) > 0:
			result.Truncation.Truncated = true
			result.Truncation.Omitted["artifact_roles"] += len(result.Artifact.Roles)
			result.Artifact.Roles = []string{}
		case result.Artifact != nil && len(result.Artifact.CapabilityIDs) > 0:
			result.Truncation.Truncated = true
			result.Truncation.Omitted["artifact_capability_ids"] += len(result.Artifact.CapabilityIDs)
			result.Artifact.CapabilityIDs = []string{}
		case len(result.Notices) > 0:
			result.Truncation.Truncated = true
			result.Truncation.Omitted["notices"]++
			result.Notices = result.Notices[:len(result.Notices)-1]
		case len(result.Actions) > 0:
			result.Truncation.Truncated = true
			result.Truncation.Omitted["actions"]++
			result.Actions = result.Actions[:len(result.Actions)-1]
		default:
			return
		}
	}
}

func jsonSize(value Result) int {
	data, _ := json.Marshal(value)
	return len(data)
}

func unique(values []string) []string {
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
