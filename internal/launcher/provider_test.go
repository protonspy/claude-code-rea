package launcher

import (
	"strings"
	"testing"
)

func TestDefaultProviderRunsTheHereticModel(t *testing.T) {
	p, err := LookupProvider(DefaultProvider)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := p.QualifiedModel(""), "nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic"; got != want {
		t.Errorf("default model = %q, want %q", got, want)
	}
	if p.KeyVar != "NANO_GPT_API_KEY" || !p.SignIn {
		t.Errorf("NanoGPT key variable %q, sign-in %v", p.KeyVar, p.SignIn)
	}
}

func TestNEARAIDefaults(t *testing.T) {
	p, err := LookupProvider("nearai")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := p.QualifiedModel(""), "nearai/deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated"; got != want {
		t.Errorf("default model = %q, want %q", got, want)
	}
	if p.KeyVar != "NEARAI_API_KEY" {
		t.Errorf("key variable = %q, want NEARAI_API_KEY", p.KeyVar)
	}
	if p.SignIn {
		t.Error("NEAR AI has no sign-in")
	}
}

func TestGivenModelReplacesTheDefault(t *testing.T) {
	p, _ := LookupProvider("nanogpt")
	for _, model := range []string{"lyber/deepseek-v4.1-flash-1017-abliterated", "nano-gpt/lyber/deepseek-v4.1-flash-1017-abliterated"} {
		if got, want := p.QualifiedModel(model), "nano-gpt/lyber/deepseek-v4.1-flash-1017-abliterated"; got != want {
			t.Errorf("QualifiedModel(%q) = %q, want %q", model, got, want)
		}
	}
}

func TestUnknownProviderNamesTheKnownOnes(t *testing.T) {
	_, err := LookupProvider("openrouter")
	if err == nil {
		t.Fatal("want an error for an unknown provider")
	}
	for _, name := range []string{"openrouter", "nanogpt", "nearai"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not name %q", err, name)
		}
	}
}
