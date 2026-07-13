package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.yaml.in/yaml/v3"
)

const (
	toolSchemaVersion     = 1
	defaultMaxActions     = 100
	maximumMaxActions     = 500
	planOutputByteLimit   = 30 << 10
	maxInlineManifestSize = 64 << 10
	currentRecipeID       = "go-agent-tool"
)

type WorkspaceInput struct {
	Workspace string `json:"workspace,omitempty" jsonschema:"existing repository directory authorized at server startup; defaults to the startup workspace"`
}

type PlanInput struct {
	Workspace        string `json:"workspace,omitempty" jsonschema:"existing repository directory authorized at server startup; defaults to the startup workspace"`
	IncludeUnchanged bool   `json:"include_unchanged,omitempty" jsonschema:"include unchanged actions in the bounded action projection; defaults to false"`
	MaxActions       int    `json:"max_actions,omitempty" jsonschema:"maximum number of projected actions to return; 0 uses the default of 100; minimum 1 and maximum 500"`
}

type ValidateManifestInput struct {
	Workspace    string `json:"workspace,omitempty" jsonschema:"authorized workspace whose bob.yaml should be validated; mutually exclusive with manifest_yaml"`
	ManifestYAML string `json:"manifest_yaml,omitempty" jsonschema:"inline Bob manifest YAML up to 65536 bytes; mutually exclusive with workspace"`
}

type RecipeDescribeInput struct {
	Recipe string `json:"recipe,omitempty" jsonschema:"embedded recipe ID; defaults to go-agent-tool"`
}

type StatsInput struct {
	Workspace string `json:"workspace,omitempty" jsonschema:"authorized workspace to summarize; defaults to the startup workspace and is mutually exclusive with all"`
	All       bool   `json:"all,omitempty" jsonschema:"summarize all retained pseudonymous workspaces; mutually exclusive with workspace"`
	SinceDays int    `json:"since_days,omitempty" jsonschema:"UTC lookback in days; 0 uses 7, maximum 365"`
}

type ErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type InspectOutput struct {
	SchemaVersion int                `json:"schema_version"`
	OK            bool               `json:"ok"`
	Workspace     string             `json:"workspace,omitempty"`
	Authority     AuthorityInfo      `json:"authority"`
	Report        *inspectpkg.Report `json:"report,omitempty"`
	Error         *ErrorInfo         `json:"error,omitempty"`
}

type PlanAction struct {
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	Code          string `json:"code,omitempty"`
	CurrentSHA256 string `json:"current_sha256,omitempty"`
	DesiredSHA256 string `json:"desired_sha256,omitempty"`
	LockedSHA256  string `json:"locked_sha256,omitempty"`
	CurrentMode   string `json:"current_mode,omitempty"`
	DesiredMode   string `json:"desired_mode,omitempty"`
	Reason        string `json:"reason,omitempty"`
}

// TruncationInfo distinguishes intentionally filtered unchanged actions from
// actions omitted by the caller's count limit or the transport byte budget.
type TruncationInfo struct {
	IncludeUnchanged  bool `json:"include_unchanged"`
	MaxActions        int  `json:"max_actions"`
	TotalActions      int  `json:"total_actions"`
	EligibleActions   int  `json:"eligible_actions"`
	FilteredUnchanged int  `json:"filtered_unchanged"`
	ReturnedActions   int  `json:"returned_actions"`
	OmittedActions    int  `json:"omitted_actions"`
	Truncated         bool `json:"truncated"`
	OutputByteLimit   int  `json:"output_byte_limit"`
	ByteLimitApplied  bool `json:"byte_limit_applied"`
}

