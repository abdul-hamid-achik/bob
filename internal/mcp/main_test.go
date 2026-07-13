package mcp

import (
	"context"
	"fmt"
	"os"
	"testing"

	inspectpkg "github.com/abdul-hamid-achik/bob/internal/inspect"
)

const (
	stdioHelperEnv       = "BOB_TEST_MCP_STDIO_HELPER"
	stdioHelperWorkspace = "BOB_TEST_MCP_WORKSPACE"
)

func TestMain(m *testing.M) {
	if os.Getenv(stdioHelperEnv) == "1" {
		server, err := NewServer(os.Getenv(stdioHelperWorkspace), inspectpkg.ExecRunner{})
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if err := server.Run(context.Background()); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
