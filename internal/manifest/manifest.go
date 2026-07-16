package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/abdul-hamid-achik/bob/internal/strsim"
)

const (
	Filename      = "bob.yaml"
	SchemaVersion = 1
	maxBytes      = 1 << 20

	// RecipeGoAgentTool scaffolds a public-ready Go and Cobra CLI.
	RecipeGoAgentTool = "go-agent-tool"
	// RecipeFiles declares an arbitrary file tree inline.
	RecipeFiles = "files"

	// Stack hygiene recipes seed a deliberately small set of repository
	// hygiene files (docs presence, .gitignore, a CI stub) for one detected
	// language stack. Seeded files are created once when missing and are
	// never updated or overwritten afterwards; application source is never
	// owned by these recipes.
	RecipeTSApp     = "ts-app"
	RecipeJSApp     = "js-app"
	RecipeVueApp    = "vue-app"
	RecipePythonApp = "python-app"
	RecipeRubyApp   = "ruby-app"
	RecipeLuaLib    = "lua-lib"
	RecipeRustCLI   = "rust-cli"
	RecipeStaticWeb = "static-web"
)

// StackRuntime is the closed runtime contract of one stack hygiene recipe.
// The first language and kind are the defaults DefaultStack selects.
type StackRuntime struct {
	Languages []string
	Kinds     []string
}

// stackRecipeRuntimes is the schema-side source of truth for stack recipe
// runtime contracts. recipe/stack.go must declare a renderer for every id in
// this map; a recipe-package test asserts the two stay in sync.
var stackRecipeRuntimes = map[string]StackRuntime{
	RecipeTSApp:     {Languages: []string{"typescript"}, Kinds: []string{"app", "monorepo"}},
	RecipeJSApp:     {Languages: []string{"javascript"}, Kinds: []string{"app", "monorepo"}},
	RecipeVueApp:    {Languages: []string{"typescript", "javascript"}, Kinds: []string{"web-app"}},
	RecipePythonApp: {Languages: []string{"python"}, Kinds: []string{"app"}},
	RecipeRubyApp:   {Languages: []string{"ruby"}, Kinds: []string{"app", "gem"}},
	RecipeLuaLib:    {Languages: []string{"lua"}, Kinds: []string{"lib", "plugin"}},
	RecipeRustCLI:   {Languages: []string{"rust"}, Kinds: []string{"cli"}},
	RecipeStaticWeb: {Languages: []string{"html"}, Kinds: []string{"site"}},
}

// StackRecipeRuntime reports the runtime contract for a stack hygiene recipe
// id, and false for every other recipe id.
func StackRecipeRuntime(recipeID string) (StackRuntime, bool) {
	runtime, ok := stackRecipeRuntimes[recipeID]
	return runtime, ok
}

// IsStackRecipe reports whether id names a stack hygiene recipe.
func IsStackRecipe(id string) bool {
	_, ok := stackRecipeRuntimes[id]
	return ok
}

