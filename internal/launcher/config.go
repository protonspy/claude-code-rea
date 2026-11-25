package launcher

import (
	"bytes"
	"encoding/json"
	"path/filepath"
)

// Session is what one OpenCode run is configured with.
type Session struct {
	Model        string            // the qualified model, "<provider id>/<model id>"
	Node         string            // the managed Node.js executable
	REAScript    string            // REA's entry script
	REAEnv       map[string]string // the environment REA's server is started with
	Instructions string            // the AGENTS.md OpenCode is given
	SkillsDir    string            // REA's skills, the one place outside the project OpenCode may read
}

// object is a JSON object that keeps its members in the order they were added. OpenCode
// applies the last permission rule that matches, so the order of its keys is meaningful,
// and encoding/json sorts the keys of a map.
type object []member

type member struct {
	key   string
	value any
}

// MarshalJSON writes the members in order.
func (o object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		key, err := json.Marshal(m.key)
		if err != nil {
			return nil, err
		}
		value, err := json.Marshal(m.value)
		if err != nil {
			return nil, err
		}
		b.Write(key)
		b.WriteByte(':')
		b.Write(value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// OpenCodeConfig returns the OpenCode configuration for s.
func OpenCodeConfig(s Session) object {
	env := s.REAEnv
	if env == nil {
		env = map[string]string{}
	}
	return object{
		{"$schema", "https://opencode.ai/config.json"},
		{"model", s.Model},
		{"small_model", s.Model},
		{"share", "disabled"},
		{"autoupdate", false},
		{"instructions", []string{s.Instructions}},
		{"mcp", object{
			{"rea", object{
				{"type", "local"},
				{"command", []string{s.Node, s.REAScript, "mcp"}},
				{"environment", env},
				{"enabled", true},
				{"timeout", 30000},
			}},
		}},
		{"permission", permissions(s.SkillsDir)},
	}
}

// permissions returns the reviewed session rules of the opencode wiki page, then REA's.
func permissions(skillsDir string) object {
	outside := object{{"*", "deny"}}
	for _, p := range uniq(filepath.Join(skillsDir, "*"), filepath.ToSlash(filepath.Join(skillsDir, "*"))) {
		outside = append(outside, member{p, "allow"})
	}
	rules := object{
		{"bash", object{
			{"*", "ask"},
			{"git status", "allow"},
			{"git diff", "allow"},
			{"git log", "allow"},
		}},
		{"edit", "ask"},
		{"read", object{
			{"*", "allow"},
			{"*.env", "deny"},
			{"*.env.*", "deny"},
			{"*.env.example", "allow"},
			{"*.pem", "deny"},
			{"*.key", "deny"},
			{"*.npmrc", "deny"},
		}},
		{"webfetch", "deny"},
		{"websearch", "deny"},
		{"codesearch", "deny"},
		{"external_directory", outside},
	}
	// REA's tools are rea_<tool>. Every one asks, then the reviewed static ones are allowed
	// by exact name, so a tool the review has not seen asks. Last match wins.
	rules = append(rules, member{"rea_*", "ask"})
	for _, tool := range reaStaticTools {
		rules = append(rules, member{"rea_" + tool, "allow"})
	}
	return rules
}

func uniq(a, b string) []string {
	if a == b {
		return []string{a}
	}
	return []string{a, b}
}
