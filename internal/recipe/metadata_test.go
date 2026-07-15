package recipe

import (
	"encoding/json"
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
	if metadata.Recipe.ID != manifest.RecipeGoAgentTool || metadata.Recipe.Version != 4 {
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
		{"unsafe path", func(md *Metadata) { md.Artifacts[0].Path = "../escape" }, "unsafe path"},
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

func maximalGoAgentManifest() manifest.Manifest {
	m := manifest.Default("acme", "github.com/acme/acme", "Acme")
	m.Integrations.BrowserVerification = "cairntrace"
	m.Integrations.Secrets = "tinyvault"
	m.Integrations.Artifacts = "fcheap"
	m.Distribution.Homebrew = true
	return m
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
