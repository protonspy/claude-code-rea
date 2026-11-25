# Rea launcher — design

## What changes

Serves R1–R8. The repository gains its first code: a Go module at the root, one binary, two
GitHub workflows. Why a Go launcher, a managed toolchain and the credential store, and what was
ruled out, is adr:0002-launch-opencode-with-rea-from-a-go-launcher.

```
go.mod                          module github.com/protonspy/claude-code-rea
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
user's and the project's configuration on every key it sets. `OPENCODE_CONFIG_DIR` points at
`rea-6.10.1/opencode`, where OpenCode finds REA's skills; it too is loaded after the global
directory. Neither touches a file OpenCode reads in other sessions.

```json
{
  "$schema": "https://opencode.ai/config.json",
  "model": "nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic",
  "small_model": "nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic",
  "share": "disabled",
  "instructions": ["<managed>/AGENTS.md"],
  "mcp": {
    "rea": {
      "type": "local",
      "command": ["<managed>/node-24.22.3/node", "<managed>/rea-6.10.1/node_modules/rea-agents/scripts/rea.mjs", "mcp"],
      "environment": { "GHIDRA_INSTALL_DIR": "…", "JAVA_HOME": "…" },
      "enabled": true,
      "timeout": 30000
    }
  },
  "permission": {
    "bash": { "*": "ask", "git status*": "allow", "git diff*": "allow", "git log*": "allow" },
    "webfetch": "ask",
    "websearch": "ask",
    "external_directory": "ask",
    "rea_*": "allow",
    "rea_*adb*": "ask",
    "rea_capture_*": "ask",
    "rea_observe_*": "ask",
    "rea_*web*": "ask",
    "rea_*browser*": "ask",
    "rea_*electron*": "ask",
    "rea_*runtime*": "ask",
    "rea_trace_native_*": "ask"
  }
}
```

**Order is the contract.** OpenCode applies the last permission rule that matches, and its keys
match MCP tools by name (`rea_<tool>`), so `rea_*` must come before every exception. Go's
`encoding/json` sorts map keys, which would make the order an accident of the alphabet; the
configuration is built from an ordered object type with its own `MarshalJSON`. The REA ask-list is
by name: anything that installs on, starts or reads a device (`adb`), launches or attaches to a
process, browser, page or application (`capture`, `observe`, `web`, `browser`, `electron`,
`runtime`) or drives a native UI (`trace_native_`). Static analysis of a file on disk is allowed.

The MCP `timeout` is OpenCode's limit for listing REA's tools — 5 s by default, short for a
Node.js process loading a 186 MB package; 30 s is what REA's own setup gives Codex.

## The key

Order (R2): the provider's variable → the credential store (`go-keyring`, service
`rea-launcher`, account = provider name) → sign-in (NanoGPT) or hidden prompt (NEAR AI) → saved.
OpenCode receives it only as `NANO_GPT_API_KEY` or `NEARAI_API_KEY` in its environment.

**Sign-in** is NanoGPT's shortcut PKCE flow, which needs no client registration:

1. a 64-byte random verifier and state; challenge = base64url(SHA-256(verifier));
2. a listener on `127.0.0.1:0` — the documented example port, 8787, is Headroom's here;
3. browser to `https://nano-gpt.com/auth?callback_url=http://127.0.0.1:<port>/callback&code_challenge=…&code_challenge_method=S256&scope=api.use models.read&state=…&client_name=rea-launcher`;
4. the callback must carry the same state, else it is rejected (R2.4);
5. `POST https://nano-gpt.com/api/v1/auth/keys` `{"grant_type":"authorization_code","code":…,"code_verifier":…}` returns `key`, a dedicated `sk-nano-` key that the user's NanoGPT key list names `OAuth: rea-launcher`, with the spend cap chosen on the consent screen.

## REA's engine

Setup runs the managed `rea doctor --provider ghidra --json` with the forwarded environment
(R4.2) and reads its top-level `healthy`. `--ghidra <dir>` and `--java-home <dir>` are saved first
with REA's own `rea config --set`, into REA's settings file, which every REA installation of the
user reads and an environment variable overrides.

## Risks

- **REA tool names are the ask-list.** A tool REA adds or renames under a name none of the
  patterns match is allowed without asking. Re-read the list on every REA pin change.
- **Not everything is hash-pinned.** `frida`, one of REA's dependencies, downloads a native
  binary from its install script, outside the shrinkwrap. OpenCode's platform packages carry no
  provenance, so their pinned hash is the registry's at the time of pinning.
- **The ephemeral loopback port is unverified.** NanoGPT documents fixed ports; the shortcut flow
  takes the callback per request, so any port should do. The first real sign-in is the test.
- **Two copies of REA's skill.** A user whose `~/.agents/skills` or `~/.claude/skills` already
  holds `reverse-engineer-anything` from another REA version gives OpenCode two skills of one
  name; which one OpenCode picks is not documented.
- **The tool-call deadline.** Ghidra's first analysis of a large binary can outlast an OpenCode
  tool call; REA's recovery guide, in the skill, covers a timed-out call.
