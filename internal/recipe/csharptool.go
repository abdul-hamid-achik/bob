package recipe

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type csharpToolTemplateData struct {
	Manifest       manifest.Manifest
	Product        manifest.Product
	RecipeVersion  int
	GitHubOwner    string
	GitHubRepo     string
	App            string
	DescriptionXML string
}

// renderCSharpTool renders the complete, reviewable .NET (C#) CLI scaffold.
// Like rust-tool, python-tool, ts-tool, and java-tool it owns a whole
// repository — a thin top-level entry point over a tested pure function, plus
// .NET tooling configuration, documentation, CI, and a release workflow — but
// it keeps no extension-registry contract.
func renderCSharpTool(m manifest.Manifest, version int) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("render csharp-tool: %w", err)
	}
	owner, repo, _ := githubModuleCoordinates(m.Product.Module)
	app := csharpAppName(m.Product.Name)
	data := csharpToolTemplateData{
		Manifest:       m,
		Product:        m.Product,
		RecipeVersion:  version,
		GitHubOwner:    owner,
		GitHubRepo:     repo,
		App:            app,
		DescriptionXML: xmlEscape(m.Product.Description),
	}

	var artifacts []Artifact
	add := func(path, source string) error {
		content, err := executeRecipeTemplate(path, source, data)
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
		{".editorconfig", csharpToolEditorConfigTemplate},
		{".gitignore", csharpToolGitignoreTemplate},
		{".github/workflows/ci.yml", csharpToolCITemplate},
		{".github/workflows/release.yml", csharpToolReleaseTemplate},
		{"AGENTS.md", csharpToolAgentsTemplate},
		{"CHANGELOG.md", csharpToolChangelogTemplate},
		{"CODE_OF_CONDUCT.md", csharpToolCodeOfConductTemplate},
		{"CONTRIBUTING.md", csharpToolContributingTemplate},
		{"LICENSE", csharpToolLicenseTemplate},
		{"README.md", csharpToolReadmeTemplate},
		{"SECURITY.md", csharpToolSecurityTemplate},
		{"Taskfile.yml", csharpToolTaskfileTemplate},
		{"docs/index.md", csharpToolDocsIndexTemplate},
		{"global.json", csharpToolGlobalJSONTemplate},
		{"src/" + app + "/" + app + ".csproj", csharpToolProjectTemplate},
		{"src/" + app + "/Cli.cs", csharpToolCliTemplate},
		{"src/" + app + "/Program.cs", csharpToolProgramTemplate},
		{"tests/" + app + ".Tests/" + app + ".Tests.csproj", csharpToolTestsProjectTemplate},
		{"tests/" + app + ".Tests/CliTests.cs", csharpToolCliTestsTemplate},
	}
	for _, item := range base {
		if err := add(item.path, item.source); err != nil {
			return nil, err
		}
	}

	seedCSharpToolLivingFiles(artifacts)

	// Render is the public sorting and path-safety boundary, but keeping the
	// producer ordered makes direct recipe tests deterministic.
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

// csharpToolLivingFiles are created once by csharp-tool@1 and then left in
// human hands. The main project file in particular changes the first time a
// real repository adds a package reference; lock-owning it would make every
// later bob check/apply fail on an ordinary edit.
var csharpToolLivingFiles = map[string]struct{}{
	"AGENTS.md":    {},
	"CHANGELOG.md": {},
	"README.md":    {},
	"global.json":  {},
}

func seedCSharpToolLivingFiles(artifacts []Artifact) {
	for i := range artifacts {
		path := artifacts[i].Path
		if _, living := csharpToolLivingFiles[path]; living {
			artifacts[i].Seed = true
			continue
		}
		// The main project file is seed-once: adding a PackageReference is a
		// normal human edit that must stay clean.
		if strings.HasPrefix(path, "src/") && strings.HasSuffix(path, ".csproj") {
			artifacts[i].Seed = true
		}
	}
}

// csharpAppName derives a valid PascalCase C# assembly/namespace name from the
// product name. Hyphen-separated segments are title-cased and a leading digit
// is prefixed with an underscore so the result is always a valid C# identifier.
func csharpAppName(name string) string {
	var b strings.Builder
	for _, segment := range strings.Split(name, "-") {
		if segment == "" {
			continue
		}
		for i, r := range segment {
			if i == 0 {
				if r >= '0' && r <= '9' {
					b.WriteRune('_')
					b.WriteRune(r)
				} else {
					b.WriteRune(unicode.ToUpper(r))
				}
			} else {
				b.WriteRune(r)
			}
		}
	}
	if b.Len() == 0 {
		return "Tool"
	}
	return b.String()
}

