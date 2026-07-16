package doctor

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/abdul-hamid-achik/bob/internal/manifest"
)

type fakeProber struct {
	missing     map[string]bool
	failVersion map[string]bool
	goVersion   string
}

func (f fakeProber) LookPath(name string) (string, error) {
	if f.missing[name] {
		return "", errors.New("missing")
	}
	return "/bin/" + name, nil
}

func (f fakeProber) Version(_ context.Context, name string, _ ...string) (string, error) {
	if f.failVersion[name] {
		return "", errors.New("probe failed")
	}
	if name == "/bin/go" {
		if f.goVersion != "" {
			return f.goVersion, nil
		}
		return "go version go1.26.5 test", nil
	}
	return name + " v1.0.0", nil
}

func TestRunDistinguishesRequiredAndOptionalTools(t *testing.T) {
	t.Parallel()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme CLI")
	result := Run(context.Background(), m, fakeProber{missing: map[string]bool{"glyph": true}})
	if !result.Ready || !result.Degraded {
		t.Fatalf("expected ready but degraded, got %#v", result)
	}
	result = Run(context.Background(), m, fakeProber{missing: map[string]bool{"go": true}})
	if result.Ready {
		t.Fatalf("expected missing Go to make result unready: %#v", result)
	}
}

func TestRunRejectsFailedRequiredProbeAndOldGo(t *testing.T) {
	t.Parallel()
	m := manifest.Default("acme", "github.com/acme/acme", "Acme CLI")
	result := Run(context.Background(), m, fakeProber{failVersion: map[string]bool{"/bin/go": true}})
	if result.Ready {
		t.Fatalf("failed required probe reported ready: %#v", result)
	}
	result = Run(context.Background(), m, fakeProber{goVersion: "go version go1.25.9 test"})
	if result.Ready || result.Checks[0].Usable {
		t.Fatalf("old Go reported usable: %#v", result)
	}
	result = Run(context.Background(), m, fakeProber{goVersion: "go version go1.26.4 test"})
	if result.Ready || result.Checks[0].Usable || !strings.Contains(result.Checks[0].Note, "Go 1.26.5 or newer") {
		t.Fatalf("pre-security-patch Go reported usable: %#v", result)
	}
}

func TestVersionAtLeast(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		value string
		want  bool
	}{
		{"go version go1.26.0 darwin/arm64", false},
		{"go version go1.26.4 darwin/arm64", false},
		{"go version go1.26.5 darwin/arm64", true},
		{"go version go1.26.6 darwin/arm64", true},
		{"go1.26.5rc1", false},
		{"go1.27.0", true},
		{"go2.0.0", true},
		{"go version go1.25.99", false},
		{"garbage", false},
	} {
		if got := versionAtLeast(test.value, 1, 26, 5); got != test.want {
			t.Errorf("versionAtLeast(%q) = %t, want %t", test.value, got, test.want)
		}
	}
}

func TestCappedBufferBoundsOutput(t *testing.T) {
	t.Parallel()
	var b cappedBuffer
	input := strings.Repeat("x", maxProbeOutput+100)
	n, err := b.Write([]byte(input))
	if err != nil || n != len(input) {
		t.Fatalf("write = %d, %v", n, err)
	}
	if len(b.buf.String()) != maxProbeOutput || !strings.Contains(b.String(), "truncated") {
		t.Fatalf("buffer was not bounded: %d", len(b.buf.String()))
	}
}

func TestRunStackRecipesRequireOnlyGit(t *testing.T) {
	t.Parallel()
	m, err := manifest.DefaultStack(manifest.RecipeTSApp, "demo", "", "A demo.", "")
	if err != nil {
		t.Fatal(err)
	}
	// A missing toolchain degrades the report without failing readiness.
	result := Run(context.Background(), m, fakeProber{missing: map[string]bool{"bun": true, "node": true, "go": true}})
	if !result.Ready || !result.Degraded {
		t.Fatalf("expected ready but degraded stack doctor, got %#v", result)
	}
	names := make([]string, 0, len(result.Checks))
	for _, check := range result.Checks {
		names = append(names, check.Name)
		if check.Name == "Go" {
			t.Fatalf("stack recipes must not probe Go: %#v", result.Checks)
		}
	}
	if len(names) != 3 || names[0] != "Git" {
		t.Fatalf("unexpected stack checks: %v", names)
	}
	// A missing Git still fails readiness.
	result = Run(context.Background(), m, fakeProber{missing: map[string]bool{"git": true}})
	if result.Ready {
		t.Fatalf("missing git must make the stack doctor unready: %#v", result)
	}
}
