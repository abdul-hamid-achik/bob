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

	"github.com/abdul-hamid-achik/bob/internal/doctor"
	"github.com/abdul-hamid-achik/bob/internal/engine"
	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/abdul-hamid-achik/bob/internal/version"
	"github.com/spf13/cobra"
)

type Dependencies struct {
	Out               io.Writer
	ErrOut            io.Writer
	Prober            doctor.Prober
	IntegrationRunner inspectpkg.Runner
}

type options struct {
	json bool
}

func Execute() error {
	return execute(os.Args[1:], Dependencies{Out: os.Stdout, ErrOut: os.Stderr, Prober: doctor.ExecProber{}, IntegrationRunner: inspectpkg.ExecRunner{}})
}

type reportedError struct{ err error }

func (e reportedError) Error() string { return e.err.Error() }
func (e reportedError) Unwrap() error { return e.err }

func execute(args []string, deps Dependencies) error {
	cmd := New(deps)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err == nil || !jsonRequested(args) || mcpRequested(args) {
		return err
	}
	var reported reportedError
	if errors.As(err, &reported) {
		return err
	}
	command := commandFromArgs(args)
	if emitErr := emitJSONStatus(deps.Out, false, command, map[string]any{
		"error": map[string]string{"code": "command_failed", "message": err.Error()},
	}, nil, nil); emitErr != nil {
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
	opts := &options{}
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
		newPlanCommand(opts),
		newApplyCommand(opts),
		newCheckCommand(opts),
		newDoctorCommand(opts, deps.Prober),
		newInspectCommand(opts, deps.IntegrationRunner),
		newMCPCommand(deps.IntegrationRunner),
		newExplainCommand(opts),
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
				return errors.New("new: --module is required")
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
			artifacts, err := recipe.Render(m)
			if err != nil {
				return fmt.Errorf("new: %w", err)
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
	var name, module, description string
	var write bool
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
			if name == "" {
				absolute, absErr := filepath.Abs(root)
				if absErr != nil {
					return fmt.Errorf("init: resolve target: %w", absErr)
				}
				name = filepath.Base(absolute)
			}
			if module == "" {
				return errors.New("init: --module is required")
			}
			m := manifest.Default(name, module, description)
			if err := m.Validate(); err != nil {
				return fmt.Errorf("init: %w", err)
			}
			if !write {
				if opts.json {
					return emitJSON(cmd.OutOrStdout(), "init", map[string]any{"path": filepath.Join(root, manifest.Filename), "manifest": m, "written": false}, nil, []string{"rerun with --write", "review with bob plan"})
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
				return emitJSON(cmd.OutOrStdout(), "init", map[string]any{"path": path, "manifest": m, "written": true}, nil, []string{"run bob plan", "review conflicts before bob apply"})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\nnext: bob plan %s\n", path, root)
			return err
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "project name (defaults to directory name)")
	cmd.Flags().StringVar(&module, "module", "", "Go module path (required)")
	cmd.Flags().StringVar(&description, "description", "", "one-line product description")
	cmd.Flags().BoolVar(&write, "write", false, "write bob.yaml")
	return cmd
}

func newPlanCommand(opts *options) *cobra.Command {
	var showContent bool
	cmd := &cobra.Command{
		Use:   "plan [path]",
		Short: "Compare the recipe with the repository without writing",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := argumentPath(args)
			plan, err := loadPlan(root)
			if err != nil {
				return fmt.Errorf("plan: %w", err)
			}
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "plan", plan, conflictWarnings(plan), planNextActions(plan))
			}
			return printPlan(cmd.OutOrStdout(), plan, showContent)
		},
	}
	cmd.Flags().BoolVar(&showContent, "content", false, "show bounded desired-content previews for create and update actions")
	return cmd
}

func newApplyCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "apply [path]",
		Short: "Apply one complete conflict-free repository plan",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := argumentPath(args)
			m, err := manifest.Load(root)
			if err != nil {
				return fmt.Errorf("apply: %w", err)
			}
			artifacts, err := recipe.Render(m)
			if err != nil {
				return fmt.Errorf("apply: %w", err)
			}
			result, err := engine.Apply(root, m, artifacts)
			if err != nil {
				if errors.Is(err, engine.ErrPlanConflicts) {
					return errors.New("apply: plan contains conflicts; run bob plan for details")
				}
				return fmt.Errorf("apply: %w", err)
			}
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "apply", result, nil, []string{"review the repository diff", "run bob check"})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "applied: %d written, %d adopted, %d unchanged; lock written: %t\n", len(result.Written), len(result.Adopted), len(result.Unchanged), result.LockWritten)
			return err
		},
	}
}

func newCheckCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "check [path]",
		Short: "Fail when managed repository state would change",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			plan, err := loadPlan(argumentPath(args))
			if err != nil {
				return fmt.Errorf("check: %w", err)
			}
			clean := !plan.LockChanged
			for _, action := range plan.Actions {
				if action.Kind != engine.ActionUnchanged {
					clean = false
					break
				}
			}
			data := map[string]any{"clean": clean, "plan": plan}
			if opts.json {
				if err := emitJSONStatus(cmd.OutOrStdout(), clean, "check", data, conflictWarnings(plan), nil); err != nil {
					return err
				}
			} else if clean {
				if _, err := fmt.Fprintln(cmd.OutOrStdout(), "clean: repository matches bob.yaml and bob.lock"); err != nil {
					return err
				}
			} else if err := printPlan(cmd.OutOrStdout(), plan, false); err != nil {
				return err
			}
			if !clean {
				failure := errors.New("check: repository drift detected")
				if opts.json {
					return reportedError{err: failure}
				}
				return failure
			}
			return nil
		},
	}
}

func newDoctorCommand(opts *options, prober doctor.Prober) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [path]",
		Short: "Probe required and selected optional development tools",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			m, err := manifest.Load(argumentPath(args))
			if err != nil {
				return fmt.Errorf("doctor: %w", err)
			}
			result := doctor.Run(context.Background(), m, prober)
			warnings := []string(nil)
			if result.Degraded {
				warnings = []string{"one or more optional tools are unavailable or failed their version probe"}
			}
			if opts.json {
				if err := emitJSONStatus(cmd.OutOrStdout(), result.Ready, "doctor", result, warnings, nil); err != nil {
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
		"product":        "deterministic repository factory and lifecycle reconciler",
		"owns":           []string{"manifest validation", "recipe rendering", "repository plan", "generated-file ownership", "safe apply", "drift detection"},
		"does_not_own":   []string{"model execution", "agent scheduling", "canonical verification", "secrets", "tool discovery", "application business logic"},
		"recipe":         []string{"go-agent-tool"},
	}
	return &cobra.Command{
		Use:   "explain",
		Short: "Describe Bob's product contract and boundaries",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "explain", data, nil, []string{"run bob recipe show go-agent-tool"})
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "Bob owns deterministic repository construction: manifest → plan → explicit apply → drift check.\nIt does not run models, schedule agents, manage secrets, or declare behavioral verification.")
			return err
		},
	}
}

func newRecipeCommand(opts *options) *cobra.Command {
	recipeCmd := &cobra.Command{Use: "recipe", Short: "Inspect the embedded recipe catalog"}
	entry := map[string]any{
		"id":          "go-agent-tool",
		"version":     engine.RecipeVersion,
		"description": "Public-ready Go and Cobra CLI with docs, CI, release plumbing, and optional ecosystem seams",
		"surfaces":    []string{"cli", "json"},
	}
	recipeCmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "List embedded recipes",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				if opts.json {
					return emitJSON(cmd.OutOrStdout(), "recipe list", []any{entry}, nil, nil)
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "go-agent-tool@%d  public-ready Go and Cobra CLI\n", engine.RecipeVersion)
				return err
			},
		},
		&cobra.Command{
			Use:   "show <id>",
			Short: "Show one embedded recipe",
			Args:  cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if args[0] != "go-agent-tool" {
					return fmt.Errorf("recipe show: unknown recipe %q", args[0])
				}
				if opts.json {
					return emitJSON(cmd.OutOrStdout(), "recipe show", entry, nil, nil)
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "go-agent-tool@%d\n  Generates a public-ready Go/Cobra CLI with machine output, diagnostics, docs, CI, release configuration, and selected integration seams.\n", engine.RecipeVersion)
				return err
			},
		},
	)
	return recipeCmd
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
		return engine.PlanResult{}, err
	}
	artifacts, err := recipe.Render(m)
	if err != nil {
		return engine.PlanResult{}, err
	}
	return engine.Plan(root, m, artifacts)
}

func printPlan(w io.Writer, plan engine.PlanResult, showContent bool) error {
	counts := map[engine.ActionKind]int{}
	for _, action := range plan.Actions {
		counts[action.Kind]++
		if _, err := fmt.Fprintf(w, "%-10s %s\n", action.Kind, action.Path); err != nil {
			return err
		}
		if showContent && (action.Kind == engine.ActionCreate || action.Kind == engine.ActionUpdate) && action.DesiredPreview != "" {
			if _, err := fmt.Fprintf(w, "--- desired preview ---\n%s\n--- end preview ---\n", action.DesiredPreview); err != nil {
				return err
			}
		}
	}
	if plan.LockChanged {
		if _, err := fmt.Fprintln(w, "lock       bob.lock"); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "\n%d create, %d update, %d adopt, %d unchanged, %d conflict\n", counts[engine.ActionCreate], counts[engine.ActionUpdate], counts[engine.ActionAdopt], counts[engine.ActionUnchanged], counts[engine.ActionConflict])
	return err
}

func conflictWarnings(plan engine.PlanResult) []string {
	if plan.ConflictCount == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%d conflict(s) block apply", plan.ConflictCount)}
}

func planNextActions(plan engine.PlanResult) []string {
	if plan.HasConflicts() {
		return []string{"resolve unmanaged or modified-file conflicts", "rerun bob plan"}
	}
	for _, action := range plan.Actions {
		if action.Kind != engine.ActionUnchanged {
			return []string{"review the plan", "run bob apply"}
		}
	}
	if plan.LockChanged {
		return []string{"review the lock update", "run bob apply"}
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
