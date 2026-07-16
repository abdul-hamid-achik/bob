// Package detect performs cheap, read-only repository stack detection for
// bob init. It inspects well-known marker files at the workspace root (plus a
// bounded shallow probe for .vue and Lua layouts), never executes anything,
// and never reads more than a bounded amount of any file.
package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// maxPackageJSONBytes bounds the only file detection parses.
const maxPackageJSONBytes = 1 << 20

// Stack is one detected language stack with the marker paths that proved it.
type Stack struct {
	// ID is a closed identifier: go, rust, python, ruby, lua, vue,
	// typescript, javascript, or static-web.
	ID      string   `json:"id"`
	Markers []string `json:"markers"`
}

// Result is the complete detection outcome for one repository root.
type Result struct {
	// Stacks lists every detected stack, most specific first. Empty when the
	// repository matched nothing (Primary is then "").
	Stacks []Stack `json:"stacks,omitempty"`
	// Primary is the stack id init should base recipe selection on.
	Primary string `json:"primary,omitempty"`
	// Monorepo reports a workspace/monorepo layout where the manifest
	// formats make that visible (turbo.json, pnpm-workspace.yaml, package
	// workspaces, go.work, Cargo workspace).
	Monorepo bool `json:"monorepo,omitempty"`
	// KindHint refines the recipe runtime.kind where detection can tell
	// (monorepo, gem, plugin); empty otherwise.
	KindHint string `json:"kind_hint,omitempty"`
	// PackageManager is the JavaScript-family package manager implied by the
	// lockfile present, when one is.
	PackageManager string `json:"package_manager,omitempty"`
	// Signals are secondary observations (sass, tailwind, postcss, vite,
	// tsconfig...) that do not select a recipe but inform a human.
	Signals []string `json:"signals,omitempty"`
}

// Detected reports whether any stack was recognized.
func (r Result) Detected() bool { return len(r.Stacks) > 0 }

// Has reports whether id is among the detected stacks.
func (r Result) Has(id string) bool {
	for _, stack := range r.Stacks {
		if stack.ID == id {
			return true
		}
	}
	return false
}

// Describe renders the detection as one short human-readable line, e.g.
// "typescript (bun.lock, package.json, tsconfig.json; monorepo)".
func (r Result) Describe() string {
	if !r.Detected() {
		return "unknown (no recognized stack markers)"
	}
	primary := r.Stacks[0]
	for _, stack := range r.Stacks {
		if stack.ID == r.Primary {
			primary = stack
			break
		}
	}
	detail := strings.Join(primary.Markers, ", ")
	if r.Monorepo {
		detail += "; monorepo"
	}
	return r.Primary + " (" + detail + ")"
}

// primaryPrecedence orders detected stacks for Primary selection. Compiled
// and scripting language markers outrank JavaScript-family markers because a
// package.json frequently exists only for tooling in those repositories, and
// vue outranks the generic typescript/javascript stacks it implies.
var primaryPrecedence = []string{"go", "rust", "ruby", "python", "lua", "vue", "typescript", "javascript", "static-web"}