const csharpToolGlobalJSONTemplate = `{
  "sdk": {
    "version": "8.0.100",
    "rollForward": "latestFeature"
  }
}
`

const csharpToolProjectTemplate = `<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <OutputType>Exe</OutputType>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <AssemblyName>[[.App]]</AssemblyName>
    <RootNamespace>[[.App]]</RootNamespace>
    <Description>[[.DescriptionXML]]</Description>
  </PropertyGroup>

</Project>
`

const csharpToolTestsProjectTemplate = `<Project Sdk="Microsoft.NET.Sdk">

  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <ImplicitUsings>enable</ImplicitUsings>
    <Nullable>enable</Nullable>
    <IsPackable>false</IsPackable>
    <IsTestProject>true</IsTestProject>
  </PropertyGroup>

  <ItemGroup>
    <PackageReference Include="Microsoft.NET.Test.Sdk" Version="17.11.1" />
    <PackageReference Include="xunit" Version="2.9.2" />
    <PackageReference Include="xunit.runner.visualstudio" Version="2.8.2">
      <IncludeAssets>runtime; build; native; contentfiles; analyzers; buildtransitive</IncludeAssets>
      <PrivateAssets>all</PrivateAssets>
    </PackageReference>
  </ItemGroup>

  <ItemGroup>
    <ProjectReference Include="../../src/[[.App]]/[[.App]].csproj" />
  </ItemGroup>

</Project>
`

const csharpToolCliTemplate = `namespace [[.App]];

/// <summary>
/// Pure core and command-line entry point for [[.Product.Name]].
/// </summary>
public static class Cli
{
    /// <summary>Returns a friendly greeting for the given name.</summary>
    public static string Greet(string name) => $"Hello, {name}!";

    /// <summary>
    /// Runs the CLI: greets the first positional argument (default "world")
    /// and, with --json, writes a machine-readable greeting object to stdout.
    /// </summary>
    public static int Run(string[] args)
    {
        var name = "world";
        var json = false;
        foreach (var arg in args)
        {
            if (arg == "--json")
            {
                json = true;
            }
            else if (!arg.StartsWith("--"))
            {
                name = arg;
            }
        }

        var message = Greet(name);
        if (json)
        {
            Console.WriteLine($"{{\"greeting\":\"{JsonEscape(message)}\"}}");
        }
        else
        {
            Console.WriteLine(message);
        }

        return 0;
    }

    private static string JsonEscape(string value) =>
        value.Replace("\\", "\\\\").Replace("\"", "\\\"");
}
`

const csharpToolProgramTemplate = `using [[.App]];

// Thin entry point for the [[.Product.Name]] command line.
return Cli.Run(args);
`

const csharpToolCliTestsTemplate = `using [[.App]];
using Xunit;

namespace [[.App]].Tests;

/// <summary>Unit tests for the pure <see cref="Cli"/> core.</summary>
public sealed class CliTests
{
    [Fact]
    public void Greet_FormatsAName()
    {
        Assert.Equal("Hello, world!", Cli.Greet("world"));
        Assert.Equal("Hello, cli!", Cli.Greet("cli"));
    }
}
`

const csharpToolGitignoreTemplate = `bin/
obj/
.vs/
*.user
.DS_Store
.env
.env.*
!.env.example
`

const csharpToolEditorConfigTemplate = `root = true

[*]
charset = utf-8
end_of_line = lf
insert_final_newline = true
trim_trailing_whitespace = true
indent_style = space
indent_size = 4

[*.md]
trim_trailing_whitespace = false
`

const csharpToolTaskfileTemplate = `version: "3"

tasks:
  default:
    cmds:
      - task --list

  format:
    desc: Format the source tree with dotnet format.
    cmds:
      - dotnet format src/[[.App]]/[[.App]].csproj --verbosity minimal

  build:
    desc: Build the main project with warnings as errors.
    cmds:
      - dotnet build src/[[.App]]/[[.App]].csproj -warnaserror --nologo

  test:
    desc: Run the test suite.
    cmds:
      - dotnet test tests/[[.App]].Tests/[[.App]].Tests.csproj --nologo

  run:
    desc: Run the CLI.
    cmds:
      - dotnet run --project src/[[.App]]/[[.App]].csproj

  check:
    desc: Run the canonical non-mutating verification gate.
    aliases: [verify]
    cmds:
      - task: build
      - task: test
`

const csharpToolCITemplate = `name: CI

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
  build:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      - uses: actions/setup-dotnet@67a3573c9a986a3f9c594539f4ab511d57bb3ce9 # v4
        with:
          dotnet-version: "8.0.x"
      - run: dotnet build src/[[.App]]/[[.App]].csproj -warnaserror --nologo
      - run: dotnet test tests/[[.App]].Tests/[[.App]].Tests.csproj --nologo
`

