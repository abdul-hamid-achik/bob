package studio

import (
	"context"
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
)

// Options configures the read-only Studio process.
type Options struct {
	SinglePane bool
	Source     Source
}

// Run launches Bob Studio. It refuses non-interactive and dumb terminals so
// automation receives a normal error instead of terminal control sequences.
func Run(ctx context.Context, workspace string, opts Options) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateTerminal(
		term.IsTerminal(os.Stdin.Fd()),
		term.IsTerminal(os.Stdout.Fd()),
		os.Getenv("TERM"),
	); err != nil {
		return err
	}
	model := NewModelWithContext(ctx, workspace, opts.Source, opts.SinglePane)
	_, err := tea.NewProgram(model, tea.WithContext(ctx)).Run()
	if err != nil {
		return fmt.Errorf("run studio: %w", err)
	}
	return nil
}

func validateTerminal(stdinTTY, stdoutTTY bool, terminal string) error {
	if !stdinTTY || !stdoutTTY {
		return fmt.Errorf("bob studio requires an interactive terminal; use 'bob inspect --json' or 'bob plan --json' for automation")
	}
	if strings.EqualFold(strings.TrimSpace(terminal), "dumb") {
		return fmt.Errorf("bob studio does not support TERM=dumb; use 'bob inspect' or 'bob plan' instead")
	}
	return nil
}
