package main

import (
	"os"

	"github.com/bcostea/claude-split/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
