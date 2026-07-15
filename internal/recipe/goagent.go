package recipe

import (
	"bytes"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type goAgentTemplateData struct {
	Manifest      manifest.Manifest
	Product       manifest.Product
	RecipeVersion int
	Integrations  []goAgentIntegration
	DoctorChecks  []goAgentDoctorCheck
	GitHubOwner   string
	GitHubRepo    string
}

type goAgentIntegration struct {
	Name    string
	Binary  string
	Purpose string
}

type goAgentDoctorCheck struct {
	Name     string
	Binary   string
	Required bool
}

func renderGoAgentTool(m manifest.Manifest, version int) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("render go-agent-tool: %w", err)
	}
	if err := validateGoModulePath(m.Product.Module); err != nil {
		return nil, fmt.Errorf("render go-agent-tool: %w", err)
	}
	owner, repo, githubModule := githubModuleCoordinates(m.Product.Module)
	if m.Distribution.Homebrew && !githubModule {
		return nil, fmt.Errorf("render go-agent-tool: homebrew distribution requires a github.com/<owner>/<repo> module")
	}

	data := goAgentTemplateData{
		Manifest:      m,
		Product:       m.Product,
		RecipeVersion: version,
		Integrations:  selectedGoAgentIntegrations(m),
		GitHubOwner:   owner,
		GitHubRepo:    repo,
	}
	data.DoctorChecks = selectedGoAgentDoctorChecks(m, data.Integrations)

	var artifacts []Artifact
	add := func(path, source string) error {
		content, err := executeGoAgentTemplate(path, source, data)
		if err != nil {
			return err
		}
		artifacts = append(artifacts, Artifact{Path: path, Mode: 0o644, Content: content})
		return nil
	}

	type templateArtifact struct {
		path   string
		source string
	}
	base := []templateArtifact{
		{".gitignore", goAgentGitignoreTemplate},
		{".golangci.yml", goAgentGolangCITemplate},
		{"AGENTS.md", goAgentAgentsTemplate},
		{"CHANGELOG.md", goAgentChangelogTemplate},
		{"CLAUDE.md", goAgentClaudeTemplate},
		{"CODE_OF_CONDUCT.md", goAgentCodeOfConductTemplate},
		{"CONTRIBUTING.md", goAgentContributingTemplate},
		{"LICENSE", goAgentLicenseTemplate},
		{"README.md", goAgentReadmeTemplate},
		{"SECURITY.md", goAgentSecurityTemplate},
		{"Taskfile.yml", goAgentTaskfileTemplate},
		{"cmd/" + m.Product.Name + "/main.go", goAgentMainTemplate},
		{"go.mod", goAgentGoModTemplate},
		{"go.sum", goAgentGoSumTemplate},
		{"internal/cli/root.go", goAgentRootTemplate},
		{"internal/cli/root_test.go", goAgentRootTestTemplate},
		{"internal/version/version.go", goAgentVersionTemplate},
	}
	if version >= 4 {
		base = append(base,
			templateArtifact{"internal/cli/registry.go", goAgentRegistryTemplate},
			templateArtifact{"internal/cli/registry_test.go", goAgentRegistryTestTemplate},
		)
	}
	for _, item := range base {
		if err := add(item.path, item.source); err != nil {
			return nil, err
		}
	}

	if githubModule {
		for _, item := range []struct {
			path   string
			source string
		}{
			{".github/ISSUE_TEMPLATE/bug.yml", goAgentBugIssueTemplate},
			{".github/ISSUE_TEMPLATE/config.yml", goAgentIssueConfigTemplate},
			{".github/ISSUE_TEMPLATE/feature.yml", goAgentFeatureIssueTemplate},
			{".github/dependabot.yml", goAgentDependabotTemplate},
			{".github/pull_request_template.md", goAgentPullRequestTemplate},
		} {
			if err := add(item.path, item.source); err != nil {
				return nil, err
			}
		}
	}

	if m.Distribution.GitHubActions {
		if err := add(".github/workflows/ci.yml", goAgentCITemplate); err != nil {
			return nil, err
		}
		if m.Distribution.GoReleaser {
			if err := add(".github/workflows/release.yml", goAgentReleaseWorkflowTemplate); err != nil {
				return nil, err
			}
		}
	}
	if m.Distribution.GoReleaser {
		if err := add(".goreleaser.yaml", goAgentGoReleaserTemplate); err != nil {
			return nil, err
		}
	}

	switch m.Distribution.Docs {
	case "markdown":
		if err := add("docs/index.md", goAgentDocsIndexTemplate); err != nil {
			return nil, err
		}
	}

	if m.Integrations.TerminalVerification == "glyphrun" {
		if err := add("glyphrun.config.yml", goAgentGlyphrunConfigTemplate); err != nil {
			return nil, err
		}
		if err := add("specs/help.yml", goAgentGlyphrunHelpTemplate); err != nil {
			return nil, err
		}
	}

	// Render is the public sorting and path-safety boundary, but keeping the
	// producer ordered makes direct recipe tests and future callers deterministic.
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

func executeGoAgentTemplate(name, source string, data goAgentTemplateData) ([]byte, error) {
	tmpl, err := template.New(name).
		Delims("[[", "]]").
		Option("missingkey=error").
		Funcs(template.FuncMap{
			"quote": strconv.Quote,
		}).
		Parse(source)
	if err != nil {
		return nil, fmt.Errorf("render %s: parse template: %w", name, err)
	}
	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("render %s: %w", name, err)
	}
	content := bytes.TrimRight(out.Bytes(), "\n")
	content = append(content, '\n')
	return content, nil
}