type PlanOutput struct {
	SchemaVersion int                        `json:"schema_version"`
	OK            bool                       `json:"ok"`
	Workspace     string                     `json:"workspace,omitempty"`
	Authority     AuthorityInfo              `json:"authority"`
	PlanDigest    string                     `json:"plan_digest,omitempty"`
	Clean         bool                       `json:"clean"`
	LockChanged   bool                       `json:"lock_changed"`
	ConflictCount int                        `json:"conflict_count"`
	Counts        inspectpkg.ActionCounts    `json:"counts"`
	Actions       []PlanAction               `json:"actions"`
	Truncation    TruncationInfo             `json:"truncation"`
	Warnings      []string                   `json:"warnings"`
	NextActions   []inspectpkg.CommandAction `json:"next_actions"`
	Error         *ErrorInfo                 `json:"error,omitempty"`
}

type CheckOutput struct {
	SchemaVersion int                        `json:"schema_version"`
	OK            bool                       `json:"ok"`
	Workspace     string                     `json:"workspace,omitempty"`
	Authority     AuthorityInfo              `json:"authority"`
	PlanDigest    string                     `json:"plan_digest,omitempty"`
	Clean         bool                       `json:"clean"`
	LockChanged   bool                       `json:"lock_changed"`
	ConflictCount int                        `json:"conflict_count"`
	Counts        inspectpkg.ActionCounts    `json:"counts"`
	Warnings      []string                   `json:"warnings"`
	NextActions   []inspectpkg.CommandAction `json:"next_actions"`
	Error         *ErrorInfo                 `json:"error,omitempty"`
}

type RecipeRef struct {
	ID      string `json:"id"`
	Version int    `json:"version"`
}

type ValidateManifestOutput struct {
	SchemaVersion         int                `json:"schema_version"`
	OK                    bool               `json:"ok"`
	Source                string             `json:"source,omitempty"`
	Workspace             string             `json:"workspace,omitempty"`
	Authority             AuthorityInfo      `json:"authority"`
	ManifestSchemaVersion int                `json:"manifest_schema_version,omitempty"`
	Recipe                *RecipeRef         `json:"recipe,omitempty"`
	Manifest              *manifest.Manifest `json:"manifest,omitempty"`
	Warnings              []string           `json:"warnings"`
	Error                 *ErrorInfo         `json:"error,omitempty"`
}

type RecipeChoice struct {
	Field  string   `json:"field"`
	Values []string `json:"values"`
}

type RecipeDescription struct {
	ID                    string         `json:"id"`
	Version               int            `json:"version"`
	ManifestSchemaVersion int            `json:"manifest_schema_version"`
	Description           string         `json:"description"`
	Surfaces              []string       `json:"surfaces"`
	SupportedChoices      []RecipeChoice `json:"supported_choices"`
}

type RecipeDescribeOutput struct {
	SchemaVersion int                `json:"schema_version"`
	OK            bool               `json:"ok"`
	Recipe        *RecipeDescription `json:"recipe,omitempty"`
	Error         *ErrorInfo         `json:"error,omitempty"`
}

type StatsOutput struct {
	SchemaVersion int             `json:"schema_version"`
	OK            bool            `json:"ok"`
	Enabled       bool            `json:"enabled"`
	LocalOnly     bool            `json:"local_only"`
	Authority     AuthorityInfo   `json:"authority"`
	Stats         telemetry.Stats `json:"stats"`
	Error         *ErrorInfo      `json:"error,omitempty"`
}

