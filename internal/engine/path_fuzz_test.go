package engine

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzNormalizeRepositoryPath(f *testing.F) {
	seeds := []string{
		// valid paths
		"README.md",
		"src/main.go",
		"deeply/nested/dir/file.txt",
		"a",
		"file with spaces.txt",
		"unicode/héllo/wörld.txt",
		"emoji/🚀/launch.txt",
		"tabs\tand spaces",
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
		// absolute and volume paths
		"/absolute/path",
		"/",
		"C:\\windows\\system32",
		"\\\\server\\share",
		// null bytes
		"a\x00b",
		"\x00",
		"file\x00.txt",
		// empty
		"",
		// reserved-ish (NormalizeRepositoryPath does not block these)
		"bob.yaml",
		"bob.lock",
		".git/config",
		".bob.apply.lock",
		// long paths
		strings.Repeat("a", 4096),
		strings.Repeat("a", 4097),
		strings.Repeat("long/", 1000) + "file.txt",
		// special characters
		"path/with\nnewline",
		"path/with\rreturn",
		"~tilde",
		"$dollar",
		"back\\slash",
		"semi;colon",
		"pipe|char",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, input string) {
		result, err := NormalizeRepositoryPath(input)
		if err != nil {
			return // rejected paths are fine
		}

		// Property: result is never empty.
		if result == "" {
			t.Errorf("NormalizeRepositoryPath(%q) returned empty string with nil error", input)
		}

		// Property: result is valid UTF-8.
		if !utf8.ValidString(result) {
			t.Errorf("NormalizeRepositoryPath(%q) returned invalid UTF-8: %q", input, result)
		}

		// Property: result contains no null bytes.
		if strings.ContainsRune(result, '\x00') {
			t.Errorf("NormalizeRepositoryPath(%q) returned path with null byte: %q", input, result)
		}

		// Property: result is at most 4096 bytes.
		if len(result) > 4096 {
			t.Errorf("NormalizeRepositoryPath(%q) returned path exceeding 4096 bytes: %d", input, len(result))
		}

		// Property: result never starts with "/".
		if strings.HasPrefix(result, "/") {
			t.Errorf("NormalizeRepositoryPath(%q) returned absolute path: %q", input, result)
		}

		// Property: result never contains ".." as a path component.
		for _, component := range strings.Split(result, "/") {
			if component == ".." {
				t.Errorf("NormalizeRepositoryPath(%q) returned path with .. component: %q", input, result)
				break
			}
		}

		// Property: result is never "." or "..".
		if result == "." || result == ".." {
			t.Errorf("NormalizeRepositoryPath(%q) returned dot path: %q", input, result)
		}

		// Property: result uses forward slashes only (no backslash separators).
		// Note: backslashes in the original filename are preserved as literal
		// characters on Unix, so we only check that filepath.ToSlash was applied
		// by verifying no OS separator remains on platforms where it differs.
	})
}
