package launcher

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTheLayoutFollowsEachPlatformsArchive(t *testing.T) {
	win := Layout{Root: "R", OS: "windows"}
	if win.Node() != filepath.Join("R", "node-"+NodeVersion, "node.exe") ||
		win.NPM() != filepath.Join("R", "node-"+NodeVersion, "node_modules", "npm", "bin", "npm-cli.js") ||
		win.OpenCode() != filepath.Join("R", "opencode-"+OpenCodeVersion, "bin", "opencode.exe") {
		t.Errorf("windows layout: %s, %s, %s", win.Node(), win.NPM(), win.OpenCode())
	}
	unix := Layout{Root: "R", OS: "linux"}
	if unix.Node() != filepath.Join("R", "node-"+NodeVersion, "bin", "node") ||
		unix.NPM() != filepath.Join("R", "node-"+NodeVersion, "lib", "node_modules", "npm", "bin", "npm-cli.js") ||
		unix.OpenCode() != filepath.Join("R", "opencode-"+OpenCodeVersion, "bin", "opencode") {
		t.Errorf("linux layout: %s, %s, %s", unix.Node(), unix.NPM(), unix.OpenCode())
	}
}

func TestTheDefaultSystemUsesTheUserDirectoriesAndThePins(t *testing.T) {
	sys, err := DefaultSystem("v1")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(sys.Root) != "rea-launcher" || filepath.Base(sys.ConfigDir) != "rea-launcher" {
		t.Errorf("root %q, config %q", sys.Root, sys.ConfigDir)
	}
	if sys.Pins.REA.Integrity != DefaultPins().REA.Integrity || sys.AuthURL != nanoGPTAuthURL || sys.TokenURL != nanoGPTTokenURL {
		t.Error("the default system does not use the pins and NanoGPT's endpoints")
	}
	if sys.Version != "v1" || sys.Run == nil || sys.Store == nil || sys.OpenURL == nil || sys.SignInTimeout != signInTimeout {
		t.Errorf("incomplete default system: %+v", sys)
	}
}

func exchangeWith(t *testing.T, handler http.HandlerFunc) (string, error) {
	srv := httptest.NewServer(handler)
	defer srv.Close()
	o := OAuth{TokenURL: srv.URL, Client: http.DefaultClient}
	return o.exchange(t.Context(), "c", "v")
}

