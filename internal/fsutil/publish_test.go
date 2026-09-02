package fsutil

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// TestPublishNoReplaceFallsBackWithoutHardLinks simulates a filesystem that
// refuses hard links and proves the exclusive-create fallback publishes the
// staged bytes, keeps the requested mode, removes the staged file, and still
// never replaces an existing destination. It is not parallel because it
// swaps the package-level link primitive.
func TestPublishNoReplaceFallsBackWithoutHardLinks(t *testing.T) {
	original := linkFile
	linkFile = func(string, string) error { return &os.LinkError{Op: "link", Err: syscall.ENOTSUP} }
	t.Cleanup(func() { linkFile = original })

	dir := t.TempDir()
	staged := filepath.Join(dir, ".bob-stage-x")
	if err := os.WriteFile(staged, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	destination := filepath.Join(dir, "out.sh")
	if err := PublishNoReplace(staged, destination, 0o755); err != nil {
		t.Fatalf("fallback publish failed: %v", err)
	}
	data, err := os.ReadFile(destination)
	if err != nil || string(data) != "payload" {
		t.Fatalf("destination = %q, %v", data, err)
	}
	info, err := os.Stat(destination)
	if err != nil || info.Mode().Perm() != 0o755 {
		t.Fatalf("mode = %v, %v", info.Mode(), err)
	}
	if _, err := os.Stat(staged); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("staged file should be removed: %v", err)
	}

	// The fallback must never overwrite an existing destination.
	if err := os.WriteFile(staged, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = PublishNoReplace(staged, destination, 0o644)
	if err == nil || !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected ErrExist from fallback, got %v", err)
	}
	if data, _ := os.ReadFile(destination); string(data) != "payload" {
		t.Fatalf("existing destination was replaced: %q", data)
	}
	if _, err := os.Stat(staged); err != nil {
		t.Fatalf("staged file must survive a refused publish: %v", err)
	}
}

func TestPublishNoReplaceRefusesExistingDestinationViaLink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	staged := filepath.Join(dir, "staged")
	destination := filepath.Join(dir, "dest")
	if err := os.WriteFile(staged, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destination, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := PublishNoReplace(staged, destination, 0o644); !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected ErrExist, got %v", err)
	}
	if data, _ := os.ReadFile(destination); string(data) != "old" {
		t.Fatalf("destination replaced: %q", data)
	}
}
