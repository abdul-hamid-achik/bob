// Package inspect summarizes Bob-managed repository state and explicitly
// requested specialist index status without performing repair or search.
package inspect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/workspace"
)

const SchemaVersion = 1

const (
	ProbeNotSelected  = "not_selected"
	ProbeNotRequested = "not_requested"
	ProbeUnavailable  = "unavailable"
	ProbeComplete     = "complete"
	ProbeTimedOut     = "timed_out"
	ProbeFailed       = "failed"
	ProbeInvalid      = "invalid_output"
	ProbeWrongProject = "wrong_project"

	IndexNotIndexed   = "not_indexed"
	IndexFresh        = "fresh"
	IndexStale        = "stale"
	IndexIncompatible = "incompatible"
	IndexUnknown      = "unknown"
)

type Options struct {
	ProbeIntegrations bool
	ProbeTimeout      time.Duration
}

type ActionCounts struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Adopt     int `json:"adopt"`
	Unchanged int `json:"unchanged"`
	Conflict  int `json:"conflict"`
}

type Repository struct {
	State         string       `json:"state"`
	ManifestPath  string       `json:"manifest_path"`
	Recipe        string       `json:"recipe,omitempty"`
	Ready         bool         `json:"ready"`
	Converged     bool         `json:"converged"`
	LockChanged   bool         `json:"lock_changed"`
	ManagedFiles  int          `json:"managed_files"`
	ConflictCount int          `json:"conflict_count"`
	Actions       ActionCounts `json:"actions"`
	Error         string       `json:"error,omitempty"`
}

type Probe struct {
	State  string   `json:"state"`
	CWD    string   `json:"cwd"`
	Argv   []string `json:"argv"`
	Detail string   `json:"detail,omitempty"`
}

type PendingChanges struct {
	New      int `json:"new"`
	Modified int `json:"modified"`
	Deleted  int `json:"deleted"`
	Total    int `json:"total"`
}

type Index struct {
	State        string          `json:"state"`
	Registered   *bool           `json:"registered,omitempty"`
	Fresh        *bool           `json:"fresh,omitempty"`
	Files        int             `json:"files,omitempty"`
	Nodes        int             `json:"nodes,omitempty"`
	Edges        int             `json:"edges,omitempty"`
	Vectors      int             `json:"vectors,omitempty"`
	PreciseEdges int             `json:"precise_edges,omitempty"`
	Chunks       int             `json:"chunks,omitempty"`
	Embeddings   int             `json:"embeddings,omitempty"`
	Pending      *PendingChanges `json:"pending,omitempty"`
}