// Detect inspects root and reports every recognized stack. A missing or
// unreadable root yields an empty result rather than an error: detection is
// advisory and init performs its own path validation.
func Detect(root string) Result {
	var result Result
	entries, err := os.ReadDir(root)
	if err != nil {
		return result
	}
	names := make(map[string]bool, len(entries))
	dirs := make(map[string]bool)
	var rockspecs, gemspecs, rootVueFiles []string
	for _, entry := range entries {
		name := entry.Name()
		names[name] = true
		if entry.IsDir() {
			dirs[name] = true
		}
		switch {
		case strings.HasSuffix(name, ".rockspec"):
			rockspecs = append(rockspecs, name)
		case strings.HasSuffix(name, ".gemspec"):
			gemspecs = append(gemspecs, name)
		case strings.HasSuffix(name, ".vue"):
			rootVueFiles = append(rootVueFiles, name)
		}
	}

	addStack := func(id string, markers ...string) {
		sort.Strings(markers)
		result.Stacks = append(result.Stacks, Stack{ID: id, Markers: markers})
	}
	addSignal := func(signal string) {
		for _, existing := range result.Signals {
			if existing == signal {
				return
			}
		}
		result.Signals = append(result.Signals, signal)
	}

	// Go.
	var goMarkers []string
	for _, marker := range []string{"go.mod", "go.work"} {
		if names[marker] {
			goMarkers = append(goMarkers, marker)
		}
	}
	if len(goMarkers) > 0 {
		addStack("go", goMarkers...)
		if names["go.work"] {
			result.Monorepo = true
		}
	}

	// Rust.
	if names["Cargo.toml"] {
		addStack("rust", "Cargo.toml")
		if containsBounded(filepath.Join(root, "Cargo.toml"), "[workspace]") {
			result.Monorepo = true
		}
	}

	// Ruby. A .gemspec means gem; Gemfile/Rakefile alone mean app.
	var rubyMarkers []string
	for _, marker := range []string{"Gemfile", "Rakefile"} {
		if names[marker] {
			rubyMarkers = append(rubyMarkers, marker)
		}
	}
	rubyMarkers = append(rubyMarkers, gemspecs...)
	if len(rubyMarkers) > 0 {
		addStack("ruby", rubyMarkers...)
		if len(gemspecs) > 0 {
			result.KindHint = "gem"
		}
	}

	// Python.
	var pythonMarkers []string
	for _, marker := range []string{"pyproject.toml", "setup.py", "setup.cfg", "requirements.txt", "Pipfile"} {
		if names[marker] {
			pythonMarkers = append(pythonMarkers, marker)
		}
	}
	if len(pythonMarkers) > 0 {
		addStack("python", pythonMarkers...)
	}

	// Lua: rockspecs, a .luarc.json, or the Neovim plugin layout (init.lua
	// or a lua/ directory at the root).
	var luaMarkers []string
	luaMarkers = append(luaMarkers, rockspecs...)
	if names[".luarc.json"] {
		luaMarkers = append(luaMarkers, ".luarc.json")
	}
	neovimLayout := names["init.lua"] || dirs["lua"]
	if names["init.lua"] {
		luaMarkers = append(luaMarkers, "init.lua")
	}
	if dirs["lua"] {
		luaMarkers = append(luaMarkers, "lua/")
	}
	if len(luaMarkers) > 0 {
		addStack("lua", luaMarkers...)
		if neovimLayout && len(rockspecs) == 0 {
			result.KindHint = "plugin"
		}
	}

	// JavaScript family: package.json, lockfiles, tsconfig.
	var nodeMarkers []string
	hasPackageJSON := names["package.json"]
	if hasPackageJSON {
		nodeMarkers = append(nodeMarkers, "package.json")
	}
	lockManagers := []struct{ file, manager string }{
		{"bun.lock", "bun"}, {"bun.lockb", "bun"},
		{"pnpm-lock.yaml", "pnpm"}, {"yarn.lock", "yarn"}, {"package-lock.json", "npm"},
	}
	for _, lock := range lockManagers {
		if names[lock.file] {
			nodeMarkers = append(nodeMarkers, lock.file)
			if result.PackageManager == "" {
				result.PackageManager = lock.manager
			}
		}
	}
	hasTSConfig := names["tsconfig.json"]
	if hasTSConfig {
		nodeMarkers = append(nodeMarkers, "tsconfig.json")
		addSignal("tsconfig")
	}

	pkg := readPackageJSON(filepath.Join(root, "package.json"))
	if hasPackageJSON {
		if names["turbo.json"] || names["pnpm-workspace.yaml"] || len(pkg.workspaces) > 0 {
			result.Monorepo = true
		}
	}
	for _, entry := range []struct{ file, signal string }{
		{"turbo.json", "turborepo"},
		{"tailwind.config.js", "tailwind"}, {"tailwind.config.ts", "tailwind"}, {"tailwind.config.cjs", "tailwind"},
		{"postcss.config.js", "postcss"}, {"postcss.config.cjs", "postcss"}, {"postcss.config.mjs", "postcss"},
		{"vite.config.js", "vite"}, {"vite.config.ts", "vite"}, {"vite.config.mjs", "vite"},
	} {
		if names[entry.file] {
			addSignal(entry.signal)
		}
	}
	if hasSassFiles(root, entries) {
		addSignal("sass")
	}

	// Vue: a vue dependency or .vue files (root or shallow src/). Vue wins
	// over generic typescript/javascript, which stay listed after it.
	vueByDependency := pkg.hasDependency("vue")
	vueFiles := append([]string(nil), rootVueFiles...)
	if len(vueFiles) == 0 {
		vueFiles = shallowVueFiles(filepath.Join(root, "src"))
	}
	if vueByDependency || len(vueFiles) > 0 {
		markers := make([]string, 0, len(vueFiles)+1)
		if vueByDependency {
			markers = append(markers, "package.json (vue dependency)")
		}
		markers = append(markers, vueFiles...)
		addStack("vue", markers...)
	}

	if len(nodeMarkers) > 0 || hasTSConfig {
		if hasTSConfig {
			addStack("typescript", nodeMarkers...)
		} else if staticWebToolingOnly(pkg) {
			markers := append([]string(nil), nodeMarkers...)
			if names["index.html"] {
				markers = append(markers, "index.html")
			}
			addStack("static-web", markers...)
		} else if hasPackageJSON {
			addStack("javascript", nodeMarkers...)
		}
	} else if names["index.html"] {
		addStack("static-web", "index.html")
	}

	sortStacks(result.Stacks)
	if len(result.Stacks) > 0 {
		result.Primary = result.Stacks[0].ID
	}
	if result.Monorepo && (result.Primary == "typescript" || result.Primary == "javascript") && result.KindHint == "" {
		result.KindHint = "monorepo"
	}
	sort.Strings(result.Signals)
	return result
}

