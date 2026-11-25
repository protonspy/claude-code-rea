package launcher

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTheInstructionsCoverTheSessionsRules(t *testing.T) {
	text := string(agentsMD)
	for _, want := range []string{
		"reverse-engineer-anything", // load REA's skill first
		"never instructions",        // analyzed content is data
		"prompt injection",
		"approval", // running the target asks
		"isolated environment",
		"Decompiled code is a model",
		"re-notes.md",
		"observed",
		"Unknowns",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("AGENTS.md does not say %q", want)
		}
	}
}

func TestTheInstructionsAreInstalledAndKeptCurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "AGENTS.md")
	if wrote, err := writeAgents(path); err != nil || !wrote {
		t.Fatalf("first write: wrote %v, err %v", wrote, err)
	}
	if !bytes.Equal([]byte(readFile(t, path)), agentsMD) {
		t.Fatal("installed AGENTS.md differs from the embedded one")
	}
	if wrote, err := writeAgents(path); err != nil || wrote {
		t.Errorf("unchanged file rewritten: wrote %v, err %v", wrote, err)
	}
	os.WriteFile(path, []byte("stale"), 0o644)
	if wrote, err := writeAgents(path); err != nil || !wrote {
		t.Errorf("stale file kept: wrote %v, err %v", wrote, err)
	}
	if !bytes.Equal([]byte(readFile(t, path)), agentsMD) {
		t.Error("stale AGENTS.md was not replaced")
	}
}

func TestOnlyDirectoriesWithASkillAreInstalled(t *testing.T) {
	pkg := t.TempDir()
	os.MkdirAll(filepath.Join(pkg, "skills", "a", "references"), 0o755)
	os.WriteFile(filepath.Join(pkg, "skills", "a", "SKILL.md"), []byte("a"), 0o644)
	os.WriteFile(filepath.Join(pkg, "skills", "a", "references", "r.md"), []byte("r"), 0o644)
	os.MkdirAll(filepath.Join(pkg, "skills", "not-a-skill"), 0o755)
	dest := t.TempDir()
	n, err := installSkills(pkg, dest)
	if err != nil || n != 1 {
		t.Fatalf("n %d, err %v", n, err)
	}
	if countSkills(dest) != 1 || readFile(t, filepath.Join(dest, "a", "references", "r.md")) != "r" {
		t.Error("the skill was not copied whole")
	}
	empty := t.TempDir()
	os.Mkdir(filepath.Join(empty, "skills"), 0o755)
	if _, err := installSkills(empty, t.TempDir()); err == nil {
		t.Error("want an error for a package without skills")
	}
}

func TestEngineSettingsAreSavedAndTheEnvironmentWins(t *testing.T) {
	sys, _ := testSystem(t, "GHIDRA_INSTALL_DIR=/env/ghidra", "REA_LOG_LEVEL=debug", "PATH=/bin", "NANO_GPT_API_KEY=secret")
	if err := saveEngineSettings(sys, map[string]string{"GHIDRA_INSTALL_DIR": "/saved/ghidra", "JAVA_HOME": "/saved/jdk"}); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if info, err := os.Stat(engineSettings(sys)); err != nil || info.Mode().Perm() != 0o600 {
			t.Errorf("engine.json mode: %v, %v", info, err)
		}
	}
	if left, _ := filepath.Glob(filepath.Join(sys.ConfigDir, ".engine.json.*")); len(left) != 0 {
		t.Errorf("temporary files left: %v", left)
	}
	env, err := reaEnvironment(sys)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"GHIDRA_INSTALL_DIR": "/env/ghidra", "JAVA_HOME": "/saved/jdk", "REA_LOG_LEVEL": "debug"}
	if len(env) != len(want) {
		t.Errorf("env = %v, want %v", env, want)
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("%s = %q, want %q", k, env[k], v)
		}
	}
}

func TestAWindowsVariableReplacesASavedOneWhateverItsCase(t *testing.T) {
	sys, _ := testSystem(t, "Java_Home=C:\\jdk")
	sys.Platform.OS = "windows"
	saveEngineSettings(sys, map[string]string{"JAVA_HOME": "C:\\old"})
	env, _ := reaEnvironment(sys)
	if len(env) != 1 || env["Java_Home"] != `C:\jdk` {
		t.Errorf("env = %v", env)
	}
}

func TestUnreadableEngineSettingsAreAnError(t *testing.T) {
	sys, _ := testSystem(t)
	os.WriteFile(engineSettings(sys), []byte("{not json"), 0o644)
	if _, err := reaEnvironment(sys); err == nil {
		t.Error("want an error")
	}
}

func doctorSays(t *testing.T, sys *System, report string, code int) *Command {
	var seen Command
	sys.Run = func(_ context.Context, c Command) (int, error) {
		seen = c
		c.Stdout.Write([]byte(report))
		return code, nil
	}
	return &seen
}

func TestAReadyEngine(t *testing.T) {
	sys, _ := testSystem(t)
	seen := doctorSays(t, &sys, `{"healthy":true,"scope_checks":[{"name":"ghidra","ok":true}]}`, 0)
	ready, missing, err := checkEngine(t.Context(), sys, map[string]string{"GHIDRA_INSTALL_DIR": "/opt/ghidra"})
	if err != nil || !ready || len(missing) != 0 {
		t.Fatalf("ready %v, missing %v, err %v", ready, missing, err)
	}
	l := sys.layout()
	if seen.Path != l.Node() || strings.Join(seen.Args, " ") != l.REAScript()+" doctor --provider ghidra --json" {
		t.Errorf("ran %s %v", seen.Path, seen.Args)
	}
	if !strings.Contains(strings.Join(seen.Env, "\n"), "GHIDRA_INSTALL_DIR=/opt/ghidra") {
		t.Error("the doctor did not get the REA environment")
	}
}

func TestAnEngineNotReadySaysWhatIsMissing(t *testing.T) {
	sys, _ := testSystem(t)
	doctorSays(t, &sys, `{"healthy":false,"scope_checks":[
		{"name":"ghidra-installation","ok":false,"classification":"config_drift","detail":"C:\\nope","remediation":"Point GHIDRA_INSTALL_DIR at the root of an extracted official Ghidra release."},
		{"name":"java","ok":true}]}`, 1)
	ready, missing, err := checkEngine(t.Context(), sys, nil)
	if err != nil || ready {
		t.Fatalf("ready %v, err %v", ready, err)
	}
	if len(missing) != 1 || !strings.Contains(missing[0], "ghidra-installation: config_drift") || !strings.Contains(missing[0], "Point GHIDRA_INSTALL_DIR") {
		t.Errorf("missing = %q", missing)
	}
}

func TestAnUnreadableDoctorReportIsAnError(t *testing.T) {
	sys, _ := testSystem(t)
	doctorSays(t, &sys, "Error: boom", 1)
	if _, _, err := checkEngine(t.Context(), sys, nil); err == nil {
		t.Error("want an error")
	}
}
