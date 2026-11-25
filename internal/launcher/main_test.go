package launcher

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// opencodeRun is what the launcher ran OpenCode with.
type opencodeRun struct {
	ran  bool
	args []string
	env  map[string][]string
}

// withOpenCode makes sys.Run answer OpenCode with code and REA's doctor with doctor, and
// hand anything else to the npm of fakeToolchain.
func withOpenCode(t *testing.T, sys *System, code int, doctor string) *opencodeRun {
	t.Helper()
	npm := sys.Run
	run := &opencodeRun{env: map[string][]string{}}
	l := sys.layout()
	sys.Run = func(ctx context.Context, c Command) (int, error) {
		switch {
		case c.Path == l.OpenCode():
			run.ran = true
			run.args = c.Args
			for _, kv := range c.Env {
				k, v, _ := strings.Cut(kv, "=")
				run.env[k] = append(run.env[k], v)
			}
			return code, nil
		case len(c.Args) > 1 && c.Args[1] == "doctor":
			c.Stdout.Write([]byte(doctor))
			return 0, nil
		}
		return npm(ctx, c)
	}
	return run
}

func TestLaunchRunsOpenCodeWithTheSessionAndTheKey(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=sk-nano-secret", "OPENCODE_CONFIG_CONTENT={}", "OPENCODE_ENABLE_EXA=1", "PATH=/bin")
	fakeToolchain(t, &sys)
	run := withOpenCode(t, &sys, 7, "")
	ignored := false
	sys.IgnoreInterrupts = func() { ignored = true }

	if code := Main([]string{"--", "run", "explain main"}, sys); code != 7 {
		t.Fatalf("exit %d, want OpenCode's 7; stderr:\n%s", code, io.stderr)
	}
	if strings.Join(run.args, "|") != "run|--auto|explain main" {
		t.Errorf("OpenCode args = %q", run.args)
	}
	if got := run.env["NANO_GPT_API_KEY"]; len(got) != 1 || got[0] != "sk-nano-secret" {
		t.Errorf("key variable = %q", got)
	}
	cfg := run.env["OPENCODE_CONFIG_CONTENT"]
	if len(cfg) != 1 {
		t.Fatalf("OPENCODE_CONFIG_CONTENT = %q", cfg)
	}
	var doc struct{ Model, Instructions any }
	if err := json.Unmarshal([]byte(cfg[0]), &doc); err != nil || doc.Model != "nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic:thinking" {
		t.Errorf("config %s: %v", cfg[0], err)
	}
	l := sys.layout()
	if got := run.env["OPENCODE_CONFIG_DIR"]; len(got) != 1 || got[0] != l.ConfigDir() {
		t.Errorf("OPENCODE_CONFIG_DIR = %q", got)
	}
	if !ignored {
		t.Error("interrupts were not left to OpenCode")
	}
	for _, name := range []string{"OPENCODE_DISABLE_PROJECT_CONFIG", "OPENCODE_DISABLE_CLAUDE_CODE", "OPENCODE_DISABLE_EXTERNAL_SKILLS", "OPENCODE_DISABLE_AUTOUPDATE", "OPENCODE_DISABLE_LSP_DOWNLOAD"} {
		if got := run.env[name]; len(got) != 1 || got[0] != "1" {
			t.Errorf("%s = %q, want 1: the session directory may hold the target", name, got)
		}
	}
	if _, ok := run.env["OPENCODE_ENABLE_EXA"]; ok {
		t.Error("OPENCODE_ENABLE_EXA reached OpenCode, which turns its web search back on")
	}
	var rea struct {
		MCP struct {
			REA struct {
				Environment map[string]*string `json:"environment"`
			} `json:"rea"`
		} `json:"mcp"`
	}
	json.Unmarshal([]byte(cfg[0]), &rea)
	for _, v := range keyVars() {
		if got := rea.MCP.REA.Environment[v]; got == nil || *got != "" {
			t.Errorf("REA's environment does not blank %s, so a target REA runs inherits the key", v)
		}
	}
	if !strings.Contains(io.stderr.String(), "NanoGPT") || !strings.Contains(io.stderr.String(), "DeepSeek-V4.1-Flash-1017-heretic") {
		t.Errorf("stderr does not name the provider and model:\n%s", io.stderr)
	}
	if string(agentsMD) != readFile(t, l.Agents()) {
		t.Error("AGENTS.md was not installed")
	}
	for where, text := range map[string]string{"stdout": io.stdout.String(), "stderr": io.stderr.String(), "config": cfg[0], "args": strings.Join(run.args, " ")} {
		if strings.Contains(text, "sk-nano-secret") {
			t.Errorf("the key appears in %s", where)
		}
	}
}

