package launcher

import (
	"fmt"
	"path/filepath"
)

// The pinned versions. Changing one means changing its hashes below, from the source each
// comment names, and re-reading REA's tool names against the permission rules.
const (
	NodeVersion     = "24.22.3"
	REAVersion      = "6.10.1"
	OpenCodeVersion = "1.19.5"
)

// Pins is what the launcher downloads, per platform.
type Pins struct {
	Node     map[Platform]Artifact
	OpenCode map[Platform]Artifact
	REA      Artifact
}

// DefaultPins returns the pinned downloads.
func DefaultPins() Pins {
	// Node.js: https://nodejs.org/dist/v24.22.3/SHASUMS256.txt, as SRI.
	node := func(platform, ext, format, sri string) Artifact {
		file := fmt.Sprintf("node-v%s-%s.%s", NodeVersion, platform, ext)
		return Artifact{Name: "Node.js " + NodeVersion, File: file, URL: "https://nodejs.org/dist/v" + NodeVersion + "/" + file, Integrity: sri, Format: format, Strip: 1}
	}
	// OpenCode: each platform package's dist.integrity in the npm registry.
	opencode := func(platform, sri string) Artifact {
		pkg := "opencode-" + platform
		file := fmt.Sprintf("%s-%s.tgz", pkg, OpenCodeVersion)
		return Artifact{Name: "OpenCode " + OpenCodeVersion, File: file, URL: "https://registry.npmjs.org/" + pkg + "/-/" + file, Integrity: sri, Format: formatTarGz, Strip: 1}
	}
	return Pins{
		Node: map[Platform]Artifact{
			{"windows", "amd64"}: node("win-x64", "zip", formatZip, "sha256-5hKvmfYmq0FeoiWva85clx/rRIy1yuJQuxw/pQxffZ8="),
			{"windows", "arm64"}: node("win-arm64", "zip", formatZip, "sha256-s8x0opWznU8nMnHpb1jAbdKzV1fKEqvsZMfQWYtDTxQ="),
			{"linux", "amd64"}:   node("linux-x64", "tar.gz", formatTarGz, "sha256-Bh7D4wYhFXBbVrUfqcwzCrUNNLaWk4fU5LGlDGgJrMI="),
			{"linux", "arm64"}:   node("linux-arm64", "tar.gz", formatTarGz, "sha256-JhryryUN/X6fJc8VHRsnE6Ct+PEKmtEWxJR/aGJ6/+g="),
			{"darwin", "amd64"}:  node("darwin-x64", "tar.gz", formatTarGz, "sha256-l3YUrwUEyBBF5/2FWW28oOX5UblmgVFtra/tzKjER/c="),
			{"darwin", "arm64"}:  node("darwin-arm64", "tar.gz", formatTarGz, "sha256-/24X1IdLdtJyISuQjMAg1ugV+6Wmx+qqBq3XJYQDB/0="),
		},
		OpenCode: map[Platform]Artifact{
			{"windows", "amd64"}: opencode("windows-x64", "sha512-xCVbrA8A/qQTAdBIKyhK5tx3/mcq18yF82Mi8phry0MJb2631nnnpxF9m6FFfWAVHyer4qFGrO7hbWXesHQ1Gg=="),
			{"windows", "arm64"}: opencode("windows-arm64", "sha512-aySF/MDji5KO3I5yl9Ec4g8kW/QIx4Jj5FXwBhh/FrDu0w6L8HAOGH7cyjFb3t8TW7pxRkH3Ei3rfkWAE7cScw=="),
			{"linux", "amd64"}:   opencode("linux-x64", "sha512-i1qh7VooLGML9xX6ud93y0JA8j/1aPeDYRKjsmYXrJX1TjIWHgBBMkEbEJPZthpWbsJg//XfBgR6Kb7+NPaJiQ=="),
			{"linux", "arm64"}:   opencode("linux-arm64", "sha512-u2HFRI6v1nNMVKiV34An8XrGBxJeZ+RCg2tYu3SqlGODKqKsr5RoEbaX1tnsfGb+rETh9AJVX9kS8dSZXPufsA=="),
			{"darwin", "amd64"}:  opencode("darwin-x64", "sha512-+DO7IMPTCrX12UJXnL4pL3ckcwR6BR6f6tA9QIlVQ5nZx66T5U7FRMJT4n5rsOnJxLM/pZ2vfX1G8vrnvXO8FA=="),
			{"darwin", "arm64"}:  opencode("darwin-arm64", "sha512-0MbSbKkmwVHtgP31GJyyECe6QVy4T7W4Wp7s2TsmXHTPPyf01ne3dWSX97X9dkNK28Lc6kt+zMPQUP/9GgDLZg=="),
		},
		// REA: the registry's dist.integrity, which matches the SLSA provenance subject built
		// from morluto/rea at 8bb01b5, the commit the rea-agents-6.10.1 tag names.
		REA: Artifact{
			Name:      "REA " + REAVersion,
			File:      "rea-agents-" + REAVersion + ".tgz",
			URL:       "https://registry.npmjs.org/rea-agents/-/rea-agents-" + REAVersion + ".tgz",
			Integrity: "sha512-3AWrfFEXppvC98FWhZR2Tzulnqs5xJQrSKmAub6+QME/I58Pr3h8pHqqwM30z2DvlnIx/8Hsu8woMngfi4SNyg==",
		},
	}
}

