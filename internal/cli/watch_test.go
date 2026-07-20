package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
	"github.com/spf13/cobra"
)

func TestWatchJSONMutuallyExclusive(t *testing.T) {
	t.Parallel()
	_, _, err := executeForTest("plan", "--watch", "--json")
	if err == nil {
		t.Fatal("expected error for --watch --json")
	}
	if !strings.Contains(err.Error(), "--watch and --json are mutually exclusive") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWatchLoopExitsOnContextCancel(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestManifest(t, root)

	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	// Cancel after the initial plan renders.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	if err := runWatchLoop(ctx, cmd, root, false, false, false); err != nil {
		t.Fatalf("runWatchLoop: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "bob watch:") {
		t.Fatalf("expected watch header, got: %s", output)
	}
	if !strings.Contains(output, "stopped after") {
		t.Fatalf("expected stop summary, got: %s", output)
	}
}

func TestWatchReplansOnChange(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestManifest(t, root)

	origInterval := watchPollInterval
	watchPollInterval = 20 * time.Millisecond
	defer func() { watchPollInterval = origInterval }()

	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	go func() {
		// Wait for the initial plan, then modify bob.yaml.
		time.Sleep(50 * time.Millisecond)
		writeTestManifestWithDescription(t, root, "changed description")
		// Wait for the re-plan to be picked up, then cancel.
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	if err := runWatchLoop(ctx, cmd, root, false, false, false); err != nil {
		t.Fatalf("runWatchLoop: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "bob.yaml changed, replanning...") {
		t.Fatalf("expected replan header, got: %s", output)
	}
}

func TestWatchHandlesDeletedManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestManifest(t, root)

	origInterval := watchPollInterval
	watchPollInterval = 20 * time.Millisecond
	defer func() { watchPollInterval = origInterval }()

	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	go func() {
		time.Sleep(50 * time.Millisecond)
		_ = os.Remove(filepath.Join(root, manifest.Filename))
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	if err := runWatchLoop(ctx, cmd, root, false, false, false); err != nil {
		t.Fatalf("runWatchLoop: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "bob.yaml not found, waiting...") {
		t.Fatalf("expected not-found message, got: %s", output)
	}
}

func TestWatchHandlesInvalidManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestManifest(t, root)

	origInterval := watchPollInterval
	watchPollInterval = 20 * time.Millisecond
	defer func() { watchPollInterval = origInterval }()

	ctx, cancel := context.WithCancel(context.Background())
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	go func() {
		time.Sleep(50 * time.Millisecond)
		// Write invalid YAML.
		if err := os.WriteFile(filepath.Join(root, manifest.Filename), []byte("{{invalid yaml"), 0o644); err != nil {
			t.Error(err)
		}
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	if err := runWatchLoop(ctx, cmd, root, false, false, false); err != nil {
		t.Fatalf("runWatchLoop: %v", err)
	}
	output := out.String()
	if !strings.Contains(output, "bob.yaml changed, replanning...") {
		t.Fatalf("expected replan header, got: %s", output)
	}
	if !strings.Contains(output, "next: fix bob.yaml") {
		t.Fatalf("expected fix hint, got: %s", output)
	}
}

func writeTestManifest(t *testing.T, root string) {
	t.Helper()
	writeTestManifestWithDescription(t, root, "A test project.")
}

func writeTestManifestWithDescription(t *testing.T, root, description string) {
	t.Helper()
	m := manifest.Default("testproj", "github.com/test/testproj", description)
	if err := manifest.WriteFile(filepath.Join(root, manifest.Filename), m, true); err != nil {
		t.Fatal(err)
	}
}
