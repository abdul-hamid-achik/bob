package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	Filename      = "bob.yaml"
	SchemaVersion = 1
	maxBytes      = 1 << 20
)

var projectNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Manifest is the human-owned declaration of a repository Bob can construct.
type Manifest struct {
	SchemaVersion int          `json:"schema_version" yaml:"schema_version"`
	Recipe        string       `json:"recipe" yaml:"recipe"`
	Product       Product      `json:"product" yaml:"product"`
	Runtime       Runtime      `json:"runtime" yaml:"runtime"`
	Surfaces      Surfaces     `json:"surfaces" yaml:"surfaces"`
	Integrations  Integrations `json:"integrations" yaml:"integrations"`
	Distribution  Distribution `json:"distribution" yaml:"distribution"`
}

type Product struct {
	Name        string `json:"name" yaml:"name"`
	Module      string `json:"module" yaml:"module"`
	Description string `json:"description" yaml:"description"`
	Visibility  string `json:"visibility" yaml:"visibility"`
	License     string `json:"license" yaml:"license"`
}

type Runtime struct {
	Language string `json:"language" yaml:"language"`
	Kind     string `json:"kind" yaml:"kind"`
}

type Surfaces struct {
	CLI    bool `json:"cli" yaml:"cli"`
	JSON   bool `json:"json" yaml:"json"`
	MCP    bool `json:"mcp" yaml:"mcp"`
	Studio bool `json:"studio" yaml:"studio"`
}

type Integrations struct {
	CodeStructure        string `json:"code_structure" yaml:"code_structure"`
	SemanticSearch       string `json:"semantic_search" yaml:"semantic_search"`
	TerminalVerification string `json:"terminal_verification" yaml:"terminal_verification"`
	BrowserVerification  string `json:"browser_verification" yaml:"browser_verification"`
	Secrets              string `json:"secrets" yaml:"secrets"`
	Artifacts            string `json:"artifacts" yaml:"artifacts"`
}

type Distribution struct {
	GitHubActions bool   `json:"github_actions" yaml:"github_actions"`
	GoReleaser    bool   `json:"goreleaser" yaml:"goreleaser"`
	Homebrew      bool   `json:"homebrew" yaml:"homebrew"`
	Docs          string `json:"docs" yaml:"docs"`
}

func Default(name, module, description string) Manifest {
	if description == "" {
		description = "A local-first, agent-ready command-line tool."
	}
	return Manifest{
		SchemaVersion: SchemaVersion,
		Recipe:        "go-agent-tool",
		Product: Product{
			Name:        name,
			Module:      module,
			Description: description,
			Visibility:  "public",
			License:     "MIT",
		},
		Runtime:  Runtime{Language: "go", Kind: "cli"},
		Surfaces: Surfaces{CLI: true, JSON: true},
		Integrations: Integrations{
			CodeStructure:        "codemap",
			SemanticSearch:       "vecgrep",
			TerminalVerification: "glyphrun",
			BrowserVerification:  "none",
			Secrets:              "none",
			Artifacts:            "none",
		},
		Distribution: Distribution{
			GitHubActions: true,
			GoReleaser:    true,
			Docs:          "markdown",
		},
	}
}

