package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/detect"
	"github.com/abdul-hamid-achik/bob/internal/doctor"
	"github.com/abdul-hamid-achik/bob/internal/engine"
	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/strsim"
	"github.com/abdul-hamid-achik/bob/internal/telemetry"
	"github.com/abdul-hamid-achik/bob/internal/version"
	"github.com/spf13/cobra"
)

type Dependencies struct {
	Out               io.Writer
	ErrOut            io.Writer
	Prober            doctor.Prober
	IntegrationRunner inspectpkg.Runner
	Recorder          telemetry.Recorder
	Telemetry         *telemetry.Store
	StudioRunner      StudioRunner
	metrics           *commandMetrics
}

type options struct {
	json    bool
	metrics *commandMetrics
}

// trackWorkspace records the workspace path an in-flight command resolved,
// independent of telemetry action counts, so a later failure can thread the
// path into next_actions guidance even when the command never reaches a
// successful plan (for example, a missing manifest).
func (o *options) trackWorkspace(path string) {
	if o == nil || o.metrics == nil {
		return
	}
	o.metrics.workspace = path
}

func Execute() error {
	deps := Dependencies{Out: os.Stdout, ErrOut: os.Stderr, Prober: doctor.ExecProber{}, IntegrationRunner: inspectpkg.ExecRunner{}}
	store, recorder, warning := loadTelemetryRuntime()
	deps.Telemetry = store
	deps.Recorder = recorder
	if warning != "" {
		_, _ = fmt.Fprintf(os.Stderr, "bob: warning: %s\n", warning)
	}
	return execute(os.Args[1:], deps)
}

type reportedError struct{ err error }

func (e reportedError) Error() string { return e.err.Error() }
func (e reportedError) Unwrap() error { return e.err }

func execute(args []string, deps Dependencies) error {
	if deps.metrics == nil {
		deps.metrics = &commandMetrics{}
	}
	started := time.Now()
	cmd := New(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	recordCLI(cmd.Context(), deps, args, time.Since(started), err)
	if err == nil {
		return nil
	}
	var reported reportedError
	if errors.As(err, &reported) {
		return err
	}
	workspace := deps.metrics.workspace
	if !jsonRequested(args) || mcpRequested(args) {
		printHumanFailure(deps.ErrOut, err, workspace)
		return err
	}
	command := commandFromArgs(args)
	code := classifyErrorCode(err)
	next := nextActionsForFailure(err, workspace)
	if emitErr := emitJSONStatus(deps.Out, false, command, map[string]any{
		"error": map[string]string{"code": code, "message": err.Error()},
	}, nil, next); emitErr != nil {
		return fmt.Errorf("%w; emit JSON error: %v", err, emitErr)
	}
	return err
}

func New(deps Dependencies) *cobra.Command {
	if deps.Out == nil {
		deps.Out = io.Discard
	}
	if deps.ErrOut == nil {
		deps.ErrOut = io.Discard
	}
	if deps.Prober == nil {
		deps.Prober = doctor.ExecProber{}
	}
	if deps.IntegrationRunner == nil {
		deps.IntegrationRunner = inspectpkg.ExecRunner{}
	}
	if deps.Recorder == nil {
		deps.Recorder = telemetry.Noop{}
	}
	opts := &options{metrics: deps.metrics}
	root := &cobra.Command{
		Use:           "bob",
		Short:         "deterministic repository factory for agent-native developer tools",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(deps.Out)
	root.SetErr(deps.ErrOut)
	root.PersistentFlags().BoolVar(&opts.json, "json", false, "write a versioned JSON envelope to stdout")
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(
		newNewCommand(opts),
		newInitCommand(opts),
		newContextCommand(opts),
		newPathCommand(opts),
		newPlaybookCommand(opts),
		newPlanCommand(opts),
		newApplyCommand(opts),
		newCheckCommand(opts),
		newDoctorCommand(opts, deps.Prober),
		newInspectCommand(opts, deps.IntegrationRunner),
		newMCPCommand(deps.IntegrationRunner, deps.Recorder, deps.Telemetry),
		newConfigCommand(opts),
		newStatsCommand(opts, deps.Telemetry),
		newStudioCommand(opts, deps.StudioRunner, deps.IntegrationRunner, deps.Telemetry),
		newExplainCommand(opts),
		newLearnCommand(opts),
		newRecipeCommand(opts),
		newVersionCommand(opts),
	)
	return root
}

func newNewCommand(opts *options) *cobra.Command {
	var module, description, target string
	var write bool
	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Preview or create a new repository from the go-agent-tool recipe",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if module == "" {
				return classifyInvalidInput(errors.New("new: --module is required"))
			}
			if target == "" {
				target = name
			}
			canonicalTarget, err := safeWorkspacePath(target, false)
			if err != nil {
				return fmt.Errorf("new: validate target: %w", err)
			}
			target = canonicalTarget
			m := manifest.Default(name, module, description)
			captureWorkspaceMetrics(opts, target, true)
			artifacts, err := recipe.Render(m)
			if err != nil {
				return fmt.Errorf("new: %w", classifyInvalidInput(err))
			}
			paths := artifactPaths(artifacts)
			if !write {
				if opts.json {
					return emitJSON(cmd.OutOrStdout(), "new", map[string]any{"target": target, "manifest": m, "artifacts": paths, "written": false}, nil, []string{"rerun with --write to create the repository"})
				}
				data, _ := manifest.Encode(m)
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n# %d files would be created under %s\n", data, len(paths), target)
				return err
			}
			if err := ensureEmptyTarget(target); err != nil {
				return fmt.Errorf("new: %w", err)
			}
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("new: create target: %w", err)
			}
			if err := manifest.WriteFile(filepath.Join(target, manifest.Filename), m, false); err != nil {
				return fmt.Errorf("new: %w", err)
			}
			result, err := engine.Apply(target, m, artifacts)
			if err != nil {
				return fmt.Errorf("new: apply: %w", err)
			}
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "new", map[string]any{"target": target, "result": result}, nil, []string{"review the generated repository", "run bob check in the target"})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "created %s with %d managed files\n", target, len(result.Written)+len(result.Adopted)+len(result.Unchanged))
			return err
		},
	}
	cmd.Flags().StringVar(&module, "module", "", "Go module path (required)")
	cmd.Flags().StringVar(&description, "description", "", "one-line product description")
	cmd.Flags().StringVar(&target, "dir", "", "target directory (defaults to the project name)")
	cmd.Flags().BoolVar(&write, "write", false, "create the manifest and repository files")
	return cmd
}