type Profile struct {
	Status         string `json:"status,omitempty"`
	Matches        *bool  `json:"matches,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Model          string `json:"model,omitempty"`
	Dimensions     int    `json:"dimensions,omitempty"`
	ProviderHealth string `json:"provider_health"`
}

type Integration struct {
	Name       string   `json:"name"`
	Selected   bool     `json:"selected"`
	Available  bool     `json:"available"`
	BinaryPath string   `json:"binary_path,omitempty"`
	Probe      Probe    `json:"probe"`
	Index      Index    `json:"index"`
	ProjectKey string   `json:"project_key,omitempty"`
	Profile    *Profile `json:"profile,omitempty"`
}

// CommandAction is intentionally argv-shaped. Consumers must make an explicit
// authority decision instead of copying a shell string.
type CommandAction struct {
	Reason                    string   `json:"reason"`
	CWD                       string   `json:"cwd"`
	Argv                      []string `json:"argv"`
	RequiresExplicitAuthority bool     `json:"requires_explicit_authority"`
}

type Report struct {
	SchemaVersion int             `json:"schema_version"`
	Workspace     string          `json:"workspace"`
	Repository    Repository      `json:"repository"`
	Integrations  []Integration   `json:"integrations"`
	Degraded      bool            `json:"degraded"`
	Warnings      []string        `json:"warnings"`
	NextActions   []CommandAction `json:"next_actions"`
}

// Run inspects a workspace. Specialist processes run only when explicitly
// requested because their current status commands may open tool-owned stores,
// migrate metadata, or contact a configured provider.
func Run(ctx context.Context, root string, opts Options, runner Runner) (Report, error) {
	canonical, err := workspace.Resolve(root, true)
	if err != nil {
		return Report{}, err
	}
	if runner == nil {
		runner = ExecRunner{}
	}
	if opts.ProbeTimeout <= 0 {
		opts.ProbeTimeout = 10 * time.Second
	}
	report := Report{
		SchemaVersion: SchemaVersion,
		Workspace:     canonical,
		Repository: Repository{
			State:        "missing_manifest",
			ManifestPath: filepath.Join(canonical, manifest.Filename),
		},
		Warnings:    []string{},
		NextActions: []CommandAction{},
	}
	if opts.ProbeIntegrations {
		report.Warnings = append(report.Warnings, "specialist status probes were explicitly requested; Codemap may open tool-owned stores and Vecgrep may contact its configured provider")
	}

	m, manifestOK := inspectRepository(canonical, &report)
	selected := map[string]bool{}
	if manifestOK {
		selected["codemap"] = m.Integrations.CodeStructure == "codemap"
		selected["vecgrep"] = m.Integrations.SemanticSearch == "vecgrep"
	}
	for _, name := range []string{"codemap", "vecgrep"} {
		integration := inspectIntegration(ctx, canonical, name, selected[name], opts, runner)
		report.Integrations = append(report.Integrations, integration)
		mergeIntegration(&report, integration, opts.ProbeIntegrations)
	}
	return report, nil
}

func inspectRepository(root string, report *Report) (manifest.Manifest, bool) {
	if _, err := os.Stat(report.Repository.ManifestPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			report.Repository.Error = "bob.yaml is missing"
			report.Warnings = append(report.Warnings, "workspace is not initialized for Bob")
			report.NextActions = append(report.NextActions, CommandAction{
				Reason: "initialize a Bob manifest after choosing the public module path", CWD: root,
				Argv: []string{"bob", "init", ".", "--module", "<module>", "--write"}, RequiresExplicitAuthority: true,
			})
			return manifest.Manifest{}, false
		}
		report.Repository.State = "invalid_manifest"
		report.Repository.Error = boundedDetail(err.Error())
		report.Warnings = append(report.Warnings, "bob.yaml could not be read")
		return manifest.Manifest{}, false
	}
	m, err := manifest.Load(root)
	if err != nil {
		report.Repository.State = "invalid_manifest"
		report.Repository.Error = boundedDetail(err.Error())
		report.Warnings = append(report.Warnings, "bob.yaml is invalid")
		return manifest.Manifest{}, false
	}
	report.Repository.Recipe = m.Recipe
	artifacts, err := recipe.Render(m)
	if err != nil {
		report.Repository.State = "plan_error"
		report.Repository.Error = boundedDetail(err.Error())
		report.Warnings = append(report.Warnings, "recipe rendering failed")
		return m, true
	}
	plan, err := engine.Plan(root, m, artifacts)
	if err != nil {
		report.Repository.State = "plan_error"
		report.Repository.Error = boundedDetail(err.Error())
		report.Warnings = append(report.Warnings, "repository planning failed")
		return m, true
	}
	report.Repository.ManagedFiles = len(plan.Actions)
	report.Repository.LockChanged = plan.LockChanged
	report.Repository.ConflictCount = plan.ConflictCount
	for _, action := range plan.Actions {
		switch action.Kind {
		case engine.ActionCreate:
			report.Repository.Actions.Create++
		case engine.ActionUpdate:
			report.Repository.Actions.Update++
		case engine.ActionAdopt:
			report.Repository.Actions.Adopt++
		case engine.ActionUnchanged:
			report.Repository.Actions.Unchanged++
		case engine.ActionConflict:
			report.Repository.Actions.Conflict++
		}
	}
	report.Repository.Converged = !plan.LockChanged && report.Repository.Actions.Unchanged == len(plan.Actions)
	report.Repository.Ready = !plan.HasConflicts()
	switch {
	case plan.HasConflicts():
		report.Repository.State = "conflicted"
		report.Warnings = append(report.Warnings, fmt.Sprintf("%d Bob plan conflict(s) block apply", plan.ConflictCount))
		report.NextActions = append(report.NextActions, CommandAction{
			Reason: "review and resolve Bob ownership conflicts", CWD: root,
			Argv: []string{"bob", "plan", "."}, RequiresExplicitAuthority: false,
		})
	case report.Repository.Converged:
		report.Repository.State = "clean"
	default:
		report.Repository.State = "drifted"
		report.NextActions = append(report.NextActions, CommandAction{
			Reason: "review the complete conflict-free plan before applying", CWD: root,
			Argv: []string{"bob", "plan", "."}, RequiresExplicitAuthority: false,
		})
	}
	return m, true
}

func inspectIntegration(ctx context.Context, root, name string, selected bool, opts Options, runner Runner) Integration {
	integration := Integration{
		Name: name, Selected: selected,
		Probe: Probe{State: ProbeNotSelected, CWD: root, Argv: []string{}},
		Index: Index{State: IndexUnknown},
	}
	path, err := runner.LookPath(name)
	if err == nil {
		integration.Available = true
		integration.BinaryPath = path
	}
	if !selected {
		return integration
	}
	integration.Probe.State = ProbeNotRequested
	if !integration.Available {
		integration.Probe.State = ProbeUnavailable
		return integration
	}
	if !opts.ProbeIntegrations {
		return integration
	}

	args := []string{"status", "--format", "json"}
	if name == "codemap" {
		args = []string{"--json", "--path", root, "status"}
	}
	integration.Probe.Argv = append([]string{name}, args...)
	probeCtx, cancel := context.WithTimeout(ctx, opts.ProbeTimeout)
	result := runner.Run(probeCtx, root, name, args...)
	cancel()
	if result.TimedOut {
		integration.Probe.State = ProbeTimedOut
		integration.Probe.Detail = "status probe exceeded its deadline"
		return integration
	}
	if result.StdoutTruncated || result.StderrTruncated {
		integration.Probe.State = ProbeInvalid
		integration.Probe.Detail = "status output exceeded Bob's capture limit"
		return integration
	}
	if result.Err != nil {
		integration.Probe.State = ProbeFailed
		integration.Probe.Detail = boundedDetail(string(result.Stderr))
		if integration.Probe.Detail == "" {
			integration.Probe.Detail = boundedDetail(result.Err.Error())
		}
		return integration
	}
	if name == "codemap" {
		return decodeCodemap(root, integration, result.Stdout)
	}
	return decodeVecgrep(root, integration, result.Stdout)
}

type codemapStatus struct {
	OK           *bool  `json:"ok"`
	Error        string `json:"error"`
	Code         string `json:"code"`
	Root         string `json:"root"`
	Registered   bool   `json:"registered"`
	Nodes        int    `json:"nodes"`
	Edges        int    `json:"edges"`
	Files        int    `json:"files"`
	Vectors      int    `json:"vectors"`
	PreciseEdges int    `json:"precise_edges"`
	ProjectKey   string `json:"project_key"`
	Stale        *struct {
		Changed int `json:"changed"`
		New     int `json:"new"`
		Deleted int `json:"deleted"`
	} `json:"stale"`
}

func decodeCodemap(root string, integration Integration, data []byte) Integration {
	var status codemapStatus
	if err := decodeOne(data, &status); err != nil {
		return invalidProbe(integration, err)
	}
	if status.OK != nil && !*status.OK {
		integration.Probe.State = ProbeFailed
		integration.Probe.Detail = boundedDetail(strings.TrimSpace(status.Code + ": " + status.Error))
		return integration
	}
	if !sameWorkspace(root, status.Root) {
		integration.Probe.State = ProbeWrongProject
		integration.Probe.Detail = "Codemap returned status for a different project root"
		return integration
	}
	integration.Probe.State = ProbeComplete
	integration.ProjectKey = status.ProjectKey
	registered := status.Registered
	integration.Index = Index{
		State: IndexUnknown, Registered: &registered, Files: status.Files, Nodes: status.Nodes,
		Edges: status.Edges, Vectors: status.Vectors, PreciseEdges: status.PreciseEdges,
	}
	if !status.Registered || status.Files == 0 || status.Nodes == 0 {
		integration.Index.State = IndexNotIndexed
		return integration
	}
	if status.Stale == nil {
		return integration
	}
	pending := PendingChanges{New: status.Stale.New, Modified: status.Stale.Changed, Deleted: status.Stale.Deleted}
	pending.Total = pending.New + pending.Modified + pending.Deleted
	integration.Index.Pending = &pending
	fresh := pending.Total == 0
	integration.Index.Fresh = &fresh
	if fresh {
		integration.Index.State = IndexFresh
	} else {
		integration.Index.State = IndexStale
	}
	return integration
}

type vecgrepStatus struct {
	ProjectRoot    string `json:"project_root"`
	Provider       string `json:"provider"`
	EmbeddingModel string `json:"embedding_model"`
	Dimensions     int    `json:"dimensions"`
	ProfileStatus  string `json:"profile_status"`
	ProfileMatches *bool  `json:"profile_matches"`
	IndexFresh     *bool  `json:"index_fresh"`
	Stats          struct {
		Files      int `json:"files"`
		Chunks     int `json:"chunks"`
		Embeddings int `json:"embeddings"`
	} `json:"stats"`
	Pending *struct {
		New      int `json:"new_files"`
		Modified int `json:"modified_files"`
		Deleted  int `json:"deleted_files"`
		Total    int `json:"total_pending"`
	} `json:"pending_changes"`
}

func decodeVecgrep(root string, integration Integration, data []byte) Integration {
	var status vecgrepStatus
	if err := decodeOne(data, &status); err != nil {
		return invalidProbe(integration, err)
	}
	if !sameWorkspace(root, status.ProjectRoot) {
		integration.Probe.State = ProbeWrongProject
		integration.Probe.Detail = "Vecgrep returned status for a different project root"
		return integration
	}
	integration.Probe.State = ProbeComplete
	integration.Index = Index{
		State: IndexUnknown, Fresh: status.IndexFresh, Files: status.Stats.Files,
		Chunks: status.Stats.Chunks, Embeddings: status.Stats.Embeddings,
	}
	integration.Profile = &Profile{
		Status: status.ProfileStatus, Matches: status.ProfileMatches, Provider: status.Provider,
		Model: status.EmbeddingModel, Dimensions: status.Dimensions, ProviderHealth: "not_reported",
	}
	if status.Pending != nil {
		integration.Index.Pending = &PendingChanges{
			New: status.Pending.New, Modified: status.Pending.Modified,
			Deleted: status.Pending.Deleted, Total: status.Pending.Total,
		}
	}
	switch {
	case status.Stats.Files == 0 || status.Stats.Chunks == 0:
		integration.Index.State = IndexNotIndexed
	case status.ProfileMatches != nil && !*status.ProfileMatches:
		integration.Index.State = IndexIncompatible
	case status.Pending != nil && status.Pending.Total > 0:
		integration.Index.State = IndexStale
	case status.IndexFresh != nil && *status.IndexFresh:
		integration.Index.State = IndexFresh
	}
	return integration
}

func mergeIntegration(report *Report, integration Integration, probed bool) {
	if !integration.Selected {
		return
	}
	if integration.Probe.State != ProbeComplete || integration.Index.State != IndexFresh {
		report.Degraded = true
	}
	switch integration.Probe.State {
	case ProbeNotRequested:
		report.Warnings = append(report.Warnings, integration.Name+" index status was not probed")
		appendAction(report, CommandAction{
			Reason: "explicitly probe selected specialist index status", CWD: report.Workspace,
			Argv: []string{"bob", "inspect", ".", "--probe-integrations"}, RequiresExplicitAuthority: true,
		})
	case ProbeUnavailable:
		report.Warnings = append(report.Warnings, integration.Name+" is selected but its binary is unavailable")
	case ProbeComplete:
		mergeIndexState(report, integration)
	default:
		report.Warnings = append(report.Warnings, fmt.Sprintf("%s status probe %s: %s", integration.Name, integration.Probe.State, integration.Probe.Detail))
	}
	if probed && integration.Name == "vecgrep" && integration.Probe.State == ProbeComplete {
		report.Warnings = append(report.Warnings, "Vecgrep status does not report live provider health")
	}
}

func appendAction(report *Report, candidate CommandAction) {
	for _, action := range report.NextActions {
		if action.CWD == candidate.CWD && equalArgv(action.Argv, candidate.Argv) {
			return
		}
	}
	report.NextActions = append(report.NextActions, candidate)
}

func equalArgv(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func mergeIndexState(report *Report, integration Integration) {
	switch integration.Index.State {
	case IndexNotIndexed:
		report.Warnings = append(report.Warnings, integration.Name+" is selected but not indexed")
		if integration.Name == "codemap" {
			report.NextActions = append(report.NextActions,
				CommandAction{Reason: "register the Codemap project", CWD: report.Workspace, Argv: []string{"codemap", "--path", report.Workspace, "init"}, RequiresExplicitAuthority: true},
				CommandAction{Reason: "build the Codemap structural index", CWD: report.Workspace, Argv: []string{"codemap", "--path", report.Workspace, "index", "--precise"}, RequiresExplicitAuthority: true},
			)
		} else {
			report.NextActions = append(report.NextActions,
				CommandAction{Reason: "register the Vecgrep project", CWD: report.Workspace, Argv: []string{"vecgrep", "init"}, RequiresExplicitAuthority: true},
				CommandAction{Reason: "build the Vecgrep semantic index", CWD: report.Workspace, Argv: []string{"vecgrep", "index", "--no-progress"}, RequiresExplicitAuthority: true},
			)
		}
	case IndexStale:
		report.Warnings = append(report.Warnings, integration.Name+" index is stale")
		argv := []string{"vecgrep", "index", "--no-progress"}
		if integration.Name == "codemap" {
			argv = []string{"codemap", "--path", report.Workspace, "index", "--precise"}
		}
		report.NextActions = append(report.NextActions, CommandAction{Reason: "refresh the stale " + integration.Name + " index", CWD: report.Workspace, Argv: argv, RequiresExplicitAuthority: true})
	case IndexIncompatible:
		report.Warnings = append(report.Warnings, integration.Name+" index profile is incompatible with current configuration")
	case IndexUnknown:
		report.Warnings = append(report.Warnings, integration.Name+" index freshness is unknown")
	}
}

func decodeOne(data []byte, dst any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON documents")
		}
		return fmt.Errorf("trailing JSON: %w", err)
	}
	return nil
}

func invalidProbe(integration Integration, err error) Integration {
	integration.Probe.State = ProbeInvalid
	integration.Probe.Detail = boundedDetail(err.Error())
	return integration
}

func sameWorkspace(want, got string) bool {
	if strings.TrimSpace(got) == "" {
		return false
	}
	canonical, err := workspace.Resolve(got, true)
	return err == nil && canonical == want
}

func boundedDetail(value string) string {
	value = strings.TrimSpace(value)
	if line, _, ok := strings.Cut(value, "\n"); ok {
		value = strings.TrimSpace(line)
	}
	const limit = 512
	if len(value) > limit {
		return value[:limit] + "…"
	}
	return value
}
