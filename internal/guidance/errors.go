package guidance

import (
	"errors"
	"fmt"
)

const (
	ErrorInputInvalid     = "input_invalid"
	ErrorWorkspaceInvalid = "workspace_invalid"
	ErrorManifestInvalid  = "manifest_invalid"
)

// CodedError carries a stable domain error code from a transport-neutral
// guidance service to a CLI or MCP projection. The wrapped error remains
// available through errors.Is/errors.As.
type CodedError struct {
	code string
	err  error
}

func (e *CodedError) Error() string { return e.err.Error() }

func (e *CodedError) Unwrap() error { return e.err }

func (e *CodedError) GuidanceCode() string { return e.code }

// WithErrorCode annotates err with a stable guidance code. Existing coded
// errors are preserved so higher service layers cannot accidentally relabel a
// precise input or manifest failure as a generic service failure.
func WithErrorCode(code string, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := ErrorCode(err); ok {
		return err
	}
	if code == "" {
		return fmt.Errorf("guidance error code is required: %w", err)
	}
	return &CodedError{code: code, err: err}
}

// ErrorCode extracts a stable code through wrapped errors.
func ErrorCode(err error) (string, bool) {
	var coded interface{ GuidanceCode() string }
	if !errors.As(err, &coded) || coded.GuidanceCode() == "" {
		return "", false
	}
	return coded.GuidanceCode(), true
}