func (s *Server) handleInspect(ctx context.Context, _ *sdkmcp.CallToolRequest, in WorkspaceInput) (*sdkmcp.CallToolResult, *InspectOutput, error) {
	started := time.Now()
	root := ""
	outcome, reason := telemetry.OutcomeError, telemetry.ReasonInternal
	var recordedCounts inspectpkg.ActionCounts
	recipeSelected := false
	defer func() {
		s.recordOperation(ctx, telemetry.OperationInspect, root, outcome, reason, recordedCounts, recipeSelected, started)
	}()
	var authErr *authorityError
	root, authErr = s.authority.resolve(in.Workspace)
	if authErr != nil {
		reason = reasonFromToolCode(authErr.code)
		out := &InspectOutput{
			SchemaVersion: toolSchemaVersion, OK: false, Authority: s.authority.info(""),
			Error: &ErrorInfo{Code: authErr.code, Message: authErr.message},
		}
		return &sdkmcp.CallToolResult{IsError: true}, out, nil
	}
	report, err := inspectpkg.Run(ctx, root, inspectpkg.Options{}, s.runner)
	if err != nil {
		out := &InspectOutput{
			SchemaVersion: toolSchemaVersion, OK: false, Workspace: root, Authority: s.authority.info(root),
			Error: &ErrorInfo{Code: "inspect_failed", Message: err.Error()},
		}
		return &sdkmcp.CallToolResult{IsError: true}, out, nil
	}
	outcome, reason = telemetry.OutcomeOK, ""
	recordedCounts = report.Repository.Actions
	recipeSelected = report.Repository.Recipe == currentRecipeID
	out := &InspectOutput{
		SchemaVersion: toolSchemaVersion, OK: true, Workspace: root,
		Authority: s.authority.info(root), Report: &report,
	}
	return &sdkmcp.CallToolResult{}, out, nil
}

func (s *Server) handlePlan(ctx context.Context, _ *sdkmcp.CallToolRequest, in PlanInput) (*sdkmcp.CallToolResult, *PlanOutput, error) {
	started := time.Now()
	root := ""
	outcome, reason := telemetry.OutcomeError, telemetry.ReasonInternal
	var recordedCounts inspectpkg.ActionCounts
	recipeSelected := false
	defer func() {
		s.recordOperation(ctx, telemetry.OperationPlan, root, outcome, reason, recordedCounts, recipeSelected, started)
	}()
	maxActions, err := normalizeMaxActions(in.MaxActions)
	if err != nil {
		reason = telemetry.ReasonInvalidInput
		return s.planFailure("", "input_invalid", err, in.IncludeUnchanged, defaultMaxActions)
	}
	var authErr *authorityError
	root, authErr = s.authority.resolve(in.Workspace)
	if authErr != nil {
		reason = reasonFromToolCode(authErr.code)
		return s.planFailure("", authErr.code, authErr, in.IncludeUnchanged, maxActions)
	}
	plan, code, err := buildPlan(root)
	if err != nil {
		reason = reasonFromToolCode(code)
		return s.planFailure(root, code, err, in.IncludeUnchanged, maxActions)
	}
	recipeSelected = plan.Recipe.ID == currentRecipeID
	out := s.projectPlan(root, plan, in.IncludeUnchanged, maxActions)
	recordedCounts = out.Counts
	outcome, reason = telemetry.OutcomeOK, ""
	if plan.HasConflicts() {
		outcome, reason = telemetry.OutcomeConflict, telemetry.ReasonOwnershipConflict
	}
	return &sdkmcp.CallToolResult{}, out, nil
}

func (s *Server) handleCheck(ctx context.Context, _ *sdkmcp.CallToolRequest, in WorkspaceInput) (*sdkmcp.CallToolResult, *CheckOutput, error) {
	started := time.Now()
	root := ""
	outcome, reason := telemetry.OutcomeError, telemetry.ReasonInternal
	var recordedCounts inspectpkg.ActionCounts
	recipeSelected := false
	defer func() {
		s.recordOperation(ctx, telemetry.OperationCheck, root, outcome, reason, recordedCounts, recipeSelected, started)
	}()
	var authErr *authorityError
	root, authErr = s.authority.resolve(in.Workspace)
	if authErr != nil {
		reason = reasonFromToolCode(authErr.code)
		return s.checkFailure("", authErr.code, authErr)
	}
	plan, code, err := buildPlan(root)
	if err != nil {
		reason = reasonFromToolCode(code)
		return s.checkFailure(root, code, err)
	}
	recipeSelected = plan.Recipe.ID == currentRecipeID
	counts, clean, warnings, nextActions := summarizePlan(root, plan)
	recordedCounts = counts
	outcome, reason = telemetry.OutcomeOK, ""
	if plan.HasConflicts() {
		outcome, reason = telemetry.OutcomeConflict, telemetry.ReasonOwnershipConflict
	} else if !clean {
		outcome, reason = telemetry.OutcomeDrift, telemetry.ReasonOwnershipConflict
	}
	out := &CheckOutput{
		SchemaVersion: toolSchemaVersion, OK: true, Workspace: root, Authority: s.authority.info(root),
		PlanDigest: digestPlan(plan), Clean: clean, LockChanged: plan.LockChanged,
		ConflictCount: plan.ConflictCount, Counts: counts, Warnings: warnings, NextActions: nextActions,
	}
	return &sdkmcp.CallToolResult{}, out, nil
}