func (m Manifest) Validate() error {
	var problems []string
	if m.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Sprintf("schema_version must be %d", SchemaVersion))
	}
	if m.Recipe != "go-agent-tool" {
		problems = append(problems, "recipe must be go-agent-tool")
	}
	if !projectNamePattern.MatchString(m.Product.Name) {
		problems = append(problems, "product.name must start with a letter and contain only lowercase letters, digits, and hyphens")
	}
	if strings.TrimSpace(m.Product.Module) == "" || strings.ContainsAny(m.Product.Module, " \t\r\n") {
		problems = append(problems, "product.module must be a non-empty Go module path without whitespace")
	} else if invalid := unsupportedModuleRune(m.Product.Module); invalid != 0 {
		problems = append(problems, fmt.Sprintf("product.module contains unsupported character %q", invalid))
	} else if strings.HasPrefix(m.Product.Module, "/") || strings.HasSuffix(m.Product.Module, "/") || strings.Contains(m.Product.Module, "//") {
		problems = append(problems, "product.module must contain non-empty slash-separated path segments")
	} else {
		for _, segment := range strings.Split(m.Product.Module, "/") {
			if segment == "." || segment == ".." {
				problems = append(problems, "product.module cannot contain . or .. path segments")
				break
			}
		}
	}
	if strings.TrimSpace(m.Product.Description) == "" {
		problems = append(problems, "product.description is required")
	}
	if m.Product.Visibility != "public" && m.Product.Visibility != "private" {
		problems = append(problems, "product.visibility must be public or private")
	}
	if m.Product.License != "MIT" {
		problems = append(problems, "product.license must be MIT in schema version 1")
	}
	if m.Runtime.Language != "go" || m.Runtime.Kind != "cli" {
		problems = append(problems, "go-agent-tool requires runtime.language=go and runtime.kind=cli")
	}
	if !m.Surfaces.CLI {
		problems = append(problems, "go-agent-tool requires surfaces.cli=true")
	}
	if !m.Surfaces.JSON {
		problems = append(problems, "go-agent-tool requires surfaces.json=true")
	}
	if m.Surfaces.MCP || m.Surfaces.Studio {
		problems = append(problems, "mcp and studio surfaces are reserved for a later recipe version")
	}
	validateChoice := func(field, value string, allowed ...string) {
		for _, candidate := range allowed {
			if value == candidate {
				return
			}
		}
		problems = append(problems, fmt.Sprintf("%s must be one of %s", field, strings.Join(allowed, ", ")))
	}
	validateChoice("integrations.code_structure", m.Integrations.CodeStructure, "none", "codemap")
	validateChoice("integrations.semantic_search", m.Integrations.SemanticSearch, "none", "vecgrep")
	validateChoice("integrations.terminal_verification", m.Integrations.TerminalVerification, "none", "glyphrun")
	validateChoice("integrations.browser_verification", m.Integrations.BrowserVerification, "none", "cairntrace")
	validateChoice("integrations.secrets", m.Integrations.Secrets, "none", "tinyvault")
	validateChoice("integrations.artifacts", m.Integrations.Artifacts, "none", "fcheap")
	validateChoice("distribution.docs", m.Distribution.Docs, "none", "markdown")
	if m.Distribution.Homebrew && !m.Distribution.GoReleaser {
		problems = append(problems, "distribution.homebrew requires distribution.goreleaser=true")
	}
	if m.Distribution.Homebrew && m.Product.Visibility != "public" {
		problems = append(problems, "distribution.homebrew requires product.visibility=public")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func unsupportedModuleRune(modulePath string) rune {
	for _, r := range modulePath {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '-', '.', '_', '~', '/':
			continue
		default:
			return r
		}
	}
	return 0
}

func Load(root string) (Manifest, error) {
	return LoadFile(filepath.Join(root, Filename))
}

func LoadFile(path string) (Manifest, error) {
	var m Manifest
	info, err := os.Lstat(path)
	if err != nil {
		return m, fmt.Errorf("read manifest: %w", err)
	}
	if !info.Mode().IsRegular() {
		return m, fmt.Errorf("read manifest: %s is not a regular file", path)
	}
	if info.Size() > maxBytes {
		return m, fmt.Errorf("read manifest: file exceeds %d bytes", maxBytes)
	}
	f, err := os.Open(path)
	if err != nil {
		return m, fmt.Errorf("read manifest: %w", err)
	}
	defer func() { _ = f.Close() }()
	dec := yaml.NewDecoder(io.LimitReader(f, maxBytes+1))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return m, fmt.Errorf("decode manifest: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return m, errors.New("decode manifest: multiple YAML documents are not supported")
		}
		return m, fmt.Errorf("decode manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return m, fmt.Errorf("validate manifest: %w", err)
	}
	return m, nil
}

func Encode(m Manifest) ([]byte, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("encode manifest: %w", err)
	}
	return bytes.TrimSpace(data), nil
}

func WriteFile(path string, m Manifest, overwrite bool) error {
	data, err := Encode(m)
	if err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	parentInfo, err := os.Lstat(filepath.Dir(path))
	if err != nil || parentInfo.Mode()&os.ModeSymlink != 0 || !parentInfo.IsDir() {
		return fmt.Errorf("write manifest: parent is not a regular directory")
	}
	if info, statErr := os.Lstat(path); statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("write manifest: %s is not a regular file", path)
		}
		if !overwrite {
			return fmt.Errorf("write manifest: %s already exists", path)
		}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("write manifest: %w", statErr)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".bob-manifest-*")
	if err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write manifest: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write manifest: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	if !overwrite {
		if err := os.Link(tmpName, path); err != nil {
			return fmt.Errorf("write manifest: publish without replacement: %w", err)
		}
		if err := os.Remove(tmpName); err != nil {
			return fmt.Errorf("write manifest: remove temporary file: %w", err)
		}
		return nil
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("write manifest: replace: %w", err)
	}
	return nil
}