func TestOpenCodeArgumentsPassWithoutASeparator(t *testing.T) {
	sys, _ := testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	run := withOpenCode(t, &sys, 0, "")
	Main([]string{"--model", "lyber/deepseek-v4.1-flash-1017-abliterated", "run", "hi"}, sys)
	if strings.Join(run.args, "|") != "run|--auto|hi" {
		t.Errorf("OpenCode args = %q", run.args)
	}
	if !strings.Contains(run.env["OPENCODE_CONFIG_CONTENT"][0], `"nano-gpt/lyber/deepseek-v4.1-flash-1017-abliterated"`) {
		t.Error("the given model is not the session's model")
	}
}

func TestPrintConfigNeedsNeitherKeyNorToolchain(t *testing.T) {
	sys, io := testSystem(t)
	if code := Main([]string{"--provider", "nearai", "--print-config"}, sys); code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, io.stderr)
	}
	var doc struct{ Model string }
	if err := json.Unmarshal(io.stdout.Bytes(), &doc); err != nil || doc.Model != "nearai/deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated" {
		t.Errorf("printed %s: %v", io.stdout, err)
	}
	if _, err := os.Stat(sys.layout().Downloads()); err == nil {
		t.Error("print-config downloaded something")
	}
}

func TestBadArgumentsExitTwo(t *testing.T) {
	sys, io := testSystem(t)
	if code := Main([]string{"--provider", "openrouter"}, sys); code != 2 || !strings.Contains(io.stderr.String(), "nanogpt") {
		t.Errorf("unknown provider: exit %d, stderr %q", code, io.stderr)
	}
	if code := Main([]string{"--no-such-flag"}, sys); code != 2 {
		t.Errorf("unknown flag: exit %d", code)
	}
	if code := Main([]string{"-h"}, sys); code != 0 {
		t.Errorf("-h: exit %d", code)
	}
}

func TestLaunchWithoutAKeyStartsNothing(t *testing.T) {
	sys, io := testSystem(t)
	fakeToolchain(t, &sys)
	run := withOpenCode(t, &sys, 0, "")
	if code := Main([]string{"--provider", "nearai"}, sys); code != 1 {
		t.Errorf("exit %d", code)
	}
	if run.ran {
		t.Error("OpenCode started without a key")
	}
	if !strings.Contains(io.stderr.String(), "NEARAI_API_KEY") {
		t.Errorf("stderr does not name the variable:\n%s", io.stderr)
	}
}

func TestVersionNamesEveryPin(t *testing.T) {
	sys, io := testSystem(t)
	if Main([]string{"version"}, sys) != 0 {
		t.Fatal("version failed")
	}
	for _, want := range []string{"rea-launcher test", NodeVersion, REAVersion, OpenCodeVersion} {
		if !strings.Contains(io.stdout.String(), want) {
			t.Errorf("version output lacks %q:\n%s", want, io.stdout)
		}
	}
	io.stdout.Reset()
	if Main([]string{"help"}, sys) != 0 || !strings.Contains(io.stdout.String(), "setup") {
		t.Error("help does not show the usage")
	}
}

func TestSetupReportsEveryStep(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	withOpenCode(t, &sys, 0, `{"healthy":true}`)
	ghidra := fakeGhidra(t, "12.2", "25")
	if code := Main([]string{"setup", "--ghidra", ghidra}, sys); code != 0 {
		t.Fatalf("exit %d:\n%s%s", code, io.stdout, io.stderr)
	}
	out := io.stdout.String()
	for _, want := range []string{"toolchain", "1 REA skill", "key", "from NANO_GPT_API_KEY", "Ghidra is ready"} {
		if !strings.Contains(out, want) {
			t.Errorf("setup output lacks %q:\n%s", want, out)
		}
	}
	settings, _ := loadEngineSettings(sys)
	if settings["GHIDRA_INSTALL_DIR"] != ghidra {
		t.Errorf("saved settings = %v", settings)
	}
}

func TestSetupWithAnEngineNotReadyStillSucceeds(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	withOpenCode(t, &sys, 0, `{"healthy":false,"scope_checks":[{"name":"ghidra","ok":false,"classification":"missing_analysis_engine"}]}`)
	if code := Main([]string{"setup"}, sys); code != 0 {
		t.Fatalf("exit %d:\n%s", code, io.stdout)
	}
	if out := io.stdout.String(); !strings.Contains(out, "WARN") || !strings.Contains(out, "missing_analysis_engine") || !strings.Contains(out, "--ghidra") {
		t.Errorf("setup output:\n%s", out)
	}
}

