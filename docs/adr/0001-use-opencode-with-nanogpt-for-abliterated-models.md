---
status: accepted
---

# 0001 · Use OpenCode with NanoGPT for abliterated models

## Context

This repository bridges abliterated models into a coding agent. On 2026-11-25 the ground
looked like this:

- **The machine.** One RTX 3060 with 12 GB and 32 GB of RAM. The abliterated models worth
  coding with — DeepSeek V4.1 Flash is 284B parameters with 13B active
  ([NVIDIA](https://docs.api.nvidia.com/nim/reference/deepseek-ai-deepseek-v4-flash)) — do not
  fit, so the model has to be hosted.
- **Claude Code.** It speaks only the Anthropic Messages format (plus Bedrock and Vertex), and
  Anthropic does not support routing it to non-Claude models through any gateway — see
  [[claude-code-backend]]. It is tuned for Claude: Venice's own Claude Code guide warns other
  models "may work" without "an equivalent experience"
  ([Venice](https://docs.venice.ai/guides/integrations/claude-code)). Reaching an
  OpenAI-format provider from it takes a translator, the [[litellm-gateway]].
- **OpenCode.** An open-source agent built to run any provider. Its catalog, models.dev,
  already lists both NanoGPT (`nano-gpt`) and NEAR AI (`nearai`) with their abliterated
  models and their tool-calling flags ([models.dev](https://models.dev/api.json)). Version 1.18
  is installed.
- **The providers.** Seven were compared. Four of the five comparison articles found were
  written by providers ranking themselves, so prices and policies were taken from each
  provider's own pages and model catalogs wherever those could be read.

## Decision

Abliterated models run in **OpenCode**, through **NanoGPT**, with
`nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic` as the default model: tool calling, 512K
context, $0.198 in and $0.792 out per million tokens
([NanoGPT catalog](https://api.nano-gpt.com/api/v1/models?detailed=true)). **NEAR AI** is the
alternative when the material must not be readable by whoever hosts the model: its
abliterated models run in an attested TEE — see [[near-ai]]. **Claude Code** stays the
harness for Opus on the subscription; nothing about it changes.

Ruled out:

- **Claude Code as the harness for abliterated models** — through NanoGPT's own
  `/api/v1/messages` or through LiteLLM. Unsupported by Anthropic, tuned for a different model,
  and the LiteLLM route adds a translation layer to every tool call. It stays documented, not
  chosen.
- **Refuseless** — the cheapest listing for the same model ($0.12 / $0.45 per AllModels, a
  competing vendor), but its own site refused every fetch: no terms, privacy policy, operator
  or jurisdiction could be read.
- **OpenRouter** — the most used model aggregator, and it serves no abliterated models: its one
  uncensored entry is a Dolphin fine-tune.
- **Venice** — the same kind of access at a higher price for the DeepSeek V4.1 Flash variants
  ($0.33 / $1.32 per Experiential Labs, a competing vendor), and its Anonymized tier hands the
  prompt to the model's host.
- **Featherless** — the largest catalog of community models, sold by subscription ($25 a month
  for chat, $50 of credits for the API), and its documentation could not be read.
- **Arli AI** — uncensored through fine-tuned LoRAs rather than abliteration, sold by
  subscription.
- **A local model** (Qwen3-Coder 30B Heretic) — fits only with part of it in system RAM, was
  never measured, and is far weaker. It remains reachable as a tool: [[local-model-as-tool]].

## Consequences

- **Code leaves the machine.** Everything in the session goes to NanoGPT and on to the host
  that serves the model — "sable" for the Heretic model, whose record shows zero data
  retention and TEE as `unknown`. No secrets, and no employer's code, go through this route;
  that is what NEAR AI is for.
- **One host, no fallback inside NanoGPT.** The Heretic model lists a single provider. When it
  is down the model is down; the fallback is another model id or NEAR AI.
- **Weaker tool use than Claude.** The Heretic record shows no parallel tool calls and no
  structured output, and nothing here has been run end to end. The first real session is the
  test.
- **The key is within the agent's reach.** OpenCode needs the NanoGPT key in its environment
  or in its credentials file, and every command the agent runs can read either. The key is a
  dedicated one with a spend cap, and shell commands ask before they run — see [[opencode]].
- **Two harnesses to keep.** Claude Code's hooks and skills do not carry over to OpenCode;
  `scc init --opencode` scaffolds the same methodology there.
- **Revisit when** the Heretic model's retention becomes known or changes, the first sessions
  show its tool calls failing, OpenCode moves to its version 2 configuration format, or a
  provider that was ruled out for unreadable terms publishes them.
