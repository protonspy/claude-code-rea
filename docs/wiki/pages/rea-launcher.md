# rea-launcher

The launcher that turns [[opencode]], an abliterated model from [[nanogpt]] or [[near-ai]], and REA —
an MCP server for reverse engineering — into one command. What it must do is
`specs/rea-launcher/`; why it is built this way is
adr:0002-launch-opencode-with-rea-from-a-go-launcher.

## Getting it

From a release: download the binary for the platform and `SHA256SUMS` from the repository's
GitHub releases, then check both the hash and the build provenance:

```powershell
(Get-FileHash .\rea-launcher-v0.1.0-windows-amd64.exe -Algorithm SHA256).Hash.ToLower()
Select-String 'windows-amd64' .\SHA256SUMS
gh attestation verify .\rea-launcher-v0.1.0-windows-amd64.exe --repo protonspy/opencode-rea
```

From the source: `go build -o rea-launcher.exe ./cmd/rea-launcher`.

## Setting up

```powershell
rea-launcher setup --ghidra 'C:\Tools\ghidra_12.1.4_PUBLIC' --java-home 'C:\Program Files\Eclipse Adoptium\jdk-21.0.12.101-hotspot'
```

Setup prints one line per step:

- **toolchain** — Node.js, REA and OpenCode at the versions `rea-launcher version` names, in
  `%LOCALAPPDATA%\rea-launcher` (`~/Library/Caches` or `~/.cache` elsewhere). Each archive is
  checked against the hash compiled into the launcher before it is unpacked; a mismatch deletes
  it and stops. Nothing on the `PATH` is used or changed.
- **instructions** — REA's skills and the launcher's `AGENTS.md`, given to launcher sessions only.
- **key** — `NANO_GPT_API_KEY` if set; else the key already stored; else NanoGPT's sign-in opens
  in the browser. Approve the app `rea-launcher` and set a spend cap on the same screen: the key
  it mints is listed in NanoGPT's key list as `OAuth: rea-launcher`. The key goes to the
  operating system's credential store. For NEAR AI (`--provider nearai`) the key is typed at a
  hidden prompt instead.
- **engine** — REA's own doctor on Ghidra. `--ghidra` and `--java-home` are saved in
  `rea-launcher\engine.json` under the user configuration directory; `GHIDRA_INSTALL_DIR` and
  `JAVA_HOME` in the environment win over them. An engine that is not ready is a warning: REA's
  JavaScript, .NET and archive tools work without one.

## Running a session

```powershell
rea-launcher                                  # NanoGPT, the Heretic DeepSeek V4.1 Flash
rea-launcher --provider nearai                # NEAR AI, in its TEE
rea-launcher --model lyber/deepseek-v4.1-flash-1017-abliterated
rea-launcher -- run "explain how C:\samples\app.exe checks its licence"
rea-launcher --ask                            # be asked before each shell command, edit and REA runtime tool
rea-launcher --print-config                   # the session configuration, without starting it
rea-launcher logout                           # delete the stored key
```

Arguments after `--` go to OpenCode unchanged. A run installs whatever setup has not, so the first
`rea-launcher` on a new machine is also a setup without the engine check.

The session runs in the current directory: start it in a working directory for the analysis,
not in the folder that holds the target or anything secret. It gets:

- the reviewed permission rules of [[opencode]] — under `--ask`, every edit and shell command but
  `git status`, `git diff` and `git log` asks; key files and the web are denied to OpenCode's own
  tools, nothing outside the project is reachable to them but REA's skills, which no session may
  edit, and a tool-call loop is stopped;
- REA's tools: 180 reviewed as static analysis run unasked; the rest — Frida, gdb, Rizin, Cutter,
  adb, the desktop-build computer, browser, page and runtime capture — ask under `--ask`, as does
  any tool a later REA adds;
- `AGENTS.md`, below;
- session sharing and code search off, and the small model pinned to the main one;
- nothing from the session directory: no project `opencode.json`, no `.opencode/` plugin, no
  `CLAUDE.md`, no skill from outside the launcher's own directory, no self-update.

