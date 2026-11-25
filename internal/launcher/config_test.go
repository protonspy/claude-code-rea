package launcher

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func testSession() Session {
	return Session{
		Model:        "nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic",
		Node:         "/cache/node-24.22.3/bin/node",
		REAScript:    "/cache/rea-6.10.1/node_modules/rea-agents/scripts/rea.mjs",
		REAEnv:       map[string]string{"GHIDRA_INSTALL_DIR": "/opt/ghidra"},
		Instructions: "/cache/AGENTS.md",
		SkillsDir:    "/cache/rea-6.10.1/opencode/skills",
	}
}

// rule is one permission rule in the order the configuration declares it.
type rule struct{ pattern, action string }

// orderedRules reads a JSON object's members in document order, which encoding/json's maps lose.
func orderedRules(t *testing.T, raw json.RawMessage) []rule {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("not an object: %s", raw)
	}
	var rules []rule
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			t.Fatal(err)
		}
		var action string
		if err := dec.Decode(&action); err != nil {
			t.Fatalf("rule %v: %v", key, err)
		}
		rules = append(rules, rule{key.(string), action})
	}
	return rules
}

// decide applies OpenCode's rule: the last rule whose wildcard pattern matches wins.
func decide(rules []rule, name string) string {
	action := ""
	for _, r := range rules {
		re := "^" + strings.ReplaceAll(regexp.QuoteMeta(r.pattern), `\*`, ".*") + "$"
		if regexp.MustCompile(re).MatchString(name) {
			action = r.action
		}
	}
	return action
}

// permissionRules returns the top-level permission rules with a flat action, in order, and
// the per-pattern rules of each tool that has them.
func permissionRules(t *testing.T, cfg []byte) (top []rule, nested map[string][]rule) {
	t.Helper()
	var doc struct {
		Permission json.RawMessage `json:"permission"`
	}
	if err := json.Unmarshal(cfg, &doc); err != nil {
		t.Fatal(err)
	}
	dec := json.NewDecoder(bytes.NewReader(doc.Permission))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("permission is not an object: %s", doc.Permission)
	}
	nested = map[string][]rule{}
	for dec.More() {
		key, _ := dec.Token()
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			t.Fatal(err)
		}
		if bytes.HasPrefix(raw, []byte("{")) {
			nested[key.(string)] = orderedRules(t, raw)
			continue
		}
		var action string
		if err := json.Unmarshal(raw, &action); err != nil {
			t.Fatalf("permission %v: %v", key, err)
		}
		top = append(top, rule{key.(string), action})
	}
	return top, nested
}