func newInitCommand(opts *options) *cobra.Command {
	var name, module, description, recipeID string
	var write, force bool
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Preview or write a Bob manifest in an existing repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := argumentPath(args)
			canonicalRoot, err := safeWorkspacePath(root, true)
			if err != nil {
				return fmt.Errorf("init: inspect target: %w", err)
			}
			root = canonicalRoot
			captureWorkspaceMetrics(opts, root, true)
			if name == "" {
				absolute, absErr := filepath.Abs(root)
				if absErr != nil {
					return fmt.Errorf("init: resolve target: %w", absErr)
				}
				name = filepath.Base(absolute)
			}

			detection := detect.Detect(root)
			chosen, err := chooseInitRecipe(recipeID, detection)
			if err != nil {
				return classifyInvalidInput(fmt.Errorf("init: %w", err))
			}
			mismatch := initStackMismatch(root, chosen, detection)
			if mismatch != "" && write && !force {
				return classifyInvalidInput(fmt.Errorf("init: %s; pass --force to write anyway", mismatch))
			}
			var warnings []string
			if mismatch != "" {
				warnings = append(warnings, mismatch)
				if !opts.json {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", mismatch)
				}
			}

			m, err := buildInitManifest(chosen, name, module, description, detection)
			if err != nil {
				return classifyInvalidInput(fmt.Errorf("init: %w", err))
			}
			if err := m.Validate(); err != nil {
				return fmt.Errorf("init: %w", classifyInvalidInput(err))
			}
			if !write {
				if opts.json {
					return emitJSON(cmd.OutOrStdout(), "init", map[string]any{"path": filepath.Join(root, manifest.Filename), "manifest": m, "detection": detection, "written": false}, warnings, []string{"rerun with --write", "review with bob plan"})
				}
				data, _ := manifest.Encode(m)
				_, err := fmt.Fprintln(cmd.OutOrStdout(), string(data))
				return err
			}
			path := filepath.Join(root, manifest.Filename)
			if err := manifest.WriteFile(path, m, false); err != nil {
				return fmt.Errorf("init: %w", err)
			}
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "init", map[string]any{"path": path, "manifest": m, "detection": detection, "written": true}, warnings, []string{"run bob plan", "review conflicts before bob apply"})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\nnext: bob plan %s\n", path, root)
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (defaults to directory name)")
	cmd.Flags().StringVar(&module, "module", "", "module path; required by go-agent-tool, optional repository identity for stack recipes")
	cmd.Flags().StringVar(&description, "description", "", "one-line product description")
	cmd.Flags().StringVar(&recipeID, "recipe", "", "recipe id (defaults to the recipe matching the detected stack; see bob recipe list)")
	cmd.Flags().BoolVar(&write, "write", false, "write bob.yaml")
	cmd.Flags().BoolVar(&force, "force", false, "write even when the chosen recipe does not match the detected stack")
	return cmd
}

// chooseInitRecipe resolves the recipe init should use: an explicit --recipe
// value when given, otherwise the recipe matching the detected primary stack,
// otherwise the historical go-agent-tool default for empty or unrecognized
// repositories.
func chooseInitRecipe(explicit string, detection detect.Result) (string, error) {
	if explicit != "" {
		if explicit == manifest.RecipeFiles {
			return "", errors.New("recipe files declares its file tree inline; write bob.yaml by hand instead of using bob init")
		}
		if _, err := recipe.Version(explicit); err != nil {
			return "", err
		}
		return explicit, nil
	}
	if detection.Detected() {
		if id, ok := recipe.ForStack(detection.Primary); ok {
			return id, nil
		}
		return "", fmt.Errorf("no built-in recipe matches the detected stack %s; available recipes: %s (rerun with --recipe <id>)", detection.Describe(), strings.Join(recipe.IDs(), ", "))
	}
	return manifest.RecipeGoAgentTool, nil
}

