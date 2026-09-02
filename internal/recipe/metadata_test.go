package recipe

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

func TestResolveMetadataSortedAndCrossReferenced(t *testing.T) {
	t.Parallel()
	m := maximalGoAgentManifest()
	metadata, err := ResolveMetadata(m)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateMetadata(metadata, artifacts); err != nil {
		t.Fatal(err)
	}
	assertSortedMetadataIDs(t, metadata)
	if metadata.Recipe.ID != manifest.RecipeGoAgentTool || metadata.Recipe.Version != 6 {
		t.Fatalf("recipe = %#v", metadata.Recipe)
	}
	foundCommandExtension := false
	for _, extension := range metadata.ExtensionPoints {
		if extension.ID == "cli.command_files" {
			foundCommandExtension = true
			if !containsString(extension.PlaybookIDs, "add-cli-command") {
				t.Fatalf("command extension playbooks = %#v", extension.PlaybookIDs)
			}
		}
	}
	if !foundCommandExtension {
		t.Fatal("v4 metadata omitted cli.command_files")
	}
}

func TestResolveMetadataCapabilitySelections(t *testing.T) {
	t.Parallel()
	base := manifest.Default("acme", "github.com/acme/acme", "Acme")
	base.Integrations = manifest.Integrations{CodeStructure: "none", SemanticSearch: "none", TerminalVerification: "none", BrowserVerification: "none", Secrets: "none", Artifacts: "none"}
	base.Distribution = manifest.Distribution{Docs: "none"}
	tests := []struct {
		id     string
		mutate func(*manifest.Manifest)
	}{
		{"distribution.github_actions", func(m *manifest.Manifest) { m.Distribution.GitHubActions = true }},
		{"distribution.goreleaser", func(m *manifest.Manifest) { m.Distribution.GoReleaser = true }},
		{"distribution.homebrew", func(m *manifest.Manifest) { m.Distribution.GoReleaser = true; m.Distribution.Homebrew = true }},
		{"docs.markdown", func(m *manifest.Manifest) { m.Distribution.Docs = "markdown" }},
		{"integration.cairntrace", func(m *manifest.Manifest) { m.Integrations.BrowserVerification = "cairntrace" }},
		{"integration.codemap", func(m *manifest.Manifest) { m.Integrations.CodeStructure = "codemap" }},
		{"integration.fcheap", func(m *manifest.Manifest) { m.Integrations.Artifacts = "fcheap" }},
		{"integration.glyphrun", func(m *manifest.Manifest) { m.Integrations.TerminalVerification = "glyphrun" }},
		{"integration.tinyvault", func(m *manifest.Manifest) { m.Integrations.Secrets = "tinyvault" }},
		{"integration.vecgrep", func(m *manifest.Manifest) { m.Integrations.SemanticSearch = "vecgrep" }},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			m := base
			tc.mutate(&m)
			metadata, err := ResolveMetadata(m)
			if err != nil {
				t.Fatal(err)
			}
			capability := findCapability(t, metadata, tc.id)
			if capability.Selection != "enabled" {
				t.Fatalf("selection = %q", capability.Selection)
			}
			if len(capability.ArtifactIDs) == 0 {
				t.Fatalf("%s has no materialization evidence", tc.id)
			}
		})
	}
}

func TestFilesMetadataIsGeneric(t *testing.T) {
	t.Parallel()
	m := manifest.Manifest{SchemaVersion: 1, Recipe: manifest.RecipeFiles, Product: manifest.Product{Name: "web-app", Description: "Web service"}, Files: []manifest.FileDecl{{Path: "cmd/server/main.go", Content: "package main\n"}, {Path: "package.json", Content: "{}\n"}}}
	metadata, err := ResolveMetadata(m)
	if err != nil {
		t.Fatal(err)
	}
	if len(metadata.Capabilities) != 2 {
		t.Fatalf("capabilities = %#v", metadata.Capabilities)
	}
	for _, artifact := range metadata.Artifacts {
		if len(artifact.Roles) != 1 || artifact.Roles[0] != "declared_file" {
			t.Fatalf("inferred semantics for %s: %#v", artifact.Path, artifact.Roles)
		}
		if !strings.HasPrefix(artifact.ID, "files:") {
			t.Fatalf("artifact id = %q", artifact.ID)
		}
	}
}

