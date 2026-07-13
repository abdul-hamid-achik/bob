package inspect

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
)

const (
	maxStdout = 1 << 20
	maxStderr = 16 << 10
)

// CommandResult is the bounded observation of one specialist status command.
type CommandResult struct {
	Stdout          []byte
	Stderr          []byte
	Err             error
	TimedOut        bool
	StdoutTruncated bool
	StderrTruncated bool
}

// Runner keeps binary discovery and optional specialist probes fakeable.
type Runner interface {
	LookPath(string) (string, error)
	Run(context.Context, string, string, ...string) CommandResult
}

// ExecRunner executes direct argv without a shell and caps captured output.
type ExecRunner struct{}

func (ExecRunner) LookPath(name string) (string, error) { return exec.LookPath(name) }

func (ExecRunner) Run(ctx context.Context, dir, name string, args ...string) CommandResult {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	stdout := &limitedBuffer{limit: maxStdout}
	stderr := &limitedBuffer{limit: maxStderr}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	return CommandResult{
		Stdout:          append([]byte(nil), stdout.Bytes()...),
		Stderr:          append([]byte(nil), stderr.Bytes()...),
		Err:             err,
		TimedOut:        errors.Is(ctx.Err(), context.DeadlineExceeded),
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
	}
}

type limitedBuffer struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := b.limit - b.buf.Len()
	if remaining <= 0 {
		b.truncated = true
		return n, nil
	}
	if len(p) > remaining {
		p = p[:remaining]
		b.truncated = true
	}
	_, _ = b.buf.Write(p)
	return n, nil
}

func (b *limitedBuffer) Bytes() []byte { return b.buf.Bytes() }