// initStackMismatch reports a non-empty human-readable mismatch description
// when the chosen recipe claims stacks and none of them were detected in the
// repository. Recipes with no stack claim and repositories with no detected
// stack never mismatch.
func initStackMismatch(root, recipeID string, detection detect.Result) string {
	claimed := recipe.Stacks(recipeID)
	if len(claimed) == 0 || !detection.Detected() {
		return ""
	}
	for _, stack := range claimed {
		if detection.Has(stack) {
			return ""
		}
	}
	message := fmt.Sprintf("repository at %s looks like %s, but recipe %s targets %s", root, detection.Describe(), recipeID, strings.Join(claimed, ", "))
	if id, ok := recipe.ForStack(detection.Primary); ok {
		message += fmt.Sprintf("; recipe %s matches the detected stack (rerun with --recipe %s)", id, id)
	} else {
		message += fmt.Sprintf("; available recipes: %s", strings.Join(recipe.IDs(), ", "))
	}
	return message
}

// buildInitManifest constructs the default manifest for the chosen recipe.
// go-agent-tool keeps its required Go module path; stack recipes treat the
// module as optional repository identity and take their runtime kind from
// detection where the recipe supports the detected hint.
func buildInitManifest(recipeID, name, module, description string, detection detect.Result) (manifest.Manifest, error) {
	if recipeID == manifest.RecipeGoAgentTool {
		if module == "" {
			return manifest.Manifest{}, errors.New("--module is required for recipe go-agent-tool")
		}
		return manifest.Default(name, module, description), nil
	}
	kind := ""
	if runtime, ok := manifest.StackRecipeRuntime(recipeID); ok {
		for _, candidate := range runtime.Kinds {
			if candidate == detection.KindHint {
				kind = detection.KindHint
				break
			}
		}
	}
	return manifest.DefaultStack(recipeID, name, module, description, kind)
}

func newPlanCommand(opts *options) *cobra.Command {
	var showContent, conflictsOnly bool
	cmd := &cobra.Command{
		Use:   "plan [path]",
		Short: "Compare the recipe with the repository without writing",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := argumentPath(args)
			opts.trackWorkspace(root)
			plan, err := loadPlan(root)
			if err != nil {
				return fmt.Errorf("plan: %w", err)
			}
			capturePlanMetrics(opts, root, plan)
			if opts.json {
				displayed := plan
				if conflictsOnly {
					displayed = filterConflictsOnly(plan, showContent)
				}
				data := planJSONProjection(displayed, plan)
				return emitJSON(cmd.OutOrStdout(), "plan", data, conflictWarnings(plan), planNextActions(plan, root))
			}
			return printPlan(cmd.OutOrStdout(), plan, showContent, conflictsOnly)
		},
	}
	cmd.Flags().BoolVar(&showContent, "content", false, "show bounded desired-content previews for create/update/conflict actions, plus current-content previews for conflicts")
	cmd.Flags().BoolVar(&conflictsOnly, "conflicts-only", false, "show only conflicting actions (compact output for capped agent harnesses)")
	return cmd
}

func newApplyCommand(opts *options) *cobra.Command {
	var expectedPlanDigest string
	cmd := &cobra.Command{
		Use:   "apply [path]",
		Short: "Apply one complete conflict-free repository plan",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := argumentPath(args)
			opts.trackWorkspace(root)
			result, err := engine.ApplyWorkspaceWithOptions(root, engine.ApplyOptions{ExpectedPlanDigest: expectedPlanDigest})
			if result.Plan.Recipe.ID != "" {
				captureWorkspaceMetrics(opts, root, result.Plan.Recipe.ID == manifest.RecipeGoAgentTool)
			}
			if err != nil {
				var mismatch *engine.PlanDigestMismatchError
				if errors.As(err, &mismatch) {
					failure := newExitError(ExitPlanMismatch, fmt.Errorf("apply: %w", mismatch))
					if opts.json {
						data := map[string]any{
							"expected_plan_digest": mismatch.ExpectedPlanDigest,
							"actual_plan_digest":   mismatch.ActualPlanDigest,
							"error": map[string]string{
								"code":    "plan_digest_mismatch",
								"message": mismatch.Error(),
							},
						}
						next := nextActionsForFailure(failure, root)
						if emitErr := emitJSONStatus(cmd.OutOrStdout(), false, "apply", data, nil, next); emitErr != nil {
							return fmt.Errorf("%w; emit JSON error: %v", failure, emitErr)
						}
						return reportedError{err: failure}
					}
					return failure
				}
				if errors.Is(err, engine.ErrInvalidPlanDigest) {
					return classifyInvalidInput(err)
				}
				if errors.Is(err, engine.ErrWorkspaceContract) {
					return classifyInvalidInput(err)
				}
				if errors.Is(err, engine.ErrPlanConflicts) {
					failure := newExitError(ExitConflicts, errors.New("apply: plan contains conflicts; run bob plan for details"))
					conflicts := conflictSummaries(result.Plan.Actions)
					if opts.json {
						data := map[string]any{
							"error":     map[string]string{"code": "conflicts", "message": failure.Error()},
							"conflicts": conflicts,
						}
						next := nextActionsForFailure(failure, root)
						if emitErr := emitJSONStatus(cmd.OutOrStdout(), false, "apply", data, conflictWarnings(result.Plan), next); emitErr != nil {
							return fmt.Errorf("%w; emit JSON error: %v", failure, emitErr)
						}
						return reportedError{err: failure}
					}
					if printErr := printConflicts(cmd.OutOrStdout(), conflicts); printErr != nil {
						return printErr
					}
					return failure
				}
				return fmt.Errorf("apply: %w", err)
			}
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "apply", result.Receipt, nil, []string{"review the repository diff", withWorkspaceArg("run bob check", root)})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "applied: %d written, %d adopted, %d unchanged; lock written: %t\n", len(result.Written), len(result.Adopted), len(result.Unchanged), result.LockWritten)
			return err
		},
	}
	cmd.Flags().StringVar(&expectedPlanDigest, "expect-plan-digest", "", "apply only when a fresh plan matches this exact sha256:<64-lowercase-hex> digest")
	return cmd
}