func (s *Server) handleValidateManifest(ctx context.Context, _ *sdkmcp.CallToolRequest, in ValidateManifestInput) (*sdkmcp.CallToolResult, *ValidateManifestOutput, error) {
	started := time.Now()
	recordedRoot := ""
	outcome, reason := telemetry.OutcomeError, telemetry.ReasonInternal
	recipeSelected := false
	defer func() {
		s.recordOperation(ctx, telemetry.OperationValidateManifest, recordedRoot, outcome, reason, inspectpkg.ActionCounts{}, recipeSelected, started)
	}()
	hasWorkspace := strings.TrimSpace(in.Workspace) != ""
	hasInline := in.ManifestYAML != ""
	if hasWorkspace == hasInline {
		reason = telemetry.ReasonInvalidInput
		return s.manifestFailure("", "", "input_invalid", errors.New("provide exactly one of workspace or manifest_yaml"))
	}

	var (
		m      manifest.Manifest
		root   string
		source string
		err    error
	)
	if hasWorkspace {
		source = "workspace"
		var authErr *authorityError
		root, authErr = s.authority.resolve(in.Workspace)
		if authErr != nil {
			reason = reasonFromToolCode(authErr.code)
			return s.manifestFailure(source, "", authErr.code, authErr)
		}
		recordedRoot = root
		m, err = manifest.Load(root)
	} else {
		source = "inline"
		if len(in.ManifestYAML) > maxInlineManifestSize {
			reason = telemetry.ReasonInvalidInput
			return s.manifestFailure(source, "", "manifest_too_large", fmt.Errorf("manifest_yaml exceeds the %d-byte input limit", maxInlineManifestSize))
		}
		m, err = decodeManifest([]byte(in.ManifestYAML))
	}
	if err != nil {
		reason = telemetry.ReasonInvalidManifest
		return s.manifestFailure(source, root, "manifest_invalid", err)
	}
	recipeSelected = m.Recipe == currentRecipeID
	recipeVersion, err := recipe.Version(m.Recipe)
	if err != nil {
		reason = telemetry.ReasonInvalidManifest
		return s.manifestFailure(source, root, "recipe_invalid", err)
	}
	out := &ValidateManifestOutput{
		SchemaVersion: toolSchemaVersion, OK: true, Source: source, Workspace: root,
		Authority: s.authority.info(root), ManifestSchemaVersion: manifest.SchemaVersion,
		Recipe: &RecipeRef{ID: m.Recipe, Version: recipeVersion}, Manifest: &m,
		Warnings: []string{},
	}
	outcome, reason = telemetry.OutcomeOK, ""
	return &sdkmcp.CallToolResult{}, out, nil
}

