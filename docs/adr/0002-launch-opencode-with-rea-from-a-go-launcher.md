---
status: accepted
---

# 0002 · Launch OpenCode with REA from a Go launcher

## Context

adr:0001-use-opencode-with-nanogpt-for-abliterated-models settled the harness, OpenCode, and the
provider, NanoGPT. Running a reverse-engineering session with it on 2026-11-25 took more than
that:

- **REA** — the MCP server that does the reverse engineering — ships only as the npm package
  `rea-agents`; its GitHub releases carry no assets. Version 6.10.1 needs Node.js `^24.22.1`, and
  the machine had 24.18.1. REA's own `rea setup` registers it for a whole agent, in the agent's
  user-wide configuration.
- **The key.** NanoGPT offers an OAuth PKCE sign-in that mints a dedicated `sk-nano-` key with a
  spend cap chosen at consent, long-lived, with no refresh token and no revocation endpoint
  ([NanoGPT](https://docs.nano-gpt.com/api-reference/miscellaneous/oauth-pkce)). Until now this
  repository kept every key in an environment variable of one shell.
- **OpenCode** takes a whole configuration for one run in `OPENCODE_CONFIG_CONTENT` and an extra
  configuration directory in `OPENCODE_CONFIG_DIR`, both loaded after the user's files — see
  [[opencode]].
- The user asked for the entry point to be **one Go binary** that sets everything up.

## Decision

A Go binary, `rea-launcher`, is the entry point — specified in `specs/rea-launcher/`.

- **It owns a toolchain.** Node.js 24.22.3, REA 6.10.1 and OpenCode 1.19.5 are installed in the
  launcher's own cache directory, at versions and hashes compiled into the binary, and used in
  place of anything on the `PATH`. A download whose hash differs is deleted, never unpacked. REA's
  pinned hash was matched against the registry's integrity and against its SLSA provenance, built
  from `morluto/rea` at the commit the `rea-agents-6.10.1` tag names. OpenCode 1.19.5 is the
  version NanoGPT's OpenCode guide was tested with.
- **It configures one run.** REA's MCP server, REA's skills, the reverse-engineering
  `AGENTS.md`, the model and the permission rules reach OpenCode through `OPENCODE_CONFIG_CONTENT`
  and `OPENCODE_CONFIG_DIR`. No file another OpenCode session reads is written, and no other
  agent's REA registration is touched.
- **The key lives in the operating system's credential store** — Windows Credential Manager,
  macOS Keychain, the Secret Service on Linux — under `rea-launcher`. It comes from NanoGPT's
  sign-in, from NEAR AI's key typed at a hidden prompt, or from the provider's environment
  variable, which still wins. This amends the repository's rule that keys live only in a
  shell's environment.

Ruled out:

- **`rea setup --client opencode`** — registers REA in OpenCode's user-wide configuration, so
  every OpenCode session gets REA's tools, with no permission rules of its own.
- **A project `opencode.json`** — has to exist in every directory analysed, and the directory
  holding a target is not one to write into.
- **The system Node.js and a global npm install** — the system Node was too old for REA, and
  upgrading it changes every other tool that runs on it.
- **`npx -y rea-agents@<version> mcp` on each start** — resolves the package over the network at
  every launch, can outlast OpenCode's wait for the tool list, and REA documents a
  package-runner failure mode for it (`ERR_MODULE_NOT_FOUND`, issue 5259).
- **Signing in on every run, keeping nothing** — a consent screen per launch, and possibly a new
  key per consent.
- **OpenCode's `auth.json`** — plain text, readable by any process of the user.

## Consequences

- **Updating REA, OpenCode or Node.js is a release of the launcher.** New pins, new hashes, and for
  REA a new snapshot of its tool catalog and a new review of the static tools allowed unasked.
- **The key is at rest.** Encrypted by the operating system for the user, but any process of the
  user can ask for it — as it could read the environment variable before. The spend cap set at
  NanoGPT's consent screen is what bounds a leak; `rea-launcher logout` deletes the stored key,
  and revoking it is done in NanoGPT's key list.
- **Not all of it is pinned.** REA's dependency `frida` downloads a native binary from its install
  script, and OpenCode's platform packages carry no provenance.
- **About 450 MB in the cache directory**, most of it REA and OpenCode.
- **A Go toolchain to build the launcher**, and two GitHub workflows: tests on every pull request,
  binaries with `SHA256SUMS` and provenance on every `v*` tag.
- **Revisit when** OpenCode 2 becomes the version to run, REA changes how it is distributed, or
  NanoGPT adds revocation or short-lived tokens to its OAuth.
