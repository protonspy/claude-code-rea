// Command rea-launcher starts OpenCode with an abliterated model and REA attached.
package main

import (
	"os"

	"github.com/protonspy/claude-code-rea/internal/launcher"
)

// version is set at release time with -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	os.Exit(launcher.Main(os.Args[1:], launcher.DefaultSystem(version)))
}
