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
		return PublishNoReplace(name, path, perm)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("rename temporary file: %w", err)
	}
	return nil
}

// linkFile is the hard-link primitive behind PublishNoReplace. Tests override
// it to simulate filesystems that do not support hard links.
var linkFile = os.Link

// PublishNoReplace publishes a fully written, closed staged file at
// destination without ever replacing an existing destination, then removes
// the staged file. The primary path is an atomic hard link. When the
// filesystem refuses hard links (exFAT, some SMB/NFS mounts, some FUSE and
// container volumes) it falls back to an exclusive O_EXCL create that copies
// the staged bytes: the no-overwrite guarantee is preserved, and a partially
// written fallback destination is removed on any error so Bob never leaves a
// half-published artifact behind. An existing destination is always an error.
func PublishNoReplace(staged, destination string, perm fs.FileMode) error {
	err := linkFile(staged, destination)
	if err == nil {
		return removeStaged(staged)
	}
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("link staged file: %w", err)
	}
	if fallbackErr := copyExclusive(staged, destination, perm); fallbackErr != nil {
		if errors.Is(fallbackErr, os.ErrExist) {
			return fmt.Errorf("publish without replace: %w", fallbackErr)
		}
		return fmt.Errorf("link staged file: %w (exclusive-create fallback: %v)", err, fallbackErr)
	}
	return removeStaged(staged)
}

func removeStaged(staged string) error {
	if err := os.Remove(staged); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove staged file: %w", err)
	}
	return nil
}

// copyExclusive creates destination with O_EXCL and copies the staged bytes
// into it. It is only reached when hard links are unavailable.
func copyExclusive(staged, destination string, perm fs.FileMode) error {
	data, err := os.ReadFile(staged)
	if err != nil {
		return fmt.Errorf("read staged file: %w", err)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm.Perm())
	if err != nil {
		return fmt.Errorf("create destination exclusively: %w", err)
	}
	fail := func(cause error) error {
		_ = file.Close()
		_ = os.Remove(destination)
		return cause
	}
	if err := file.Chmod(perm.Perm()); err != nil {
		return fail(fmt.Errorf("chmod destination: %w", err))
	}
	if _, err := file.Write(data); err != nil {
		return fail(fmt.Errorf("write destination: %w", err))
	}
	if err := file.Sync(); err != nil {
		return fail(fmt.Errorf("sync destination: %w", err))
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(destination)
		return fmt.Errorf("close destination: %w", err)
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
