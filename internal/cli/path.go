package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/abdul-hamid-achik/bob/internal/pathinfo"
	"github.com/spf13/cobra"
)

func newPathCommand(opts *options) *cobra.Command {
	var workspaceFlag string
	var batch bool
	cmd := &cobra.Command{
		Use:   "path <repository-relative-path> [workspace]",
		Short: "Explain Bob's exact relationship to one repository path",
		Args: func(cmd *cobra.Command, args []string) error {
			if batch {
				return cobra.RangeArgs(1, pathinfo.MaxBatchPaths)(cmd, args)
			}
			return cobra.RangeArgs(1, 2)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !batch && len(args) == 2 && workspaceFlag != "" {
				return classifyInvalidInput(errors.New("path: provide workspace either positionally or with --workspace, not both"))
			}
			root := workspaceFlag
			if !batch && len(args) == 2 {
				root = args[1]
			}
			if root == "" {
				root = "."
			}
			opts.trackWorkspace(root)
			if batch {
				result, err := pathinfo.LoadBatch(root, args)
				if err != nil {
					return fmt.Errorf("path: %w", classifyInvalidInput(err))
				}
				opts.trackWorkspace(result.Workspace)
				if opts.json {
					return emitJSON(cmd.OutOrStdout(), "path", result, nil, nil)
				}
				for _, item := range result.Results {
					if err := printPath(cmd.OutOrStdout(), item); err != nil {
						return err
					}
				}
				return nil
			}
			result, err := pathinfo.Load(root, args[0])
			if err != nil {
				return fmt.Errorf("path: %w", classifyInvalidInput(err))
			}
			opts.trackWorkspace(result.Workspace)
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "path", result, nil, nil)
			}
			return printPath(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&workspaceFlag, "workspace", "", "workspace containing bob.yaml")
	cmd.Flags().BoolVar(&batch, "batch", false, "classify up to seven paths using one workspace plan")
	return cmd
}

func printPath(w io.Writer, result pathinfo.Result) error {
	if _, err := fmt.Fprintf(w, "%s: %s (%s)\n", result.Path, result.Classification, result.State); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "exists: %t; human edit effect: %s\n", result.Exists, result.HumanEditEffect); err != nil {
		return err
	}
	if result.PlanAction != nil {
		if _, err := fmt.Fprintf(w, "next plan: %s [%s]\n", result.PlanAction.Kind, result.PlanAction.Code); err != nil {
			return err
		}
	}
	if result.Artifact != nil {
		if _, err := fmt.Fprintf(w, "artifact: %s; capabilities: %v\n", result.Artifact.ID, result.Artifact.CapabilityIDs); err != nil {
			return err
		}
	}
	if len(result.ExtensionPoints) > 0 {
		if _, err := fmt.Fprintf(w, "extension points: %v\n", result.ExtensionPoints); err != nil {
			return err
		}
	}
	if len(result.RelatedPlaybooks) > 0 {
		_, err := fmt.Fprintf(w, "related playbooks: %v\n", result.RelatedPlaybooks)
		return err
	}
	return nil
}
