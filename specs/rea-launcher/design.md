# Rea launcher — design

## What changes

Serves R1–R8. The repository gains its first code: a Go module at the root, one binary, two
GitHub workflows. Why a Go launcher, a managed toolchain and the credential store, and what was
ruled out, is adr:0002-launch-opencode-with-rea-from-a-go-launcher.

```
go.mod                          module github.com/protonspy/opencode-rea
cmd/rea-launcher/main.go        os.Exit(launcher.Main(args, launcher.DefaultSystem()))
internal/launcher/
  main.go       subcommands and flags: launch (default) · setup · logout · version
  system.go     System: every effect the launcher has — files, network, processes, store, terminal
  provider.go   the provider table and model resolution                       R1
  key.go        key resolution: variable, store, sign-in or prompt, save       R2
  oauth.go      NanoGPT OAuth PKCE                                              R2.3, R2.4
  pins.go       the pinned versions, URLs and hashes, per platform             R3
  toolchain.go  download, verify, unpack, publish                              R3
  archive.go    zip and tar.gz extraction, contained to the destination        R3.4
  rea.go        REA environment, skills, doctor and settings                   R4
  agents.md     the reverse-engineering instructions, embedded                 R4.7
  config.go     the OpenCode configuration as ordered JSON                     R1.4, R4, R5
  opencode.go   running OpenCode                                               R6
.github/workflows/ci.yml        R8.1
.github/workflows/release.yml   R8.2
scripts/check-format.sh, scripts/check-test.sh   the format and test gates
```

`System` is the seam that makes the launcher testable without a network, a keyring or a real
OpenCode: tests build one with an `httptest` server for downloads and sign-in, a map for the
store, and a command runner that records what it was asked to run.

## The managed directory

`os.UserCacheDir()/rea-launcher` — `%LOCALAPPDATA%\rea-launcher` on Windows. Everything in it
can be downloaded again, which is why it is the cache and not the config directory.

```
node-24.22.3/        Node.js, the top directory of its archive stripped
rea-6.10.1/          npm --prefix install of the verified REA tarball
  node_modules/rea-agents/scripts/rea.mjs
  opencode/skills/<name>/          REA's skills, copied from the package    R4.3
opencode-1.19.5/bin/opencode[.exe] the platform package's binary
AGENTS.md            the embedded instructions, rewritten when they differ   R4.7
downloads/           verified archives
```

**Install is publish-by-rename** (R3.4): each component is unpacked into `.tmp-<name>-<random>`
beside its final path and renamed into place only when complete; a final path that exists is an
installed component (R3.5), and a temporary one left by a crash is never read. The REA install
runs the managed npm — `node npm-cli.js install --prefix <tmp> --omit=dev --no-audit --no-fund
<verified tgz>` — with the managed Node first on `PATH`, so dependencies resolve through the
`npm-shrinkwrap.json` inside the verified tarball, each with its own integrity hash.

**Hashes are SRI strings** — `sha256-<base64>` for Node.js, taken from its `SHASUMS256.txt`;
`sha512-<base64>` for the npm tarballs, the registry's `dist.integrity`. One format, one check.
For REA the pinned hash was also matched against its SLSA provenance, whose source is
`morluto/rea` at the commit the `rea-agents-6.10.1` tag names.

## The OpenCode configuration

Passed in `OPENCODE_CONFIG_CONTENT`, which OpenCode loads after every file, so it overrides the
user's configuration on every key it sets. `OPENCODE_CONFIG_DIR` points at
`rea-6.10.1/opencode`, where OpenCode finds REA's skills; it too is loaded after the global
directory. Neither touches a file OpenCode reads in other sessions.