func githubModuleCoordinates(modulePath string) (owner, repo string, ok bool) {
	parts := strings.Split(strings.TrimSpace(modulePath), "/")
	if len(parts) < 3 || parts[0] != "github.com" || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func validateGoModulePath(modulePath string) error {
	if strings.HasPrefix(modulePath, "/") || strings.HasSuffix(modulePath, "/") || strings.Contains(modulePath, "//") {
		return fmt.Errorf("product.module %q is not a valid Go module path", modulePath)
	}
	for _, segment := range strings.Split(modulePath, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("product.module %q is not a valid Go module path", modulePath)
		}
	}
	for _, r := range modulePath {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			continue
		}
		switch r {
		case '-', '.', '_', '~', '/':
			continue
		default:
			return fmt.Errorf("product.module %q contains unsupported character %q", modulePath, r)
		}
	}
	return nil
}

func selectedGoAgentIntegrations(m manifest.Manifest) []goAgentIntegration {
	var integrations []goAgentIntegration
	add := func(selected, want, name, binary, purpose string) {
		if selected == want {
			integrations = append(integrations, goAgentIntegration{Name: name, Binary: binary, Purpose: purpose})
		}
	}
	add(m.Integrations.CodeStructure, "codemap", "codemap", "codemap", "structural code context and impact analysis")
	add(m.Integrations.SemanticSearch, "vecgrep", "vecgrep", "vecgrep", "local semantic and hybrid code search")
	add(m.Integrations.TerminalVerification, "glyphrun", "glyphrun", "glyph", "terminal behavior verification")
	add(m.Integrations.BrowserVerification, "cairntrace", "cairntrace", "cairn", "browser behavior verification")
	add(m.Integrations.Secrets, "tinyvault", "tinyvault", "tvault", "value-safe local secret injection")
	add(m.Integrations.Artifacts, "fcheap", "file.cheap", "fcheap", "portable artifact stashing and restoration")
	sort.Slice(integrations, func(i, j int) bool { return integrations[i].Name < integrations[j].Name })
	return integrations
}

func selectedGoAgentDoctorChecks(m manifest.Manifest, integrations []goAgentIntegration) []goAgentDoctorCheck {
	checks := []goAgentDoctorCheck{
		{Name: "go", Binary: "go", Required: true},
		{Name: "golangci-lint", Binary: "golangci-lint"},
		{Name: "task", Binary: "task"},
	}
	for _, integration := range integrations {
		checks = append(checks, goAgentDoctorCheck{Name: integration.Name, Binary: integration.Binary})
	}
	if m.Distribution.GoReleaser {
		checks = append(checks, goAgentDoctorCheck{Name: "goreleaser", Binary: "goreleaser"})
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].Name < checks[j].Name })
	return checks
}

const goAgentGitignoreTemplate = `/bin/
/dist/
/coverage.out
/coverage.html
/.bob.apply.lock
*.test
.DS_Store
.env
.env.*
!.env.example
.glyphrun/runs/
.glyphrun/tmp/
node_modules/
docs/.vitepress/cache/
docs/.vitepress/dist/
`

const goAgentGoModTemplate = `module [[.Product.Module]]

go 1.26.5

require github.com/spf13/cobra v1.10.2

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)
`

const goAgentGoSumTemplate = `github.com/cpuguy83/go-md2man/v2 v2.0.6/go.mod h1:oOW0eioCTA6cOiMLiUPZOpcVxMig6NIQQ7OS05n1F4g=
github.com/inconshreveable/mousetrap v1.1.0 h1:wN+x4NVGpMsO7ErUn/mUI3vEoE6Jt13X2s0bqwp9tc8=
github.com/inconshreveable/mousetrap v1.1.0/go.mod h1:vpF70FUmC8bwa3OWnCshd2FqLfsEA9PFc4w1p2J65bw=
github.com/russross/blackfriday/v2 v2.1.0/go.mod h1:+Rmxgy9KzJVeS9/2gXHxylqXiyQDYRxCVz55jmeOWTM=
github.com/spf13/cobra v1.10.2 h1:DMTTonx5m65Ic0GOoRY2c16WCbHxOOw6xxezuLaBpcU=
github.com/spf13/cobra v1.10.2/go.mod h1:7C1pvHqHw5A4vrJfjNwvOdzYu0Gml16OCs2GRiTUUS4=
github.com/spf13/pflag v1.0.9 h1:9exaQaMOCwffKiiiYk6/BndUBv+iRViNW+4lEMi0PvY=
github.com/spf13/pflag v1.0.9/go.mod h1:McXfInJRrz4CZXVZOBLb0bTZqETkiAhM9Iw0y3An2Bg=
go.yaml.in/yaml/v3 v3.0.4/go.mod h1:DhzuOOF2ATzADvBadXxruRBLzYTpT36CKvDb3+aBEFg=
gopkg.in/check.v1 v0.0.0-20161208181325-20d25e280405/go.mod h1:Co6ibVJAznAaIkqp8huTwlJQCZ016jof/cbN4VW5Yz0=
`

const goAgentMainTemplate = `package main

import (
	"fmt"
	"os"
	"os/exec"

	[[quote (printf "%s/internal/cli" .Product.Module)]]
	[[quote (printf "%s/internal/version" .Product.Module)]]
)

func main() {
	[[if ge .RecipeVersion 4]]cmd, err := cli.New(version.Current(), cli.Dependencies{
		Out:      os.Stdout,
		ErrOut:   os.Stderr,
		LookPath: exec.LookPath,
	})
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	[[else]]cmd := cli.New(version.Current(), cli.Dependencies{
		Out:      os.Stdout,
		ErrOut:   os.Stderr,
		LookPath: exec.LookPath,
	})
	[[end]]if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`

