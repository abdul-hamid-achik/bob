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
			if candidate.minMajor > 0 && !versionAtLeast(version, candidate.minMajor, candidate.minMinor) {
				check.Usable = false
				check.Note = strings.TrimSpace(strings.Join([]string{candidate.note, fmt.Sprintf("requires Go %d.%d or newer", candidate.minMajor, candidate.minMinor)}, "; "))
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
	tools := []tool{
		{name: "Go", command: "go", args: []string{"version"}, required: true, note: "builds the generated project", minMajor: 1, minMinor: 26},
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

func versionAtLeast(output string, wantMajor, wantMinor int) bool {
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
		return major > wantMajor || major == wantMajor && minor >= wantMinor
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
