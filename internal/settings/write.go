package settings

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/abdul-hamid-achik/bob/internal/fsutil"
	"go.yaml.in/yaml/v3"
)

// Encode returns the canonical public settings document.
func Encode(value Settings) ([]byte, error) {
	if value.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("settings schema_version must be %d", SchemaVersion)
	}
	if err := validate(value); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode settings: %w", err)
	}
	return append(bytes.TrimSpace(data), '\n'), nil
}

// WriteFile creates a new private settings file without replacing an existing
// path. The containing Bob config directory is private and the publication is
// an atomic same-filesystem link.
func WriteFile(path string, value Settings) error {
	if !filepath.IsAbs(path) {
		return errors.New("settings path must be absolute")
	}
	data, err := Encode(value)
	if err != nil {
		return err
	}
	dir := filepath.Dir(filepath.Clean(path))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create settings directory: %w", err)
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return fmt.Errorf("inspect settings directory: %w", err)
	}
	if fsutil.IsSymlinkOrNotDir(info) {
		return errors.New("settings directory must be a directory, not a symlink")
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure settings directory: %w", err)
	}
	temporary, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return fmt.Errorf("create settings temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure settings temporary file: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write settings temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync settings temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close settings temporary file: %w", err)
	}
	if err := os.Link(temporaryPath, path); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return errors.New("settings file already exists")
		}
		return fmt.Errorf("publish settings file: %w", err)
	}
	return nil
}