func marshalConfig(t *testing.T) []byte {
	t.Helper()
	cfg, err := json.Marshal(OpenCodeConfig(testSession()))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestREAToolsThatDriveSomethingAsk(t *testing.T) {
	top, _ := permissionRules(t, marshalConfig(t))
	cases := map[string]string{
		"rea_binary_session":                  "allow",
		"rea_procedure_pseudo_code":           "allow",
		"rea_xrefs":                           "allow",
		"rea_list_strings":                    "allow",
		"rea_inspect_artifact":                "allow",
		"rea_install_adb_package":             "ask",
		"rea_push_adb_file":                   "ask",
		"rea_list_adb_devices":                "ask",
		"rea_capture_process_scenario":        "ask",
		"rea_capture_native_ui_scenario":      "ask",
		"rea_observe_native_calls":            "ask",
		"rea_inspect_web_page":                "ask",
		"rea_list_browser_targets":            "ask",
		"rea_inspect_electron_page":           "ask",
		"rea_observe_javascript_runtime":      "ask",
		"rea_list_javascript_runtime_targets": "ask",
		"rea_trace_native_ui_action":          "ask",
		"rea_gdb_console":                     "ask",
		"rea_start_gdb_session":               "ask",
		"rea_start_frida_session":             "ask",
		"rea_instrument_with_frida":           "ask",
		"rea_execute_rizin_command":           "ask",
		"rea_cutter_command":                  "ask",
		"rea_start_build_computer":            "ask",
		"rea_record_build_consent":            "ask",
		"rea_a_tool_from_a_later_rea":         "ask",
		"webfetch":                            "deny",
		"websearch":                           "deny",
		"codesearch":                          "deny",
		"doom_loop":                           "deny",
	}
	for tool, want := range cases {
		if got := decide(top, tool); got != want {
			t.Errorf("%s: %q, want %q", tool, got, want)
		}
	}
}

func TestShellCommandsAskButExactGitReads(t *testing.T) {
	_, nested := permissionRules(t, marshalConfig(t))
	cases := map[string]string{
		"git status":                       "allow",
		"git diff":                         "allow",
		"git log":                          "allow",
		"git difftool":                     "ask",
		"git diff --no-index a /etc/hosts": "ask",
		"git push":                         "ask",
		"rm -rf /":                         "ask",
	}
	for cmd, want := range cases {
		if got := decide(nested["bash"], cmd); got != want {
			t.Errorf("bash %q: %q, want %q", cmd, got, want)
		}
	}
}

func TestKeyFilesAreNotRead(t *testing.T) {
	_, nested := permissionRules(t, marshalConfig(t))
	cases := map[string]string{
		"main.go":          "allow",
		"src/.env":         "deny",
		"src/.env.local":   "deny",
		"src/.env.example": "allow",
		"certs/server.pem": "deny",
		"id.key":           "deny",
		"/home/u/.npmrc":   "deny",
	}
	for path, want := range cases {
		if got := decide(nested["read"], path); got != want {
			t.Errorf("read %q: %q, want %q", path, got, want)
		}
	}
}

func TestOnlyREASkillsAreReachableOutsideTheProject(t *testing.T) {
	_, nested := permissionRules(t, marshalConfig(t))
	skills := testSession().SkillsDir
	cases := map[string]string{
		filepath.Join(skills, "reverse-engineer-anything", "references", "evidence-workflows.md"): "allow",
		"/etc/passwd":                    "deny",
		"/cache/rea-6.10.1/package.json": "deny",
	}
	for path, want := range cases {
		if got := decide(nested["external_directory"], path); got != want {
			t.Errorf("external_directory %q: %q, want %q", path, got, want)
		}
	}
}

func TestSessionSettings(t *testing.T) {
	var doc struct {
		Model        string   `json:"model"`
		SmallModel   string   `json:"small_model"`
		Share        string   `json:"share"`
		Instructions []string `json:"instructions"`
		MCP          map[string]struct {
			Type        string            `json:"type"`
			Command     []string          `json:"command"`
			Environment map[string]string `json:"environment"`
			Enabled     bool              `json:"enabled"`
			Timeout     int               `json:"timeout"`
		} `json:"mcp"`
	}
	if err := json.Unmarshal(marshalConfig(t), &doc); err != nil {
		t.Fatal(err)
	}
	s := testSession()
	if doc.Model != s.Model || doc.SmallModel != s.Model {
		t.Errorf("model %q, small model %q, want both %q", doc.Model, doc.SmallModel, s.Model)
	}
	if doc.Share != "disabled" {
		t.Errorf("share = %q, want disabled", doc.Share)
	}
	if len(doc.Instructions) != 1 || doc.Instructions[0] != s.Instructions {
		t.Errorf("instructions = %v", doc.Instructions)
	}
	rea, ok := doc.MCP["rea"]
	if !ok {
		t.Fatal("no rea MCP server")
	}
	if rea.Type != "local" || !rea.Enabled || rea.Timeout < 30000 {
		t.Errorf("rea server = %+v", rea)
	}
	if want := []string{s.Node, s.REAScript, "mcp"}; strings.Join(rea.Command, "|") != strings.Join(want, "|") {
		t.Errorf("rea command = %v, want %v", rea.Command, want)
	}
	if rea.Environment["GHIDRA_INSTALL_DIR"] != "/opt/ghidra" {
		t.Errorf("rea environment = %v", rea.Environment)
	}
}

func TestEmptyREAEnvironmentIsAnObject(t *testing.T) {
	s := testSession()
	s.REAEnv = nil
	cfg, err := json.Marshal(OpenCodeConfig(s))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(cfg, []byte(`"environment":{}`)) {
		t.Errorf("environment is not an empty object in %s", cfg)
	}
}

// reaTool is one tool of the REA snapshot, with the effects REA declares for it.
type reaTool struct {
	Name    string `json:"name"`
	Effects struct {
		ExecutesForeignCode bool `json:"executesForeignCode"`
		MutatesDevice       bool `json:"mutatesDevice"`
		ChangesUIState      bool `json:"changesUiState"`
	} `json:"effects"`
}

func reaSnapshot(t *testing.T) map[string]reaTool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "rea-"+REAVersion+"-tools.json"))
	if err != nil {
		t.Fatalf("no snapshot of REA %s's tools: %v", REAVersion, err)
	}
	var tools []reaTool
	if err := json.Unmarshal(data, &tools); err != nil {
		t.Fatal(err)
	}
	byName := map[string]reaTool{}
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	return byName
}

// engineBacked are static tools REA flags as running foreign code or changing UI state
// because the analysis engine — Ghidra, Hopper — runs its own scripts or moves its own
// cursor. The target does not run.
var engineBacked = map[string]bool{
	"analyze_function": true, "batch_decompile": true, "decompile_all": true, "goto_address": true,
	"inspect_native_api": true, "procedure_assembly": true, "procedure_pseudo_code": true,
	"read_function_instructions": true, "trace_native_values": true,
}

func TestTheStaticToolsAreREAsAndRunNothing(t *testing.T) {
	snapshot := reaSnapshot(t)
	runtime := regexp.MustCompile(`adb|frida|gdb|rizin|cutter|capture_|observe_|browser|electron_page|runtime_target|build_computer|build_capture|build_task|build_consent|ui_action|webmcp|web_page`)
	for _, name := range reaStaticTools {
		tool, ok := snapshot[name]
		switch {
		case !ok:
			t.Errorf("%s is not a REA %s tool", name, REAVersion)
		case runtime.MatchString(name):
			t.Errorf("%s drives something at runtime and must ask", name)
		case (tool.Effects.ExecutesForeignCode || tool.Effects.MutatesDevice || tool.Effects.ChangesUIState) && !engineBacked[name]:
			t.Errorf("%s declares runtime effects %+v and is not a reviewed engine-backed tool", name, tool.Effects)
		}
	}
}

func TestEveryREAToolOutsideTheReviewAsks(t *testing.T) {
	top, _ := permissionRules(t, marshalConfig(t))
	static := map[string]bool{}
	for _, name := range reaStaticTools {
		static[name] = true
	}
	for name := range reaSnapshot(t) {
		want := "ask"
		if static[name] {
			want = "allow"
		}
		if got := decide(top, "rea_"+name); got != want {
			t.Errorf("rea_%s: %q, want %q", name, got, want)
		}
	}
}

func TestEditsAskButREASkillsAreNeverWritten(t *testing.T) {
	_, nested := permissionRules(t, marshalConfig(t))
	skill := filepath.Join(testSession().SkillsDir, "reverse-engineer-anything", "SKILL.md")
	cases := map[string]string{
		"main.go":               "ask",
		skill:                   "deny",
		filepath.ToSlash(skill): "deny",
	}
	for path, want := range cases {
		if got := decide(nested["edit"], path); got != want {
			t.Errorf("edit %q: %q, want %q", path, got, want)
		}
	}
}
