package launcher

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
)

// keyVars are the environment variables every known provider's key travels in.
func keyVars() []string {
	vars := make([]string, len(providers))
	for i, p := range providers {
		vars[i] = p.KeyVar
	}
	return vars
}

// withAutoApproval adds OpenCode's --auto to args, and reports whether it applies. OpenCode
// accepts --auto only on its interface — no arguments, flags, or a project directory — and
// on run, after the command name; on any other command it is a parse error, so those are
// left as they are.
func withAutoApproval(args []string) ([]string, bool) {
	if slices.Contains(args, "--auto") {
		return args, true
	}
	switch {
	case len(args) == 0 || strings.HasPrefix(args[0], "-"):
		return append([]string{"--auto"}, args...), true
	case args[0] == "run":
		return append([]string{"run", "--auto"}, args[1:]...), true
	}
	if info, err := os.Stat(args[0]); err == nil && info.IsDir() {
		return append([]string{"--auto"}, args...), true
	}
	return args, false
}

// runOpenCode runs the managed OpenCode on the launcher's terminal with cfg and p's key in
// its environment, and returns OpenCode's exit code. The session directory may hold an
// analysed target, so OpenCode reads no configuration, plugin or Claude Code file from it,
// loads no skill from outside the launcher's own directory, and neither updates itself nor
// downloads language servers.
func runOpenCode(ctx context.Context, sys System, p Provider, model, key string, cfg []byte, args []string) (int, error) {
	l := sys.layout()
	env := sys.environWith(map[string]string{
		p.KeyVar:                           key,
		"OPENCODE_CONFIG_CONTENT":          string(cfg),
		"OPENCODE_CONFIG_DIR":              l.ConfigDir(),
		"OPENCODE_DISABLE_PROJECT_CONFIG":  "1",
		"OPENCODE_DISABLE_CLAUDE_CODE":     "1",
		"OPENCODE_DISABLE_EXTERNAL_SKILLS": "1",
		"OPENCODE_DISABLE_AUTOUPDATE":      "1",
		"OPENCODE_DISABLE_LSP_DOWNLOAD":    "1",
	}, "OPENCODE_ENABLE_EXA")
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
