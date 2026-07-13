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
}

func TestVersionAtLeast(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		value string
		want  bool
	}{{"go version go1.26.0 darwin/arm64", true}, {"go1.27.1", true}, {"go version go1.25.9", false}, {"garbage", false}} {
		if got := versionAtLeast(test.value, 1, 26); got != test.want {
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
