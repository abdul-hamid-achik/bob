package fsutil

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateArtifactPathAcceptsAndCanonicalizes(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"README.md":          "README.md",
		"./src/main.go":      "src/main.go",
		"a//b/../c.txt":      "a/c.txt",
		"sub/bob.yaml":       "sub/bob.yaml",
		"my.bob.yaml":        "my.bob.yaml",
		"not-git/config":     "not-git/config",
		".github/ci.yml":     ".github/ci.yml",
		"trailing/slash/":    "trailing/slash",
		"unicode/héllo.txt":  "unicode/héllo.txt",
		".gitignore":         ".gitignore",
		"bob.lockfile":       "bob.lockfile",
		".bob.apply.lock.md": ".bob.apply.lock.md",
	}
	for input, want := range cases {
		got, err := ValidateArtifactPath(input)
		if err != nil {
			t.Errorf("%q: unexpected error %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("%q: got %q, want %q", input, got, want)
		}
	}
}

func TestValidateArtifactPathRejectsUnsafe(t *testing.T) {
	t.Parallel()
	unsafe := []string{"", ".", "..", "../escape", "a/../../b", "/absolute", "a\x00b", "\x00", "\\\\server\\share"}
	for _, input := range unsafe {
		_, err := ValidateArtifactPath(input)
		if !errors.Is(err, ErrUnsafeArtifactPath) {
			t.Errorf("%q: got %v, want ErrUnsafeArtifactPath", input, err)
		}
	}
}

func TestValidateArtifactPathRejectsReservedAndChildren(t *testing.T) {
	t.Parallel()
	reserved := []string{
		".git", ".git/config", ".git/hooks/pre-commit",
		ManifestFilename, ManifestFilename + "/child", "./" + ManifestFilename,
		LockFilename, LockFilename + "/child",
		ApplyLockFilename, ApplyLockFilename + "/child",
	}
	for _, input := range reserved {
		_, err := ValidateArtifactPath(input)
		if !errors.Is(err, ErrReservedArtifactPath) {
			t.Errorf("%q: got %v, want ErrReservedArtifactPath", input, err)
		}
		if err != nil && !strings.Contains(err.Error(), input) {
			t.Errorf("%q: error %q should name the original path", input, err)
		}
	}
}