// StackRecipeIDs returns the sorted stack hygiene recipe identifiers.
func StackRecipeIDs() []string {
	ids := make([]string, 0, len(stackRecipeRuntimes))
	for id := range stackRecipeRuntimes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

var (
	projectNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	varKeyPattern      = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	fileModePattern    = regexp.MustCompile(`^[0-7]{3,4}$`)
)

// recipeIDs is the closed set of recipe identifiers Validate accepts. Keep in
// sync with recipe.IDs(); manifest cannot import recipe (recipe imports
// manifest), so this list is the schema-side source of truth for the id set.
var recipeIDs = func() []string {
	ids := append([]string{RecipeFiles, RecipeGoAgentTool}, StackRecipeIDs()...)
	sort.Strings(ids)
	return ids
}()

// Manifest is the human-owned declaration of a repository Bob can construct.
type Manifest struct {
	SchemaVersion int               `json:"schema_version" yaml:"schema_version"`
	Recipe        string            `json:"recipe" yaml:"recipe"`
	Product       Product           `json:"product" yaml:"product"`
	Runtime       Runtime           `json:"runtime" yaml:"runtime"`
	Surfaces      Surfaces          `json:"surfaces" yaml:"surfaces"`
	Integrations  Integrations      `json:"integrations" yaml:"integrations"`
	Distribution  Distribution      `json:"distribution" yaml:"distribution"`
	Vars          map[string]string `json:"vars,omitempty" yaml:"vars,omitempty"`
	Files         []FileDecl        `json:"files,omitempty" yaml:"files,omitempty"`
}

// FileDecl is one file the files recipe materializes verbatim, subject only
// to the ${vars.*} substitution pass.
type FileDecl struct {
	Path    string `json:"path" yaml:"path"`
	Mode    string `json:"mode,omitempty" yaml:"mode,omitempty"`
	Content string `json:"content" yaml:"content"`
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
		Recipe:        RecipeGoAgentTool,
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

// describeValue renders the offending value for a validation problem string.
// Empty values are named explicitly rather than rendered as `""`, since a
// bare pair of quotes reads as noise to a weak model scanning error text.
func describeValue(value string) string {
	if value == "" {
		return "empty"
	}
	return fmt.Sprintf("%q", value)
}

// suggestionSuffix appends a did-you-mean hint when value is within edit
// distance 2 of one of allowed. It returns "" when no candidate is close
// enough, so call sites can append unconditionally.
func suggestionSuffix(value string, allowed []string) string {
	if suggestion, ok := strsim.Closest(value, allowed, 2); ok {
		return fmt.Sprintf("; did you mean %q?", suggestion)
	}
	return ""
}

func (m Manifest) Validate() error {
	var problems []string
	if m.SchemaVersion != SchemaVersion {
		problems = append(problems, fmt.Sprintf("schema_version must be %d (got %d)", SchemaVersion, m.SchemaVersion))
	}
	if m.Recipe != RecipeGoAgentTool && m.Recipe != RecipeFiles && !IsStackRecipe(m.Recipe) {
		problems = append(problems, fmt.Sprintf("recipe must be one of %s (got %s)%s", strings.Join(recipeIDs, ", "), describeValue(m.Recipe), suggestionSuffix(m.Recipe, recipeIDs)))
	}
	if !projectNamePattern.MatchString(m.Product.Name) {
		problems = append(problems, fmt.Sprintf("product.name must start with a letter and contain only lowercase letters, digits, and hyphens (got %s)", describeValue(m.Product.Name)))
	}
	if strings.TrimSpace(m.Product.Description) == "" {
		problems = append(problems, "product.description is required")
	}

	switch {
	case m.Recipe == RecipeFiles:
		problems = append(problems, m.validateFilesRecipe()...)
	case m.Recipe == RecipeGoAgentTool:
		problems = append(problems, m.validateGoAgentToolRecipe()...)
	case IsStackRecipe(m.Recipe):
		problems = append(problems, m.validateStackRecipe()...)
	default:
		// An unrecognized recipe is already reported above; there is no
		// per-recipe schema to check further.
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// validateGoAgentToolRecipe implements every go-agent-tool validation rule.
// Error strings are load-bearing: keep their leading constraint text stable;
// the trailing "(got ...)" and "; did you mean ...?" suffixes are additive.
func (m Manifest) validateGoAgentToolRecipe() []string {
	var problems []string
	if strings.TrimSpace(m.Product.Module) == "" || strings.ContainsAny(m.Product.Module, " \t\r\n") {
		problems = append(problems, "product.module must be a non-empty Go module path without whitespace")
	} else if invalid := unsupportedModuleRune(m.Product.Module); invalid != 0 {
		problems = append(problems, fmt.Sprintf("product.module contains unsupported character %q", invalid))
	} else if strings.HasPrefix(m.Product.Module, "/") || strings.HasSuffix(m.Product.Module, "/") || strings.Contains(m.Product.Module, "//") {
		problems = append(problems, fmt.Sprintf("product.module must contain non-empty slash-separated path segments (got %s)", describeValue(m.Product.Module)))
	} else {
		for _, segment := range strings.Split(m.Product.Module, "/") {
			if segment == "." || segment == ".." {
				problems = append(problems, fmt.Sprintf("product.module cannot contain . or .. path segments (got %s)", describeValue(m.Product.Module)))
				break
			}
		}
	}
	if m.Product.Visibility != "public" && m.Product.Visibility != "private" {
		problems = append(problems, fmt.Sprintf("product.visibility must be public or private (got %s)%s", describeValue(m.Product.Visibility), suggestionSuffix(m.Product.Visibility, []string{"public", "private"})))
	}
	if m.Product.License != "MIT" {
		problems = append(problems, fmt.Sprintf("product.license must be MIT in schema version 1 (got %s)", describeValue(m.Product.License)))
	}
	if m.Runtime.Language != "go" || m.Runtime.Kind != "cli" {
		problems = append(problems, fmt.Sprintf("go-agent-tool requires runtime.language=go and runtime.kind=cli (got language=%s, kind=%s)", describeValue(m.Runtime.Language), describeValue(m.Runtime.Kind)))
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
		problems = append(problems, fmt.Sprintf("%s must be one of %s (got %s)%s", field, strings.Join(allowed, ", "), describeValue(value), suggestionSuffix(value, allowed)))
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
	if len(m.Vars) > 0 {
		problems = append(problems, "vars is only supported by recipe files")
	}
	if len(m.Files) > 0 {
		problems = append(problems, "files is only supported by recipe files")
	}
	return problems
}

// optionalModuleProblems validates product.module for recipes where the
// module is an optional repository identity string rather than a required Go
// module path. An empty module is fine; a non-empty module must satisfy the
// same shape rules the go-agent-tool recipe enforces.
func optionalModuleProblems(module string) []string {
	var problems []string
	if strings.TrimSpace(module) == "" {
		return problems
	}
	if strings.ContainsAny(module, " \t\r\n") {
		problems = append(problems, "product.module must be a non-empty Go module path without whitespace")
	} else if invalid := unsupportedModuleRune(module); invalid != 0 {
		problems = append(problems, fmt.Sprintf("product.module contains unsupported character %q", invalid))
	} else if strings.HasPrefix(module, "/") || strings.HasSuffix(module, "/") || strings.Contains(module, "//") {
		problems = append(problems, fmt.Sprintf("product.module must contain non-empty slash-separated path segments (got %s)", describeValue(module)))
	} else {
		for _, segment := range strings.Split(module, "/") {
			if segment == "." || segment == ".." {
				problems = append(problems, fmt.Sprintf("product.module cannot contain . or .. path segments (got %s)", describeValue(module)))
				break
			}
		}
	}
	return problems
}

// optionalIdentityProblems validates the optional visibility and license
// fields shared by the files recipe and every stack hygiene recipe.
func optionalIdentityProblems(p Product) []string {
	var problems []string
	if p.Visibility != "" && p.Visibility != "public" && p.Visibility != "private" {
		problems = append(problems, fmt.Sprintf("product.visibility must be public or private (got %s)%s", describeValue(p.Visibility), suggestionSuffix(p.Visibility, []string{"public", "private"})))
	}
	if p.License != "" {
		if strings.TrimSpace(p.License) == "" || len(p.License) > 64 {
			problems = append(problems, fmt.Sprintf("product.license must be a non-empty, non-whitespace string of at most 64 characters (got %s)", describeValue(p.License)))
		}
	}
	return problems
}

// validateStackRecipe implements the shared validation rules for every stack
// hygiene recipe. The per-recipe runtime contract comes from
// stackRecipeRuntimes; everything else is identical across stacks: the module
// is an optional repository identity, surfaces and integrations are unused,
// and distribution supports only the github_actions toggle.
func (m Manifest) validateStackRecipe() []string {
	runtime := stackRecipeRuntimes[m.Recipe]
	var problems []string
	problems = append(problems, optionalModuleProblems(m.Product.Module)...)
	problems = append(problems, optionalIdentityProblems(m.Product)...)
	if !containsValue(runtime.Languages, m.Runtime.Language) {
		problems = append(problems, fmt.Sprintf("%s requires runtime.language to be one of %s (got %s)%s", m.Recipe, strings.Join(runtime.Languages, ", "), describeValue(m.Runtime.Language), suggestionSuffix(m.Runtime.Language, runtime.Languages)))
	}
	if !containsValue(runtime.Kinds, m.Runtime.Kind) {
		problems = append(problems, fmt.Sprintf("%s requires runtime.kind to be one of %s (got %s)%s", m.Recipe, strings.Join(runtime.Kinds, ", "), describeValue(m.Runtime.Kind), suggestionSuffix(m.Runtime.Kind, runtime.Kinds)))
	}
	if m.Surfaces != (Surfaces{}) {
		problems = append(problems, fmt.Sprintf("surfaces is not used by recipe %s", m.Recipe))
	}
	if m.Integrations != (Integrations{}) {
		problems = append(problems, fmt.Sprintf("integrations is not used by recipe %s", m.Recipe))
	}
	if m.Distribution.GoReleaser {
		problems = append(problems, fmt.Sprintf("distribution.goreleaser is not supported by recipe %s", m.Recipe))
	}
	if m.Distribution.Homebrew {
		problems = append(problems, fmt.Sprintf("distribution.homebrew is not supported by recipe %s", m.Recipe))
	}
	if m.Distribution.Docs != "" && m.Distribution.Docs != "none" {
		problems = append(problems, fmt.Sprintf("distribution.docs must be none for recipe %s (got %s)", m.Recipe, describeValue(m.Distribution.Docs)))
	}
	if len(m.Vars) > 0 {
		problems = append(problems, "vars is only supported by recipe files")
	}
	if len(m.Files) > 0 {
		problems = append(problems, "files is only supported by recipe files")
	}
	return problems
}

func containsValue(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// DefaultStack builds the default manifest for one stack hygiene recipe. The
// module is optional repository identity; kind selects one of the recipe's
// allowed runtime kinds and defaults to the first when empty.
func DefaultStack(recipeID, name, module, description, kind string) (Manifest, error) {
	runtime, ok := StackRecipeRuntime(recipeID)
	if !ok {
		return Manifest{}, fmt.Errorf("recipe %q is not a stack hygiene recipe", recipeID)
	}
	if kind == "" {
		kind = runtime.Kinds[0]
	}
	if !containsValue(runtime.Kinds, kind) {
		return Manifest{}, fmt.Errorf("recipe %s does not support runtime.kind %q (allowed: %s)", recipeID, kind, strings.Join(runtime.Kinds, ", "))
	}
	if description == "" {
		description = "A local-first, agent-ready repository."
	}
	return Manifest{
		SchemaVersion: SchemaVersion,
		Recipe:        recipeID,
		Product: Product{
			Name:        name,
			Module:      module,
			Description: description,
		},
		Runtime:      Runtime{Language: runtime.Languages[0], Kind: kind},
		Distribution: Distribution{GitHubActions: true},
	}, nil
}

// validateFilesRecipe implements every files-recipe validation rule. The
// files recipe declares an inline file tree; it does not use the
// go-agent-tool product, runtime, surfaces, integrations, or distribution
// fields, so those must stay zero-valued.
func (m Manifest) validateFilesRecipe() []string {
	var problems []string
	problems = append(problems, optionalModuleProblems(m.Product.Module)...)
	problems = append(problems, optionalIdentityProblems(m.Product)...)
	if m.Runtime != (Runtime{}) {
		problems = append(problems, "runtime is not used by recipe files")
	}
	if m.Surfaces != (Surfaces{}) {
		problems = append(problems, "surfaces is not used by recipe files")
	}
	if m.Integrations != (Integrations{}) {
		problems = append(problems, "integrations is not used by recipe files")
	}
	if m.Distribution != (Distribution{}) {
		problems = append(problems, "distribution is not used by recipe files")
	}

	varKeys := make([]string, 0, len(m.Vars))
	for key := range m.Vars {
		varKeys = append(varKeys, key)
	}
	sort.Strings(varKeys)
	for _, key := range varKeys {
		if !varKeyPattern.MatchString(key) {
			problems = append(problems, fmt.Sprintf("vars key %q must start with a lowercase letter and contain only lowercase letters, digits, and underscores", key))
		}
	}

	if len(m.Files) == 0 {
		problems = append(problems, "files must declare at least one file")
	}
	seenPaths := make(map[string]struct{}, len(m.Files))
	for i, decl := range m.Files {
		if strings.TrimSpace(decl.Path) == "" {
			problems = append(problems, fmt.Sprintf("files[%d].path must not be empty", i))
			continue
		}
		if _, err := ParseFileMode(decl.Mode); err != nil {
			problems = append(problems, fmt.Sprintf("files[%d] (%s): %s (got %s)", i, decl.Path, err, describeValue(decl.Mode)))
		}
		canonical := filepath.ToSlash(filepath.Clean(decl.Path))
		if _, exists := seenPaths[canonical]; exists {
			problems = append(problems, fmt.Sprintf("files declares duplicate path %q", canonical))
			continue
		}
		seenPaths[canonical] = struct{}{}
	}
	return problems
}

// ParseFileMode parses a files-recipe mode string. An empty string yields the
// default 0o644. A non-empty string must be a 3-4 digit octal permission
// string with no bits outside 0o777 (setuid, setgid, and sticky are
// rejected).
func ParseFileMode(mode string) (os.FileMode, error) {
	if mode == "" {
		return 0o644, nil
	}
	if !fileModePattern.MatchString(mode) {
		return 0, errors.New(`mode must be an octal permission string like "0644"`)
	}
	value, err := strconv.ParseUint(mode, 8, 32)
	if err != nil || value & ^uint64(0o777) != 0 {
		return 0, errors.New(`mode must be an octal permission string like "0644"`)
	}
	return os.FileMode(value), nil
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

// LoadWithSource loads and validates root/bob.yaml and returns the exact
// bounded bytes that were decoded. Mutation callers can retain the source
// bytes transiently and prove that the human-owned contract did not change
// between planning and publication.
func LoadWithSource(root string) (Manifest, []byte, error) {
	return LoadFileWithSource(filepath.Join(root, Filename))
}

func LoadFile(path string) (Manifest, error) {
	m, _, err := LoadFileWithSource(path)
	return m, err
}

// LoadFileWithSource is LoadFile plus the exact validated source snapshot.
// The returned byte slice is owned by the caller and is never persisted by
// this package.
func LoadFileWithSource(path string) (Manifest, []byte, error) {
	var m Manifest
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil, fmt.Errorf("no %s found in %s; run: bob init --module <module> --write to create one: %w", Filename, filepath.Dir(path), os.ErrNotExist)
		}
		return m, nil, fmt.Errorf("read manifest: %w", err)
	}
	if !info.Mode().IsRegular() {
		return m, nil, fmt.Errorf("read manifest: %s is not a regular file", path)
	}
	if info.Size() > maxBytes {
		return m, nil, fmt.Errorf("read manifest: file exceeds %d bytes", maxBytes)
	}
	f, err := os.Open(path)
	if err != nil {
		return m, nil, fmt.Errorf("read manifest: %w", err)
	}
	defer func() { _ = f.Close() }()
	openedInfo, err := f.Stat()
	if err != nil {
		return m, nil, fmt.Errorf("read manifest: %w", err)
	}
	if !openedInfo.Mode().IsRegular() || !os.SameFile(info, openedInfo) {
		return m, nil, errors.New("read manifest: file changed while it was being opened")
	}
	source, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return m, nil, fmt.Errorf("read manifest: %w", err)
	}
	if len(source) > maxBytes {
		return m, nil, fmt.Errorf("read manifest: file exceeds %d bytes", maxBytes)
	}
	dec := yaml.NewDecoder(bytes.NewReader(source))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return m, nil, fmt.Errorf("decode manifest: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			return m, nil, errors.New("decode manifest: multiple YAML documents are not supported")
		}
		return m, nil, fmt.Errorf("decode manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return m, nil, fmt.Errorf("validate manifest: %w", err)
	}
	return m, source, nil
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
