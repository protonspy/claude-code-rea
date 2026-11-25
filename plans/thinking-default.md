---
autonomy: auto
ci: wait
---

# Thinking default

Make NanoGPT's `:thinking` variant of the Heretic DeepSeek V4.1 Flash the model `rea-launcher`
runs when no provider and no model are given.

## Why

The user wants reasoning on by default. `pqhaz/DeepSeek-V4.1-Flash-1017-heretic:thinking` is the
same checkpoint on the same host, with the same context, output limit and price, per the NanoGPT
catalog on 2026-11-25. Done when the launcher's default is the variant and every artifact that
names the default says so.

## Paths

- `internal/launcher/provider.go`
- `specs/rea-launcher/requirements.md`, `specs/rea-launcher/design.md`
- `docs/wiki/pages/nanogpt.md`, `docs/wiki/pages/rea-launcher.md`

## References

- `specs/rea-launcher/` — R1.1 names the default model; amended here as a delta
- `docs/adr/0001-use-opencode-with-nanogpt-for-abliterated-models.md` — chose the Heretic model; the variant keeps that decision

## Out of scope

- A separate `small_model`: the launcher keeps writing the session model for both.
- NEAR AI's default model.

## Tasks

- [x] 1.1 (Unit) Set NanoGPT's default model to `pqhaz/DeepSeek-V4.1-Flash-1017-heretic:thinking` and update the tests that assert it
- [x] 1.2 (Unit) Amend R1.1, the design's sample configuration and the wiki pages to name the `:thinking` variant
  _Depends 1.1_

## Done when

- `scc check` and `scc validate` exit 0.
- `rea-launcher --print-config` shows `nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic:thinking` as `model`.