func newCheckCommand(opts *options) *cobra.Command {
	var conflictsOnly bool
	cmd := &cobra.Command{
		Use:   "check [path]",
		Short: "Fail when managed repository state would change",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := argumentPath(args)
			opts.trackWorkspace(root)
			plan, err := loadPlan(root)
			if err != nil {
				return fmt.Errorf("check: %w", err)
			}
			capturePlanMetrics(opts, root, plan)
			clean := !plan.LockChanged
			for _, action := range plan.Actions {
				if action.Kind != engine.ActionUnchanged {
					clean = false
					break
				}
			}
			var failure error
			if !clean {
				code := ExitDrift
				if plan.HasConflicts() {
					code = ExitConflicts
				}
				failure = newExitError(code, errors.New("check: repository drift detected"))
			}
			displayed := plan
			if conflictsOnly {
				displayed = filterConflictsOnly(plan, false)
			}
			reportPlan := planJSONProjection(displayed, plan)
			digest := engine.DigestPlan(plan)
			data := map[string]any{
				"clean": clean, "plan": reportPlan,
				"plan_digest_version": digest.Version,
				"plan_digest":         digest.Qualified(),
			}
			if opts.json {
				var next []string
				if failure != nil {
					next = nextActionsForFailure(failure, root)
				}
				if err := emitJSONStatus(cmd.OutOrStdout(), clean, "check", data, conflictWarnings(plan), next); err != nil {
					return err
				}
			} else if clean {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "clean: repository matches bob.yaml and bob.lock"); err != nil {
					return err
				}
			} else if err := printPlan(cmd.OutOrStdout(), plan, false, conflictsOnly); err != nil {
				return err
			}
			if failure != nil {
				if opts.json {
					return reportedError{err: failure}
				}
				return failure
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&conflictsOnly, "conflicts-only", false, "show only conflicting actions (compact output for capped agent harnesses)")
	return cmd
}

func newDoctorCommand(opts *options, prober doctor.Prober) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [path]",
		Short: "Probe required and selected optional development tools",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := argumentPath(args)
			m, err := manifest.Load(root)
			captureWorkspaceMetrics(opts, root, m.Recipe == "go-agent-tool")
			if err != nil {
				return fmt.Errorf("doctor: %w", classifyInvalidInput(err))
			}
			result := doctor.Run(context.Background(), m, prober)
			warnings := []string(nil)
			if result.Degraded {
				warnings = []string{"one or more optional tools are unavailable or failed their version probe"}
			}
			if opts.json {
				if err := emitJSONStatus(cmd.OutOrStdout(), result.Ready, "doctor", result, warnings, doctorNextActions(result)); err != nil {
					return err
				}
			} else {
				for _, check := range result.Checks {
					status := "missing"
					if check.Usable {
						status = "ok"
					} else if check.Found {
						status = "error"
					}
					detail := check.Version
					if !check.Usable && check.Note != "" {
						detail = check.Note
					}
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-14s %s\n", status, check.Name, detail); err != nil {
						return err
					}
				}
			}
			if !result.Ready {
				failure := errors.New("doctor: required tools are missing")
				if opts.json {
					return reportedError{err: failure}
				}
				return failure
			}
			return nil
		},
	}
}

func newExplainCommand(opts *options) *cobra.Command {
	data := map[string]any{
		"schema_version": 1,
		"product":        "deterministic repository factory, contract compiler, guidance provider, and lifecycle reconciler",
		"owns":           []string{"manifest validation", "recipe rendering", "workspace context", "path classification", "bounded playbooks", "repository plan", "generated-file ownership", "safe apply", "drift detection"},
		"does_not_own":   []string{"model execution", "agent scheduling", "canonical verification", "secrets", "tool discovery", "application business logic"},
		"recipe":         recipe.IDs(),
	}
	return &cobra.Command{
		Use:   "explain",
		Short: "Describe Bob's product contract and boundaries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "explain", data, nil, []string{"run bob recipe list"})
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Bob compiles repository intent into context, path guidance, closed playbooks, and a deterministic plan: manifest → review → explicit apply → drift check.\nIt does not run models, schedule agents, manage secrets, or declare behavioral verification.")
			return err
		},
	}
}

