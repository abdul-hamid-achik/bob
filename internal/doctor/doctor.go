package doctor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

const maxProbeOutput = 4096

type Check struct {
	Name     string `json:"name"`
	Command  string `json:"command"`
	Required bool   `json:"required"`
	Found    bool   `json:"found"`
	Usable   bool   `json:"usable"`
	Path     string `json:"path,omitempty"`
	Version  string `json:"version,omitempty"`
	Note     string `json:"note,omitempty"`
}

type Result struct {
	Ready    bool    `json:"ready"`
	Degraded bool    `json:"degraded"`
	Checks   []Check `json:"checks"`
}

type Prober interface {
	LookPath(string) (string, error)
	Version(context.Context, string, ...string) (string, error)
}

type ExecProber struct{}

func (ExecProber) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (ExecProber) Version(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var output cappedBuffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	err := cmd.Run()
	value := strings.TrimSpace(output.String())
	if err != nil {
		if value == "" {
			return "", err
		}
		return value, fmt.Errorf("%w: %s", err, value)
	}
	return firstLine(value), nil
}

type tool struct {
	name     string
	command  string
	args     []string
	required bool
	note     string
	minMajor int
	minMinor int
	minPatch int
}

func Run(ctx context.Context, m manifest.Manifest, prober Prober) Result {
	tools := selectedTools(m)
	result := Result{Ready: true, Checks: make([]Check, 0, len(tools))}
	for _, candidate := range tools {
		check := Check{
			Name:     candidate.name,
			Command:  candidate.command,
			Required: candidate.required,
			Note:     candidate.note,
		}
		path, err := prober.LookPath(candidate.command)
		if err != nil {
			if candidate.required {
				result.Ready = false
			} else {
				result.Degraded = true
			}
			result.Checks = append(result.Checks, check)
			continue
		}
		check.Found = true
		check.Path = path
		probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		version, probeErr := prober.Version(probeCtx, path, candidate.args...)
		cancel()
		if probeErr == nil {
			check.Version = version
			check.Usable = true
			if candidate.minMajor > 0 && !versionAtLeast(version, candidate.minMajor, candidate.minMinor, candidate.minPatch) {
				check.Usable = false
				check.Note = strings.TrimSpace(strings.Join([]string{candidate.note, fmt.Sprintf("requires Go %d.%d.%d or newer", candidate.minMajor, candidate.minMinor, candidate.minPatch)}, "; "))
			}
		} else {
			check.Note = strings.TrimSpace(strings.Join([]string{candidate.note, "version probe failed: " + probeErr.Error()}, "; "))
		}
		if !check.Usable {
			if candidate.required {
				result.Ready = false
			} else {
				result.Degraded = true
			}
		}
		result.Checks = append(result.Checks, check)
	}
	return result
}

func selectedTools(m manifest.Manifest) []tool {
	if manifest.IsStackRecipe(m.Recipe) || m.Recipe == manifest.RecipeFiles {
		return stackTools(m)
	}
	tools := []tool{
		{name: "Go", command: "go", args: []string{"version"}, required: true, note: "builds the generated project", minMajor: 1, minMinor: 26, minPatch: 5},
		{name: "Git", command: "git", args: []string{"--version"}, required: true, note: "provides repository identity and release history"},
	}
	if m.Distribution.GitHubActions || m.Distribution.GoReleaser {
		tools = append(tools, tool{name: "Task", command: "task", args: []string{"--version"}, note: "runs the generated development workflow"})
	}
	if m.Distribution.GoReleaser {
		tools = append(tools, tool{name: "GoReleaser", command: "goreleaser", args: []string{"--version"}, note: "packages tagged releases"})
	}
	selected := map[string]tool{
		"codemap":    {name: "Codemap", command: "codemap", args: []string{"--version"}, note: "structural code intelligence"},
		"vecgrep":    {name: "Vecgrep", command: "vecgrep", args: []string{"--version"}, note: "semantic code discovery"},
		"glyphrun":   {name: "Glyphrun", command: "glyph", args: []string{"--version"}, note: "terminal behavior verification"},
		"cairntrace": {name: "Cairntrace", command: "cairn", args: []string{"--version"}, note: "browser behavior verification"},
		"tinyvault":  {name: "TinyVault", command: "tvault", args: []string{"--version"}, note: "secret-safe command execution"},
		"fcheap":     {name: "file.cheap", command: "fcheap", args: []string{"--version"}, note: "durable artifact storage"},
	}
	values := []string{
		m.Integrations.CodeStructure,
		m.Integrations.SemanticSearch,
		m.Integrations.TerminalVerification,
		m.Integrations.BrowserVerification,
		m.Integrations.Secrets,
		m.Integrations.Artifacts,
	}
	seen := map[string]bool{}
	for _, value := range values {
		candidate, ok := selected[value]
		if !ok || seen[value] {
			continue
		}
		seen[value] = true
		tools = append(tools, candidate)
	}
	optional := tools[2:]
	sort.SliceStable(optional, func(i, j int) bool { return optional[i].name < optional[j].name })
	return tools
}

