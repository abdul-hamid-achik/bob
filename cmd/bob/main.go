package main

import (
	"os"

	"github.com/abdul-hamid-achik/bob/internal/cli"
)

func main() {
	// Execute prints its own stderr diagnostics (including corrective
	// "next:" steps) so the error line and its guidance stay adjacent and
	// are never duplicated here.
	os.Exit(cli.ExitCode(cli.Execute()))
}
