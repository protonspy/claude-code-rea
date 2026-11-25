# This project

`claude-code-rea` is the bridge that lets Claude Code run abliterated models — local ones
through Ollama, hosted ones through an OpenAI-compatible API — next to or instead of the
Anthropic backend. The research, the decisions and how to connect each backend live in
`docs/`. The code is `rea-launcher`, a Go module at the root (`cmd/rea-launcher`,
`internal/launcher`) specified in `specs/rea-launcher/`.

**The delivery gate is the exception, because it is run rather than read.** Record
each of build, format, lint and test once with `scc check set <gate> "<command>"`;
`scc check` and the `pre-push` hook run them. A project with no such step says so
once with `scc check skip <gate>` — that is a decision, and it is not the same as
leaving one unrecorded.

## Commands

```bash
go build ./...                    # build gate
bash scripts/check-format.sh      # format gate: gofmt -l has nothing to say
go vet ./...                      # lint gate
bash scripts/check-test.sh        # test gate: go test with coverage, prints {"total", "coverage"}
go test ./internal/launcher/ -run <Name>   # one test while working

scc check                         # the four gates, as pre-push runs them
scc validate                      # the artifacts and the knowledge base
```

CI (`.github/workflows/ci.yml`) runs the four gates on Linux and Windows and `scc validate`.
A `v*` tag runs `.github/workflows/release.yml`: binaries for six platforms, `SHA256SUMS` and
build provenance.

## Conventions

- **Language:** English — code, docs, commits and PRs.
- **Branch names:** `feat/<slug>`, `fix/<slug>`, `docs/<slug>`, `chore/<slug>`
- **Commits:** Conventional Commits.
- **Secrets:** API keys (near.ai, Anthropic, a gateway master key) only in environment
  variables of the shell that needs them, or in the operating system's credential store
  through `rea-launcher` (adr:0002-launch-opencode-with-rea-from-a-go-launcher) — never in a
  file of this repository, and never in a user-wide `settings.json` `env` block, which would
  switch every Claude Code session off the subscription.

## Boundaries

- `.claude/` rules, agents and skills are written by `scc`; `scc update` brings new
  versions in. Only this file is ours to edit.
