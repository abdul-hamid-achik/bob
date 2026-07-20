package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockFileInfo implements fs.FileInfo for testing.
type mockFileInfo struct {
	mode fs.FileMode
}

func (m mockFileInfo) Name() string       { return "mock" }
func (m mockFileInfo) Size() int64        { return 0 }
func (m mockFileInfo) Mode() fs.FileMode  { return m.mode }
func (m mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m mockFileInfo) IsDir() bool        { return m.mode.IsDir() }
func (m mockFileInfo) Sys() any           { return nil }

func TestIsSymlinkOrNotDir(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		info fs.FileInfo
		want bool
	}{
		{"regular dir", mockFileInfo{mode: fs.ModeDir | 0o755}, false},
		{"symlink to dir", mockFileInfo{mode: fs.ModeSymlink | fs.ModeDir | 0o777}, true},
		{"regular file", mockFileInfo{mode: 0o644}, true},
		{"symlink", mockFileInfo{mode: fs.ModeSymlink | 0o777}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsSymlinkOrNotDir(tt.info); got != tt.want {
				t.Errorf("IsSymlinkOrNotDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsSymlinkOrNotRegular(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		info fs.FileInfo
		want bool
	}{
		{"regular file", mockFileInfo{mode: 0o644}, false},
		{"symlink", mockFileInfo{mode: fs.ModeSymlink | 0o777}, true},
		{"directory", mockFileInfo{mode: fs.ModeDir | 0o755}, true},
		{"symlink to file", mockFileInfo{mode: fs.ModeSymlink | 0o644}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsSymlinkOrNotRegular(tt.info); got != tt.want {
				t.Errorf("IsSymlinkOrNotRegular() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDirEntryIsSymlinkOrNotDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		got := DirEntryIsSymlinkOrNotDir(entry)
		if entry.Name() == "subdir" && got {
			t.Errorf("DirEntryIsSymlinkOrNotDir(subdir) = true, want false")
		}
		if entry.Name() == "file.txt" && !got {
			t.Errorf("DirEntryIsSymlinkOrNotDir(file.txt) = false, want true")
		}
	}
}

func TestWriteAtomic(t *testing.T) {
	t.Parallel()
	t.Run("creates file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "out.txt")
		if err := WriteAtomic(path, []byte("hello"), 0o644, false); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "hello" {
			t.Errorf("content = %q, want %q", data, "hello")
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Errorf("perm = %o, want 644", info.Mode().Perm())
		}
	})
	t.Run("replaces existing file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "out.txt")
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := WriteAtomic(path, []byte("new"), 0o600, false); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "new" {
			t.Errorf("content = %q, want %q", data, "new")
		}
	})
	t.Run("no replace fails on existing", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "out.txt")
		if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := WriteAtomic(path, []byte("new"), 0o644, true)
		if err == nil {
			t.Fatal("expected error for no-replace on existing file")
		}
		if !errors.Is(err, fs.ErrExist) {
			t.Errorf("error = %v, want fs.ErrExist", err)
		}
	})
	t.Run("no replace creates new file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		path := filepath.Join(dir, "out.txt")
		if err := WriteAtomic(path, []byte("fresh"), 0o644, true); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "fresh" {
			t.Errorf("content = %q, want %q", data, "fresh")
		}
	})
}

func TestDecodeStrictYAML(t *testing.T) {
	t.Parallel()
	type sample struct {
		Name string `yaml:"name"`
		Age  int    `yaml:"age"`
	}
	t.Run("valid document", func(t *testing.T) {
		t.Parallel()
		got, err := DecodeStrictYAML[sample]([]byte("name: alice\nage: 30\n"))
		if err != nil {
			t.Fatal(err)
		}
		if got.Name != "alice" || got.Age != 30 {
			t.Errorf("got %+v", got)
		}
	})
	t.Run("rejects unknown fields", func(t *testing.T) {
		t.Parallel()
		_, err := DecodeStrictYAML[sample]([]byte("name: alice\nunknown: true\n"))
		if err == nil {
			t.Fatal("expected error for unknown field")
		}
	})
	t.Run("rejects multiple documents", func(t *testing.T) {
		t.Parallel()
		_, err := DecodeStrictYAML[sample]([]byte("name: alice\n---\nname: bob\n"))
		if !errors.Is(err, ErrMultipleDocuments) {
			t.Errorf("error = %v, want ErrMultipleDocuments", err)
		}
	})
	t.Run("rejects invalid yaml", func(t *testing.T) {
		t.Parallel()
		_, err := DecodeStrictYAML[sample]([]byte(":\n  - invalid\n"))
		if err == nil {
			t.Fatal("expected error for invalid yaml")
		}
	})
}