func newLearnCommand(opts *options) *cobra.Command {
	commands := []map[string]any{
		{"name": "new", "purpose": "preview or create a new repository from the go-agent-tool recipe; --write authorizes creation", "mutates": true, "json": true},
		{"name": "init", "purpose": "preview or write a Bob manifest in an existing repository; --write authorizes creation", "mutates": true, "json": true},
		{"name": "context", "purpose": "describe the bounded workspace-specific repository contract without writing or probing specialists", "mutates": false, "json": true},
		{"name": "path", "purpose": "classify one exact path through Bob's real ownership and extension contracts", "mutates": false, "json": true},
		{"name": "playbook", "purpose": "list, show, or resolve a closed recipe procedure without executing its steps", "mutates": false, "json": true},
		{"name": "plan", "purpose": "compare recipe, lock, and working tree without writing", "mutates": false, "json": true},
		{"name": "apply", "purpose": "apply one complete conflict-free repository plan; refuses all writes when any conflict exists", "mutates": true, "json": true},
		{"name": "check", "purpose": "exit non-zero when managed repository state would change; CI drift gate", "mutates": false, "json": true},
		{"name": "doctor", "purpose": "probe required and selected optional development tools", "mutates": false, "json": true},
		{"name": "inspect", "purpose": "summarize Bob state; --probe-integrations explicitly authorizes bounded specialist probes", "mutates": false, "json": true},
		{"name": "config", "purpose": "inspect or initialize XDG user settings; init previews unless --write", "mutates": true, "json": true},
		{"name": "stats", "purpose": "summarize privacy-bounded local usage aggregates", "mutates": false, "json": true},
		{"name": "studio", "purpose": "interactive read-only repository and usage board; rejects --json", "mutates": false, "json": false},
		{"name": "mcp", "purpose": "serve nine repository-read-only MCP tools over stdio", "mutates": false, "json": false},
		{"name": "explain", "purpose": "describe Bob's product contract and boundaries", "mutates": false, "json": true},
		{"name": "learn", "purpose": "emit this onboarding brief for coding agents", "mutates": false, "json": true},
		{"name": "recipe", "purpose": "inspect the embedded recipe catalog", "mutates": false, "json": true},
		{"name": "version", "purpose": "print version and build metadata", "mutates": false, "json": true},
	}
	recipeIDs := recipe.IDs()
	recipes := make([]map[string]any, 0, len(recipeIDs))
	for _, id := range recipeIDs {
		entry := recipeCatalogEntry(id)
		recipes = append(recipes, map[string]any{"id": entry["id"], "version": entry["version"], "description": entry["description"]})
	}
	data := map[string]any{
		"schema_version": 1,
		"product":        "deterministic repository factory, contract compiler, guidance provider, and lifecycle reconciler",
		"summary":        "Bob compiles bob.yaml through a versioned recipe, compares desired artifacts with the working tree and bob.lock, and applies only changes whose ownership is proven.",
		"lifecycle": []string{
			"bob new|init previews by default; --write authorizes creation",
			"bob plan compares desired state with the repository and lock without writing",
			"bob apply writes only absent, identical, or previously managed files and refuses when any conflict exists",
			"bob check exits non-zero in CI when generated infrastructure drifts",
		},
		"commands": commands,
		"recipes":  recipes,
		"recommended_agent_bootstrap": []string{
			"bob learn --json",
			"bob context --json",
			"bob plan --json",
			"bob check --json",
		},
		"json_envelope": map[string]any{
			"flag":   "--json",
			"fields": []string{"schema_version", "ok", "command", "data", "warnings", "next_actions"},
			"notes":  "JSON stdout is machine-clean; diagnostics go to stderr. Every failure emits ok:false with data.error.code (see error_codes below), data.error.message, and next_actions holding concrete, copy-pasteable corrective commands. Human (non-JSON) failures print the same corrective steps as \"next: ...\" lines on stderr after the error line.",
		},
		"invariants": []string{
			"context, path, playbook, plan, check, plain inspect, stats, studio, explain, and learn never mutate repositories",
			"apply preflights the complete plan and writes nothing when any conflict exists",
			"Bob never overwrites an unmanaged differing file",
			"a managed file updates only when its current hash matches the prior lock",
			"repeated apply converges to a no-op",
		},
		"exit_codes": map[string]string{
			"0": "success; bob plan always exits 0 even when it finds conflicts, because plan is a read-only report",
			"1": "unclassified command failure",
			"2": "apply refused a conflicted plan, or check found an ownership conflict",
			"3": "check found drift with no ownership conflict",
			"4": "invalid input: a missing or invalid manifest, a bad flag or argument, or an unrecognized recipe id",
			"5": "apply refused because the fresh plan differs from the reviewed plan digest",
		},
		"error_codes": map[string]string{
			"missing_manifest":     "no bob.yaml was found at the resolved workspace path",
			"manifest_invalid":     "bob.yaml failed to parse or failed Validate; the message lists every problem",
			"input_invalid":        "a flag, argument, or recipe id was invalid",
			"conflicts":            "the plan contains one or more ownership conflicts; apply refused every write",
			"workspace_invalid":    "the workspace path could not be resolved safely",
			"plan_digest_mismatch": "the fresh apply plan differs from the explicitly reviewed plan digest; no repository writes occurred",
			"command_failed":       "an unclassified failure; read the message for detail",
		},
		"mcp": map[string]any{
			"serve":     "bob mcp serve <workspace>",
			"authority": "repository read-only; defaults to an exact startup workspace allowlist",
			"tools": []string{
				"bob_context", "bob_path", "bob_playbook", "bob_plan", "bob_check",
				"bob_inspect", "bob_stats", "bob_recipe_describe", "bob_validate_manifest",
			},
		},
		"boundaries": []string{"model execution", "agent scheduling", "canonical verification", "secrets", "tool discovery", "application business logic"},
		"docs": map[string]any{
			"site":      "https://bobcli.dev",
			"agents":    "https://bobcli.dev/agents",
			"reference": "https://bobcli.dev/reference/cli",
		},
	}
	return &cobra.Command{
		Use:   "learn",
		Short: "Emit a one-shot onboarding brief for coding agents",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "learn", data, nil, []string{"run bob context <workspace> --json", "run bob plan <workspace> --json"})
			}
			var b strings.Builder
			b.WriteString("Bob: deterministic repository factory, contract compiler, guidance provider, and lifecycle reconciler.\n")
			b.WriteString("Lifecycle: new|init (preview; --write creates) -> plan (read-only) -> apply (explicit, conflict-free only) -> check (CI drift gate).\n")
			b.WriteString("Guarantees: context/path/playbook/plan/check/inspect/stats/learn never write; one conflict means zero writes; unmanaged differing files are never overwritten; repeated apply converges to a no-op.\n")
			b.WriteString("Guidance: context describes the workspace contract; path classifies one exact repository path; playbook resolves a closed typed procedure but never executes it.\n")
			b.WriteString("Machine use: add --json to any non-interactive command for a versioned envelope {schema_version, ok, command, data, warnings, next_actions}; stdout stays machine-clean.\n")
			b.WriteString("On failure: every command emits a closed error code (missing_manifest, manifest_invalid, input_invalid, conflicts, workspace_invalid, plan_digest_mismatch, command_failed) plus next_actions with runnable corrective commands; the same steps print as \"next: ...\" lines on stderr without --json.\n")
			b.WriteString("Compact output: add --conflicts-only to plan or check to see only conflicting actions, which is friendlier to a capped agent harness.\n")
			b.WriteString("MCP: bob mcp serve <workspace> exposes nine read-only tools, including bob_context, bob_path, and bob_playbook; repository mutation remains on the approved shell path.\n")
			b.WriteString("Recipes: run bob recipe list, or bob recipe show <id> for the full contract of go-agent-tool (Go/Cobra CLI scaffold), files (declare any file tree inline), or a stack hygiene recipe (ts-app, js-app, vue-app, python-app, ruby-app, lua-lib, rust-cli, static-web) that seeds docs/.gitignore/CI once and never touches application source. bob init auto-selects the recipe matching the detected repository stack.\n")
			b.WriteString("Out of scope: models, agent scheduling, secrets, verification claims, application business logic.\n")
			b.WriteString("Docs: https://bobcli.dev (agents guide: https://bobcli.dev/agents). Start with: bob learn --json, then bob context --json.\n")
			_, err := fmt.Fprint(cmd.OutOrStdout(), b.String())
			return err
		},
	}
}