const goAgentRootTemplate = `// Package cli owns the command-line projection. Domain behavior should live in
// separate packages and be injected here so commands remain small and testable.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	[[quote (printf "%s/internal/version" .Product.Module)]]
	"github.com/spf13/cobra"
)

// Dependencies contains process-bound capabilities. Tests can replace every
// field without changing global state or invoking real binaries.
type Dependencies struct {
	Out      io.Writer
	ErrOut   io.Writer
	LookPath func(string) (string, error)
}

type options struct {
	json bool
}

type doctorDefinition struct {
	Name     string
	Binary   string
	Required bool
}

type DoctorCheck struct {
	Name      string ` + "`json:\"name\"`" + `
	Binary    string ` + "`json:\"binary\"`" + `
	Required  bool   ` + "`json:\"required\"`" + `
	Available bool   ` + "`json:\"available\"`" + `
	Path      string ` + "`json:\"path,omitempty\"`" + `
}

type DoctorReport struct {
	SchemaVersion int           ` + "`json:\"schema_version\"`" + `
	OK            bool          ` + "`json:\"ok\"`" + `
	Checks        []DoctorCheck ` + "`json:\"checks\"`" + `
}

var doctorDefinitions = []doctorDefinition{
[[range .DoctorChecks]]	{Name: [[quote .Name]], Binary: [[quote .Binary]], Required: [[.Required]]},
[[end]]}

// New builds a command tree over explicit dependencies. It performs no work
// until Execute is called.
[[if ge .RecipeVersion 4]]func New(info version.Info, deps Dependencies) (*cobra.Command, error) {
[[else]]func New(info version.Info, deps Dependencies) *cobra.Command {
[[end]]	if deps.Out == nil {
		deps.Out = io.Discard
	}
	if deps.ErrOut == nil {
		deps.ErrOut = io.Discard
	}
	if deps.LookPath == nil {
		deps.LookPath = func(string) (string, error) { return "", fmt.Errorf("path lookup unavailable") }
	}

	opts := &options{}
	root := &cobra.Command{
		Use:           [[quote .Product.Name]],
		Short:         [[quote .Product.Description]],
		Version:       info.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetOut(deps.Out)
	root.SetErr(deps.ErrOut)
	root.SetVersionTemplate([[quote .Product.Name]] + " version {{.Version}}\n")
	root.PersistentFlags().BoolVar(&opts.json, "json", false, "write machine-readable JSON to stdout")
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newDoctorCommand(opts, deps), newVersionCommand(opts, info))
[[if ge .RecipeVersion 4]]	if err := addExtensionCommands(root, opts, deps, extensionFactories); err != nil {
		return nil, err
	}
	return root, nil
[[else]]	return root
[[end]]}

func newVersionCommand(opts *options, info version.Info) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build metadata",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.json {
				return writeJSON(cmd.OutOrStdout(), struct {
					SchemaVersion int    ` + "`json:\"schema_version\"`" + `
					Name          string ` + "`json:\"name\"`" + `
					Version       string ` + "`json:\"version\"`" + `
					Commit        string ` + "`json:\"commit\"`" + `
					Date          string ` + "`json:\"date\"`" + `
				}{1, [[quote .Product.Name]], info.Version, info.Commit, info.Date})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "%s version %s (%s) %s\n", [[quote .Product.Name]], info.Version, info.Commit, info.Date)
			return err
		},
	}
}

func newDoctorCommand(opts *options, deps Dependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check required development tools",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			report := inspectDoctor(doctorDefinitions, deps.LookPath)
			if opts.json {
				if err := writeJSON(cmd.OutOrStdout(), report); err != nil {
					return err
				}
			} else {
				for _, check := range report.Checks {
					status := "optional"
					if check.Required {
						status = "missing"
					}
					if check.Available {
						status = "ok"
					}
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-14s %s\n", status, check.Name, check.Path); err != nil {
						return err
					}
				}
			}
			if !report.OK {
				var missing []string
				for _, check := range report.Checks {
					if check.Required && !check.Available {
						missing = append(missing, check.Binary)
					}
				}
				return fmt.Errorf("doctor: missing required tools: %s", strings.Join(missing, ", "))
			}
			return nil
		},
	}
}

func inspectDoctor(definitions []doctorDefinition, lookPath func(string) (string, error)) DoctorReport {
	report := DoctorReport{SchemaVersion: 1, OK: true, Checks: make([]DoctorCheck, 0, len(definitions))}
	for _, definition := range definitions {
		path, err := lookPath(definition.Binary)
		check := DoctorCheck{Name: definition.Name, Binary: definition.Binary, Required: definition.Required, Available: err == nil, Path: path}
		if err != nil && definition.Required {
			report.OK = false
			check.Path = ""
		}
		report.Checks = append(report.Checks, check)
	}
	sort.Slice(report.Checks, func(i, j int) bool { return report.Checks[i].Name < report.Checks[j].Name })
	return report
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
`

const goAgentVersionTemplate = `// Package version contains release metadata injected by GoReleaser or go build.
package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

type Info struct {
	Version string
	Commit  string
	Date    string
}

func Current() Info {
	return Info{Version: Version, Commit: Commit, Date: Date}
}
`