const csharpToolReleaseTemplate = `name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: read

jobs:
  release:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      - uses: actions/setup-dotnet@67a3573c9a986a3f9c594539f4ab511d57bb3ce9 # v4
        with:
          dotnet-version: "8.0.x"
      - run: dotnet test tests/[[.App]].Tests/[[.App]].Tests.csproj --nologo
      - run: dotnet publish src/[[.App]]/[[.App]].csproj -c Release -o dist
      - uses: actions/upload-artifact@65c4c4a1ddee5b72f698fdd19549f0f0fb45cf08 # v4.4.0
        with:
          name: [[.App]]-${{ github.ref_name }}
          path: dist/
`

const csharpToolReadmeTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Install

~~~bash
dotnet build src/[[.App]]/[[.App]].csproj
~~~

This produces ` + "`src/[[.App]]/bin/Debug/net8.0/[[.App]]`" + `, a self-contained
executable.

## Usage

~~~bash
dotnet run --project src/[[.App]]/[[.App]].csproj -- world
dotnet run --project src/[[.App]]/[[.App]].csproj -- world --json
~~~

## Development

~~~bash
dotnet test tests/[[.App]].Tests/[[.App]].Tests.csproj
task check
~~~

See [AGENTS.md](AGENTS.md) for the agent and contributor contract,
[CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow, and
[SECURITY.md](SECURITY.md) for security reporting instructions.

## License

MIT — see [LICENSE](LICENSE).
`

const csharpToolAgentsTemplate = `# AGENTS.md

This file is the source of truth for agents and contributors working on [[.Product.Name]].

## Product

[[.Product.Description]]

## Architecture

- ` + "`src/[[.App]]/Cli.cs`" + ` owns the pure ` + "`Greet`" + ` function and the CLI entry point.
- ` + "`src/[[.App]]/Program.cs`" + ` is the thin top-level entry point.
- ` + "`tests/[[.App]].Tests/CliTests.cs`" + ` covers the pure function through xUnit.
- ` + "`src/[[.App]]/[[.App]].csproj`" + ` is human-owned: adding a package reference is a normal edit.

## Commands

~~~bash
dotnet build src/[[.App]]/[[.App]].csproj -warnaserror
dotnet test tests/[[.App]].Tests/[[.App]].Tests.csproj
~~~

## Invariants

1. Keep ` + "`src/[[.App]]/Program.cs`" + ` thin; put behavior in ` + "`src/[[.App]]/Cli.cs`" + `.
2. Add or update tests for every behavior change.
3. Never commit credentials, private data, or local environment files.
`

const csharpToolChangelogTemplate = `# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial public repository foundation.
`

const csharpToolContributingTemplate = `# Contributing

Thank you for improving [[.Product.Name]].

Read [AGENTS.md](AGENTS.md) before changing behavior or public contracts.

1. Open an issue for substantial behavior or contract changes.
2. Create a focused branch and keep unrelated changes out of the diff.
3. Add or update tests for every behavior change.
4. Run ` + "`task check`" + ` before opening a pull request.

Pull requests should explain the user impact and the verification performed.
Never include credentials or private data. Community participation follows
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md). Contributions are licensed under the
repository's MIT License.
`

const csharpToolCodeOfConductTemplate = `# Code of Conduct

This community policy is based on the
[Contributor Covenant 2.1](https://www.contributor-covenant.org/version/2/1/code_of_conduct/).

Be respectful, inclusive, and constructive. Harassment, threats,
discrimination, sexualized conduct, personal attacks, trolling, and publishing
another person's private information are not accepted.

This standard applies in repository issues, pull requests, reviews, and other
spaces where someone represents the project. Maintainers may edit or remove
contributions, limit participation, or ban a participant when needed to protect
the community.

Report conduct concerns privately using a monitored contact method. Do not post
sensitive report details in a public issue. Reports should be handled promptly,
fairly, and as confidentially as practical.

This policy is adapted from the Contributor Covenant, version 2.1, available at
https://www.contributor-covenant.org/version/2/1/code_of_conduct/ and licensed
under Creative Commons Attribution 4.0.
`

const csharpToolSecurityTemplate = `# Security Policy

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

const csharpToolLicenseTemplate = `MIT License

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

const csharpToolDocsIndexTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Start here

~~~bash
dotnet build src/[[.App]]/[[.App]].csproj
dotnet run --project src/[[.App]]/[[.App]].csproj -- world
~~~

## Development

Run ` + "`task check`" + ` before opening a pull request. See [AGENTS.md](../AGENTS.md)
for the agent contract.
`