const filesRecipeExampleManifest = `schema_version: 1
recipe: files
product:
  name: my-app
  description: A generated web service
vars:
  project_name: my-app
  port: "8080"
files:
  - path: package.json
    content: |
      {"name": "${vars.project_name}"}
  - path: scripts/run.sh
    mode: "0755"
    content: |
      #!/usr/bin/env bash
      echo "listening on ${vars.port}"
`

func recipeCatalogEntry(id string) map[string]any {
	version, err := recipe.Version(id)
	if err != nil {
		return nil
	}
	switch id {
	case "files":
		return map[string]any{
			"id":          "files",
			"version":     version,
			"description": "declare any file tree inline; bob materializes it with plan/apply safety",
			"surfaces":    []string{"cli", "json"},
			"manifest_schema": map[string]any{
				"vars":  `map[string]string; keys must match ^[a-z][a-z0-9_]*$; declared-but-unused vars are fine`,
				"files": `list of {path, mode, content}; path must resolve inside the workspace; mode is an optional 3-4 digit octal permission string like "0644" (default "0644"; setuid, setgid, and sticky bits are rejected); content is written verbatim after substitution`,
			},
			"substitution": map[string]any{
				"pattern":    `\$\{vars\.([a-z][a-z0-9_]*)\}`,
				"rule":       "one literal-replacement regex pass, not a template language; text that does not match the pattern (including a shell's own ${FOO}) passes through untouched",
				"unresolved": "a reference to an undeclared var is a render-time error; every unresolved reference across every file is collected, sorted, deduped, and reported in one error alongside its file path",
			},
			"path_safety": []string{
				"paths cannot be absolute or escape the workspace",
				"paths cannot target .git, bob.yaml, or bob.lock",
				"duplicate paths, compared after the same canonicalization Bob uses for ownership, are rejected at validate time",
			},
			"ownership_note": "Bob owns file existence, mode, and byte-for-byte convergence for every declared path; it does not maintain file content over time. The person or agent editing bob.yaml owns what the content means and how it evolves.",
			"example":        filesRecipeExampleManifest,
		}
	case "go-agent-tool":
		return map[string]any{
			"id":          "go-agent-tool",
			"version":     version,
			"description": "Public-ready Go and Cobra CLI with docs, CI, release plumbing, and optional ecosystem seams",
			"surfaces":    []string{"cli", "json"},
		}
	default:
		info, ok := recipe.StackInfoFor(id)
		if !ok {
			return nil
		}
		return map[string]any{
			"id":             info.ID,
			"version":        version,
			"description":    info.Description,
			"language":       info.LanguageLabel,
			"stacks":         info.Stacks,
			"seeded_paths":   info.SeededPaths,
			"ownership_note": "Every artifact is a seed: created once when missing, never recorded in bob.lock, never updated or overwritten. Application source is never touched.",
			"surfaces":       []string{"cli", "json"},
		}
	}
}

