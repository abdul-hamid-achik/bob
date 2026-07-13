package cli

import (
	"errors"
	"fmt"
	"os"

	bobpaths "github.com/abdul-hamid-achik/bob/internal/paths"
	"github.com/abdul-hamid-achik/bob/internal/settings"
	"github.com/spf13/cobra"
)

func newConfigCommand(opts *options) *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Inspect or initialize Bob's XDG user settings",
	}
	command.AddCommand(newConfigShowCommand(opts), newConfigInitCommand(opts))
	return command
}

func newConfigShowCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show effective settings and resolved XDG paths",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			layout, err := bobpaths.Resolve()
			if err != nil {
				return fmt.Errorf("config show: %w", err)
			}
			value, err := settings.LoadFile(layout.ConfigFile)
			if err != nil {
				return fmt.Errorf("config show: %w", err)
			}
			_, statErr := os.Lstat(layout.ConfigFile)
			exists := statErr == nil
			if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
				return fmt.Errorf("config show: inspect settings: %w", statErr)
			}
			data := map[string]any{
				"config_file":           layout.ConfigFile,
				"config_exists":         exists,
				"data_dir":              layout.DataDir,
				"state_dir":             layout.StateDir,
				"cache_dir":             layout.CacheDir,
				"settings":              value,
				"telemetry_destination": "local_xdg_state_only",
			}
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "config show", data, nil, nil)
			}
			status := "default (file missing)"
			if exists {
				status = "loaded"
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "config     %s  %s\nstate      %s\ndata       %s\ncache      %s\ntelemetry  enabled=%t retention=%dd daily_cap=%d local-only\n",
				layout.ConfigFile, status, layout.StateDir, layout.DataDir, layout.CacheDir,
				value.Telemetry.Enabled, value.Telemetry.RetentionDays, value.Telemetry.MaxEventsPerDay)
			return err
		},
	}
}

func newConfigInitCommand(opts *options) *cobra.Command {
	var telemetryEnabled, write bool
	command := &cobra.Command{
		Use:   "init",
		Short: "Preview or create private XDG user settings",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			layout, err := bobpaths.Resolve()
			if err != nil {
				return fmt.Errorf("config init: %w", err)
			}
			value := settings.Default()
			value.Telemetry.Enabled = telemetryEnabled
			encoded, err := settings.Encode(value)
			if err != nil {
				return fmt.Errorf("config init: %w", err)
			}
			if !write {
				if opts.json {
					return emitJSON(cmd.OutOrStdout(), "config init", map[string]any{
						"path": layout.ConfigFile, "settings": value, "written": false,
					}, nil, []string{"rerun with --write to create the settings file"})
				}
				_, err := fmt.Fprintf(cmd.OutOrStdout(), "# %s (preview; nothing written)\n%s", layout.ConfigFile, encoded)
				return err
			}
			if err := settings.WriteFile(layout.ConfigFile, value); err != nil {
				return fmt.Errorf("config init: %w", err)
			}
			if opts.json {
				return emitJSON(cmd.OutOrStdout(), "config init", map[string]any{
					"path": layout.ConfigFile, "settings": value, "written": true,
				}, nil, []string{"run bob config show", "run bob stats"})
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\ntelemetry is %s and never leaves this machine\n", layout.ConfigFile, enabledWord(telemetryEnabled))
			return err
		},
	}
	command.Flags().BoolVar(&telemetryEnabled, "telemetry", false, "enable privacy-bounded local telemetry")
	command.Flags().BoolVar(&write, "write", false, "create the settings file")
	return command
}

func enabledWord(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}