func TestSetupFailsOnAFailedStep(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	sys.Run = func(context.Context, Command) (int, error) { return 1, nil }
	if code := Main([]string{"setup"}, sys); code != 1 || !strings.Contains(io.stdout.String(), "FAIL") {
		t.Errorf("exit %d:\n%s", code, io.stdout)
	}

	sys, io = testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	withOpenCode(t, &sys, 0, `{"healthy":true}`)
	if code := Main([]string{"setup", "--java-home", filepath.Join(t.TempDir(), "missing")}, sys); code != 1 {
		t.Errorf("a missing --java-home: exit %d:\n%s", code, io.stdout)
	}

	sys, io = testSystem(t)
	fakeToolchain(t, &sys)
	withOpenCode(t, &sys, 0, `{"healthy":true}`)
	if code := Main([]string{"setup", "--provider", "nearai"}, sys); code != 1 || !strings.Contains(io.stdout.String(), "FAIL  key") {
		t.Errorf("no key: exit %d:\n%s", code, io.stdout)
	}
}

func TestLogoutForgetsTheKey(t *testing.T) {
	sys, io := testSystem(t)
	io.store.keys["nanogpt"] = "k"
	if Main([]string{"logout"}, sys) != 0 || !strings.Contains(io.stdout.String(), "Removed") {
		t.Errorf("logout output:\n%s", io.stdout)
	}
	io.stdout.Reset()
	if Main([]string{"logout"}, sys) != 0 || !strings.Contains(io.stdout.String(), "No NanoGPT key") {
		t.Errorf("second logout output:\n%s", io.stdout)
	}
}

func TestRunCommandReturnsTheExitCode(t *testing.T) {
	if os.Getenv("REA_LAUNCHER_HELPER") != "" {
		return
	}
	code, err := runCommand(t.Context(), Command{
		Path: os.Args[0],
		Args: []string{"-test.run=TestHelperProcess"},
		Env:  append(os.Environ(), "REA_LAUNCHER_HELPER=3"),
	})
	if err != nil || code != 3 {
		t.Errorf("code %d, err %v", code, err)
	}
	if _, err := runCommand(t.Context(), Command{Path: filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Error("want an error for a command that cannot start")
	}
}

// TestHelperProcess is the child of TestRunCommandReturnsTheExitCode.
func TestHelperProcess(t *testing.T) {
	code, err := strconv.Atoi(os.Getenv("REA_LAUNCHER_HELPER"))
	if err != nil {
		return
	}
	os.Exit(code)
}

func TestAutomaticApprovalGoesWhereOpenCodeAcceptsIt(t *testing.T) {
	project := t.TempDir()
	cases := []struct {
		in, want []string
		auto     bool
	}{
		{nil, []string{"--auto"}, true},
		{[]string{"-c"}, []string{"--auto", "-c"}, true},
		{[]string{project}, []string{"--auto", project}, true},
		{[]string{"run", "explain"}, []string{"run", "--auto", "explain"}, true},
		{[]string{"run", "--auto", "x"}, []string{"run", "--auto", "x"}, true},
		{[]string{"debug", "paths"}, []string{"debug", "paths"}, false},
		{[]string{"mcp", "list"}, []string{"mcp", "list"}, false},
		{[]string{"--auto"}, []string{"--auto"}, true},
		{[]string{"run", "--auto"}, []string{"run", "--auto"}, true},
		{[]string{"run", "explain --auto"}, []string{"run", "--auto", "explain --auto"}, true},
		{[]string{"debug", "paths", "--auto"}, []string{"debug", "paths", "--auto"}, false},
		{[]string{"no-such-dir"}, []string{"no-such-dir"}, false},
	}
	for _, c := range cases {
		got, auto := withAutoApproval(c.in)
		if strings.Join(got, "|") != strings.Join(c.want, "|") || auto != c.auto {
			t.Errorf("withAutoApproval(%q) = %q, %v; want %q, %v", c.in, got, auto, c.want, c.auto)
		}
	}
}

func TestAutomaticApprovalWarnsAndAskRestoresAsking(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	run := withOpenCode(t, &sys, 0, "")
	Main(nil, sys)
	if strings.Join(run.args, "|") != "--auto" || !strings.Contains(io.stderr.String(), "approvals are automatic") || !strings.Contains(io.stderr.String(), "--ask") {
		t.Errorf("default: args %q, stderr:\n%s", run.args, io.stderr)
	}

	io.stderr.Reset()
	Main([]string{"--ask"}, sys)
	if len(run.args) != 0 || strings.Contains(io.stderr.String(), "approvals are automatic") {
		t.Errorf("--ask: args %q, stderr:\n%s", run.args, io.stderr)
	}

	io.stderr.Reset()
	Main([]string{"--", "debug", "paths"}, sys)
	if strings.Join(run.args, "|") != "debug|paths" || strings.Contains(io.stderr.String(), "approvals are automatic") {
		t.Errorf("debug: args %q, stderr:\n%s", run.args, io.stderr)
	}
}

func TestAskWarnsWhenAutoIsPassedThrough(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	withOpenCode(t, &sys, 0, "")
	Main([]string{"--ask", "--", "--auto"}, sys)
	if !strings.Contains(io.stderr.String(), "despite --ask") {
		t.Errorf("stderr:\n%s", io.stderr)
	}
}