func newRecipeCommand(opts *options) *cobra.Command {
	recipeCmd := &cobra.Command{Use: "recipe", Short: "Inspect the embedded recipe catalog"}
	recipeCmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List embedded recipes",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				ids := recipe.IDs()
				entries := make([]any, 0, len(ids))
				for _, id := range ids {
					entries = append(entries, recipeCatalogEntry(id))
				}
				if opts.json {
					return emitJSON(cmd.OutOrStdout(), "recipe list", entries, nil, nil)
				}
				for _, id := range ids {
					entry := recipeCatalogEntry(id)
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s@%d  %s\n", entry["id"], entry["version"], entry["description"]); err != nil {
						return err
					}
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "show <id>",
			Short: "Show one embedded recipe",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				entry := recipeCatalogEntry(args[0])
				if entry == nil {
					msg := fmt.Sprintf("recipe show: unknown recipe %q", args[0])
					if suggestion, ok := strsim.Closest(args[0], recipe.IDs(), 2); ok {
						msg += fmt.Sprintf("; did you mean %q?", suggestion)
					}
					return errors.New(msg)
				}
				if opts.json {
					return emitJSON(cmd.OutOrStdout(), "recipe show", entry, nil, nil)
				}
				if args[0] == "files" {
					schema := entry["manifest_schema"].(map[string]any)
					substitution := entry["substitution"].(map[string]any)
					_, err := fmt.Fprintf(cmd.OutOrStdout(), "files@%d\n  %s\n\n  Manifest schema:\n    vars: %s\n    files: %s\n\n  Substitution:\n    pattern: %s\n    rule: %s\n    unresolved: %s\n\n  Path safety:\n    - %s\n\n  Ownership: %s\n\n  Example manifest:\n%s",
						entry["version"], entry["description"],
						schema["vars"], schema["files"],
						substitution["pattern"], substitution["rule"], substitution["unresolved"],
						strings.Join(entry["path_safety"].([]string), "\n    - "),
						entry["ownership_note"],
						indentLines(filesRecipeExampleManifest, "    "),
					)
					return err
				}
				if args[0] == "go-agent-tool" {
					_, err := fmt.Fprintf(cmd.OutOrStdout(), "go-agent-tool@%d\n  Generates a public-ready Go/Cobra CLI with machine output, diagnostics, docs, CI, release configuration, and selected integration seams.\n", entry["version"])
					return err
				}
				info, _ := recipe.StackInfoFor(args[0])
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s@%d\n  %s\n\n  Stack: %s\n  Seeded paths (created once when missing, never updated or lock-owned):\n    - %s\n\n  Ownership: Bob never owns application source for this recipe; every artifact is a one-time hygiene seed the human owns from the moment it exists.\n",
					info.ID, entry["version"], info.Description, info.LanguageLabel, strings.Join(info.SeededPaths, "\n    - "))
				return err
			},
		},
	)
	return recipeCmd
}

func indentLines(text, prefix string) string {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n") + "\n"
}

func newVersionCommand(opts *options) *cobra.Command {
	data := map[string]any{"name": "bob", "version": version.Version, "commit": version.Commit, "date": version.Date}
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "version", data, nil, nil)
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "bob version %s (%s) %s\n", version.Version, version.Commit, version.Date)
			return err
		},
	}
}

func argumentPath(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	return "."
}

func loadPlan(root string) (engine.PlanResult, error) {
	m, err := manifest.Load(root)
	if err != nil {
		return engine.PlanResult{}, classifyInvalidInput(err)
	}
	artifacts, err := recipe.Render(m)
	if err != nil {
		return engine.PlanResult{}, classifyInvalidInput(err)
	}
	return engine.Plan(root, m, artifacts)
}