// forPlatform returns the platform's Node.js and OpenCode downloads.
func (p Pins) forPlatform(pl Platform) (node, opencode Artifact, err error) {
	node, okNode := p.Node[pl]
	opencode, okOpenCode := p.OpenCode[pl]
	if !okNode || !okOpenCode {
		return Artifact{}, Artifact{}, fmt.Errorf("unsupported platform %s; rea-launcher runs on windows, darwin and linux, on amd64 or arm64", pl)
	}
	return node, opencode, nil
}

// Layout is where each managed component lives under the managed directory.
type Layout struct {
	Root string
	OS   string
}

func (l Layout) exe(name string) string {
	if l.OS == "windows" {
		return name + ".exe"
	}
	return name
}

func (l Layout) NodeDir() string { return filepath.Join(l.Root, "node-"+NodeVersion) }

// Node is the managed Node.js executable.
func (l Layout) Node() string {
	if l.OS == "windows" {
		return filepath.Join(l.NodeDir(), "node.exe")
	}
	return filepath.Join(l.NodeDir(), "bin", "node")
}

// NodeBin is the directory holding the Node.js executable, put first on npm's PATH.
func (l Layout) NodeBin() string { return filepath.Dir(l.Node()) }

// NPM is npm's entry script inside the managed Node.js.
func (l Layout) NPM() string {
	if l.OS == "windows" {
		return filepath.Join(l.NodeDir(), "node_modules", "npm", "bin", "npm-cli.js")
	}
	return filepath.Join(l.NodeDir(), "lib", "node_modules", "npm", "bin", "npm-cli.js")
}

func (l Layout) READir() string     { return filepath.Join(l.Root, "rea-"+REAVersion) }
func (l Layout) REAPackage() string { return filepath.Join(l.READir(), "node_modules", "rea-agents") }
func (l Layout) REAScript() string  { return filepath.Join(l.REAPackage(), "scripts", "rea.mjs") }

// ConfigDir is the OpenCode configuration directory of launcher sessions.
func (l Layout) ConfigDir() string { return filepath.Join(l.READir(), "opencode") }
func (l Layout) SkillsDir() string { return filepath.Join(l.ConfigDir(), "skills") }

func (l Layout) OpenCodeDir() string { return filepath.Join(l.Root, "opencode-"+OpenCodeVersion) }
func (l Layout) OpenCode() string    { return filepath.Join(l.OpenCodeDir(), "bin", l.exe("opencode")) }

func (l Layout) Agents() string    { return filepath.Join(l.Root, "AGENTS.md") }
func (l Layout) Downloads() string { return filepath.Join(l.Root, "downloads") }