The session directory may hold the target, so nothing in it is configuration (R5.5): OpenCode
merges a project `opencode.json` — a planted MCP server, a loosened rule — and runs `.opencode/`
plugins with the key in their environment. OpenCode 1.19.5 is started with
`OPENCODE_DISABLE_PROJECT_CONFIG`, `OPENCODE_DISABLE_CLAUDE_CODE`,
`OPENCODE_DISABLE_EXTERNAL_SKILLS` (which also drops stale copies of REA's skill in
`~/.agents/skills` and `~/.claude/skills`), `OPENCODE_DISABLE_AUTOUPDATE` and
`OPENCODE_DISABLE_LSP_DOWNLOAD` set, and without `OPENCODE_ENABLE_EXA`. Checked against the real
binary: with them a planted `opencode.json` is absent from `opencode debug config`, and REA's
skill still loads from the launcher's directory.

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic",
  "small_model": "nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic",
  "share": "disabled",
  "autoupdate": false,
  "instructions": ["<managed>/AGENTS.md"],
  "mcp": {
    "rea": {
      "type": "local",
      "command": ["<managed>/node-24.22.3/node", "<managed>/rea-6.10.1/node_modules/rea-agents/scripts/rea.mjs", "mcp"],
      "environment": { "GHIDRA_INSTALL_DIR": "…", "JAVA_HOME": "…", "NANO_GPT_API_KEY": "", "NEARAI_API_KEY": "" },
      "enabled": true,
      "timeout": 30000
    }
  },
  "permission": {
    "bash": { "*": "ask", "git status": "allow", "git diff": "allow", "git log": "allow" },
    "edit": "ask",
    "read": { "*": "allow", "*.env": "deny", "*.env.*": "deny", "*.env.example": "allow",
              "*.pem": "deny", "*.key": "deny", "*.npmrc": "deny" },
    "webfetch": "deny",
    "websearch": "deny",
    "codesearch": "deny",
    "external_directory": { "*": "deny", "<managed>/rea-6.10.1/opencode/skills/*": "allow" },
    "rea_*": "ask",
    "rea_address_name": "allow",
    "…": "allow",
    "rea_xrefs": "allow"
  }
}
```

**Order is the contract.** OpenCode applies the last permission rule that matches, and its keys
match MCP tools by name (`rea_<tool>`), so `rea_*` must come before every exception. Go's
`encoding/json` sorts map keys, which would make the order an accident of the alphabet; the
configuration is built from an ordered object type with its own `MarshalJSON`.

**REA's tools ask unless reviewed** (R5.3). `rea_*` asks, then 180 of REA 6.10.1's 261 tools are
allowed by exact name (`internal/launcher/rea_tools.go`): analysis of files on disk and of the
analysis session. The other 81 — adb, Frida, gdb, Rizin, Cutter, the desktop-build computer,
browser, page, Electron and runtime capture and observation — ask, and so does any tool a later REA
adds. REA's declared effects are too coarse to decide alone: Hopper-backed decompilation declares
UI changes and network access, Ghidra-backed analysis declares foreign code. The test reads a
snapshot of REA's catalog (`testdata/rea-6.10.1-tools.json`) and fails if an allowed tool is not
REA's, belongs to a runtime family, or declares runtime effects without being a reviewed
engine-backed tool.

The rest are the reviewed session rules of [[opencode]], plus `codesearch` denied, with one
exception added:
`external_directory` lets OpenCode read REA's skills, whose `references/` the skill tells the
model to read, and nothing else outside the project.

The MCP `timeout` is OpenCode's limit for listing REA's tools — 5 s by default, short for a
Node.js process loading a 186 MB package; 30 s is what REA's own setup gives Codex.

## The key

Order (R2): the provider's variable → the credential store (`go-keyring`, service
`rea-launcher`, account = provider name) → sign-in (NanoGPT) or hidden prompt (NEAR AI) → saved.
OpenCode receives it only as `NANO_GPT_API_KEY` or `NEARAI_API_KEY` in its environment. OpenCode
starts REA with its own environment merged with the configured one, and REA starts targets from its
own, so the configured one blanks both variables; npm runs without them (R2.13). OpenCode's shell
tool and language servers still inherit the key.

**Sign-in** is NanoGPT's shortcut PKCE flow, which needs no client registration:

1. a 64-byte random verifier and state; challenge = base64url(SHA-256(verifier));
2. a listener on `127.0.0.1:47913`, or any free port when that one is busy — the documented
   example port, 8787, is Headroom's here. NanoGPT turns each callback URL into its own client
   (`ngpt_callback_<hash>`, a different one per port, seen against the live service), so a fixed
   port lets a later sign-in meet the same client and grant instead of minting another key;
3. browser to `https://nano-gpt.com/auth?callback_url=http://127.0.0.1:<port>/callback&code_challenge=…&code_challenge_method=S256&scope=api.use models.read&state=…&client_name=rea-launcher`;
4. only a GET to `127.0.0.1:<port>` carrying the same state counts; anything else — a page probing
   the port, a rebound host name — gets 400 and the sign-in keeps waiting (R2.14);
5. `POST https://nano-gpt.com/api/v1/auth/keys` `{"grant_type":"authorization_code","code":…,"code_verifier":…}` returns `key`, a dedicated `sk-nano-` key that the user's NanoGPT key list names `OAuth: rea-launcher`, with the spend cap chosen on the consent screen.

## REA's engine

Setup runs the managed `rea doctor --provider ghidra --json` with the session's REA environment
and reads its top-level `healthy`; when false, each failing entry of `scope_checks` is reported
as `name: classification — detail`, with REA's `remediation`.

`--ghidra <dir>` and `--java-home <dir>` are saved as `GHIDRA_INSTALL_DIR` and `JAVA_HOME` in
`os.UserConfigDir()/rea-launcher/engine.json` — the launcher's own settings, not REA's: REA 6.10.1
has no `rea config` command, and REA's documented settings file would also reach the REA that
Claude Code runs. The REA environment of a session is those settings, overridden by the same
variables, `GHIDRA_*` and `REA_*`, from the launcher's environment (R4.2, R4.5). With neither,
REA searches the usual install locations itself.

## Risks

- **The allow-list is a review.** A REA pin change needs a new catalog snapshot and a new review
  of `rea_tools.go`; until then a tool REA renamed asks rather than runs.
- **Not everything is hash-pinned.** `frida`, one of REA's dependencies, downloads a native
  binary from its install script, outside the shrinkwrap. OpenCode's platform packages carry no
  provenance, so their pinned hash is the registry's at the time of pinning.
- **A sign-in on a fallback port is a new app to NanoGPT.** Its consent screen comes up again and
  the key list may gain another `OAuth: rea-launcher` key; delete the old one there.
- **Git in a hostile tree.** `git status`, `git diff` and `git log` run unasked, and a target's
  `.git/config` can name a pager or fsmonitor command. OpenCode runs git in the project itself
  too, so the defence is the one the wiki gives: start the session in a working directory, not in
  the target's.
- **The tool-call deadline.** Ghidra's first analysis of a large binary can outlast an OpenCode
  tool call; REA's recovery guide, in the skill, covers a timed-out call.
