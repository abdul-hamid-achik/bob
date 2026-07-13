package main

import (
	"fmt"
	"os"

	"github.com/abdul-hamid-achik/bob/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "bob:", err)
		os.Exit(1)
	}
}