const goAgentRootTestTemplate = `package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	[[quote (printf "%s/internal/version" .Product.Module)]]
)

func TestVersionJSON(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
[[if ge .RecipeVersion 4]]	cmd, err := New(version.Info{Version: "v0.1.0", Commit: "abc123", Date: "today"}, Dependencies{
		Out:      &output,
		LookPath: func(string) (string, error) { return "/bin/tool", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
[[else]]	cmd := New(version.Info{Version: "v0.1.0", Commit: "abc123", Date: "today"}, Dependencies{
		Out:      &output,
		LookPath: func(string) (string, error) { return "/bin/tool", nil },
	})
[[end]]	cmd.SetArgs([]string{"--json", "version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		SchemaVersion int    ` + "`json:\"schema_version\"`" + `
		Version       string ` + "`json:\"version\"`" + `
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 1 || result.Version != "v0.1.0" {
		t.Fatalf("unexpected version output: %#v", result)
	}
}

func TestDoctorReportsMissingDependency(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
[[if ge .RecipeVersion 4]]	cmd, err := New(version.Info{}, Dependencies{
		Out: &output,
		LookPath: func(name string) (string, error) {
			if name == "go" {
				return "", errors.New("missing")
			}
			return "/bin/" + name, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
[[else]]	cmd := New(version.Info{}, Dependencies{
		Out: &output,
		LookPath: func(name string) (string, error) {
			if name == "go" {
				return "", errors.New("missing")
			}
			return "/bin/" + name, nil
		},
	})
[[end]]	cmd.SetArgs([]string{"doctor"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected missing dependency error")
	}
}

func TestDoctorAllowsMissingOptionalTool(t *testing.T) {
	t.Parallel()
	var output bytes.Buffer
[[if ge .RecipeVersion 4]]	cmd, err := New(version.Info{}, Dependencies{
		Out: &output,
		LookPath: func(name string) (string, error) {
			if name != "go" {
				return "", errors.New("missing")
			}
			return "/bin/go", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
[[else]]	cmd := New(version.Info{}, Dependencies{
		Out: &output,
		LookPath: func(name string) (string, error) {
			if name != "go" {
				return "", errors.New("missing")
			}
			return "/bin/go", nil
		},
	})
[[end]]	cmd.SetArgs([]string{"doctor"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("optional dependency made doctor fail: %v", err)
	}
}
`

const goAgentRegistryTemplate = `package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type commandBuilder func(*options, Dependencies) *cobra.Command

type commandFactory struct {
	id    string
	build commandBuilder
}

var extensionFactories []commandFactory

// registerCommand declares one human-owned command factory. Registration is
// collected during package initialization and validated when New constructs a
// command tree. Bob-owned composition files must not be edited to add commands.
func registerCommand(id string, build commandBuilder) {
	extensionFactories = append(extensionFactories, commandFactory{id: id, build: build})
}

func addExtensionCommands(root *cobra.Command, opts *options, deps Dependencies, registrations []commandFactory) error {
	factories := append([]commandFactory(nil), registrations...)
	sort.SliceStable(factories, func(i, j int) bool { return factories[i].id < factories[j].id })

	seenIDs := make(map[string]struct{}, len(factories))
	for _, factory := range factories {
		if !validCommandID(factory.id) {
			return fmt.Errorf("invalid extension command id %q", factory.id)
		}
		if _, exists := seenIDs[factory.id]; exists {
			return fmt.Errorf("duplicate extension command id %q", factory.id)
		}
		seenIDs[factory.id] = struct{}{}
		if factory.build == nil {
			return fmt.Errorf("extension command %q has a nil builder", factory.id)
		}
	}

	// Cobra adds its help command lazily during execution, so it is not present
	// in root.Commands while this registry is validated. Reserve the name here
	// to make the extension contract independent of Cobra's initialization
	// timing.
	commandNames := map[string]string{"help": "Cobra help command"}
	for _, command := range root.Commands() {
		commandNames[command.Name()] = "built-in command"
	}
	for _, factory := range factories {
		command := factory.build(opts, deps)
		if command == nil {
			return fmt.Errorf("extension command %q returned a nil command", factory.id)
		}
		name := command.Name()
		if name == "" {
			return fmt.Errorf("extension command %q returned a command without a name", factory.id)
		}
		if owner, exists := commandNames[name]; exists {
			return fmt.Errorf("extension command %q duplicates command name %q owned by %s", factory.id, name, owner)
		}
		commandNames[name] = fmt.Sprintf("extension %q", factory.id)
		root.AddCommand(command)
	}
	return nil
}

func validCommandID(id string) bool {
	if id == "" || id[0] < 'a' || id[0] > 'z' || id[len(id)-1] == '-' {
		return false
	}
	previousHyphen := false
	for _, char := range id {
		if char == '-' {
			if previousHyphen {
				return false
			}
			previousHyphen = true
			continue
		}
		if char < 'a' || char > 'z' {
			if char < '0' || char > '9' {
				return false
			}
		}
		previousHyphen = false
	}
	return true
}
`