func printPlan(w io.Writer, plan engine.PlanResult, showContent, conflictsOnly bool) error {
	counts := map[engine.ActionKind]int{}
	for _, action := range plan.Actions {
		counts[action.Kind]++
		if conflictsOnly && action.Kind != engine.ActionConflict {
			continue
		}
		if _, err := fmt.Fprintf(w, "%-10s %s\n", action.Kind, action.Path); err != nil {
			return err
		}
		if showContent && (action.Kind == engine.ActionCreate || action.Kind == engine.ActionUpdate || action.Kind == engine.ActionConflict) && action.DesiredPreview != "" {
			if _, err := fmt.Fprintf(w, "--- desired preview ---\n%s\n--- end preview ---\n", action.DesiredPreview); err != nil {
				return err
			}
		}
		if showContent && action.Kind == engine.ActionConflict && action.CurrentPreview != "" {
			if _, err := fmt.Fprintf(w, "--- current preview ---\n%s\n--- end preview ---\n", action.CurrentPreview); err != nil {
				return err
			}
		}
	}
	if plan.LockChanged && !conflictsOnly {
		if _, err := fmt.Fprintln(w, "lock       bob.lock"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "\n%d create, %d update, %d adopt, %d unchanged, %d conflict\n", counts[engine.ActionCreate], counts[engine.ActionUpdate], counts[engine.ActionAdopt], counts[engine.ActionUnchanged], counts[engine.ActionConflict])
	return err
}

// filterConflictsOnly returns a copy of plan whose Actions are trimmed to
// kind=conflict entries. includePreviews controls whether the surviving
// entries keep their desired/current content previews; the --conflicts-only
// flag is meant for compact output, so previews are stripped by default and
// only kept when the caller also asked for --content.
func filterConflictsOnly(plan engine.PlanResult, includePreviews bool) engine.PlanResult {
	filtered := plan
	actions := make([]engine.Action, 0, plan.ConflictCount)
	for _, action := range plan.Actions {
		if action.Kind != engine.ActionConflict {
			continue
		}
		if !includePreviews {
			action.DesiredPreview = ""
			action.CurrentPreview = ""
		}
		actions = append(actions, action)
	}
	filtered.Actions = actions
	return filtered
}

// conflictSummaries reduces a plan's actions to the compact {path, code,
// reason} shape apply's own conflict envelope reports, so a caller does not
// need a second `bob plan` round trip to see what blocked apply.
func conflictSummaries(actions []engine.Action) []map[string]any {
	conflicts := make([]map[string]any, 0)
	for _, action := range actions {
		if action.Kind != engine.ActionConflict {
			continue
		}
		conflicts = append(conflicts, map[string]any{
			"path":   action.Path,
			"code":   action.Code,
			"reason": action.Reason,
		})
	}
	return conflicts
}

// printConflicts prints each conflicting path with its code and reason,
// capped at 20 rows so a bounded agent harness never has to scroll past a
// wall of conflicts to find its own next step.
func printConflicts(w io.Writer, conflicts []map[string]any) error {
	const limit = 20
	shown := conflicts
	remaining := 0
	if len(conflicts) > limit {
		shown = conflicts[:limit]
		remaining = len(conflicts) - limit
	}
	for _, conflict := range shown {
		if _, err := fmt.Fprintf(w, "conflict   %s  [%s] %s\n", conflict["path"], conflict["code"], conflict["reason"]); err != nil {
			return err
		}
	}
	if remaining > 0 {
		if _, err := fmt.Fprintf(w, "...and %d more\n", remaining); err != nil {
			return err
		}
	}
	return nil
}

// doctorNextActions surfaces each unusable check's own note as a corrective
// step. It returns an empty (non-nil) slice when every check is usable or no
// unusable check carries a note, since Bob does not invent remediation text
// it was not given.
func doctorNextActions(result doctor.Result) []string {
	next := make([]string, 0)
	for _, check := range result.Checks {
		if check.Usable || check.Note == "" {
			continue
		}
		next = append(next, fmt.Sprintf("%s: %s", check.Name, check.Note))
	}
	return next
}

func conflictWarnings(plan engine.PlanResult) []string {
	if plan.ConflictCount == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%d conflict(s) block apply", plan.ConflictCount)}
}

func planNextActions(plan engine.PlanResult, workspace string) []string {
	if plan.HasConflicts() {
		return []string{"resolve unmanaged or modified-file conflicts", withWorkspaceArg("rerun bob plan", workspace)}
	}
	for _, action := range plan.Actions {
		if action.Kind != engine.ActionUnchanged {
			return []string{"review the plan", withWorkspaceArg("run bob apply", workspace)}
		}
	}
	if plan.LockChanged {
		return []string{"review the lock update", withWorkspaceArg("run bob apply", workspace)}
	}
	return []string{"repository is converged"}
}

func artifactPaths(artifacts []recipe.Artifact) []string {
	paths := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		paths = append(paths, artifact.Path)
	}
	sort.Strings(paths)
	return paths
}

func ensureEmptyTarget(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("target %s is not a directory", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("target %s is not empty; use bob init for an existing repository", path)
	}
	return nil
}

func jsonRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" || arg == "--json=true" {
			return true
		}
	}
	return false
}

func commandFromArgs(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg
	}
	return "bob"
}

func mcpRequested(args []string) bool {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		return arg == "mcp"
	}
	return false
}
