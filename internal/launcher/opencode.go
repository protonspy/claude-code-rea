package launcher

import (
	"context"
	"fmt"
)

// runOpenCode runs the managed OpenCode on the launcher's terminal with cfg and p's key in
// its environment, and returns OpenCode's exit code.
func runOpenCode(ctx context.Context, sys System, p Provider, model, key string, cfg []byte, args []string) (int, error) {
	l := sys.layout()
	env := sys.environWith(map[string]string{
		p.KeyVar:                  key,
		"OPENCODE_CONFIG_CONTENT": string(cfg),
		"OPENCODE_CONFIG_DIR":     l.ConfigDir(),
	})
	fmt.Fprintf(sys.Stderr, "rea-launcher: this session's content is sent to %s, model %s\n", p.Label, model)
	sys.IgnoreInterrupts()
	return sys.Run(ctx, Command{
		Path:   l.OpenCode(),
		Args:   args,
		Env:    env,
		Stdin:  sys.Stdin,
		Stdout: sys.Stdout,
		Stderr: sys.Stderr,
	})
}