const goAgentRegistryTestTemplate = `package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestExtensionFactoriesBuildInStableIDOrder(t *testing.T) {
	var order []string
	factory := func(id, name string) commandFactory {
		return commandFactory{id: id, build: func(*options, Dependencies) *cobra.Command {
			order = append(order, id)
			return &cobra.Command{Use: name}
		}}
	}
	root := &cobra.Command{Use: "test"}
	if err := addExtensionCommands(root, &options{}, Dependencies{}, []commandFactory{
		factory("zeta", "last"),
		factory("alpha", "first"),
	}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"alpha", "zeta"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("build order = %v, want %v", order, want)
	}
}

func TestExtensionFactoriesRejectInvalidRegistrations(t *testing.T) {
	valid := func(name string) commandBuilder {
		return func(*options, Dependencies) *cobra.Command { return &cobra.Command{Use: name} }
	}
	tests := []struct {
		name          string
		registrations []commandFactory
		want          string
	}{
		{name: "invalid id", registrations: []commandFactory{{id: "Bad", build: valid("bad")}}, want: "invalid extension command id"},
		{name: "duplicate id", registrations: []commandFactory{{id: "same", build: valid("one")}, {id: "same", build: valid("two")}}, want: "duplicate extension command id"},
		{name: "nil builder", registrations: []commandFactory{{id: "nil-builder"}}, want: "nil builder"},
		{name: "nil command", registrations: []commandFactory{{id: "nil-command", build: func(*options, Dependencies) *cobra.Command { return nil }}}, want: "nil command"},
		{name: "reserved Cobra help name", registrations: []commandFactory{{id: "custom-help", build: valid("help")}}, want: "Cobra help command"},
		{name: "duplicate command name", registrations: []commandFactory{{id: "one", build: valid("same")}, {id: "two", build: valid("same")}}, want: "duplicates command name"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := &cobra.Command{Use: "test"}
			err := addExtensionCommands(root, &options{}, Dependencies{}, test.registrations)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestExtensionFactoryCannotShadowBuiltInCommand(t *testing.T) {
	root := &cobra.Command{Use: "test"}
	root.AddCommand(&cobra.Command{Use: "doctor"})
	err := addExtensionCommands(root, &options{}, Dependencies{}, []commandFactory{{
		id: "custom-doctor",
		build: func(*options, Dependencies) *cobra.Command {
			return &cobra.Command{Use: "doctor"}
		},
	}})
	if err == nil || !strings.Contains(err.Error(), "built-in command") {
		t.Fatalf("error = %v, want built-in command collision", err)
	}
}
`

const goAgentReadmeTemplate = `# [[.Product.Name]]

[[.Product.Description]]

[[if and .Manifest.Distribution.GitHubActions .GitHubOwner]][![CI](https://github.com/[[.GitHubOwner]]/[[.GitHubRepo]]/actions/workflows/ci.yml/badge.svg)](https://github.com/[[.GitHubOwner]]/[[.GitHubRepo]]/actions/workflows/ci.yml)

[[end]]## Features

- A focused Cobra CLI with human-readable output and a global ` + "`--json`" + ` mode.
- Stable ` + "`doctor`" + ` and ` + "`version`" + ` commands for people, scripts, and agents.
- Explicit dependency injection at the command boundary for fast, isolated tests.
[[if .Integrations]]- Optional local-first integrations:
[[range .Integrations]]  - **[[.Name]]** — [[.Purpose]].
[[end]][[else]]- No optional ecosystem tools are required.
[[end]]
## Install

~~~bash
go install [[.Product.Module]]/cmd/[[.Product.Name]]@latest
~~~

Or build from a checkout:

~~~bash
task build
./bin/[[.Product.Name]] --help
~~~

## Quick start

~~~bash
[[.Product.Name]] doctor
[[.Product.Name]] version --json
~~~

The JSON surface writes structured data to stdout. Diagnostics and failures go
to stderr so shell pipelines can parse stdout safely.
[[if ge .RecipeVersion 4]]
## Adding commands

Create an internal/cli/<command>.go file and its test beside it. Keep the file
in package cli and register one deterministic factory from init:

~~~go
func init() {
	registerCommand("hello", newHelloCommand)
}
~~~

Do not edit Bob-owned internal/cli/root.go or internal/cli/registry.go. The
registry validates stable IDs, builders, returned commands, and command-name
collisions whenever the command tree is constructed.
[[end]]
## Development

~~~bash
task check
task build
[[if eq .Manifest.Integrations.TerminalVerification "glyphrun"]]task e2e
[[end]]~~~

[[if .Manifest.Distribution.GoReleaser]]The release check inside ` + "`task check`" + ` expects this checkout to be an
initialized Git repository with its intended remote configured.

[[end]]
See [CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow and
[SECURITY.md](SECURITY.md) for security reporting instructions.

## License

MIT — see [LICENSE](LICENSE).
`

const goAgentAgentsTemplate = `# AGENTS.md

This file is the source of truth for agents and contributors working on [[.Product.Name]].

## Product

[[.Product.Description]]

## Architecture

- ` + "`cmd/[[.Product.Name]]`" + ` is the process entry point and contains no business logic.
- ` + "`internal/cli`" + ` owns Cobra commands and depends on explicit process capabilities.
- ` + "`internal/version`" + ` owns build metadata.
- Add domain behavior in a focused package and inject it into the CLI layer.
[[if ge .RecipeVersion 4]]
## Command extensions

- Add a command by creating internal/cli/<command>.go and internal/cli/<command>_test.go.
- Keep extension files in package cli and call registerCommand("stable-id", builder) from init.
- Do not edit Bob-owned internal/cli/root.go or internal/cli/registry.go.
- Registration IDs use lowercase kebab form. Duplicate IDs, nil builders, nil commands, and duplicate command names fail command-tree construction.
[[end]]
## Commands

~~~bash
task fmt-check
task lint
task test
task race
task vuln
task build
task check
[[if eq .Manifest.Integrations.TerminalVerification "glyphrun"]]task e2e
[[end]]~~~

## Invariants

1. Keep machine-readable stdout valid and send progress or diagnostics to stderr.
2. Keep command handlers thin; test domain behavior without executing the process.
3. Do not read secret values into logs, errors, fixtures, or generated artifacts.
4. Preserve deterministic output and stable JSON field meanings.
5. Add tests for behavior changes and update terminal specs when user-visible CLI behavior changes.

## Optional tools

[[if .Integrations]][[range .Integrations]]- ` + "`[[.Binary]]`" + `: [[.Purpose]].
[[end]][[else]]None. The core development workflow requires only Go.
[[end]]`

