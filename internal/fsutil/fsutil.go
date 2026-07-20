// Package fsutil provides shared filesystem utilities used across Bob's
// internal packages.
package fsutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"
)

// ErrMultipleDocuments is returned by DecodeStrictYAML when the source
// contains more than one YAML document.
var ErrMultipleDocuments = errors.New("multiple YAML documents are not supported")

// IsSymlinkOrNotDir reports whether info describes a symlink or a
// non-directory entry.
func IsSymlinkOrNotDir(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0 || !info.IsDir()
}

// IsSymlinkOrNotRegular reports whether info describes a symlink or a
// non-regular file.
func IsSymlinkOrNotRegular(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular()
}

// DirEntryIsSymlinkOrNotDir reports whether entry is a symlink or not a
// directory.
func DirEntryIsSymlinkOrNotDir(entry fs.DirEntry) bool {
	return entry.Type()&fs.ModeSymlink != 0 || !entry.IsDir()
}

// WriteAtomic writes data to path atomically by writing to a temporary file
// in the same directory and then publishing it. When noReplace is true the
// publication uses os.Link (fails if path exists); otherwise it uses
// os.Rename (replaces an existing file).
func WriteAtomic(path string, data []byte, perm fs.FileMode, noReplace bool) error {
	parent := filepath.Dir(path)
	tmp, err := os.CreateTemp(parent, ".bob-atomic-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if err := tmp.Chmod(perm.Perm()); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temporary file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temporary file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if noReplace {
		if err := os.Link(name, path); err != nil {
			return fmt.Errorf("link temporary file: %w", err)
		}
		return nil
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("rename temporary file: %w", err)
	}
	return nil
}

// DecodeStrictYAML decodes a single YAML document from data into T, rejecting
// unknown fields and multiple documents.
func DecodeStrictYAML[T any](data []byte) (T, error) {
	var target T
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&target); err != nil {
		return target, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return target, ErrMultipleDocuments
		}
		return target, err
	}
	return target, nil
}