func TestResolvedMetadataUsesArraysForClosedLists(t *testing.T) {
	t.Parallel()
	metadata, err := ResolveMetadata(maximalGoAgentManifest())
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), ":null") {
		t.Fatalf("metadata contains a nullable closed-list field: %s", data)
	}
}

func TestValidateMetadataRejectsBrokenContracts(t *testing.T) {
	t.Parallel()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	metadata, err := ResolveMetadata(m)
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(*Metadata)
		want   string
	}{
		{"duplicate id", func(md *Metadata) { md.Capabilities[1].ID = md.Capabilities[0].ID }, "duplicate"},
		{"unsafe path", func(md *Metadata) { md.Artifacts[0].Path = "../escape" }, "unsafe artifact path"},
		{"unrendered path", func(md *Metadata) { md.Artifacts[0].Path = "not-rendered.txt" }, "unrendered"},
		{"unknown artifact", func(md *Metadata) { md.Capabilities[0].ArtifactIDs = append(md.Capabilities[0].ArtifactIDs, "missing") }, "unknown artifact"},
		{"unknown capability", func(md *Metadata) { md.Artifacts[0].CapabilityIDs = append(md.Artifacts[0].CapabilityIDs, "missing") }, "unknown capability"},
		{"bad template", func(md *Metadata) { md.ExtensionPoints[0].CreatePatterns = []string{"../<file>.go"} }, "invalid path template"},
		{"playbook unknown capability", func(md *Metadata) { md.Playbooks[0].CapabilityIDs = []string{"missing"} }, "unknown capability"},
		{"playbook unknown extension", func(md *Metadata) { md.Playbooks[0].ExtensionPointIDs = []string{"missing"} }, "unknown extension point"},
		{"playbook unsafe path", func(md *Metadata) { md.Playbooks[0].Boundary.Create = []string{"../<file>.go"} }, "invalid guidance path template"},
		{"playbook unknown dependency", func(md *Metadata) { md.Playbooks[0].Steps[0].DependsOn = []string{"missing"} }, "unknown step"},
		{"schema", func(md *Metadata) { md.SchemaVersion++ }, "unsupported metadata schema"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			data, marshalErr := json.Marshal(metadata)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			var broken Metadata
			if unmarshalErr := json.Unmarshal(data, &broken); unmarshalErr != nil {
				t.Fatal(unmarshalErr)
			}
			tc.mutate(&broken)
			if err := ValidateMetadata(broken, artifacts); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

// TestDeclaredSurfaceCapabilities proves surfaces.mcp and surfaces.studio are
// projected as descriptive capabilities: selection follows the manifest
// boolean, no artifact is claimed (the recipe renders nothing for them), and
// the limitation names the descriptive contract.
func TestDeclaredSurfaceCapabilities(t *testing.T) {
	t.Parallel()
	tests := []struct {
		id      string
		enabled bool
		mutate  func(*manifest.Manifest)
	}{
		{"surface.mcp", false, func(m *manifest.Manifest) {}},
		{"surface.mcp", true, func(m *manifest.Manifest) { m.Surfaces.MCP = true }},
		{"surface.studio", false, func(m *manifest.Manifest) {}},
		{"surface.studio", true, func(m *manifest.Manifest) { m.Surfaces.Studio = true }},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			t.Parallel()
			m := manifest.Default("acme", "github.com/acme/acme", "Acme")
			tc.mutate(&m)
			metadata, err := ResolveMetadata(m)
			if err != nil {
				t.Fatal(err)
			}
			capability := findCapability(t, metadata, tc.id)
			want := "disabled"
			if tc.enabled {
				want = "enabled"
			}
			if capability.Selection != want {
				t.Fatalf("selection = %q, want %q", capability.Selection, want)
			}
			if len(capability.ArtifactIDs) != 0 {
				t.Fatalf("declared surface claims artifacts: %#v", capability.ArtifactIDs)
			}
			if !containsString(capability.Limitations, "declared surface; the recipe neither generates nor verifies it") {
				t.Fatalf("limitations = %#v", capability.Limitations)
			}
		})
	}
}

func maximalGoAgentManifest() manifest.Manifest {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	m.Integrations.BrowserVerification = "cairntrace"
	m.Integrations.Secrets = "tinyvault"
	m.Integrations.Artifacts = "fcheap"
	m.Distribution.Homebrew = true
	return m
}