const goAgentClaudeTemplate = `# CLAUDE.md

Read [AGENTS.md](AGENTS.md) first. It is the source of truth for architecture,
commands, safety invariants, and optional-tool boundaries in this repository.

Keep repository work deterministic, keep structured stdout clean, and verify
changes with ` + "`task check`" + ` before handing them off.
`

const goAgentCodeOfConductTemplate = `# Code of Conduct

This community policy is based on the
[Contributor Covenant 2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).

Be respectful, inclusive, and constructive. Harassment, threats,
discrimination, sexualized conduct, personal attacks, trolling, and publishing
another person's private information are not accepted.

This standard applies in repository issues, pull requests, reviews, and other
spaces where someone represents the project. Maintainers may edit or remove
contributions, limit participation, or ban a participant when needed to protect
the community.

[[if .GitHubOwner]]Report conduct concerns privately using a monitored contact method on the
[repository owner's profile](https://github.com/[[.GitHubOwner]]). Before
publishing, the owner must ensure that profile names an appropriate private
channel. [[else]]Before publishing, replace this sentence with a monitored private conduct
contact. [[end]]Do not post sensitive report details in a public issue. Reports
should be handled promptly, fairly, and as confidentially as practical.

This policy is adapted from the Contributor Covenant, version 2.1, available at
https://www.contributor-covenant.org/version/2/1/code_of_conduct/ and licensed
under Creative Commons Attribution 4.0.
`

const goAgentChangelogTemplate = `# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial public repository foundation.
`

const goAgentContributingTemplate = `# Contributing

Thank you for improving [[.Product.Name]].

Read [AGENTS.md](AGENTS.md) before changing package boundaries, security
behavior, or public contracts.

1. Open an issue for substantial behavior or contract changes.
2. Create a focused branch and keep unrelated changes out of the diff.
3. Add or update tests for every behavior change.
4. Run ` + "`task check`" + ` before opening a pull request.
[[if eq .Manifest.Integrations.TerminalVerification "glyphrun"]]5. Run ` + "`task e2e`" + ` when CLI output or interaction changes.
[[end]]
Pull requests should explain the user impact, compatibility implications, and
verification performed. Never include credentials, private data, generated
artifact packs, or local environment files.

[[if and .GitHubOwner (eq .Product.Visibility "public")]]Security-sensitive reports belong in the repository's GitHub private
vulnerability-reporting channel after the maintainer enables it. [[else]]Security-sensitive reports must follow the monitored private channel configured
in SECURITY.md before publication. [[end]]Community participation follows
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Contributions are licensed under the
repository's MIT License.
`

const goAgentSecurityTemplate = `# Security Policy

## Supported versions

Before the first tagged release, security fixes are made on ` + "`main`" + `. After
the first release, the latest release and ` + "`main`" + ` are supported.

## Reporting a vulnerability

[[if and .GitHubOwner (eq .Product.Visibility "public")]]Use [GitHub private vulnerability reporting](https://github.com/[[.GitHubOwner]]/[[.GitHubRepo]]/security/advisories/new).
Repository maintainers must enable that feature before publishing this project.
[[else]]This scaffold cannot name a configured private reporting channel. Before
publishing, replace this paragraph with an actually monitored private contact.
[[end]]
Do not open a public issue for an unpatched vulnerability. Include the affected
version, impact, reproduction steps, and any suggested mitigation. Do not
include real credentials or unrelated personal data.
`

const goAgentBugIssueTemplate = `name: Bug report
description: Report incorrect or unsafe [[.Product.Name]] behavior
title: "bug: "
labels: [bug]
body:
  - type: textarea
    id: behavior
    attributes:
      label: What happened?
      description: Include the exact command, expected result, and actual result.
    validations:
      required: true
  - type: dropdown
    id: surface
    attributes:
      label: Surface
      options:
        - CLI (human output)
        - CLI (machine-readable output)
        - Generated repository
        - CI or release
    validations:
      required: true
  - type: textarea
    id: reproduction
    attributes:
      label: Minimal reproduction
      description: Remove secrets and private paths before posting.
    validations:
      required: true
  - type: input
    id: version
    attributes:
      label: Version
      placeholder: [[.Product.Name]] version
    validations:
      required: true
  - type: input
    id: environment
    attributes:
      label: Operating system and architecture
      placeholder: macOS arm64 or Ubuntu amd64
    validations:
      required: true
`

const goAgentFeatureIssueTemplate = `name: Feature request
description: Propose a focused product or workflow improvement
title: "feat: "
labels: [enhancement]
body:
  - type: textarea
    id: problem
    attributes:
      label: Problem
      description: What user problem should this change solve?
    validations:
      required: true
  - type: textarea
    id: contract
    attributes:
      label: Smallest useful contract
      description: Show the smallest command, JSON, or file shape that solves the problem.
    validations:
      required: true
  - type: textarea
    id: safety
    attributes:
      label: Ownership and safety
      description: Which files, processes, secrets, or remote systems would this observe or change?
    validations:
      required: true
  - type: textarea
    id: alternatives
    attributes:
      label: Alternatives considered
      description: What can users do today, and why is it insufficient?
`

