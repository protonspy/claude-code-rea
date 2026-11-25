---
autonomy: auto
ci: wait
branch: feat/thinking-default
delivery: in-progress
pr: 7
---

# Rea launcher — requirements

## Purpose

`rea-launcher` is one Go binary that makes a reverse-engineering session with an abliterated
model a single command. It installs the managed toolchain — Node.js, REA and OpenCode at pinned
versions, each checked against a hash compiled into the binary — signs in to NanoGPT, keeps the
provider key in the operating system's credential store, finds REA's analysis engine, installs REA's skills and a reverse-engineering `AGENTS.md`, and
starts OpenCode with REA attached and the agent gated, configured for that one run. The harness, the
provider and the default model are the ones
adr:0001-use-opencode-with-nanogpt-for-abliterated-models chose; how the launcher wires and
installs them is adr:0002-launch-opencode-with-rea-from-a-go-launcher.

## R1 · Provider and model

- **R1.1** The launcher shall start OpenCode with `nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic:thinking` as the model when no provider and no model are given.
- **R1.2** Where the NEAR AI provider is selected, the launcher shall use `nearai/deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated` as the default model and `NEARAI_API_KEY` as the key variable.
- **R1.3** Where a model is given, the launcher shall use that model of the selected provider in place of the provider's default model.
- **R1.4** The launcher shall set OpenCode's small model to the same model as its main model.
- **R1.5** If the selected provider is not one the launcher knows, then the launcher shall exit with an error naming the known providers, without starting OpenCode.

## R2 · The provider key

- **R2.1** When the provider's key variable is set, the launcher shall use its value as the key.
- **R2.2** When the provider's key variable is unset and the credential store holds a key for the provider, the launcher shall use the stored key.
- **R2.3** When NanoGPT is the provider and no key is available, the launcher shall obtain one through NanoGPT's OAuth PKCE sign-in, receiving the callback on a loopback address, opening the sign-in page in the browser and printing its URL to standard error.
- **R2.4** If a callback carrying the sign-in's state reports an error, the code exchange fails, or no such callback arrives within five minutes, then the launcher shall exit with an error, without starting OpenCode.
- **R2.14** If a request reaches the sign-in's callback without the state sent, with another method than GET or under another host name than the loopback address, then the launcher shall refuse it and keep waiting.
- **R2.5** When NEAR AI is the provider, no key is available and standard input is a terminal, the launcher shall prompt for the key without echoing it.
- **R2.6** If NEAR AI is the provider, no key is available and standard input is not a terminal, then the launcher shall exit with an error naming the variable, without starting OpenCode.
- **R2.7** If the key entered at the prompt is empty, then the launcher shall exit with an error, without starting OpenCode.
- **R2.8** When the launcher obtains a key through sign-in or the prompt, the launcher shall save it in the credential store under the provider's name.
- **R2.9** When asked to log out, the launcher shall delete the provider's key from the credential store.
- **R2.10** The launcher shall pass the key to OpenCode through the provider's key variable and through no other channel.
- **R2.11** The launcher shall not write the key to a file, to a command line, to the OpenCode configuration or to its own output.
- **R2.12** If the credential store cannot be reached, then the launcher shall warn on standard error and use the key for the current run only.
- **R2.13** The launcher shall keep every provider's key variable out of the environment of npm and blank it in the environment of REA's server, so neither the install scripts nor a target REA runs can read a key.

## R3 · The managed toolchain

- **R3.1** The launcher shall install Node.js 24.22.3, REA 6.10.1 and OpenCode 1.19.5 under its own directory in the user's cache, and use those copies rather than any on the `PATH`.
- **R3.2** The launcher shall check every downloaded archive against the hash compiled into it for that version and platform before unpacking it.
- **R3.3** If a downloaded archive's hash differs from the compiled hash, then the launcher shall delete the archive and exit with an error naming the expected and the actual hash, without installing it.
- **R3.4** The launcher shall make an installed copy visible under its final path only after its archive was verified and fully unpacked, so an interrupted install leaves nothing a later run takes for installed.
- **R3.5** When a component of the managed toolchain is already installed, the launcher shall use it without downloading it again.
- **R3.6** If the operating system and architecture are not one of Windows, macOS and Linux on x64 or arm64, then the launcher shall exit with an error naming the platform.

## R4 · REA

