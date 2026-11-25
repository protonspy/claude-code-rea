---
autonomy: auto
ci: no-wait
lang: en
status: approved
checksum: c2ede830a3f4e3dea4357760266a78d3e33461c857119c7c081195a1f22f066b
---

# Connecting Claude Code to other models

Document, from the official Claude Code and LiteLLM docs, how Claude Code can reach a
model that is not Anthropic's: a hosted OpenAI-compatible API such as near.ai, or a
local Ollama model, and how to keep Opus while delegating to a local model.

## Why

This repository exists to bridge abliterated models into Claude Code, and the first
thing to settle is what Claude Code allows: which API format it speaks, what changes
when it is pointed elsewhere, and what that costs the subscription login. Done means
those facts sit in the wiki with their sources, so the implementation plans that follow
start from them instead of re-deriving them.

## Paths

- `docs/wiki/pages/`
- `docs/wiki/index.md`
- `docs/wiki/changelog.md`
- `docs/glossary.md`

## Out of scope

- Installing or running LiteLLM, Ollama or any model — no code and no end-to-end test
  here; the pages say what is documented, not what was measured.
- Choosing which abliterated model to run — the model research is a later batch.
- An ADR for the architecture — written when the implementation plan commits to one.

## Tasks

- [x] 1.1 (Unit) Write the wiki page on the Claude Code backend: the API formats it speaks, the environment variables that redirect it, what a credential variable does to the subscription, one base URL per session, and what a subagent's `model` field can and cannot select
- [x] 1.2 (Unit) Write the wiki page on the LiteLLM gateway: translating Claude Code's requests to an OpenAI-compatible endpoint, with near.ai as the worked example, launched from its own shell
- [x] 1.3 (Unit) Write the wiki page on keeping Opus and calling a local model as a tool, through an MCP server or a shell call to Ollama
- [x] 1.4 (Unit) Add the glossary terms the pages use, link every page from the wiki index, and log them in the changelog
  _Depends 1.1, 1.2, 1.3_

## Done when

- `scc validate` exits 0.
- Every page under `docs/wiki/pages/` is linked from `docs/wiki/index.md` and named in `docs/wiki/changelog.md`.
- Every claim about Claude Code or LiteLLM behaviour links the documentation page it came from.
