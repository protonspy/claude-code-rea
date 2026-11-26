# rea-launcher

One command that starts [OpenCode](https://opencode.ai) with an abliterated model and
[REA](https://www.npmjs.com/package/rea-agents) — an MCP server for reverse engineering — attached,
on a toolchain the launcher installs itself at pinned versions and hashes.

```powershell
rea-launcher -- run "explain how C:\samples\app.exe checks its licence"
```

An **abliterated model** is one whose refusal direction was removed from its weights, so it answers
what the original would decline. The launcher runs one from a provider:

| Provider | Default model | Why |
|---|---|---|
| [NanoGPT](docs/wiki/pages/nanogpt.md) (default) | `pqhaz/DeepSeek-V4.1-Flash-1017-heretic:thinking` | cheap, tool calling, sign-in with a spend cap |
| [NEAR AI](docs/wiki/pages/near-ai.md) (`--provider nearai`) | `deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated` | served inside an attested TEE |

Why OpenCode and NanoGPT: [ADR 0001](docs/adr/0001-use-opencode-with-nanogpt-for-abliterated-models.md).
Why a Go launcher: [ADR 0002](docs/adr/0002-launch-opencode-with-rea-from-a-go-launcher.md).

## Security: read this before the first session

**Sessions approve automatically by default** ([ADR 0003](docs/adr/0003-approve-launcher-sessions-automatically.md)).
The model is abliterated and reads untrusted targets. An instruction planted in a binary can make
it run any shell command, read any file you can, start the sample through Frida or gdb, or read
the provider key from OpenCode's environment or the credential store and send it out with `curl` —
the configuration's denials bind OpenCode's own file and web tools, not the shell. What such a
session leaves behind (a Git hook, a build file, a change to the launcher's directory) outlasts it,
and a later session, `--ask` included, runs on top of it: after an automatic session that read
untrusted content, treat the working directory and the launcher's directory as tainted. The
launcher prints this warning too, but OpenCode's interface covers it as soon as it starts.

So:

- run sessions in a disposable VM with no secrets;
- use a provider key with a low spend cap, or a small prepaid balance;
- start the session in an empty working directory, not in the folder holding the target;
- or pass `--ask`, read each command in full, and reject anything that sends data out or reads
  credentials.

## Getting it

Download the binary for your platform and `SHA256SUMS` from
[Releases](https://github.com/protonspy/opencode-rea/releases) — Windows, macOS and Linux, amd64
and arm64 — then check the hash and the build provenance:

```powershell
(Get-FileHash .\rea-launcher-v0.3.0-windows-amd64.exe -Algorithm SHA256).Hash.ToLower()
Select-String 'windows-amd64' .\SHA256SUMS
gh attestation verify .\rea-launcher-v0.3.0-windows-amd64.exe --repo protonspy/opencode-rea
```

Or build it from source with Go (version in `go.mod`):

```powershell
go build -o rea-launcher.exe ./cmd/rea-launcher
```

## Setting up

```powershell
rea-launcher setup --ghidra 'C:\Tools\ghidra_12.2_PUBLIC' --java-home 'C:\Tools\jdk-25.0.5.1+2'
```

Setup:

1. installs Node.js, REA and OpenCode into the launcher's cache directory, each archive checked
   against the hash compiled into the launcher — nothing on the `PATH` is used or changed;
2. installs REA's skills and the launcher's [`AGENTS.md`](internal/launcher/agents.md), for
   launcher sessions only;
3. gets the provider key — `NANO_GPT_API_KEY` if set, else the stored key, else NanoGPT's sign-in
   in the browser, where you set the spend cap — and stores it in the operating system's
   credential store (Windows Credential Manager, macOS Keychain, Secret Service);
4. checks the reverse-engineering engine: REA 6.10.1 needs Ghidra 12.2 or a later 12.x on JDK 25.
   Without one, REA's JavaScript, .NET and archive tools still work.

The first plain `rea-launcher` on a new machine does steps 1–3 by itself.

## Running a session

```powershell
rea-launcher                                  # NanoGPT, its default model, auto-approve
rea-launcher --provider nearai                # NEAR AI, in its TEE
rea-launcher --model lyber/deepseek-v4.1-flash-1017-abliterated
rea-launcher --ask                            # ask before each shell command, edit and REA runtime tool
rea-launcher --print-config                   # print the session configuration, start nothing
rea-launcher -- run "<prompt>"                # arguments after -- go to OpenCode unchanged
rea-launcher logout                           # delete the stored key
rea-launcher version                          # the launcher and its pinned toolchain
```

A session loads nothing from its working directory — no project `opencode.json`, `.opencode/`
plugin, `CLAUDE.md` or outside skill — and has session sharing, code search and self-update off.

The full reference, including every permission rule and how to bump a pinned version, is the
[rea-launcher wiki page](docs/wiki/pages/rea-launcher.md).

## Connecting Claude Code instead

The research behind this repository also covers pointing Claude Code itself at other models:

- [Claude Code's backend](docs/wiki/pages/claude-code-backend.md) — the variables that redirect it,
  and what that does to the subscription login
- [A LiteLLM gateway](docs/wiki/pages/litellm-gateway.md) — Claude Code on an OpenAI-compatible API
- [A local model as a tool](docs/wiki/pages/local-model-as-tool.md) — keep Claude on the
  subscription and call an Ollama model through MCP

Never put a provider key in a user-wide `settings.json` `env` block: it switches every Claude Code
session off the subscription.

## Repository

| Path | What |
|---|---|
| `cmd/rea-launcher`, `internal/launcher` | the launcher, a Go module |
| `specs/rea-launcher/` | its requirements, design and tasks |
| `docs/` | wiki, ADRs, glossary, stack and notes — [start here](docs/wiki/index.md) |
| `plans/` | smaller pieces of work |

Development commands:

```bash
go build ./...                    # build
bash scripts/check-format.sh      # gofmt finds nothing
go vet ./...                      # lint
bash scripts/check-test.sh        # tests with coverage
```

CI runs those four on Linux and Windows for every pull request. Pushing a `v*` tag builds the
release binaries with `SHA256SUMS` and a build provenance attestation.