func (s *Server) handleRecipeDescribe(ctx context.Context, _ *sdkmcp.CallToolRequest, in RecipeDescribeInput) (*sdkmcp.CallToolResult, *RecipeDescribeOutput, error) {
	started := time.Now()
	outcome, reason := telemetry.OutcomeError, telemetry.ReasonInvalidInput
	recipeSelected := false
	defer func() {
		s.recordOperation(ctx, telemetry.OperationRecipeDescribe, "", outcome, reason, inspectpkg.ActionCounts{}, recipeSelected, started)
	}()
	id := in.Recipe
	if id == "" {
		id = currentRecipeID
	}
	version, err := recipe.Version(id)
	if err != nil {
		out := &RecipeDescribeOutput{
			SchemaVersion: toolSchemaVersion, OK: false,
			Error: &ErrorInfo{Code: "recipe_unknown", Message: fmt.Sprintf("unknown recipe %q", id)},
		}
		return &sdkmcp.CallToolResult{IsError: true}, out, nil
	}
	recipeSelected = id == currentRecipeID
	out := &RecipeDescribeOutput{
		SchemaVersion: toolSchemaVersion, OK: true,
		Recipe: recipeDescription(id, version),
	}
	outcome, reason = telemetry.OutcomeOK, ""
	return &sdkmcp.CallToolResult{}, out, nil
}

func recipeDescription(id string, version int) *RecipeDescription {
	if id == "files" {
		return &RecipeDescription{
			ID: id, Version: version, ManifestSchemaVersion: manifest.SchemaVersion,
			Description: "Declares an arbitrary file tree inline in bob.yaml; Bob materializes it with the same plan/apply ownership and path safety as every other recipe. Substitution is a single ${vars.<key>} literal-replacement pass, not a template language; an undeclared reference is a render-time error. Bob owns file ownership and convergence, not the meaning of file content over time.",
			Surfaces:    []string{"cli", "json"},
		}
	}
	return &RecipeDescription{
		ID: id, Version: version, ManifestSchemaVersion: manifest.SchemaVersion,
		Description: "Public-ready Go and Cobra CLI with docs, CI, release plumbing, and optional ecosystem seams",
		Surfaces:    []string{"cli", "json"},
		SupportedChoices: []RecipeChoice{
			{Field: "integrations.code_structure", Values: []string{"none", "codemap"}},
			{Field: "integrations.semantic_search", Values: []string{"none", "vecgrep"}},
			{Field: "integrations.terminal_verification", Values: []string{"none", "glyphrun"}},
			{Field: "integrations.browser_verification", Values: []string{"none", "cairntrace"}},
			{Field: "integrations.secrets", Values: []string{"none", "tinyvault"}},
			{Field: "integrations.artifacts", Values: []string{"none", "fcheap"}},
			{Field: "distribution.docs", Values: []string{"none", "markdown"}},
		},
	}
}

func (s *Server) handleStats(ctx context.Context, _ *sdkmcp.CallToolRequest, in StatsInput) (*sdkmcp.CallToolResult, *StatsOutput, error) {
	days := in.SinceDays
	if days == 0 {
		days = 7
	}
	if days < 1 || days > 365 {
		return s.statsFailure("", "input_invalid", fmt.Errorf("since_days must be between 1 and 365, or 0 for the default"))
	}
	if in.All && strings.TrimSpace(in.Workspace) != "" {
		return s.statsFailure("", "input_invalid", errors.New("workspace and all are mutually exclusive"))
	}
	root := ""
	query := telemetry.Query{Since: time.Now().UTC().AddDate(0, 0, -days)}
	if !in.All {
		var authErr *authorityError
		root, authErr = s.authority.resolve(in.Workspace)
		if authErr != nil {
			return s.statsFailure("", authErr.code, authErr)
		}
	}
	out := &StatsOutput{
		SchemaVersion: toolSchemaVersion, OK: true, LocalOnly: true,
		Authority: s.authority.info(root),
		Stats:     telemetry.Stats{SchemaVersion: telemetry.SchemaVersion, Since: query.Since, Until: time.Now().UTC()},
	}
	if s.telemetry == nil || !s.telemetry.Enabled() {
		return &sdkmcp.CallToolResult{}, out, nil
	}
	out.Enabled = true
	if root != "" {
		workspaceID, err := s.telemetry.WorkspaceID(root)
		if err != nil {
			return s.statsFailure(root, "stats_failed", err)
		}
		query.WorkspaceID = workspaceID
	}
	stats, err := s.telemetry.Aggregate(ctx, query)
	if err != nil {
		return s.statsFailure(root, "stats_failed", err)
	}
	out.Stats = stats
	return &sdkmcp.CallToolResult{}, out, nil
}

