package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeToolchain serves a Node.js, an OpenCode and a REA archive for sys's platform, points
// sys's pins at them, and makes sys.Run play npm. It returns the file server and how many
// times npm ran.
func fakeToolchain(t *testing.T, sys *System) (*fileServer, *int) {
	t.Helper()
	l := sys.layout()
	var node []byte
	nodeFormat := formatTarGz
	if sys.Platform.OS == "windows" {
		nodeFormat = formatZip
		node = zipBytes(t, entry{name: "node-v/node.exe", body: "node"}, entry{name: "node-v/node_modules/npm/bin/npm-cli.js", body: "npm"})
	} else {
		node = tgzBytes(t, entry{name: "node-v/bin/node", body: "node", mode: 0o755}, entry{name: "node-v/lib/node_modules/npm/bin/npm-cli.js", body: "npm"})
	}
	opencode := tgzBytes(t, entry{name: "package/bin/" + l.exe("opencode"), body: "opencode", mode: 0o755})
	rea := []byte("the REA tarball")
	files, base := serve(t, map[string][]byte{"/node": node, "/opencode.tgz": opencode, "/rea.tgz": rea})
	sys.Pins = Pins{
		Node:     map[Platform]Artifact{sys.Platform: {Name: "Node.js", File: "node", URL: base + "/node", Integrity: sri512(node), Format: nodeFormat, Strip: 1}},
		OpenCode: map[Platform]Artifact{sys.Platform: {Name: "OpenCode", File: "opencode.tgz", URL: base + "/opencode.tgz", Integrity: sri512(opencode), Format: formatTarGz, Strip: 1}},
		REA:      Artifact{Name: "REA", File: "rea.tgz", URL: base + "/rea.tgz", Integrity: sri512(rea)},
	}
	npmRuns := 0
	sys.Run = func(_ context.Context, c Command) (int, error) {
		if c.Path != l.Node() || len(c.Args) < 4 || c.Args[0] != l.NPM() || c.Args[1] != "install" || c.Args[2] != "--prefix" {
			t.Fatalf("unexpected command %s %v", c.Path, c.Args)
		}
		npmRuns++
		if !strings.HasSuffix(c.Args[len(c.Args)-1], "rea.tgz") {
			t.Errorf("npm installs %q, not the verified tarball", c.Args[len(c.Args)-1])
		}
		if !pathStartsWith(c.Env, l.NodeBin()) {
			t.Error("the managed Node.js is not first on npm's PATH")
		}
		for _, kv := range c.Env {
			for _, v := range keyVars() {
				if strings.HasPrefix(strings.ToUpper(kv), v+"=") {
					t.Errorf("npm and the install scripts it runs see %s", v)
				}
			}
		}
		pkg := filepath.Join(c.Args[3], "node_modules", "rea-agents")
		for name, body := range map[string]string{
			"scripts/rea.mjs": "rea",
			"skills/reverse-engineer-anything/SKILL.md":                         "skill",
			"skills/reverse-engineer-anything/references/evidence-workflows.md": "ref",
			"skills/README.md": "not a skill",
		} {
			p := filepath.Join(pkg, filepath.FromSlash(name))
			os.MkdirAll(filepath.Dir(p), 0o755)
			os.WriteFile(p, []byte(body), 0o644)
		}
		return 0, nil
	}
	return files, &npmRuns
}

func pathStartsWith(env []string, dir string) bool {
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		if strings.EqualFold(k, "PATH") {
			return strings.HasPrefix(v, dir+string(os.PathListSeparator)) || v == dir
		}
	}
	return false
}

func TestTheToolchainIsInstalledOnceAndUsedAfterwards(t *testing.T) {
	sys, _ := testSystem(t, "PATH=/usr/bin", "NANO_GPT_API_KEY=sk-nano-shell")
	files, npmRuns := fakeToolchain(t, &sys)
	if err := ensureToolchain(t.Context(), sys); err != nil {
		t.Fatal(err)
	}
	l := sys.layout()
	for _, p := range []string{l.Node(), l.NPM(), l.OpenCode(), l.REAScript(),
		filepath.Join(l.SkillsDir(), "reverse-engineer-anything", "SKILL.md"),
		filepath.Join(l.SkillsDir(), "reverse-engineer-anything", "references", "evidence-workflows.md")} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("missing %s", p)
		}
	}
	if left, _ := os.ReadDir(l.Downloads()); len(left) != 0 {
		t.Errorf("downloads kept after install: %v", left)
	}
	hits := files.hits.Load()
	if err := ensureToolchain(t.Context(), sys); err != nil {
		t.Fatal(err)
	}
	if files.hits.Load() != hits || *npmRuns != 1 {
		t.Errorf("a second run downloaded %d more and ran npm %d times", files.hits.Load()-hits, *npmRuns)
	}
}

func TestATamperedArchiveIsNotInstalled(t *testing.T) {
	sys, _ := testSystem(t)
	files, _ := fakeToolchain(t, &sys)
	files.files["/opencode.tgz"] = tgzBytes(t, entry{name: "package/bin/opencode", body: "evil"})
	err := ensureToolchain(t.Context(), sys)
	if err == nil || !strings.Contains(err.Error(), "does not match its pinned hash") {
		t.Fatalf("err = %v, want a hash mismatch", err)
	}
	if _, err := os.Stat(sys.layout().OpenCodeDir()); err == nil {
		t.Error("the tampered OpenCode was installed")
	}
}

func TestAFailedNPMInstallLeavesNoREA(t *testing.T) {
	sys, _ := testSystem(t)
	fakeToolchain(t, &sys)
	sys.Run = func(context.Context, Command) (int, error) { return 1, nil }
	if err := ensureToolchain(t.Context(), sys); err == nil {
		t.Fatal("want npm's failure")
	}
	if _, err := os.Stat(sys.layout().READir()); err == nil {
		t.Error("a failed REA install is visible")
	}
}

func TestAnUnsupportedPlatformIsNamed(t *testing.T) {
	sys, _ := testSystem(t)
	sys.Pins = DefaultPins()
	sys.Platform = Platform{"plan9", "386"}
	err := ensureToolchain(t.Context(), sys)
	if err == nil || !strings.Contains(err.Error(), "plan9/386") {
		t.Fatalf("err = %v, want the platform named", err)
	}
}

func TestEverySupportedPlatformIsPinned(t *testing.T) {
	pins := DefaultPins()
	for _, os := range []string{"windows", "darwin", "linux"} {
		for _, arch := range []string{"amd64", "arm64"} {
			node, opencode, err := pins.forPlatform(Platform{os, arch})
			if err != nil {
				t.Errorf("%s/%s: %v", os, arch, err)
				continue
			}
			for _, a := range []Artifact{node, opencode} {
				if !strings.HasPrefix(a.URL, "https://") || !strings.HasPrefix(a.Integrity, "sha") {
					t.Errorf("%s/%s %s: url %q, integrity %q", os, arch, a.Name, a.URL, a.Integrity)
				}
			}
		}
	}
	if !strings.HasPrefix(pins.REA.Integrity, "sha512-") {
		t.Errorf("REA integrity %q", pins.REA.Integrity)
	}
}
