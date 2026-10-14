package recipe

import (
	"fmt"
	"sort"
	"strings"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type javaToolTemplateData struct {
	Manifest       manifest.Manifest
	Product        manifest.Product
	RecipeVersion  int
	GitHubOwner    string
	GitHubRepo     string
	JavaPackage    string
	DescriptionXML string
}

// renderJavaTool renders the complete, reviewable Java (Maven) CLI scaffold.
// Like rust-tool, python-tool, and ts-tool it owns a whole repository — a thin
// main entry point over a tested pure function, plus Maven tooling
// configuration, documentation, CI, and a release workflow — but it keeps no
// extension-registry contract.
func renderJavaTool(m manifest.Manifest, version int) ([]Artifact, error) {
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("render java-tool: %w", err)
	}
	owner, repo, _ := githubModuleCoordinates(m.Product.Module)
	data := javaToolTemplateData{
		Manifest:       m,
		Product:        m.Product,
		RecipeVersion:  version,
		GitHubOwner:    owner,
		GitHubRepo:     repo,
		JavaPackage:    javaPackage(m.Product.Module, m.Product.Name),
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
		{".editorconfig", javaToolEditorConfigTemplate},
		{".gitignore", javaToolGitignoreTemplate},
		{".github/workflows/ci.yml", javaToolCITemplate},
		{".github/workflows/release.yml", javaToolReleaseTemplate},
		{"AGENTS.md", javaToolAgentsTemplate},
		{"CHANGELOG.md", javaToolChangelogTemplate},
		{"CODE_OF_CONDUCT.md", javaToolCodeOfConductTemplate},
		{"CONTRIBUTING.md", javaToolContributingTemplate},
		{"LICENSE", javaToolLicenseTemplate},
		{"README.md", javaToolReadmeTemplate},
		{"SECURITY.md", javaToolSecurityTemplate},
		{"Taskfile.yml", javaToolTaskfileTemplate},
		{"checkstyle.xml", javaToolCheckstyleTemplate},
		{"docs/index.md", javaToolDocsIndexTemplate},
		{"pom.xml", javaToolPomTemplate},
		{"src/main/java/" + strings.ReplaceAll(data.JavaPackage, ".", "/") + "/Cli.java", javaToolCliTemplate},
		{"src/main/java/" + strings.ReplaceAll(data.JavaPackage, ".", "/") + "/Main.java", javaToolMainTemplate},
		{"src/test/java/" + strings.ReplaceAll(data.JavaPackage, ".", "/") + "/CliTest.java", javaToolCliTestTemplate},
	}
	for _, item := range base {
		if err := add(item.path, item.source); err != nil {
			return nil, err
		}
	}

	seedJavaToolLivingFiles(artifacts)

	// Render is the public sorting and path-safety boundary, but keeping the
	// producer ordered makes direct recipe tests deterministic.
	sort.Slice(artifacts, func(i, j int) bool { return artifacts[i].Path < artifacts[j].Path })
	return artifacts, nil
}

// javaToolLivingFiles are created once by java-tool@1 and then left in human
// hands. pom.xml in particular changes the first time a real repository adds a
// dependency; lock-owning it would make every later bob check/apply fail on an
// ordinary edit.
var javaToolLivingFiles = map[string]struct{}{
	"AGENTS.md":    {},
	"CHANGELOG.md": {},
	"README.md":    {},
	"pom.xml":      {},
}

func seedJavaToolLivingFiles(artifacts []Artifact) {
	for i := range artifacts {
		if _, living := javaToolLivingFiles[artifacts[i].Path]; living {
			artifacts[i].Seed = true
		}
	}
}

// javaPackage derives a valid Java package from the product module and name.
// When the module looks like a host path (the first slash-separated segment
// contains a dot, e.g. github.com/acme/acme-tool), the host is stripped to its
// top-level label (github.com -> com) and the remaining segments are lowercased
// with non-alphanumerics removed. Otherwise the package is
// com.example.<name>, with the same sanitization. Keyword and empty segments
// are dropped so the result is always a valid package path.
func javaPackage(module, name string) string {
	segments := []string{}
	parts := strings.Split(module, "/")
	if len(parts) > 1 && strings.Contains(parts[0], ".") {
		labels := strings.Split(parts[0], ".")
		if last := sanitizeJavaSegment(labels[len(labels)-1]); last != "" {
			segments = append(segments, last)
		}
		for _, part := range parts[1:] {
			if segment := sanitizeJavaSegment(part); segment != "" {
				segments = append(segments, segment)
			}
		}
	} else {
		segments = append(segments, "com", "example")
		if segment := sanitizeJavaSegment(name); segment != "" {
			segments = append(segments, segment)
		}
	}
	if len(segments) == 0 {
		return "com.example.tool"
	}
	return strings.Join(segments, ".")
}

// sanitizeJavaSegment lowercases a package segment, drops non-alphanumerics,
// and returns "" for empty, keyword, or otherwise unusable segments so the
// caller can omit them.
func sanitizeJavaSegment(segment string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(segment) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	s := b.String()
	if s == "" || javaKeywords[s] {
		return ""
	}
	if s[0] >= '0' && s[0] <= '9' {
		return "_" + s
	}
	return s
}