func (s *Server) statsFailure(root, code string, err error) (*sdkmcp.CallToolResult, *StatsOutput, error) {
	out := &StatsOutput{
		SchemaVersion: toolSchemaVersion, OK: false, LocalOnly: true,
		Authority: s.authority.info(root),
		Stats:     telemetry.Stats{SchemaVersion: telemetry.SchemaVersion},
		Error:     &ErrorInfo{Code: code, Message: err.Error()},
	}
	return &sdkmcp.CallToolResult{IsError: true}, out, nil
}

func buildPlan(root string) (engine.PlanResult, string, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return engine.PlanResult{}, "manifest_invalid", err
	}
	artifacts, err := recipe.Render(m)
	if err != nil {
		return engine.PlanResult{}, "recipe_invalid", err
	}
	plan, err := engine.Plan(root, m, artifacts)
	if err != nil {
		return engine.PlanResult{}, "plan_failed", err
	}
	return plan, "", nil
}

func normalizeMaxActions(value int) (int, error) {
	if value == 0 {
		return defaultMaxActions, nil
	}
	if value < 1 || value > maximumMaxActions {
		return 0, fmt.Errorf("max_actions must be between 1 and %d, or 0 for the default", maximumMaxActions)
	}
	return value, nil
}

func projectAction(action engine.Action) PlanAction {
	projected := PlanAction{
		Path: action.Path, Kind: string(action.Kind), Code: action.Code, CurrentSHA256: action.CurrentSHA256,
		DesiredSHA256: action.DesiredSHA256, LockedSHA256: action.LockedSHA256,
		DesiredMode: fmt.Sprintf("%04o", action.DesiredMode.Perm()), Reason: action.Reason,
	}
	if action.CurrentMode != 0 {
		projected.CurrentMode = fmt.Sprintf("%04o", action.CurrentMode.Perm())
	}
	return projected
}

func (s *Server) projectPlan(root string, plan engine.PlanResult, includeUnchanged bool, maxActions int) *PlanOutput {
	counts, clean, warnings, nextActions := summarizePlan(root, plan)
	eligible := make([]PlanAction, 0, len(plan.Actions))
	filteredUnchanged := 0
	for _, action := range plan.Actions {
		if !includeUnchanged && action.Kind == engine.ActionUnchanged {
			filteredUnchanged++
			continue
		}
		eligible = append(eligible, projectAction(action))
	}
	returned := len(eligible)
	if returned > maxActions {
		returned = maxActions
	}
	out := &PlanOutput{
		SchemaVersion: toolSchemaVersion, OK: true, Workspace: root, Authority: s.authority.info(root),
		PlanDigest: digestPlan(plan), Clean: clean, LockChanged: plan.LockChanged,
		ConflictCount: plan.ConflictCount, Counts: counts,
		Actions:  append([]PlanAction(nil), eligible[:returned]...),
		Warnings: warnings, NextActions: nextActions,
		Truncation: TruncationInfo{
			IncludeUnchanged: includeUnchanged, MaxActions: maxActions, TotalActions: len(plan.Actions),
			EligibleActions: len(eligible), FilteredUnchanged: filteredUnchanged,
			OutputByteLimit: planOutputByteLimit,
		},
	}
	for {
		out.Truncation.ReturnedActions = len(out.Actions)
		out.Truncation.OmittedActions = len(eligible) - len(out.Actions)
		out.Truncation.Truncated = out.Truncation.OmittedActions > 0
		data, err := json.Marshal(out)
		if err != nil || len(data) <= planOutputByteLimit || len(out.Actions) == 0 {
			break
		}
		out.Actions = out.Actions[:len(out.Actions)-1]
		out.Truncation.ByteLimitApplied = true
	}
	return out
}

