package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileCreatesPrivateStrictSettingsWithoutReplacement(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	want := Default()
	want.Telemetry.Enabled = true
	if err := WriteFile(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("settings = %#v, want %#v", got, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("settings mode = %o, want 600", info.Mode().Perm())
	}
	if err := WriteFile(path, Default()); err == nil {
		t.Fatal("second settings write unexpectedly replaced the file")
	}
}
