// Package launcher starts OpenCode with an abliterated model and REA attached, after
// installing the pinned toolchain it runs on.
package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const usageText = `Usage:
  rea-launcher [--provider nanogpt|nearai] [--model <id>] [--print-config] [--] [opencode arguments]
  rea-launcher setup [--provider nanogpt|nearai] [--ghidra <dir>] [--java-home <dir>]
  rea-launcher logout [--provider nanogpt|nearai]
  rea-launcher version

Without a subcommand, installs what is missing, then starts OpenCode with REA in the
current directory. Arguments after -- go to OpenCode unchanged.
`

// Main runs the launcher with args, the command line without the program name, and
// returns the exit status.
func Main(args []string, sys System) int {
	ctx := context.Background()
	if len(args) > 0 {
		switch args[0] {
		case "setup":
			return setup(ctx, args[1:], sys)
		case "logout":
			return logout(args[1:], sys)
		case "version":
			return version(sys)
		case "help":
			fmt.Fprint(sys.Stdout, usageText)
			return 0
		}
	}
	return launch(ctx, args, sys)
}

// flags returns a flag set that reports to sys's stderr, with the --provider flag.
func flags(name string, sys System) (*flag.FlagSet, *string) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(sys.Stderr)
	fs.Usage = func() { fmt.Fprint(sys.Stderr, usageText) }
	provider := fs.String("provider", DefaultProvider, "the model provider: nanogpt or nearai")
	return fs, provider
}

// parse parses args and returns the selected provider, or the exit status to stop with.
func parse(fs *flag.FlagSet, provider *string, args []string, sys System) (Provider, int, bool) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return Provider{}, 0, false
		}
		return Provider{}, 2, false
	}
	p, err := LookupProvider(*provider)
	if err != nil {
		fail(sys, err)
		return Provider{}, 2, false
	}
	return p, 0, true
}

func fail(sys System, err error) {
	fmt.Fprintf(sys.Stderr, "rea-launcher: %v\n", err)
}

func launch(ctx context.Context, args []string, sys System) int {
	fs, provider := flags("rea-launcher", sys)
	model := fs.String("model", "", "a model id of the provider, in place of its default")
	printConfig := fs.Bool("print-config", false, "print the OpenCode configuration and exit")
	p, status, ok := parse(fs, provider, args, sys)
	if !ok {
		return status
	}
	qualified := p.QualifiedModel(*model)
	cfg, err := sessionConfig(sys, qualified)
	if err != nil {
		fail(sys, err)
		return 1
	}
	if *printConfig {
		var out bytes.Buffer
		if err := json.Indent(&out, cfg, "", "  "); err != nil {
			fail(sys, err)
			return 1
		}
		fmt.Fprintln(sys.Stdout, out.String())
		return 0
	}
	if err := prepare(ctx, sys); err != nil {
		fail(sys, err)
		return 1
	}
	key, _, err := resolveKey(ctx, p, sys)
	if err != nil {
		fail(sys, err)
		return 1
	}
	code, err := runOpenCode(ctx, sys, p, qualified, key, cfg, fs.Args())
	if err != nil {
		fail(sys, fmt.Errorf("run OpenCode: %w", err))
		return 1
	}
	return code
}

// sessionConfig returns the OpenCode configuration of a session on model.
func sessionConfig(sys System, model string) ([]byte, error) {
	env, err := reaEnvironment(sys)
	if err != nil {
		return nil, err
	}
	l := sys.layout()
	return json.Marshal(OpenCodeConfig(Session{
		Model:        model,
		Node:         l.Node(),
		REAScript:    l.REAScript(),
		REAEnv:       env,
		Instructions: l.Agents(),
		SkillsDir:    l.SkillsDir(),
	}))
}

// prepare installs what the managed toolchain lacks and the current AGENTS.md.
func prepare(ctx context.Context, sys System) error {
	if err := ensureToolchain(ctx, sys); err != nil {
		return err
	}
	if _, err := writeAgents(sys.layout().Agents()); err != nil {
		return fmt.Errorf("install AGENTS.md: %w", err)
	}
	return nil
}

func setup(ctx context.Context, args []string, sys System) int {
	fs, provider := flags("rea-launcher setup", sys)
	ghidra := fs.String("ghidra", "", "the Ghidra installation directory REA should use")
	javaHome := fs.String("java-home", "", "the JDK REA should run Ghidra with")
	p, status, ok := parse(fs, provider, args, sys)
	if !ok {
		return status
	}
	l := sys.layout()
	failed := false
	report := func(state, step, detail string) {
		fmt.Fprintf(sys.Stdout, "%-4s  %-12s %s\n", state, step, detail)
	}
	step := func(name, detail string, err error) bool {
		if err != nil {
			report("FAIL", name, err.Error())
			failed = true
			return false
		}
		report("ok", name, detail)
		return true
	}

	if !step("toolchain", fmt.Sprintf("Node.js %s, REA %s, OpenCode %s in %s", NodeVersion, REAVersion, OpenCodeVersion, l.Root), prepare(ctx, sys)) {
		return 1
	}
	step("instructions", fmt.Sprintf("%d REA skill(s) and AGENTS.md for launcher sessions", countSkills(l.SkillsDir())), nil)

	_, source, err := resolveKey(ctx, p, sys)
	step("key", p.Label+" key "+source, err)

	set := map[string]string{}
	for name, dir := range map[string]string{"GHIDRA_INSTALL_DIR": *ghidra, "JAVA_HOME": *javaHome} {
		if dir == "" {
			continue
		}
		abs, err := existingDir(dir)
		if err != nil {
			step("engine", "", err)
			return 1
		}
		set[name] = abs
	}
	if len(set) > 0 {
		if err := saveEngineSettings(sys, set); err != nil {
			step("engine", "", err)
			return 1
		}
	}
	env, err := reaEnvironment(sys)
	if err == nil {
		var ready bool
		var missing []string
		ready, missing, err = checkEngine(ctx, sys, env)
		if err == nil && ready {
			report("ok", "engine", "Ghidra is ready")
		} else if err == nil {
			report("WARN", "engine", "Ghidra is not ready; REA's static JavaScript, .NET and archive tools still work")
			for _, m := range missing {
				fmt.Fprintf(sys.Stdout, "%20s %s\n", "", m)
			}
			fmt.Fprintf(sys.Stdout, "%20s run setup again with --ghidra <dir> and --java-home <dir>\n", "")
		}
	}
	if err != nil {
		step("engine", "", err)
	}
	if failed {
		return 1
	}
	return 0
}

// existingDir returns dir as an absolute path, or an error if it is not a directory.
func existingDir(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", abs)
	}
	return abs, nil
}

func logout(args []string, sys System) int {
	fs, provider := flags("rea-launcher logout", sys)
	p, status, ok := parse(fs, provider, args, sys)
	if !ok {
		return status
	}
	had, err := forgetKey(p, sys)
	switch {
	case err != nil:
		fail(sys, err)
		return 1
	case had:
		fmt.Fprintf(sys.Stdout, "Removed the %s key from the credential store. Revoke it in %s's key list too if it may have leaked.\n", p.Label, p.Label)
	default:
		fmt.Fprintf(sys.Stdout, "No %s key was stored.\n", p.Label)
	}
	return 0
}

func version(sys System) int {
	fmt.Fprintf(sys.Stdout, "rea-launcher %s\nNode.js %s\nREA %s\nOpenCode %s\n", sys.Version, NodeVersion, REAVersion, OpenCodeVersion)
	return 0
}