func summarizePlan(root string, plan engine.PlanResult) (inspectpkg.ActionCounts, bool, []string, []inspectpkg.CommandAction) {
	var counts inspectpkg.ActionCounts
	for _, action := range plan.Actions {
		switch action.Kind {
		case engine.ActionCreate:
			counts.Create++
		case engine.ActionUpdate:
			counts.Update++
		case engine.ActionAdopt:
			counts.Adopt++
		case engine.ActionUnchanged:
			counts.Unchanged++
		case engine.ActionConflict:
			counts.Conflict++
		}
	}
	clean := !plan.LockChanged && counts.Unchanged == len(plan.Actions)
	warnings := []string{}
	nextActions := []inspectpkg.CommandAction{}
	switch {
	case plan.HasConflicts():
		warnings = append(warnings, fmt.Sprintf("%d conflict(s) block apply", plan.ConflictCount))
		nextActions = append(nextActions, inspectpkg.CommandAction{
			Reason: "resolve Bob ownership conflicts, then replan", CWD: root,
			Argv: []string{"bob", "plan", root}, RequiresExplicitAuthority: false,
		})
	case clean:
		// The empty continuation is intentional: the repository is converged.
	default:
		nextActions = append(nextActions, inspectpkg.CommandAction{
			Reason: "apply the reviewed conflict-free plan through the approved shell path", CWD: root,
			Argv: []string{"bob", "apply", root}, RequiresExplicitAuthority: true,
		})
	}
	return counts, clean, warnings, nextActions
}

func digestPlan(plan engine.PlanResult) string {
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
	}{
		SchemaVersion: plan.SchemaVersion, Recipe: plan.Recipe, LockChanged: plan.LockChanged,
		DesiredLock: plan.DesiredLock, Actions: actions,
	}
	data, _ := json.Marshal(identity)
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func decodeManifest(data []byte) (manifest.Manifest, error) {
	var m manifest.Manifest
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		if errors.Is(err, io.EOF) {
			return m, errors.New("decode manifest: document is empty")
		}
		return m, fmt.Errorf("decode manifest: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return m, errors.New("decode manifest: multiple YAML documents are not supported")
		}
		return m, fmt.Errorf("decode manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return m, fmt.Errorf("validate manifest: %w", err)
	}
	return m, nil
}

func (s *Server) planFailure(root, code string, err error, includeUnchanged bool, maxActions int) (*sdkmcp.CallToolResult, *PlanOutput, error) {
	out := &PlanOutput{
		SchemaVersion: toolSchemaVersion, OK: false, Workspace: root, Authority: s.authority.info(root),
		Actions: []PlanAction{}, Warnings: []string{}, NextActions: []inspectpkg.CommandAction{},
		Truncation: TruncationInfo{IncludeUnchanged: includeUnchanged, MaxActions: maxActions, OutputByteLimit: planOutputByteLimit},
		Error:      &ErrorInfo{Code: code, Message: err.Error()},
	}
	return &sdkmcp.CallToolResult{IsError: true}, out, nil
}

func (s *Server) checkFailure(root, code string, err error) (*sdkmcp.CallToolResult, *CheckOutput, error) {
	out := &CheckOutput{
		SchemaVersion: toolSchemaVersion, OK: false, Workspace: root, Authority: s.authority.info(root),
		Warnings: []string{}, NextActions: []inspectpkg.CommandAction{},
		Error: &ErrorInfo{Code: code, Message: err.Error()},
	}
	return &sdkmcp.CallToolResult{IsError: true}, out, nil
}

func (s *Server) manifestFailure(source, root, code string, err error) (*sdkmcp.CallToolResult, *ValidateManifestOutput, error) {
	out := &ValidateManifestOutput{
		SchemaVersion: toolSchemaVersion, OK: false, Source: source, Workspace: root,
		Authority: s.authority.info(root), Warnings: []string{},
		Error: &ErrorInfo{Code: code, Message: err.Error()},
	}
	return &sdkmcp.CallToolResult{IsError: true}, out, nil
}