func TestTheExchangeAcceptsAnAccessTokenAlone(t *testing.T) {
	key, err := exchangeWith(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"access_token":"sk-nano-a","token_type":"Bearer"}`))
	})
	if err != nil || key != "sk-nano-a" {
		t.Errorf("key %q, err %v", key, err)
	}
}

func TestAnExchangeWithoutAKeyFails(t *testing.T) {
	for name, body := range map[string]string{"no key": `{"token_type":"Bearer"}`, "not json": `<html>`} {
		_, err := exchangeWith(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(body)) })
		if err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if _, err := (OAuth{TokenURL: "http://127.0.0.1:1", Client: http.DefaultClient}).exchange(t.Context(), "c", "v"); err == nil {
		t.Error("unreachable endpoint: want an error")
	}
}

func TestACallbackWithoutACodeFails(t *testing.T) {
	if res := readCallback(map[string][]string{"state": {"s"}}); res.err == nil {
		t.Error("want an error")
	}
}

func TestATamperedToolchainStopsTheLaunch(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k")
	files, _ := fakeToolchain(t, &sys)
	run := withOpenCode(t, &sys, 0, "")
	files.files["/rea.tgz"] = []byte("tampered")
	if code := Main(nil, sys); code != 1 {
		t.Errorf("exit %d", code)
	}
	if run.ran || !strings.Contains(io.stderr.String(), "pinned hash") {
		t.Errorf("ran %v, stderr:\n%s", run.ran, io.stderr)
	}
}

func TestBrokenEngineSettingsStopTheLaunchAndFailSetup(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	withOpenCode(t, &sys, 0, `{"healthy":true}`)
	os.WriteFile(engineSettings(sys), []byte("{"), 0o644)
	if code := Main(nil, sys); code != 1 || !strings.Contains(io.stderr.String(), "engine.json") {
		t.Errorf("launch: exit %d, stderr:\n%s", code, io.stderr)
	}
	if code := Main([]string{"setup"}, sys); code != 1 || !strings.Contains(io.stdout.String(), "FAIL  engine") {
		t.Errorf("setup: exit %d:\n%s", code, io.stdout)
	}
	if code := Main([]string{"setup", "--ghidra", t.TempDir()}, sys); code != 1 {
		t.Errorf("setup saving into broken settings: exit %d", code)
	}
}

func TestADoctorThatCannotRunFailsSetup(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	npm := sys.Run
	sys.Run = func(ctx context.Context, c Command) (int, error) {
		if len(c.Args) > 1 && c.Args[1] == "doctor" {
			return -1, errors.New("no node")
		}
		return npm(ctx, c)
	}
	if code := Main([]string{"setup"}, sys); code != 1 || !strings.Contains(io.stdout.String(), "FAIL  engine") {
		t.Errorf("exit %d:\n%s", code, io.stdout)
	}
}

func TestAJavaHomeThatIsAFileIsRefused(t *testing.T) {
	file := filepath.Join(t.TempDir(), "jdk")
	os.WriteFile(file, nil, 0o644)
	if _, err := existingDir(file); err == nil {
		t.Error("want an error for a file")
	}
}

func TestOpenCodeThatCannotStartFailsTheLaunch(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	npm := sys.Run
	l := sys.layout()
	sys.Run = func(ctx context.Context, c Command) (int, error) {
		if c.Path == l.OpenCode() {
			return -1, errors.New("exec format error")
		}
		return npm(ctx, c)
	}
	if code := Main(nil, sys); code != 1 || !strings.Contains(io.stderr.String(), "run OpenCode") {
		t.Errorf("exit %d, stderr:\n%s", code, io.stderr)
	}
}

func TestLogoutReportsAStoreFailure(t *testing.T) {
	sys, io := testSystem(t)
	io.store.err = errors.New("locked")
	if code := Main([]string{"logout"}, sys); code != 1 {
		t.Errorf("exit %d", code)
	}
	if code := Main([]string{"logout", "--provider", "x"}, sys); code != 2 {
		t.Errorf("unknown provider: exit %d", code)
	}
}

func TestASaveFailureKeepsTheKeyForTheRun(t *testing.T) {
	sys, io := testSystem(t)
	sys.Store = failingSet{io.store}
	sys.Prompt = func(string) (string, error) { return "near-key", nil }
	key, source, err := resolveKey(t.Context(), nearai(t), sys)
	if err != nil || key != "near-key" || !strings.Contains(source, "this run only") {
		t.Errorf("key %q, source %q, err %v", key, source, err)
	}
	sys.Prompt = func(string) (string, error) { return "", errors.New("closed") }
	if _, _, err := resolveKey(t.Context(), nearai(t), sys); err == nil {
		t.Error("a prompt that fails: want an error")
	}
}

// failingSet is a store that cannot save.
type failingSet struct{ *memStore }

func (failingSet) Set(string, string) error { return errors.New("read-only") }

func TestPublishReportsAFillThatCannotStart(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0o644)
	if err := publish(filepath.Join(file, "sub", "dest"), func(string) error { return nil }); err == nil {
		t.Error("want an error when the parent cannot be made")
	}
}

func TestAnAGENTSmdThatCannotBeWrittenStopsTheLaunch(t *testing.T) {
	sys, io := testSystem(t, "NANO_GPT_API_KEY=k")
	fakeToolchain(t, &sys)
	run := withOpenCode(t, &sys, 0, "")
	os.MkdirAll(filepath.Join(sys.layout().Agents(), "in-the-way"), 0o755)
	if code := Main(nil, sys); code != 1 || !strings.Contains(io.stderr.String(), "AGENTS.md") {
		t.Errorf("exit %d, stderr:\n%s", code, io.stderr)
	}
	if run.ran {
		t.Error("OpenCode started without its instructions")
	}
}

func TestExtractionIntoNothingOrOverADirectoryFails(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	if err := extract(writeTemp(t, "a.zip", zipBytes(t, entry{name: "top/a", body: "x"})), formatZip, missing, 1); err == nil {
		t.Error("extract into a missing destination: want an error")
	}
	cases := map[string][]byte{
		formatZip:   zipBytes(t, entry{name: "top/x/"}, entry{name: "top/x", body: "file over a directory"}),
		formatTarGz: tgzBytes(t, entry{name: "package/x/", dir: true}, entry{name: "package/x", body: "file over a directory"}),
	}
	for format, data := range cases {
		if err := extract(writeTemp(t, "a", data), format, t.TempDir(), 1); err == nil {
			t.Errorf("%s: a file entry over a directory entry: want an error", format)
		}
	}
	if err := extract(writeTemp(t, "a", []byte("not an archive")), formatTarGz, t.TempDir(), 1); err == nil {
		t.Error("a corrupt tar.gz: want an error")
	}
	if err := extract(writeTemp(t, "a", []byte("not an archive")), formatZip, t.TempDir(), 1); err == nil {
		t.Error("a corrupt zip: want an error")
	}
}

func TestSkillsWithNowhereToGoFail(t *testing.T) {
	pkg := t.TempDir()
	os.MkdirAll(filepath.Join(pkg, "skills", "a"), 0o755)
	os.WriteFile(filepath.Join(pkg, "skills", "a", "SKILL.md"), []byte("a"), 0o644)
	blocked := filepath.Join(t.TempDir(), "file")
	os.WriteFile(blocked, nil, 0o644)
	if _, err := installSkills(pkg, filepath.Join(blocked, "skills")); err == nil {
		t.Error("want an error when the skills directory cannot be made")
	}
	if _, err := installSkills(filepath.Join(t.TempDir(), "no-package"), t.TempDir()); err == nil {
		t.Error("want an error for a package that is not there")
	}
}
