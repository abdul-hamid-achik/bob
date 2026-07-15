package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	playbookpkg "github.com/abdul-hamid-achik/bob/internal/playbook"
	"github.com/abdul-hamid-achik/bob/internal/recipe"
	"github.com/spf13/cobra"
)

func newPlaybookCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{Use: "playbook", Short: "Inspect and resolve deterministic repository procedures"}
	cmd.AddCommand(newPlaybookListCommand(opts), newPlaybookShowCommand(opts), newPlaybookPlanCommand(opts))
	return cmd
}

func newPlaybookListCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "list [workspace]", Short: "List playbooks for the active recipe", Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := argumentPath(args)
			opts.trackWorkspace(root)
			result, err := playbookpkg.List(root)
			if err != nil {
				return fmt.Errorf("playbook list: %w", classifyInvalidInput(err))
			}
			opts.trackWorkspace(result.Workspace)
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "playbook list", result, nil, nil)
			}
			for _, summary := range result.Playbooks {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%-32s available=%-5t scope=%-15s risk=%s\n", summary.ID, summary.Available, summary.ScopeClass, summary.Risk); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newPlaybookShowCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use: "show <id> [workspace]", Short: "Show one recipe playbook", Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 2 {
				root = args[1]
			}
			opts.trackWorkspace(root)
			result, err := playbookpkg.Show(root, args[0])
			if err != nil {
				return fmt.Errorf("playbook show: %w", classifyInvalidInput(err))
			}
			opts.trackWorkspace(result.Workspace)
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "playbook show", result, nil, nil)
			}
			return printPlaybook(cmd.OutOrStdout(), result.Playbook)
		},
	}
}

func newPlaybookPlanCommand(opts *options) *cobra.Command {
	var rawValues []string
	cmd := &cobra.Command{
		Use: "plan <id> [workspace]", Short: "Resolve a playbook with typed input values", Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 2 {
				root = args[1]
			}
			values, err := parsePlaybookValues(rawValues)
			if err != nil {
				return fmt.Errorf("playbook plan: %w", classifyInvalidInput(err))
			}
			opts.trackWorkspace(root)
			result, err := playbookpkg.Plan(root, args[0], values)
			if err != nil {
				return fmt.Errorf("playbook plan: %w", classifyInvalidInput(err))
			}
			opts.trackWorkspace(result.Workspace)
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "playbook plan", result, nil, nil)
			}
			return printPlaybook(cmd.OutOrStdout(), result.Playbook)
		},
	}
	cmd.Flags().StringArrayVar(&rawValues, "set", nil, "typed playbook input as key=value (repeatable)")
	return cmd
}

func parsePlaybookValues(raw []string) (map[string]string, error) {
	if len(raw) > 32 {
		return nil, errors.New("at most 32 --set values are accepted")
	}
	values := map[string]string{}
	problems := []string{}
	for _, pair := range raw {
		if len(pair) > 4225 {
			problems = append(problems, "a --set value exceeds the 4225-byte input bound")
			continue
		}
		key, value, ok := strings.Cut(pair, "=")
		if !ok || strings.TrimSpace(key) == "" {
			problems = append(problems, fmt.Sprintf("invalid --set %q; expected key=value", pair))
			continue
		}
		if _, duplicate := values[key]; duplicate {
			problems = append(problems, fmt.Sprintf("duplicate --set key %q", key))
			continue
		}
		values[key] = value
	}
	sort.Strings(problems)
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	return values, nil
}

func printPlaybook(w io.Writer, definition recipe.PlaybookDefinition) error {
	if _, err := fmt.Fprintf(w, "%s: %s\n", definition.ID, definition.Title); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "applicable=%t available=%t scope=%s risk=%s\n", definition.Applicable, definition.Available, definition.ScopeClass, definition.Risk); err != nil {
		return err
	}
	if len(definition.BlockedBy) > 0 {
		if _, err := fmt.Fprintf(w, "blocked by: %v\n", definition.BlockedBy); err != nil {
			return err
		}
	}
	for _, step := range definition.Steps {
		if _, err := fmt.Fprintf(w, "  %s [%s/%s] %s\n", step.ID, step.Kind, step.Effect, step.Summary); err != nil {
			return err
		}
	}
	return nil
}
