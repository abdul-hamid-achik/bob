package engine

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"go.yaml.in/yaml/v3"

	"github.com/abdul-hamid-achik/bob/internal/fsutil"
)

const (
	LockFilename      = "bob.lock"
	ApplyLockFilename = ".bob.apply.lock"
	LockSchemaVersion = 1
	maxLockBytes      = 1 << 20
)

func acquireApplyLock(root string) (func(), error) {
	path := filepath.Join(root, ApplyLockFilename)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("another apply is active or %s is stale", ApplyLockFilename)
		}
		return nil, err
	}
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if _, err := fmt.Fprintf(file, "pid: %d\n", os.Getpid()); err != nil {
		cleanup()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return func() { _ = os.Remove(path) }, nil
}

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// LockFile records Bob's exact whole-file ownership. It intentionally contains
// no transient execution state, commands, environment, or secret values.
type LockFile struct {
	SchemaVersion int         `json:"schema_version" yaml:"schema_version"`
	Recipe        LockRecipe  `json:"recipe" yaml:"recipe"`
	Files         []LockEntry `json:"files" yaml:"files"`
}

type LockRecipe struct {
	ID      string `json:"id" yaml:"id"`
	Version int    `json:"version" yaml:"version"`
}

type LockEntry struct {
	Path   string `json:"path" yaml:"path"`
	SHA256 string `json:"sha256" yaml:"sha256"`
}

// LoadLock loads and strictly validates root/bob.lock.
func LoadLock(root string) (LockFile, error) {
	canonicalRoot, err := validateRoot(root)
	if err != nil {
		return LockFile{}, fmt.Errorf("load lock: %w", err)
	}
	lock, exists, _, err := loadLock(canonicalRoot)
	if err != nil {
		return LockFile{}, fmt.Errorf("load lock: %w", err)
	}
	if !exists {
		return LockFile{}, fmt.Errorf("load lock: %w", os.ErrNotExist)
	}
	return lock, nil
}

func loadLock(root string) (LockFile, bool, []byte, error) {
	data, exists, err := readLockBytes(root)
	if err != nil || !exists {
		return LockFile{}, exists, data, err
	}
	lock, err := fsutil.DecodeStrictYAML[LockFile](data)
	if err != nil {
		return LockFile{}, true, data, fmt.Errorf("decode %s: %w", LockFilename, err)
	}
	if err := validateLock(lock); err != nil {
		return LockFile{}, true, data, err
	}
	return lock, true, data, nil
}

func readLockBytes(root string) ([]byte, bool, error) {
	path := filepath.Join(root, LockFilename)
	data, exists, err := readRegularFile(path, maxLockBytes)
	if err != nil {
		return nil, false, fmt.Errorf("read %s: %w", LockFilename, err)
	}
	return data, exists, nil
}

func validateLock(lock LockFile) error {
	if lock.SchemaVersion != LockSchemaVersion {
		return fmt.Errorf("validate %s: schema_version must be %d", LockFilename, LockSchemaVersion)
	}
	if lock.Recipe.ID == "" || lock.Recipe.Version <= 0 {
		return fmt.Errorf("validate %s: recipe id and positive version are required", LockFilename)
	}
	previous := ""
	for i, entry := range lock.Files {
		path, err := validateRelativePath(entry.Path)
		if err != nil {
			return fmt.Errorf("validate %s entry %d: %w", LockFilename, i, err)
		}
		if path != entry.Path {
			return fmt.Errorf("validate %s entry %d: path %q is not canonical", LockFilename, i, entry.Path)
		}
		if !sha256Pattern.MatchString(entry.SHA256) {
			return fmt.Errorf("validate %s entry %d: sha256 must be 64 lowercase hexadecimal characters", LockFilename, i)
		}
		if i > 0 && path <= previous {
			return fmt.Errorf("validate %s: file entries must be uniquely sorted by path", LockFilename)
		}
		previous = path
	}
	return nil
}

func encodeLock(lock LockFile) ([]byte, error) {
	copyLock := lock
	copyLock.Files = append([]LockEntry(nil), lock.Files...)
	sort.Slice(copyLock.Files, func(i, j int) bool { return copyLock.Files[i].Path < copyLock.Files[j].Path })
	if err := validateLock(copyLock); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(copyLock)
	if err != nil {
		return nil, err
	}
	return append(bytes.TrimSpace(data), '\n'), nil
}

func readRegularFile(path string, maxBytes int64) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() {
		return nil, false, fmt.Errorf("%s is not a regular file", path)
	}
	if info.Size() > maxBytes {
		return nil, false, fmt.Errorf("%s exceeds %d bytes", path, maxBytes)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = file.Close() }()
	openedInfo, err := file.Stat()
	if err != nil {
		return nil, false, err
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return nil, false, fmt.Errorf("%s changed while it was being opened", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(data)) > maxBytes {
		return nil, false, fmt.Errorf("%s exceeds %d bytes", path, maxBytes)
	}
	return data, true, nil
}
