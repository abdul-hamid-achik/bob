package guidance

import (
	"errors"
	"testing"
)

func TestCodedErrorPreservesIdentityAndFirstPreciseCode(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("sentinel")
	err := WithErrorCode(ErrorInputInvalid, sentinel)
	err = WithErrorCode("context_failed", err)
	if !errors.Is(err, sentinel) {
		t.Fatal("coded error lost wrapped identity")
	}
	if code, ok := ErrorCode(err); !ok || code != ErrorInputInvalid {
		t.Fatalf("error code = %q, %v", code, ok)
	}
}

func TestErrorCodeRejectsUntypedErrors(t *testing.T) {
	t.Parallel()
	if code, ok := ErrorCode(errors.New("manifest appears in prose")); ok || code != "" {
		t.Fatalf("untyped error was classified: %q, %v", code, ok)
	}
}
