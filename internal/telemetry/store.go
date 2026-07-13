package telemetry

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	metadataFilename = "meta.json"
	versionDir       = "v1"
	maxEventBytes    = 16 << 10
	maxMetadataBytes = 4 << 10
	maxRetentionDays = 365
	maxEventsPerDay  = 10_000
)

// Config controls the local telemetry store. StateDir is Bob's resolved XDG
// state directory; the store uses its telemetry child.
type Config struct {
	StateDir        string
	Enabled         bool
	RetentionDays   int
	MaxEventsPerDay int
	Now             func() time.Time
	Random          io.Reader
}

// Store is a local-only event store. Its methods are safe for concurrent
// goroutines and cooperating processes because each event atomically claims a
// unique daily slot.
type Store struct {
	enabled         bool
	root            string
	retentionDays   int
	maxEventsPerDay int
	now             func() time.Time
	random          io.Reader
	randomMu        sync.Mutex
	workspaceKey    []byte
}

type metadata struct {
	SchemaVersion    int       `json:"schema_version"`
	CreatedAt        time.Time `json:"created_at"`
	WorkspaceHMACKey string    `json:"workspace_hmac_key"`
}

// Enabled reports whether this store is configured to persist local events.
func (store *Store) Enabled() bool {
	return store != nil && store.enabled
}

// Open initializes an enabled store's private directory and random workspace
// pseudonym key. A disabled store performs no filesystem writes.
func Open(config Config) (*Store, error) {
	now := config.Now
	if now == nil {
		now = time.Now
	}
	random := config.Random
	if random == nil {
		random = rand.Reader
	}
	store := &Store{
		enabled:         config.Enabled,
		retentionDays:   config.RetentionDays,
		maxEventsPerDay: config.MaxEventsPerDay,
		now:             now,
		random:          random,
	}
	if !config.Enabled {
		return store, nil
	}
	if !filepath.IsAbs(config.StateDir) {
		return nil, errors.New("telemetry state directory must be absolute")
	}
	if config.RetentionDays < 1 || config.RetentionDays > maxRetentionDays {
		return nil, fmt.Errorf("telemetry retention days must be between 1 and %d", maxRetentionDays)
	}
	if config.MaxEventsPerDay < 1 || config.MaxEventsPerDay > maxEventsPerDay {
		return nil, fmt.Errorf("telemetry daily cap must be between 1 and %d", maxEventsPerDay)
	}
	store.root = filepath.Join(filepath.Clean(config.StateDir), "telemetry")
	if err := secureMkdirAll(store.root); err != nil {
		return nil, fmt.Errorf("create telemetry directory: %w", err)
	}
	key, err := store.loadOrCreateMetadata()
	if err != nil {
		return nil, err
	}
	store.workspaceKey = key
	return store, nil
}

// Record validates and atomically persists one event. Callers that must not be
// affected by telemetry failures should wrap the Store with BestEffort.
func (store *Store) Record(ctx context.Context, event Event) error {
	if store == nil || !store.enabled {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateEvent(event, false); err != nil {
		return err
	}
	now := store.now().UTC()
	if _, err := store.Prune(ctx); err != nil {
		return fmt.Errorf("prune telemetry: %w", err)
	}
	idBytes, err := store.randomBytes(16)
	if err != nil {
		return fmt.Errorf("create telemetry event id: %w", err)
	}
	event.SchemaVersion = SchemaVersion
	event.EventID = "evt_" + hex.EncodeToString(idBytes)
	event.RecordedAt = now
	if err := validateEvent(event, true); err != nil {
		return err
	}
	return store.writeEvent(event)
}

// WorkspaceID returns a stable, machine-local HMAC pseudonym for an existing
// workspace. The canonical path is used as HMAC input and is never persisted.
func (store *Store) WorkspaceID(path string) (string, error) {
	if store == nil || !store.enabled {
		return "", nil
	}
	if len(store.workspaceKey) == 0 {
		return "", errors.New("telemetry is disabled")
	}
	if strings.TrimSpace(path) == "" {
		return "", errors.New("workspace path is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve workspace path: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("inspect workspace path: %w", err)
	}
	if !info.IsDir() {
		return "", errors.New("workspace path must be a directory")
	}
	canonical, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", fmt.Errorf("canonicalize workspace path: %w", err)
	}
	digest := hmac.New(sha256.New, store.workspaceKey)
	_, _ = digest.Write([]byte("bob-workspace-v1\x00"))
	_, _ = digest.Write([]byte(filepath.Clean(canonical)))
	return "w_" + hex.EncodeToString(digest.Sum(nil)[:16]), nil
}

func (store *Store) writeEvent(event Event) error {
	versionRoot, err := secureSubdirectory(store.root, versionDir)
	if err != nil {
		return fmt.Errorf("create telemetry version directory: %w", err)
	}
	dayDir, err := secureSubdirectory(versionRoot, event.RecordedAt.UTC().Format(time.DateOnly))
	if err != nil {
		return fmt.Errorf("create telemetry day directory: %w", err)
	}
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode telemetry event: %w", err)
	}
	data = append(data, '\n')
	if len(data) > maxEventBytes {
		return errors.New("telemetry event exceeds size limit")
	}

	temporary, err := writeTemporary(dayDir, ".event-*", data)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temporary) }()
	for slot := 0; slot < store.maxEventsPerDay; slot++ {
		final := filepath.Join(dayDir, fmt.Sprintf("%06d.json", slot))
		err := os.Link(temporary, final)
		if err == nil {
			return nil
		}
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return fmt.Errorf("publish telemetry event: %w", err)
	}
	return ErrDailyCap
}