- **R4.1** The launcher shall register REA with OpenCode as a local MCP server named `rea`, run as the managed Node.js on the managed REA's script with the `mcp` argument.
- **R4.2** When `JAVA_HOME` or a variable whose name starts with `GHIDRA_` or `REA_` is set and setup saved no value for it, the launcher shall pass it to the REA server's environment.
- **R4.3** The launcher shall make every skill shipped in the managed REA package available to OpenCode in its sessions only.
- **R4.4** When setting up, the launcher shall check REA's Ghidra engine with REA's own doctor and report whether it is ready.
- **R4.5** Where a Ghidra directory or a Java home is given to setup, the launcher shall save it in its own settings and pass it to the REA server's environment in every session, in place of the same variable in the launcher's environment.
- **R4.8** If the Ghidra a setup would save, together with the Java home it would run on, is not a Ghidra release REA admits or needs a newer JDK than that Java home holds, then the launcher shall report both versions and what REA needs, save nothing and exit with a non-zero status.
- **R4.9** When the launcher starts OpenCode with a Ghidra REA does not admit, or one on a JDK older than it needs, the launcher shall print a warning to standard error naming the versions and the setup flags that replace them.
- **R4.6** If REA's Ghidra engine is not ready after setup, then the launcher shall report what REA says is missing and the setup flags that supply it, and finish setup without failing.
- **R4.7** The launcher shall install the reverse-engineering instructions compiled into it as an `AGENTS.md` file in its own directory, and give that file to OpenCode as instructions in its sessions only.

## R5 · Gating the agent

The rules are the reviewed session rules of [[opencode]], plus REA's.

- **R5.1** The launcher shall configure OpenCode to ask before every edit and before running any shell command other than exactly `git status`, `git diff` and `git log`.
- **R5.2** The launcher shall configure OpenCode to deny reading `.env` files other than `.env.example`, `.pem`, `.key` and `.npmrc` files, fetching or searching the web, touching a path outside the project other than REA's skills, writing REA's skills, and repeating an identical tool call in a loop.
- **R5.3** The launcher shall configure OpenCode to allow without asking only the REA tools reviewed as static analysis — none of which runs, attaches to or drives the target, a device, a debugger, a browser, a web page or an application — and to ask before calling any other REA tool, including one the review has not seen.
- **R5.4** The launcher shall configure OpenCode with session sharing and code search disabled.
- **R5.5** The launcher shall start OpenCode with project configuration, Claude Code compatibility files, skills from outside the launcher's directory, self-update and language-server downloads disabled, and without the variable that enables web search.
- **R5.6** Where asking is not requested, the launcher shall start OpenCode's interface or its `run` command with every permission that is not denied approved without asking.
- **R5.7** While permissions are approved without asking, the launcher shall print a warning to standard error naming what runs unasked and the flag that restores asking.

## R6 · Running OpenCode

- **R6.1** When started without a subcommand, the launcher shall install whatever the managed toolchain lacks, obtain the key, and run the managed OpenCode in the current directory with the arguments that follow its own flags, on the same terminal, and exit with OpenCode's exit code.
- **R6.2** Where printing the configuration is requested, the launcher shall print the OpenCode configuration it would pass and exit without obtaining the key or starting OpenCode.
- **R6.3** When the launcher starts OpenCode, the launcher shall print one line to standard error naming the provider and the model the session's content is sent to.
- **R6.4** While OpenCode runs, the launcher shall ignore interrupt signals and leave them to OpenCode.
- **R6.5** When asked for its version, the launcher shall print its own version and the pinned versions of Node.js, REA and OpenCode.

## R7 · Setup

- **R7.1** When the setup subcommand runs, the launcher shall install the managed toolchain, obtain and save the provider key, install REA's skills and the `AGENTS.md`, and check REA's engine, then print the state of each.
- **R7.2** If a setup step fails, then the launcher shall report which step failed and exit with a non-zero status.

## R8 · CI and release

- **R8.1** When a pull request is opened or updated, the CI workflow shall build, vet and test the launcher on Linux and on Windows.
- **R8.2** When a tag named `v<version>` is pushed, the release workflow shall build the launcher for Windows, macOS and Linux on x64 and arm64 and publish the binaries in a GitHub release with a `SHA256SUMS` file and a build provenance attestation for each binary.

## Out of scope

- Installing an analysis engine — Ghidra, a JDK, Hopper, IDA or Kuna.
- Changing REA's registration in Claude Code or in any other agent.
- Writing `opencode.json` or OpenCode 2's configuration format.
- Detecting that a stored key was revoked; `logout` and a new sign-in replace it.