const goAgentIssueConfigTemplate = `blank_issues_enabled: false
contact_links:
  - name: Report a security vulnerability
[[if eq .Product.Visibility "public"]]
    url: https://github.com/[[.GitHubOwner]]/[[.GitHubRepo]]/security/advisories/new
    about: Share vulnerabilities privately with the maintainers.
[[else]]
    url: https://github.com/[[.GitHubOwner]]/[[.GitHubRepo]]/blob/main/SECURITY.md
    about: Review the repository's private security reporting instructions.
[[end]]
`

const goAgentPullRequestTemplate = `## Outcome

Describe the user-visible result.

## Verification

- [ ] ` + "`task check`" + `
- [ ] Terminal behavior specs when CLI behavior changed
- [ ] ` + "`goreleaser check`" + ` when packaging changed
- [ ] ` + "`git diff --check`" + `

## Safety

- [ ] Compatibility and wire-format impact are described.
- [ ] Filesystem, subprocess, secret, and remote effects are described.
- [ ] Tests use temporary state and do not touch real user configuration.

## Verification evidence

List the exact commands and user-visible behavior you verified.
`

const goAgentDependabotTemplate = `version: 2
updates:
  - package-ecosystem: gomod
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    groups:
      go-minor-and-patch:
        update-types: [minor, patch]

  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
    open-pull-requests-limit: 5
    groups:
      actions:
        patterns: ["*"]
`

const goAgentLicenseTemplate = `MIT License

Copyright (c) 2026 [[.Product.Name]] contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`

const goAgentTaskfileTemplate = `version: "3"

vars:
  BINARY: ./bin/[[.Product.Name]]

tasks:
  default:
    cmds:
      - task --list

  fmt:
    desc: Format Go source.
    cmds:
      - gofmt -w ./cmd ./internal

  fmt-check:
    desc: Check Go formatting without changing files.
    cmds:
      - test -z "$(gofmt -l ./cmd ./internal)"

  tidy-check:
    desc: Check module files without changing them.
    cmds:
      - go mod tidy -diff

  lint:
    desc: Run static analysis.
    cmds:
      - golangci-lint run ./...

  test:
    desc: Run unit tests.
    cmds:
      - go test ./...

  race:
    desc: Run tests with the race detector.
    env:
      CGO_ENABLED: "1"
    cmds:
      - go test -race ./...

  vet:
    desc: Run Go static analysis.
    cmds:
      - go vet ./...

  vuln:
    desc: Check dependencies for known vulnerabilities.
    cmds:
      - go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...

  cover:
    desc: Generate a local HTML coverage report.
    cmds:
      - go test -coverprofile=coverage.out ./...
      - go tool cover -html=coverage.out -o coverage.html

  build:
    desc: Build the CLI.
    cmds:
      - mkdir -p ./bin
      - go build -o {{.BINARY}} ./cmd/[[.Product.Name]]

  build-check:
    desc: Check that the CLI builds without writing into the workspace.
    cmds:
      - |
        output="$(mktemp)"
        trap 'rm -f "$output"' EXIT
        go build -o "$output" ./cmd/[[.Product.Name]]

  check:
    desc: Run the canonical non-mutating verification gate.
    aliases: [verify]
    cmds:
      - task: fmt-check
      - task: tidy-check
      - task: lint
      - task: vet
      - task: test
      - task: race
      - task: build-check
      - task: vuln
[[if .Manifest.Distribution.GoReleaser]]
  release-check:
    desc: Validate the GoReleaser configuration.
    cmds:
      - goreleaser check

  snapshot:
    desc: Build the complete release package without publishing.
    cmds:
      - goreleaser release --snapshot --clean
[[end]]
[[if eq .Manifest.Integrations.TerminalVerification "glyphrun"]]
  e2e:
    desc: Run Glyphrun terminal behavior specs.
    deps: [build]
    cmds:
      - glyph spec verify specs/help.yml --format md
      - glyph run specs/help.yml --format md
[[end]][[if eq .Manifest.Distribution.Docs "vitepress"]]
  docs:
    desc: Build the VitePress documentation site.
    cmds:
      - npm install
      - npm run docs:build

  docs-dev:
    desc: Start the VitePress development server.
    cmds:
      - npm install
      - npm run docs:dev
[[end]]`

const goAgentGolangCITemplate = `version: "2"

run:
  timeout: 5m

linters:
  default: standard

formatters:
  enable:
    - gofmt
`

