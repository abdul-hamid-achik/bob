package cli

import "errors"

// Exit code contract for cmd/bob, the only caller authorized to invoke
// os.Exit. 0 always means success, including a plan run that finds
// conflicts: plan is a read-only report, never a refusal.
const (
	ExitOK           = 0
	ExitError        = 1
	ExitConflicts    = 2
	ExitDrift        = 3
	ExitInvalidInput = 4
)

// exitError attaches one of the exit codes above to an error without
// changing its message text. Commands opt a failure into a specific code
// only where the cause is cleanly classifiable; everything else falls back
// to ExitError.
type exitError struct {
	code int
	err  error
}

// newExitError wraps err with an explicit exit code. It returns nil when err
// is nil so call sites can wrap unconditionally.
func newExitError(code int, err error) error {
	if err == nil {
		return nil
	}
	return &exitError{code: code, err: err}
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// ExitCode maps an error returned by Execute to the process exit code
// contract. A nil error is success (0); an error that never opted into a
// specific code falls back to 1.
func ExitCode(err error) int {
	if err == nil {
		return ExitOK
	}
	var classified *exitError
	if errors.As(err, &classified) {
		return classified.code
	}
	return ExitError
}

// classifyInvalidInput marks a manifest load, parse, or validation failure
// (or another cleanly-classifiable bad input, such as a missing required
// flag) as exit code 4, without altering its message text. It returns nil
// when err is nil so call sites can wrap unconditionally.
func classifyInvalidInput(err error) error {
	return newExitError(ExitInvalidInput, err)
}
