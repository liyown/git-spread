package cli

import (
	"fmt"
	"io"
)

const Version = "dev"

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "git-spread %s\n", Version)
		return 0
	}
	fmt.Fprintln(stderr, "git-spread: command parser is not initialized")
	return 2
}