// stackTools probes Git as the only required tool for stack hygiene recipes
// and the files recipe, plus the language toolchain as optional checks. Bob
// does not build these stacks itself, so a missing toolchain degrades the
// report without failing readiness.
func stackTools(m manifest.Manifest) []tool {
	tools := []tool{
		{name: "Git", command: "git", args: []string{"--version"}, required: true, note: "provides repository identity and release history"},
	}
	byLanguage := map[string][]tool{
		"typescript": {
			{name: "Bun", command: "bun", args: []string{"--version"}, note: "runs the JavaScript-family toolchain"},
			{name: "Node", command: "node", args: []string{"--version"}, note: "runs the JavaScript-family toolchain"},
		},
		"javascript": {
			{name: "Node", command: "node", args: []string{"--version"}, note: "runs the JavaScript-family toolchain"},
		},
		"python": {
			{name: "Python", command: "python3", args: []string{"--version"}, note: "runs the Python toolchain"},
		},
		"ruby": {
			{name: "Ruby", command: "ruby", args: []string{"--version"}, note: "runs the Ruby toolchain"},
			{name: "Bundler", command: "bundle", args: []string{"--version"}, note: "installs Ruby dependencies"},
		},
		"lua": {
			{name: "Lua", command: "lua", args: []string{"-v"}, note: "runs the Lua toolchain"},
			{name: "LuaRocks", command: "luarocks", args: []string{"--version"}, note: "installs Lua dependencies"},
		},
		"rust": {
			{name: "Cargo", command: "cargo", args: []string{"--version"}, note: "runs the Rust toolchain"},
		},
	}
	optional := append([]tool(nil), byLanguage[m.Runtime.Language]...)
	sort.SliceStable(optional, func(i, j int) bool { return optional[i].name < optional[j].name })
	return append(tools, optional...)
}

func versionAtLeast(output string, wantMajor, wantMinor, wantPatch int) bool {
	for _, field := range strings.Fields(output) {
		if !strings.HasPrefix(field, "go") {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(field, "go"), ".")
		if len(parts) < 2 {
			continue
		}
		major, majorErr := strconv.Atoi(parts[0])
		minor, minorErr := strconv.Atoi(parts[1])
		if majorErr != nil || minorErr != nil {
			continue
		}
		if major != wantMajor {
			return major > wantMajor
		}
		if minor != wantMinor {
			return minor > wantMinor
		}
		if len(parts) < 3 {
			return wantPatch == 0
		}
		patch, patchErr := strconv.Atoi(parts[2])
		return patchErr == nil && patch >= wantPatch
	}
	return false
}

type cappedBuffer struct {
	buf       bytes.Buffer
	truncated bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := maxProbeOutput - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return original, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buf.Write(p)
	return original, nil
}

func (b *cappedBuffer) String() string {
	value := b.buf.String()
	if b.truncated {
		value += "\n… output truncated"
	}
	return value
}

func firstLine(value string) string {
	if line, _, ok := strings.Cut(value, "\n"); ok {
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(value)
}
