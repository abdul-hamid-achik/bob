package engine

import (
	"strings"
	"testing"
)

func FuzzValidateRelativePath(f *testing.F) {
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
		// absolute and volume paths
		"/absolute/path",
		"/",
		"C:\\windows\\system32",
		// null bytes and empty
		"a\x00b",
		"\x00",
		"",
		// reserved paths — exact
		".git",
		"bob.yaml",
		"bob.lock",
		".bob.apply.lock",
		// reserved paths — children
		".git/config",
		".git/HEAD",
		"bob.yaml/child",
		"bob.lock/child",
		".bob.apply.lock/child",
		// paths that merely contain reserved names as substrings
		"not-git",
		"my.bob.yaml",
		"xbob.lockx",
		"git/config",
		"sub/.git",
		"sub/bob.yaml",
		"sub/bob.lock",
		// special characters
		"path/with\nnewline",
		"~tilde",
		"back\\slash",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	reserved := map[string]bool{
		".git":            true,
		"bob.yaml":        true,
		"bob.lock":        true,
		".bob.apply.lock": true,
	}

	f.Fuzz(func(t *testing.T, input string) {
		result, err := validateRelativePath(input)
		if err != nil {
			return // rejected paths are fine
		}

		// Property: result is never empty.
		if result == "" {
			t.Errorf("validateRelativePath(%q) returned empty string with nil error", input)
		}

		// Property: result contains no null bytes.
		if strings.ContainsRune(result, '\x00') {
			t.Errorf("validateRelativePath(%q) returned path with null byte: %q", input, result)
		}

		// Property: result never starts with "/".
		if strings.HasPrefix(result, "/") {
			t.Errorf("validateRelativePath(%q) returned absolute path: %q", input, result)
		}

		// Property: result never contains ".." as a path component.
		for _, component := range strings.Split(result, "/") {
			if component == ".." {
				t.Errorf("validateRelativePath(%q) returned path with .. component: %q", input, result)
				break
			}
		}

		// Property: result is never "." or "..".
		if result == "." || result == ".." {
			t.Errorf("validateRelativePath(%q) returned dot path: %q", input, result)
		}

		// Property: result never targets a reserved path or its children.
		if reserved[result] {
			t.Errorf("validateRelativePath(%q) returned reserved path: %q", input, result)
		}
		for name := range reserved {
			if strings.HasPrefix(result, name+"/") {
				t.Errorf("validateRelativePath(%q) returned child of reserved path %q: %q", input, name, result)
				break
			}
		}
	})
}
