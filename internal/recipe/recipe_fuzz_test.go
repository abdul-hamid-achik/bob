package recipe

import (
	"strings"
	"testing"
)

func FuzzSafePath(f *testing.F) {
	seeds := []string{
		// valid paths
		"README.md",
		"src/main.go",
		"deeply/nested/dir/file.txt",
		"a",
		"file with spaces.txt",
		"unicode/héllo/wörld.txt",
		"emoji/🚀/launch.txt",
		"trailing/slash/",
		"double//slash",
		"./leading/dot",
		"foo/./bar",
		// traversal attempts
		".",
		"..",
		"...",
		"../escape",
		"foo/..",
		"foo/../bar",
		"foo/../../bar",
		"a/b/c/../../../..",
		// absolute paths
		"/absolute/path",
		"/",
		// reserved paths — exact
		".git",
		"bob.yaml",
		"bob.lock",
		// reserved paths — children (.git only)
		".git/config",
		".git/HEAD",
		// safePath does NOT block children of bob.yaml/bob.lock
		"bob.yaml/child",
		"bob.lock/child",
		// paths that merely contain reserved names
		"not-git",
		"my.bob.yaml",
		"xbob.lockx",
		"git/config",
		"sub/.git",
		"sub/bob.yaml",
		// special characters
		"path/with\nnewline",
		"~tilde",
		"back\\slash",
		"a\x00b",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result, err := safePath(input)
		if err != nil {
			return // rejected paths are fine
		}

		// Property: result is never empty.
		if result == "" {
			t.Errorf("safePath(%q) returned empty string with nil error", input)
		}

		// Property: result is relative (never starts with "/").
		if strings.HasPrefix(result, "/") {
			t.Errorf("safePath(%q) returned absolute path: %q", input, result)
		}

		// Property: result is never "." or "..".
		if result == "." || result == ".." {
			t.Errorf("safePath(%q) returned dot path: %q", input, result)
		}

		// Property: result never contains ".." as a path component.
		for _, component := range strings.Split(result, "/") {
			if component == ".." {
				t.Errorf("safePath(%q) returned path with .. component: %q", input, result)
				break
			}
		}

		// Property: result is clean (no "." components, no double slashes).
		for _, component := range strings.Split(result, "/") {
			if component == "." {
				t.Errorf("safePath(%q) returned path with . component: %q", input, result)
				break
			}
			if component == "" {
				t.Errorf("safePath(%q) returned path with empty component (double slash): %q", input, result)
				break
			}
		}

		// Property: result never targets .git or its children.
		if result == ".git" || strings.HasPrefix(result, ".git/") {
			t.Errorf("safePath(%q) returned reserved .git path: %q", input, result)
		}

		// Property: result never equals bob.yaml or bob.lock.
		if result == "bob.yaml" || result == "bob.lock" {
			t.Errorf("safePath(%q) returned reserved path: %q", input, result)
		}
	})
}
