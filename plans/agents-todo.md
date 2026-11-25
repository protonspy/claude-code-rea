---
autonomy: auto
ci: wait
---

# Agents todo

The `AGENTS.md` that `rea-launcher` installs tells the model to keep OpenCode's to-do list for
the whole session, so the user can follow what was done and what is left.

## Why

An automatic session runs unasked, and the user sees its progress only through what the model
writes. OpenCode 1.19.5 has a `todowrite` tool whose list the interface shows beside the
conversation. Done when every launcher session's instructions require that list and the test
holds them to it.

## Paths

- `internal/launcher/agents.md`
- `internal/launcher/rea_test.go`
- `docs/wiki/pages/rea-launcher.md`

## References

- `specs/rea-launcher/` — R4.7 installs the instructions this changes
- adr:0003-approve-launcher-sessions-automatically

## Tasks

- [x] 1.1 (Unit) Require a to-do list kept with `todowrite` in the embedded `AGENTS.md`, and check it in the instructions test

## Done when

- `go test ./internal/launcher/ -run TestTheInstructionsCoverTheSessionsRules` passes and fails when `todowrite` is removed from `agents.md`.
- `scc validate` exits 0.
