---
status: accepted
---

# 0003 · Approve launcher sessions automatically

## Context

adr:0001-use-opencode-with-nanogpt-for-abliterated-models counted on shell commands asking before
they run, and the launcher of adr:0002-launch-opencode-with-rea-from-a-go-launcher configures
OpenCode so that every shell command but three Git reads, every edit, and every REA tool not
reviewed as static analysis asks. On 2026-11-25 the user asked for launcher sessions to start with
OpenCode's automatic approval instead: a reverse-engineering session asks often, and every prompt
waits for a person.

OpenCode 1.19.5 has `--auto`, which approves every permission that is not explicitly denied. It is
accepted on its interface and on `run` only.

## Decision

`rea-launcher` starts OpenCode with `--auto` by default and prints a warning saying what runs
unasked; `rea-launcher --ask` restores asking. The configuration's denials — the web, code
search, key files, paths outside the project but REA's skills, writes to REA's skills, tool-call
loops — still bind OpenCode's own file and web tools, and nothing in the session directory is
loaded. **They do not bind a shell command**, which now runs unasked: it can read any file the user
can and reach the network.

Ruled out: approving everything, `permission: "allow"`, which would also lift the denials; and
keeping asking as the default with `--auto` as an option, which the user declined.

## Consequences

- **An injected instruction runs without a person seeing it.** The model is abliterated and reads
  untrusted targets: a string in a binary can make it run a shell command, start the sample through
  Frida or gdb, edit files, or read the provider key from OpenCode's environment and send it out
  through the shell — the web denial does not cover `curl`. The defences left are the denials, a
  disposable VM with no secrets, and the spend cap on the key.
- **The damage outlasts the session.** An unattended session can edit `.git/hooks`, build files,
  or the launcher's own directory — the managed toolchain and `AGENTS.md` — through the shell, and
  a later session, `--ask` included, runs on top of what it left. After an automatic session that
  read untrusted content, treat the project and the launcher directory as tainted. REA's skills,
  which every session loads, are denied to OpenCode's edit tool but not to the shell.
- **The warning may scroll out of sight**: OpenCode's interface takes over the terminal right
  after it is printed, and it is visible again only when the session ends.
- **The consequence of adr:0001 that shell commands ask no longer holds for launcher sessions** run
  without `--ask`.
- **Revisit when** OpenCode can deny network access to shell commands, or a session is to run on a
  machine holding secrets.
