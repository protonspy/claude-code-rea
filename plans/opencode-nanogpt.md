---
autonomy: auto
ci: no-wait
lang: en
status: approved
checksum: c04278f24f8753a9b705cd165a8b20aea1f78d0bac2967198b7812b0a920f1ed
---

# OpenCode with NanoGPT

Record the choice of OpenCode as the harness and NanoGPT as the provider for abliterated
models, and document how to run them — with NEAR AI as the private alternative — from the
providers' own catalogs and docs and OpenCode's.

## Why

The research settled on OpenCode plus NanoGPT: OpenCode speaks the OpenAI format the
providers offer, with no gateway in between, and NanoGPT serves the abliterated models with
tool calling at the lowest price among providers whose terms can be read. Done means the
decision and its rejected alternatives are an ADR, and a reader can configure OpenCode for
either provider from the wiki alone, with keys kept out of files and the agent's commands
gated.

## Paths

- `docs/adr/`
- `docs/wiki/pages/`
- `docs/wiki/index.md`
- `docs/wiki/changelog.md`
- `docs/glossary.md`

## References

- `adr:0001-use-opencode-with-nanogpt-for-abliterated-models` — written by task 1.1

## Out of scope

- Installing OpenCode or running a session end to end — the pages say what the docs and
  the catalogs state, not what was measured.
- OpenCode 2's configuration format — the installed version is 1.18.
- MCP servers and per-agent models in OpenCode.

## Tasks

- [x] 1.1 (Unit) Record the decision as ADR 0001: OpenCode with NanoGPT for abliterated models, NEAR AI as the private alternative, Claude Code kept for Opus, and why each other provider and harness was ruled out
- [x] 1.2 (Unit) Write the wiki page on OpenCode as the harness for abliterated models: where its config lives, how it picks a model, where credentials come from, and the permission rules that gate the agent's commands
- [x] 1.3 (Unit) Write the wiki page on NanoGPT: the abliterated models with tool calling, the default model and its privacy caveats, and the OpenCode setup
  _Depends 1.2_
- [x] 1.4 (Unit) Write the wiki page on NEAR AI: the two abliterated models, what the TEE claim covers and how to verify it, and the OpenCode setup
  _Depends 1.2_
- [x] 1.5 (Unit) Add the glossary terms the pages use, link the new pages from the existing ones, the index and the changelog
  _Depends 1.1, 1.3, 1.4_

## Done when

- `scc validate` exits 0.
- Every new page is linked from `docs/wiki/index.md` and named in `docs/wiki/changelog.md`.
- Every model id, price and limit in the pages links the catalog or documentation page it came from.