func (store *Store) loadOrCreateMetadata() ([]byte, error) {
	path := filepath.Join(store.root, metadataFilename)
	key, err := store.readMetadata(path)
	if err == nil {
		return key, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	key, err = store.randomBytes(32)
	if err != nil {
		return nil, fmt.Errorf("create telemetry workspace key: %w", err)
	}
	document := metadata{
		SchemaVersion:    SchemaVersion,
		CreatedAt:        store.now().UTC(),
		WorkspaceHMACKey: base64.RawStdEncoding.EncodeToString(key),
	}
	data, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode telemetry metadata: %w", err)
	}
	data = append(data, '\n')
	temporary, err := writeTemporary(store.root, ".meta-*", data)
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(temporary) }()
	if err := os.Link(temporary, path); err != nil && !errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("publish telemetry metadata: %w", err)
	}
	return store.readMetadata(path)
}

func (store *Store) readMetadata(path string) ([]byte, error) {
	data, err := readPrivateRegularFile(path, maxMetadataBytes)
	if err != nil {
		return nil, err
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err == nil && header.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf("%w: metadata version %d", ErrNewerSchema, header.SchemaVersion)
	}
	var document metadata
	if err := decodeStrictJSON(data, &document); err != nil {
		return nil, fmt.Errorf("decode telemetry metadata: %w", err)
	}
	if document.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("unsupported telemetry metadata schema_version %d", document.SchemaVersion)
	}
	if document.CreatedAt.IsZero() {
		return nil, errors.New("telemetry metadata created_at is required")
	}
	key, err := base64.RawStdEncoding.DecodeString(document.WorkspaceHMACKey)
	if err != nil || len(key) != 32 {
		return nil, errors.New("telemetry metadata has an invalid workspace key")
	}
	return key, nil
}

func secureMkdirAll(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return errors.New("telemetry directory must be a directory, not a symlink")
	}
	return os.Chmod(path, 0o700)
}

func secureSubdirectory(parent, name string) (string, error) {
	if filepath.Base(name) != name || name == "." || name == ".." {
		return "", errors.New("invalid telemetry directory name")
	}
	parentInfo, err := os.Lstat(parent)
	if err != nil {
		return "", err
	}
	if parentInfo.Mode()&fs.ModeSymlink != 0 || !parentInfo.IsDir() {
		return "", errors.New("telemetry parent must be a directory, not a symlink")
	}
	path := filepath.Join(parent, name)
	if err := os.Mkdir(path, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.IsDir() {
		return "", errors.New("telemetry child must be a directory, not a symlink")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return "", err
	}
	return path, nil
}

func writeTemporary(dir, pattern string, data []byte) (path string, returnedErr error) {
	file, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("create telemetry temporary file: %w", err)
	}
	path = file.Name()
	defer func() {
		if returnedErr != nil {
			_ = file.Close()
			_ = os.Remove(path)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return "", fmt.Errorf("secure telemetry temporary file: %w", err)
	}
	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("write telemetry temporary file: %w", err)
	}
	if err := file.Sync(); err != nil {
		return "", fmt.Errorf("sync telemetry temporary file: %w", err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("close telemetry temporary file: %w", err)
	}
	return path, nil
}

func readPrivateRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("telemetry file must be a regular file, not a symlink")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("telemetry file exceeds %d bytes", limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("telemetry file exceeds %d bytes", limit)
	}
	return data, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func randomBytes(reader io.Reader, size int) ([]byte, error) {
	value := make([]byte, size)
	if _, err := io.ReadFull(reader, value); err != nil {
		return nil, err
	}
	return value, nil
}

func (store *Store) randomBytes(size int) ([]byte, error) {
	store.randomMu.Lock()
	defer store.randomMu.Unlock()
	return randomBytes(store.random, size)
}