const goAgentCITemplate = `name: CI

on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

concurrency:
  group: ci-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

jobs:
  go:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0
        with:
          go-version-file: go.mod
          cache: true
      - run: go mod tidy -diff
      - run: test -z "$(gofmt -l ./cmd ./internal)" || (gofmt -l ./cmd ./internal; exit 1)
      - run: go test -count=1 ./...
      - run: CGO_ENABLED=1 go test -race -count=1 ./...
      - run: go vet ./...
      - run: go build ./cmd/[[.Product.Name]]
      - run: go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
      - uses: golangci/golangci-lint-action@ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a # v9.3.0
        with:
          version: v2.12.2
[[if .Manifest.Distribution.GoReleaser]]      - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        with:
          distribution: goreleaser
          version: v2.17.0
          args: check
[[end]]
[[if eq .Manifest.Integrations.TerminalVerification "glyphrun"]]
  terminal-contract:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0
        with:
          go-version-file: go.mod
          cache: true
      - run: go install github.com/abdul-hamid-achik/glyphrun/cmd/glyph@v0.14.0
      - run: go build -o ./bin/[[.Product.Name]] ./cmd/[[.Product.Name]]
      - run: glyph spec verify specs/help.yml --format md
      - run: glyph run specs/help.yml --format md
[[end]][[if eq .Manifest.Distribution.Docs "vitepress"]]
  docs:
    runs-on: ubuntu-latest
    timeout-minutes: 10
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      - uses: actions/setup-node@48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e # v6
        with:
          node-version: 24
      - run: npm install
      - run: npm run docs:build
[[end]]`

const goAgentReleaseWorkflowTemplate = `name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: read

jobs:
  verify:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0
        with:
          go-version-file: go.mod
          cache: true
      - run: go mod tidy -diff
      - run: test -z "$(gofmt -l ./cmd ./internal)" || (gofmt -l ./cmd ./internal; exit 1)
      - run: go test -race -count=1 ./...
      - run: go vet ./...
      - run: go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
      - uses: golangci/golangci-lint-action@ba0d7d2ec06a0ea1cb5fa41b2e4a3ab91d21278a # v9.3.0
        with:
          version: v2.12.2
      - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        with:
          distribution: goreleaser
          version: v2.17.0
          args: check

  release:
    needs: verify
    runs-on: ubuntu-latest
    timeout-minutes: 20
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
        with:
          fetch-depth: 0
      - uses: actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16 # v6.5.0
        with:
          go-version-file: go.mod
          cache: true
      - uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7.2.3
        with:
          distribution: goreleaser
          version: v2.17.0
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
[[if .Manifest.Distribution.Homebrew]]          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}
[[end]]`

const goAgentGoReleaserTemplate = `version: 2

project_name: [[.Product.Name]]

builds:
  - id: [[.Product.Name]]
    main: ./cmd/[[.Product.Name]]
    binary: [[.Product.Name]]
    env:
      - CGO_ENABLED=0
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w
      - -X [[.Product.Module]]/internal/version.Version={{.Version}}
      - -X [[.Product.Module]]/internal/version.Commit={{.ShortCommit}}
      - -X [[.Product.Module]]/internal/version.Date={{.Date}}

archives:
  - formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - README.md
      - LICENSE

checksum:
  name_template: checksums.txt

snapshot:
  version_template: "{{ incpatch .Version }}-next"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
[[if .Manifest.Distribution.Homebrew]]
homebrew_casks:
  - name: [[.Product.Name]]
    binaries:
      - [[.Product.Name]]
    repository:
      owner: [[.GitHubOwner]]
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    directory: Casks
    homepage: https://github.com/[[.GitHubOwner]]/[[.GitHubRepo]]
    description: [[quote .Product.Description]]
    license: MIT
    caveats: "Run ` + "`[[.Product.Name]] doctor`" + ` to inspect required and optional tools; pass ` + "`--json`" + ` for machine-readable output. macOS may require explicit approval for an unsigned build."
    skip_upload: "{{ if .Env.HOMEBREW_TAP_TOKEN }}false{{ else }}true{{ end }}"
[[end]]`

const goAgentDocsIndexTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Start here

~~~bash
go install [[.Product.Module]]/cmd/[[.Product.Name]]@latest
[[.Product.Name]] doctor
[[.Product.Name]] --help
~~~

## Agent contract

- Pass ` + "`--json`" + ` for machine-readable command output.
- Treat stdout as the data channel and stderr as the diagnostics channel.
- Run ` + "`[[.Product.Name]] doctor --json`" + ` before relying on optional tools.
- Inspect ` + "`AGENTS.md`" + ` before changing the repository.
[[if ge .RecipeVersion 4]]- Add commands in human-owned internal/cli/<command>.go files through registerCommand; do not edit the generated root or registry.
[[end]]
## Development

Run ` + "`task check`" + ` before opening a pull request.
`

const goAgentGlyphrunConfigTemplate = `version: 1

artifactRoot: .glyphrun/runs
snapshotRoot: .glyphrun/snapshots

terminal:
  profile: xterm-256color
  cols: 100
  rows: 30
  normalize:
    trimRight: true
    normalizeLineEndings: true

artifacts:
  frames: true
  finalScreen: true
  snapshots: true
  agentContext: true
`

const goAgentGlyphrunHelpTemplate = `version: 1
name: [[.Product.Name]]_help

intent: |
  a user can discover the [[.Product.Name]] command surface and exit cleanly.

target:
  cmd: ["./bin/[[.Product.Name]]", "--help"]
  cwd: "."

terminal:
  cols: 100
  rows: 30
  profile: xterm-256color

preconditions:
  commands:
    - run: "go build -o ./bin/[[.Product.Name]] ./cmd/[[.Product.Name]]"
      timeoutMs: 30000

steps:
  - wait:
      process:
        exitCode: 0
      timeoutMs: 5000
  - snapshot: help

outcomes:
  - id: doctor_visible
    description: help lists the doctor command
    verify:
      screen:
        contains: "doctor"
  - id: version_visible
    description: help lists the version command
    verify:
      screen:
        contains: "version"
  - id: clean_exit
    description: help exits successfully
    verify:
      process:
        exitCode: 0
`