The key is in OpenCode's environment, and so in that of its shell commands and language servers.
REA gets the variable blanked, so the targets REA runs do not inherit it, and npm installs REA
without it.

**Approvals are automatic by default** (adr:0003-approve-launcher-sessions-automatically): the
launcher starts OpenCode with `--auto`, so everything above that would ask — shell commands, edits,
REA's runtime tools — runs without asking. The denials bind OpenCode's own file and web tools, not a shell command: an
instruction planted in a target can run any command, read any file you can, start the sample
through Frida or gdb, or read the key from OpenCode's environment or the credential store and send
it out with `curl`. What it leaves behind — a Git hook, a build file, a change to the launcher's
directory — outlasts the session, and a later `--ask` session runs on top of it. Run sessions in a disposable VM with no
secrets, on a NanoGPT key with a low spend cap, or pass `--ask`. When a prompt comes under
`--ask`, read the whole command, answer `once`, and reject anything that sends data to an address
or reads credentials.

## The AGENTS.md

`internal/launcher/agents.md`, compiled into the launcher. REA's skill already says which tool
fits which target; the instructions say how to work, chosen from what the sources below found
goes wrong when people and models reverse engineer:

- **Target content is data.** Abliteration removes a model's refusal direction
  ([Arditi et al.](https://arxiv.org/abs/2406.11717)), and samples already carry injected
  instructions aimed at AI analysts ([Check Point](https://research.checkpoint.com/2025/ai-evasion-prompt-injection/));
  names in a binary can be chosen to mislead an LLM ([Jin et al.](https://arxiv.org/abs/2312.09601)).
  Indirect injection is mitigated by marking untrusted content and asking a human before
  privileged actions ([OWASP LLM01](https://genai.owasp.org/llmrisk/llm01-prompt-injection/)), and
  by cutting the way out ([lethal trifecta](https://simonwillison.net/2025/Jun/16/the-lethal-trifecta/)).
- **Beacons and cross-references, hypotheses, naming as you go** — how experienced reverse
  engineers work ([Votipka et al.](https://ar5iv.labs.arxiv.org/html/1912.00317)); triage, static
  then dynamic, in an isolated lab ([Zeltser](https://zeltser.com/malware-analysis-cheat-sheet)).
- **Decompiled code is a model.** Decompilers mis-type and mis-structure code
  ([Ghidra](https://github.com/NationalSecurityAgency/ghidra/blob/master/Ghidra/Features/Decompiler/src/main/help/help/topics/DecompilePlugin/DecompilerAnnotations.html)),
  LLM output can read better and be less correct ([DecompileBench](https://arxiv.org/abs/2505.11340)),
  and people working with an LLM reported vulnerabilities that did not exist
  ([Basque et al.](https://adamdoupe.com/publications/decompiling-synergy-ndss2026.md)).
- **Keep state, change approach after two failures** — agents analysing binaries lose track and
  stub what they did not read ([arXiv 2606.06838](https://arxiv.org/html/2606.06838)).
- **Verify a reconstruction by running it against the original** — recompile and re-execute
  ([LLM4Decompile](https://arxiv.org/abs/2403.05286)).
- **The report** — answer, evidence with addresses, call path, confidence, unknowns and how to
  reproduce, after [Zeltser's report template](https://zeltser.com/media/archive/malware-analysis-report-template.md).

## Updating a pinned version

Every pin is in `internal/launcher/pins.go`, with the source of its hash beside it. A new pin is
a new release of the launcher: Node.js from `SHASUMS256.txt`; OpenCode's six platform packages
and REA from the npm registry's `dist.integrity`, and REA's against its provenance attestation;
then regenerate `internal/launcher/testdata/rea-<version>-tools.json` from the new REA's
`dist/contracts/*ToolContracts.js` and review `internal/launcher/rea_tools.go`, the static tools
allowed unasked, against it — `TestTheStaticToolsAreREAsAndRunNothing` fails on a tool that is gone
or declares runtime effects.