var javaKeywords = map[string]bool{
	"abstract": true, "assert": true, "boolean": true, "break": true,
	"byte": true, "case": true, "catch": true, "char": true, "class": true,
	"const": true, "continue": true, "default": true, "do": true,
	"double": true, "else": true, "enum": true, "extends": true,
	"final": true, "finally": true, "float": true, "for": true, "goto": true,
	"if": true, "implements": true, "import": true, "instanceof": true,
	"int": true, "interface": true, "long": true, "native": true, "new": true,
	"package": true, "private": true, "protected": true, "public": true,
	"return": true, "short": true, "static": true, "strictfp": true,
	"super": true, "switch": true, "synchronized": true, "this": true,
	"throw": true, "throws": true, "transient": true, "try": true,
	"void": true, "volatile": true, "while": true, "true": true,
	"false": true, "null": true, "_": true, "var": true, "record": true,
	"sealed": true, "permits": true, "yield": true,
}

// xmlEscape escapes the five XML-significant characters so a manifest product
// description can be rendered verbatim inside a pom.xml element.
func xmlEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '&':
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&quot;")
		case '\'':
			b.WriteString("&apos;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

const javaToolPomTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>

  <groupId>[[.JavaPackage]]</groupId>
  <artifactId>[[.Product.Name]]</artifactId>
  <version>0.1.0</version>
  <packaging>jar</packaging>

  <name>[[.Product.Name]]</name>
  <description>[[.DescriptionXML]]</description>

  <properties>
    <maven.compiler.release>21</maven.compiler.release>
    <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
    <junit.version>5.11.3</junit.version>
  </properties>

  <dependencies>
    <dependency>
      <groupId>org.junit.jupiter</groupId>
      <artifactId>junit-jupiter</artifactId>
      <version>${junit.version}</version>
      <scope>test</scope>
    </dependency>
  </dependencies>

  <build>
    <plugins>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-compiler-plugin</artifactId>
        <version>3.13.0</version>
        <configuration>
          <release>21</release>
        </configuration>
      </plugin>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-surefire-plugin</artifactId>
        <version>3.5.2</version>
      </plugin>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-shade-plugin</artifactId>
        <version>3.6.0</version>
        <executions>
          <execution>
            <phase>package</phase>
            <goals>
              <goal>shade</goal>
            </goals>
            <configuration>
              <transformers>
                <transformer implementation="org.apache.maven.plugins.shade.resource.ManifestResourceTransformer">
                  <mainClass>[[.JavaPackage]].Main</mainClass>
                </transformer>
              </transformers>
            </configuration>
          </execution>
        </executions>
      </plugin>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-checkstyle-plugin</artifactId>
        <version>3.6.0</version>
        <configuration>
          <configLocation>checkstyle.xml</configLocation>
          <consoleOutput>true</consoleOutput>
        </configuration>
        <executions>
          <execution>
            <phase>verify</phase>
            <goals>
              <goal>check</goal>
            </goals>
          </execution>
        </executions>
      </plugin>
    </plugins>
  </build>
</project>
`

const javaToolCliTemplate = `package [[.JavaPackage]];

/** Command-line entry point and pure core for [[.Product.Name]]. */
public final class Cli {

    private Cli() {}

    /** Returns a friendly greeting for the given name. */
    public static String greet(String name) {
        return "Hello, " + name + "!";
    }

    /**
     * Runs the CLI: greets the first positional argument (default "world")
     * and, with --json, writes a machine-readable greeting object to stdout.
     *
     * @param args command-line arguments
     */
    public static void main(String[] args) {
        String name = "world";
        boolean json = false;
        for (String arg : args) {
            if (arg.equals("--json")) {
                json = true;
            } else if (!arg.startsWith("--")) {
                name = arg;
            }
        }
        String message = greet(name);
        if (json) {
            System.out.println("{\"greeting\":\"" + jsonEscape(message) + "\"}");
        } else {
            System.out.println(message);
        }
    }

    private static String jsonEscape(String value) {
        return value.replace("\\", "\\\\").replace("\"", "\\\"");
    }
}
`

const javaToolMainTemplate = `package [[.JavaPackage]];

/** Thin JVM entry point for the [[.Product.Name]] command line. */
public final class Main {

    private Main() {}

    /**
     * Delegates to {@link Cli#main(String[])}.
     *
     * @param args command-line arguments
     */
    public static void main(String[] args) {
        Cli.main(args);
    }
}
`

const javaToolCliTestTemplate = `package [[.JavaPackage]];

import static org.junit.jupiter.api.Assertions.assertEquals;

import org.junit.jupiter.api.Test;

/** Unit tests for the pure {@link Cli} core. */
final class CliTest {

    @Test
    void greetFormatsAName() {
        assertEquals("Hello, world!", Cli.greet("world"));
        assertEquals("Hello, cli!", Cli.greet("cli"));
    }
}
`

const javaToolGitignoreTemplate = `/target/
*.class
.DS_Store
.env
.env.*
!.env.example
`

const javaToolEditorConfigTemplate = `root = true

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

const javaToolCheckstyleTemplate = `<?xml version="1.0"?>
<!DOCTYPE module PUBLIC
  "-//Checkstyle//DTD Checkstyle Configuration 1.3//EN"
  "https://checkstyle.org/dtds/configuration_1_3.dtd">

<!-- A deliberately small, universal Checkstyle ruleset for [[.Product.Name]].
     Add stricter modules as the repository grows. -->
<module name="Checker">
  <module name="FileTabCharacter"/>
  <module name="LineLength">
    <property name="max" value="100"/>
  </module>
  <module name="TreeWalker">
    <module name="AvoidStarImport"/>
    <module name="EmptyBlock"/>
    <module name="ModifierOrder"/>
    <module name="NeedBraces"/>
    <module name="WhitespaceAround"/>
  </module>
</module>
`

const javaToolTaskfileTemplate = `version: "3"

tasks:
  default:
    cmds:
      - task --list

  lint:
    desc: Run Checkstyle.
    cmds:
      - mvn -B -ntp checkstyle:check

  test:
    desc: Run the test suite.
    cmds:
      - mvn -B -ntp test

  build:
    desc: Build the executable jar.
    cmds:
      - mvn -B -ntp package

  run:
    desc: Run the CLI.
    cmds:
      - mvn -B -ntp -q package
      - java -jar target/[[.Product.Name]]-0.1.0.jar

  check:
    desc: Run the canonical non-mutating verification gate.
    aliases: [verify]
    cmds:
      - task: lint
      - task: test
      - task: build
`

const javaToolCITemplate = `name: CI

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
  java:
    runs-on: ubuntu-latest
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@9c091bb21b7c1c1d1991bb908d89e4e9dddfe3e0 # v7.0.0
      - uses: actions/setup-java@cf277c60eb25467037889841efdb72551f06f6c3 # v4
        with:
          distribution: temurin
          java-version: "21"
      - run: mvn -B -ntp verify
`

const javaToolReleaseTemplate = `name: Release

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
      - uses: actions/setup-java@cf277c60eb25467037889841efdb72551f06f6c3 # v4
        with:
          distribution: temurin
          java-version: "21"
      - run: mvn -B -ntp test
      - run: mvn -B -ntp package
      - uses: actions/upload-artifact@65c4c4a1ddee5b72f698fdd19549f0f0fb45cf08 # v4.4.0
        with:
          name: [[.Product.Name]]-${{ github.ref_name }}
          path: target/[[.Product.Name]]-0.1.0.jar
`

const javaToolReadmeTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Install

~~~bash
mvn package
~~~

This produces ` + "`target/[[.Product.Name]]-0.1.0.jar`" + `, a self-contained
executable jar.

## Usage

~~~bash
java -jar target/[[.Product.Name]]-0.1.0.jar world
java -jar target/[[.Product.Name]]-0.1.0.jar world --json
~~~

## Development

~~~bash
mvn -B -ntp verify
task check
~~~

See [AGENTS.md](AGENTS.md) for the agent and contributor contract,
[CONTRIBUTING.md](CONTRIBUTING.md) for the contribution workflow, and
[SECURITY.md](SECURITY.md) for security reporting instructions.

## License

MIT — see [LICENSE](LICENSE).
`

const javaToolAgentsTemplate = `# AGENTS.md

This file is the source of truth for agents and contributors working on [[.Product.Name]].

## Product

[[.Product.Description]]

## Architecture

- ` + "`src/main/java/[[.JavaPackage]]/Cli.java`" + ` owns the pure ` + "`greet`" + ` function and the CLI entry point.
- ` + "`src/main/java/[[.JavaPackage]]/Main.java`" + ` is the thin JVM entry point.
- ` + "`src/test/java/[[.JavaPackage]]/CliTest.java`" + ` covers the pure function through JUnit 5.
- ` + "`pom.xml`" + ` is human-owned: adding a dependency is a normal edit.

## Commands

~~~bash
mvn -B -ntp checkstyle:check
mvn -B -ntp test
mvn -B -ntp package
~~~

## Invariants

1. Keep ` + "`src/main/java/[[.JavaPackage]]/Main.java`" + ` thin; put behavior in ` + "`src/main/java/[[.JavaPackage]]/Cli.java`" + `.
2. Add or update tests for every behavior change.
3. Never commit credentials, private data, or local environment files.
`

const javaToolChangelogTemplate = `# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project uses [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Initial public repository foundation.
`

const javaToolContributingTemplate = `# Contributing

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

const javaToolCodeOfConductTemplate = `# Code of Conduct

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

const javaToolSecurityTemplate = `# Security Policy

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

const javaToolLicenseTemplate = `MIT License

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

const javaToolDocsIndexTemplate = `# [[.Product.Name]]

[[.Product.Description]]

## Start here

~~~bash
mvn package
java -jar target/[[.Product.Name]]-0.1.0.jar world
~~~

## Development

Run ` + "`task check`" + ` before opening a pull request. See [AGENTS.md](../AGENTS.md)
for the agent contract.
`
