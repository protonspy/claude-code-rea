package launcher

import (
	"errors"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func nanogpt(t *testing.T) Provider {
	p, err := LookupProvider("nanogpt")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func nearai(t *testing.T) Provider {
	p, err := LookupProvider("nearai")
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTheKeyVariableWins(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=sk-nano-env")
	io.store.keys["nanogpt"] = "sk-nano-stored"
	key, source, err := resolveKey(t.Context(), nanogpt(t), sys)
	if err != nil || key != "sk-nano-env" {
		t.Fatalf("key %q, err %v", key, err)
	}
	if !strings.Contains(source, "NANO_GPT_API_KEY") {
		t.Errorf("source = %q", source)
	}
	if io.store.sets != 0 {
		t.Error("a key from the environment was saved")
	}
}

func TestTheStoredKeyIsUsedWhenTheVariableIsUnset(t *testing.T) {
	sys, io := testSystem(t)
	io.store.keys["nanogpt"] = "sk-nano-stored"
	key, _, err := resolveKey(t.Context(), nanogpt(t), sys)
	if err != nil || key != "sk-nano-stored" {
		t.Fatalf("key %q, err %v", key, err)
	}
}

func TestNanoGPTSignsInAndSavesTheKey(t *testing.T) {
	f := &fakeNanoGPT{code: "c1", key: "sk-nano-oauth"}
	srv := httptest.NewServer(f)
	defer srv.Close()
	sys, io := testSystem(t)
	sys.TokenURL = srv.URL
	var opened url.URL
	sys.OpenURL = browser(t, f, &opened, url.Values{"code": {"c1"}})
	key, source, err := resolveKey(t.Context(), nanogpt(t), sys)
	if err != nil || key != "sk-nano-oauth" {
		t.Fatalf("key %q, err %v", key, err)
	}
	if io.store.keys["nanogpt"] != "sk-nano-oauth" {
		t.Errorf("store = %v, want the signed-in key saved", io.store.keys)
	}
	if !strings.Contains(source, "saved") {
		t.Errorf("source = %q", source)
	}
}

func TestNEARAIPromptsAndSavesTheKey(t *testing.T) {
	sys, io := testSystem(t)
	var label string
	sys.Prompt = func(l string) (string, error) { label = l; return "  near-key \n", nil }
	key, _, err := resolveKey(t.Context(), nearai(t), sys)
	if err != nil || key != "near-key" {
		t.Fatalf("key %q, err %v", key, err)
	}
	if !strings.Contains(label, "NEARAI_API_KEY") {
		t.Errorf("prompt %q does not name the variable", label)
	}
	if io.store.keys["nearai"] != "near-key" {
		t.Errorf("store = %v", io.store.keys)
	}
}

func TestNEARAIWithoutATerminalNamesTheVariable(t *testing.T) {
	sys, _ := testSystem(t)
	_, _, err := resolveKey(t.Context(), nearai(t), sys)
	if err == nil || !strings.Contains(err.Error(), "NEARAI_API_KEY") {
		t.Fatalf("err = %v, want one naming NEARAI_API_KEY", err)
	}
}

func TestAnEmptyKeyAtThePromptIsAnError(t *testing.T) {
	sys, io := testSystem(t)
	sys.Prompt = func(string) (string, error) { return "   ", nil }
	if _, _, err := resolveKey(t.Context(), nearai(t), sys); err == nil {
		t.Fatal("want an error for an empty key")
	}
	if io.store.sets != 0 {
		t.Error("an empty key was saved")
	}
}

func TestAnUnreachableStoreWarnsAndKeepsTheKeyForThisRun(t *testing.T) {
	sys, io := testSystem(t)
	io.store.err = errors.New("no secret service")
	sys.Prompt = func(string) (string, error) { return "near-key", nil }
	key, source, err := resolveKey(t.Context(), nearai(t), sys)
	if err != nil || key != "near-key" {
		t.Fatalf("key %q, err %v", key, err)
	}
	if !strings.Contains(io.stderr.String(), "credential store") || !strings.Contains(source, "this run only") {
		t.Errorf("stderr %q, source %q", io.stderr.String(), source)
	}
	if strings.Contains(io.stderr.String(), "near-key") {
		t.Error("the warning shows the key")
	}
}

func TestLogoutDeletesTheStoredKey(t *testing.T) {
	sys, io := testSystem(t)
	io.store.keys["nanogpt"] = "sk-nano-stored"
	had, err := forgetKey(nanogpt(t), sys)
	if err != nil || !had {
		t.Fatalf("had %v, err %v", had, err)
	}
	if _, ok := io.store.keys["nanogpt"]; ok {
		t.Error("the key is still stored")
	}
	if had, err := forgetKey(nanogpt(t), sys); err != nil || had {
		t.Errorf("second logout: had %v, err %v", had, err)
	}
	io.store.err = errors.New("locked")
	if _, err := forgetKey(nanogpt(t), sys); err == nil {
		t.Error("want the store's error")
	}
}

func TestVariableNamesIgnoreCaseOnWindowsOnly(t *testing.T) {
	sys, _ := testSystem(t, "nano_gpt_api_key=lower")
	sys.Platform.OS = "windows"
	if got := sys.getenv("NANO_GPT_API_KEY"); got != "lower" {
		t.Errorf("windows getenv = %q", got)
	}
	sys.Platform.OS = "linux"
	if got := sys.getenv("NANO_GPT_API_KEY"); got != "" {
		t.Errorf("linux getenv = %q", got)
	}
}