// TestSurfaceMCPEvidenceRules pins the default read-only evidence the recipe
// declares for the descriptive MCP surface: conventional package paths, a
// cmd/mcp glob, and the bounded Contains probe of the recipe-known
// entrypoint (the rule that catches subcommand-style MCP servers).
func TestSurfaceMCPEvidenceRules(t *testing.T) {
	t.Parallel()
	metadata, err := ResolveMetadata(manifest.Default("acme", "github.com/acme/acme", "Acme"))
	if err != nil {
		t.Fatal(err)
	}
	capability := findCapability(t, metadata, "surface.mcp")
	want := []EvidenceRule{
		{Path: "cmd/<product>/main.go", Contains: "mcp"},
		{Path: "cmd/mcp/**"},
		{Path: "internal/cli/mcp.go"},
		{Path: "internal/mcp"},
	}
	if !reflect.DeepEqual(capability.Evidence, want) {
		t.Fatalf("evidence = %#v, want %#v", capability.Evidence, want)
	}
	studio := findCapability(t, metadata, "surface.studio")
	if len(studio.Evidence) != 0 {
		t.Fatalf("surface.studio evidence = %#v", studio.Evidence)
	}
}

// TestValidateMetadataEvidenceRules proves metadata validation keeps
// evidence rules bounded and path-safe before any evaluation happens.
func TestValidateMetadataEvidenceRules(t *testing.T) {
	t.Parallel()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	artifacts, err := Render(m)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		rules []EvidenceRule
		want  string
	}{
		{"absolute path", []EvidenceRule{{Path: "/etc/passwd"}}, "invalid path template"},
		{"escaping path", []EvidenceRule{{Path: "../outside"}}, "invalid path template"},
		{".git target", []EvidenceRule{{Path: ".git/hooks/mcp"}}, "invalid path template"},
		{"too many rules", []EvidenceRule{
			{Path: "internal/mcp"}, {Path: "internal/cli/mcp.go"}, {Path: "cmd/mcp/**"},
			{Path: "a/1"}, {Path: "a/2"}, {Path: "a/3"}, {Path: "a/4"}, {Path: "a/5"}, {Path: "a/6"},
		}, "more than 8 evidence rules"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			metadata, err := ResolveMetadata(m)
			if err != nil {
				t.Fatal(err)
			}
			for i := range metadata.Capabilities {
				if metadata.Capabilities[i].ID == "surface.mcp" {
					metadata.Capabilities[i].Evidence = tt.rules
				}
			}
			if err := ValidateMetadata(metadata, artifacts); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func findCapability(t *testing.T, metadata Metadata, id string) CapabilityDefinition {
	t.Helper()
	for _, capability := range metadata.Capabilities {
		if capability.ID == id {
			return capability
		}
	}
	t.Fatalf("missing capability %s", id)
	return CapabilityDefinition{}
}

func assertSortedMetadataIDs(t *testing.T, metadata Metadata) {
	t.Helper()
	for _, ids := range [][]string{capabilityIDs(metadata.Capabilities), artifactIDs(metadata.Artifacts), invariantIDs(metadata.Invariants), extensionIDs(metadata.ExtensionPoints), playbookIDs(metadata.Playbooks)} {
		for i := 1; i < len(ids); i++ {
			if ids[i-1] >= ids[i] {
				t.Fatalf("IDs not sorted and unique: %#v", ids)
			}
		}
	}
}

func capabilityIDs(values []CapabilityDefinition) []string {
	out := []string{}
	for _, v := range values {
		out = append(out, v.ID)
	}
	return out
}
func artifactIDs(values []ArtifactDescriptor) []string {
	out := []string{}
	for _, v := range values {
		out = append(out, v.ID)
	}
	return out
}
func invariantIDs(values []InvariantDefinition) []string {
	out := []string{}
	for _, v := range values {
		out = append(out, v.ID)
	}
	return out
}
func extensionIDs(values []ExtensionPointDefinition) []string {
	out := []string{}
	for _, v := range values {
		out = append(out, v.ID)
	}
	return out
}

func playbookIDs(values []PlaybookDefinition) []string {
	out := []string{}
	for _, v := range values {
		out = append(out, v.ID)
	}
	return out
}
