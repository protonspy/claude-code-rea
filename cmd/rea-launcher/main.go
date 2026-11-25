// Command rea-launcher starts OpenCode with an abliterated model and REA attached.
package main

import (
	"fmt"
	"os"

	"github.com/protonspy/opencode-rea/internal/launcher"
)

// version is set at release time with -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	sys, err := launcher.DefaultSystem(version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "rea-launcher: %v\n", err)
		os.Exit(1)
	}
	os.Exit(launcher.Main(os.Args[1:], sys))
}
