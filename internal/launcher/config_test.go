package launcher

import (
	"bytes"
	"encoding/json"
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

// permissionRules returns the top-level permission rules with a flat action, in order,
// and the raw bash rules.
func permissionRules(t *testing.T, cfg []byte) (top []rule, bash []rule) {
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
	for dec.More() {
		key, _ := dec.Token()
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			t.Fatal(err)
		}
		if key == "bash" {
			bash = orderedRules(t, raw)
			continue
		}
		var action string
		if err := json.Unmarshal(raw, &action); err != nil {
			t.Fatalf("permission %v: %v", key, err)
		}
		top = append(top, rule{key.(string), action})
	}
	return top, bash
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
		"rea_binary_session":             "allow",
		"rea_procedure_pseudo_code":      "allow",
		"rea_xrefs":                      "allow",
		"rea_list_strings":               "allow",
		"rea_inspect_artifact":           "allow",
		"rea_install_adb_package":        "ask",
		"rea_push_adb_file":              "ask",
		"rea_list_adb_devices":           "ask",
		"rea_capture_process_scenario":   "ask",
		"rea_capture_native_ui_scenario": "ask",
		"rea_observe_native_calls":       "ask",
		"rea_inspect_web_page":           "ask",
		"rea_list_browser_targets":       "ask",
		"rea_inspect_electron_page":      "ask",
		"rea_observe_javascript_runtime": "ask",
		"rea_list_javascript_runtime_targets": "ask",
		"rea_trace_native_ui_action":     "ask",
		"webfetch":                       "ask",
		"websearch":                      "ask",
		"external_directory":             "ask",
	}
	for tool, want := range cases {
		if got := decide(top, tool); got != want {
			t.Errorf("%s: %q, want %q", tool, got, want)
		}
	}
}

func TestShellCommandsAskButReadingGitState(t *testing.T) {
	_, bash := permissionRules(t, marshalConfig(t))
	cases := map[string]string{
		"git status --short": "allow",
		"git diff HEAD":      "allow",
		"git log -3":         "allow",
		"git push":           "ask",
		"rm -rf /":           "ask",
		"curl https://x":     "ask",
	}
	for cmd, want := range cases {
		if got := decide(bash, cmd); got != want {
			t.Errorf("bash %q: %q, want %q", cmd, got, want)
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
