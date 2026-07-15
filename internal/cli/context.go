package cli

import (
	"fmt"
	"io"

	contextpkg "github.com/abdul-hamid-achik/bob/internal/context"
	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/spf13/cobra"
)

type planJSON struct {
	engine.PlanResult
	PlanDigestVersion int    `json:"plan_digest_version"`
	PlanDigest        string `json:"plan_digest"`
}

func planJSONProjection(displayed, complete engine.PlanResult) planJSON {
	digest := engine.DigestPlan(complete)
	return planJSON{PlanResult: displayed, PlanDigestVersion: digest.Version, PlanDigest: digest.Qualified()}
}

func newContextCommand(opts *options) *cobra.Command {
	var profileName string
	cmd := &cobra.Command{
		Use:   "context [workspace]",
		Short: "Describe the deterministic contract governing a workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := argumentPath(args)
			opts.trackWorkspace(root)
			profile := contextpkg.Profile(profileName)
			if profile == "" {
				profile = contextpkg.ProfileStandard
				if opts.json {
					profile = contextpkg.ProfileCompact
				}
			}
			result, err := contextpkg.Load(root, contextpkg.Options{Profile: profile})
			if err != nil {
				return fmt.Errorf("context: %w", classifyInvalidInput(err))
			}
			opts.trackWorkspace(result.Workspace)
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "context", result, nil, nil)
			}
			return printContext(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().StringVar(&profileName, "profile", "", "output profile: compact, standard, or full (default: standard; compact with --json)")
	return cmd
}

func printContext(w io.Writer, result contextpkg.Result) error {
	if _, err := fmt.Fprintf(w, "%s@%d  %s\n", result.Recipe.ID, result.Recipe.Version, result.Product.Name); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "repository: %s; managed: %d; conflicts: %d; lock changed: %t\n", result.Repository.State, result.Repository.ManagedFiles, result.Repository.ConflictCount, result.Repository.LockChanged); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "contract: %s\nplan: %s (v%d)\ncontext: %s\n", result.ContractDigest, result.Repository.PlanDigest, result.Repository.PlanDigestVersion, result.ContextDigest); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "capabilities:"); err != nil {
		return err
	}
	for _, capability := range result.Capabilities {
		if _, err := fmt.Fprintf(w, "  %-38s selection=%-12s materialization=%-14s availability=%-14s verification=%s\n", capability.ID, capability.Selection, capability.Materialization, capability.Availability, capability.Verification); err != nil {
			return err
		}
	}
	if len(result.EntryPoints) > 0 {
		if _, err := fmt.Fprintln(w, "entry points:"); err != nil {
			return err
		}
		for _, entry := range result.EntryPoints {
			if _, err := fmt.Fprintf(w, "  %s  %s (%s)\n", entry.ID, entry.Path, entry.Ownership); err != nil {
				return err
			}
		}
	}
	if len(result.ExtensionPoints) > 0 {
		if _, err := fmt.Fprintln(w, "extension points:"); err != nil {
			return err
		}
		for _, extension := range result.ExtensionPoints {
			if _, err := fmt.Fprintf(w, "  %s  %v\n", extension.ID, extension.CreatePatterns); err != nil {
				return err
			}
		}
	}
	for _, notice := range result.Notices {
		if _, err := fmt.Fprintf(w, "notice [%s]: %s\n", notice.Code, notice.Message); err != nil {
			return err
		}
	}
	for _, action := range result.Actions {
		if _, err := fmt.Fprintf(w, "next [%s]:", action.ReasonCode); err != nil {
			return err
		}
		for _, arg := range action.Argv {
			if _, err := fmt.Fprintf(w, " %s", arg); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}