type packageJSON struct {
	dependencies map[string]struct{}
	workspaces   []string
	exists       bool
}

func (p packageJSON) hasDependency(name string) bool {
	_, ok := p.dependencies[name]
	return ok
}

// readPackageJSON parses a bounded package.json. Any read or parse failure
// yields the zero value: detection is advisory and must not fail init.
func readPackageJSON(path string) packageJSON {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxPackageJSONBytes {
		return packageJSON{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return packageJSON{}
	}
	var decoded struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		Workspaces      json.RawMessage   `json:"workspaces"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return packageJSON{}
	}
	parsed := packageJSON{dependencies: map[string]struct{}{}, exists: true}
	for name := range decoded.Dependencies {
		parsed.dependencies[name] = struct{}{}
	}
	for name := range decoded.DevDependencies {
		parsed.dependencies[name] = struct{}{}
	}
	if len(decoded.Workspaces) > 0 {
		var list []string
		if err := json.Unmarshal(decoded.Workspaces, &list); err == nil {
			parsed.workspaces = list
		} else {
			var object struct {
				Packages []string `json:"packages"`
			}
			if err := json.Unmarshal(decoded.Workspaces, &object); err == nil {
				parsed.workspaces = object.Packages
			}
		}
	}
	return parsed
}

// staticWebTooling is the closed set of dependencies that still count as a
// static web site rather than a JavaScript application.
var staticWebTooling = map[string]struct{}{
	"sass": {}, "postcss": {}, "postcss-cli": {}, "autoprefixer": {}, "cssnano": {},
	"tailwindcss": {}, "vite": {}, "html-validate": {}, "linkinator": {}, "prettier": {},
}

// staticWebToolingOnly reports whether package.json exists and declares only
// css/static build tooling, which marks a static-web project despite the
// package.json.
func staticWebToolingOnly(pkg packageJSON) bool {
	if !pkg.exists || len(pkg.dependencies) == 0 {
		return false
	}
	for name := range pkg.dependencies {
		if _, ok := staticWebTooling[name]; !ok {
			return false
		}
	}
	return true
}

// shallowVueFiles lists .vue files one level under dir (bounded, no
// recursion). Missing directories yield nil.
func shallowVueFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var found []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".vue") {
			found = append(found, "src/"+entry.Name())
		}
	}
	return found
}

// hasSassFiles reports .scss/.sass files at the root or one level under
// styles-ish directories, bounded to avoid walking large repositories.
func hasSassFiles(root string, entries []os.DirEntry) bool {
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && (strings.HasSuffix(name, ".scss") || strings.HasSuffix(name, ".sass")) {
			return true
		}
	}
	for _, dir := range []string{"styles", "css", "scss", "sass", "assets"} {
		children, err := os.ReadDir(filepath.Join(root, dir))
		if err != nil {
			continue
		}
		for _, child := range children {
			name := child.Name()
			if !child.IsDir() && (strings.HasSuffix(name, ".scss") || strings.HasSuffix(name, ".sass")) {
				return true
			}
		}
	}
	return false
}

// containsBounded reports whether the first bounded chunk of the file at
// path contains needle. Read failures report false.
func containsBounded(path, needle string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()
	buffer := make([]byte, 64*1024)
	n, _ := file.Read(buffer)
	return strings.Contains(string(buffer[:n]), needle)
}

func sortStacks(stacks []Stack) {
	rank := make(map[string]int, len(primaryPrecedence))
	for i, id := range primaryPrecedence {
		rank[id] = i
	}
	sort.SliceStable(stacks, func(i, j int) bool { return rank[stacks[i].ID] < rank[stacks[j].ID] })
}
