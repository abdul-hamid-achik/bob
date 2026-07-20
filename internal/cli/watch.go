package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/engine"
	"github.com/abdul-hamid-achik/bob/internal/manifest"

	"github.com/spf13/cobra"
)

// watchPollInterval is the polling interval for bob.yaml changes. Tests
// override it to avoid real-time sleeps.
var watchPollInterval = time.Second

// runWatchLoop polls bob.yaml for modification-time or size changes and
// re-runs the plan on each change. It clears the screen before every
// iteration and exits cleanly when ctx is cancelled.
func runWatchLoop(ctx context.Context, cmd *cobra.Command, root string, showContent, conflictsOnly, showDiff bool) error {
	manifestPath := filepath.Join(root, manifest.Filename)
	out := cmd.OutOrStdout()

	lastModTime, lastSize, exists := statManifest(manifestPath)

	// Initial plan render.
	printWatchHeader(out, "watching bob.yaml (Ctrl+C to stop)")
	printWatchPlan(out, root, showContent, conflictsOnly, showDiff)

	ticker := time.NewTicker(watchPollInterval)
	defer ticker.Stop()

	iterations := 0
	for {
		select {
		case <-ctx.Done():
			fmt.Fprintf(out, "\nbob watch: stopped after %d plan(s)\n", iterations+1)
			return nil
		case <-ticker.C:
			modTime, size, nowExists := statManifest(manifestPath)
			changed := nowExists != exists ||
				(nowExists && (modTime != lastModTime || size != lastSize))
			if !changed {
				continue
			}
			lastModTime, lastSize, exists = modTime, size, nowExists
			iterations++

			clearScreen(out)
			if !nowExists {
				printWatchHeader(out, "bob.yaml not found, waiting...")
				continue
			}
			printWatchHeader(out, "bob.yaml changed, replanning...")
			printWatchPlan(out, root, showContent, conflictsOnly, showDiff)
		}
	}
}

// statManifest returns the modification time, size, and existence of the
// manifest file. A missing file returns zero values and exists=false.
func statManifest(path string) (time.Time, int64, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, 0, false
	}
	return info.ModTime(), info.Size(), true
}

// clearScreen writes the ANSI escape sequence that clears the terminal and
// moves the cursor to the home position.
func clearScreen(w io.Writer) {
	fmt.Fprint(w, "\033[2J\033[H")
}

// printWatchHeader writes the timestamped watch-mode header line and a
// separator rule.
func printWatchHeader(w io.Writer, message string) {
	fmt.Fprintf(w, "bob watch: %s — %s\n", time.Now().Format("2006-01-02 15:04:05"), message)
	fmt.Fprintln(w, "─────────────────────────────────────────")
}

// printWatchPlan runs a single plan iteration and writes the human-readable
// output. Manifest and recipe errors are printed inline so the loop can keep
// watching.
func printWatchPlan(w io.Writer, root string, showContent, conflictsOnly, showDiff bool) {
	plan, err := loadPlan(root)
	if err != nil {
		fmt.Fprintf(w, "bob: %v\n", err)
		fmt.Fprintln(w, "next: fix bob.yaml and save to trigger a re-plan")
		return
	}
	var diffs []engine.FileDiff
	if showDiff {
		diffs, err = loadPlanDiff(root, &plan)
		if err != nil {
			fmt.Fprintf(w, "bob: plan: %v\n", err)
			fmt.Fprintln(w, "next: fix bob.yaml and save to trigger a re-plan")
			return
		}
	}
	if err := printPlan(w, plan, showContent, conflictsOnly); err != nil {
		fmt.Fprintf(w, "bob: %v\n", err)
		return
	}
	if err := printDiffs(w, diffs); err != nil {
		fmt.Fprintf(w, "bob: %v\n", err)
	}
}
