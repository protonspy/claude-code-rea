# Claude Code backend

The **backend** is the endpoint Claude Code sends its model requests to. Out of the box it
is Anthropic's API, reached with the claude.ai subscription login. This page is what the
Claude Code documentation says about moving it somewhere else — the ground every route in
this repository stands on. Read on 2026-11-25; nothing here was measured.

## The formats it speaks

Claude Code speaks exactly three request formats
([gateway protocol](https://code.claude.com/docs/en/llm-gateway-protocol.md#api-formats)):

| Format | Selected by | Endpoints |
|---|---|---|
| Anthropic Messages | `ANTHROPIC_BASE_URL` | `POST /v1/messages`, optionally `/v1/messages/count_tokens` |
| Amazon Bedrock InvokeModel | `CLAUDE_CODE_USE_BEDROCK=1` + `ANTHROPIC_BEDROCK_BASE_URL` | Bedrock's |
| Google Cloud rawPredict | `CLAUDE_CODE_USE_VERTEX=1` + `ANTHROPIC_VERTEX_BASE_URL` | Vertex's |

OpenAI Chat Completions is not one of them. An OpenAI-compatible API — near.ai, a vLLM
server — cannot be the backend directly: something has to translate, which is the
[[litellm-gateway]].

Anthropic's position, verbatim from the [gateway page](https://code.claude.com/docs/en/llm-gateway.md):
"Anthropic doesn't endorse, maintain, or audit third-party gateway products, and doesn't
support routing Claude Code to non-Claude models through any gateway." Every route that puts
an abliterated model behind Claude Code runs outside documented support.

## The variables that move it

From [environment variables](https://code.claude.com/docs/en/env-vars.md) and
[connecting a gateway](https://code.claude.com/docs/en/llm-gateway-connect.md):

| Variable | What it does |
|---|---|
| `ANTHROPIC_BASE_URL` | Where requests go. The root URL — Claude Code appends `/v1/messages`. |
| `ANTHROPIC_AUTH_TOKEN` | Sent as `Authorization: Bearer <token>`. |
| `ANTHROPIC_API_KEY` | Sent as `x-api-key`. Approved once in interactive mode; overrides the login. |
| `apiKeyHelper` | A `settings.json` key naming a command that prints the key. Cached 5 minutes; `CLAUDE_CODE_API_KEY_HELPER_TTL_MS` changes that. |
| `ANTHROPIC_MODEL` | The session's model, sent as written. A bad id is not rejected at startup: the first request fails with "There's an issue with the selected model." |
| `ANTHROPIC_DEFAULT_OPUS_MODEL`, `_SONNET_`, `_HAIKU_`, `_FABLE_` | Which model id each alias resolves to — not which backend. The Haiku one also serves background calls. |
| `CLAUDE_CODE_SUBAGENT_MODEL` | Model for subagents; an alias or a full id. |

## What happens to the subscription

From [subscriptions and gateways](https://code.claude.com/docs/en/llm-gateway.md#subscriptions-and-gateways):

- A **credential variable** — `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_API_KEY` or `apiKeyHelper` —
  replaces the subscription login for the whole session, billed per token to whoever owns
  that credential.
- With `ANTHROPIC_BASE_URL` set and **no** credential variable, the claude.ai login stays
  active **and is sent to the gateway**. Pointing the base URL at a third party without a
  credential variable hands that third party the login token.
- The saved login stays on disk; unsetting the variables switches back.

So the variables belong to the one shell that launches the redirected session. In the `env`
block of the user-wide `settings.json` they apply to every session, and every session leaves
the subscription.

## One backend per session

`ANTHROPIC_BASE_URL` "changes where requests are sent, not which model answers them"
([model configuration](https://code.claude.com/docs/en/model-config.md)), and it covers the
main model, every subagent and the background calls alike. Mixing Opus on the subscription
with another backend in one session is not documented.

A subagent cannot pick its own backend either
([subagents](https://code.claude.com/docs/en/sub-agents.md)):

- Its frontmatter has no field for a base URL or environment variables.
- Its `model:` field takes `sonnet`, `opus`, `haiku`, `fable`, a full model id or `inherit`.
  An unknown id is sent as written — to the same backend.
- The Agent tool's per-call `model` takes an alias only.
- Precedence: the per-call `model`, then the frontmatter `model:`, then
  `CLAUDE_CODE_SUBAGENT_MODEL`, then the main model. `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1`
  makes the variable win over both; the variable alone does not change the built-in
  Explore and Plan agents.

Two ways remain to use another model without giving up Opus: a gateway that routes by model
name — which needs a credential variable, so Opus moves off the subscription and onto a paid
key — or the other model reached as a tool, which leaves the backend alone:
[[local-model-as-tool]].

## Known failures on a non-Anthropic backend

From [gateway troubleshooting](https://code.claude.com/docs/en/llm-gateway-connect.md#troubleshoot-gateway-errors)
and [model configuration](https://code.claude.com/docs/en/model-config.md#correct-the-window-for-a-gateway-or-custom-model-id):

- An unknown model id gets an assumed context window: set `CLAUDE_CODE_MAX_CONTEXT_TOKENS`.
- A 400 with "Extra inputs are not permitted", or one about an `anthropic-beta` header:
  set `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1`.
- Adaptive-thinking errors and rejected tool schemas are the backend's problem, not
  something a Claude Code setting fixes.
- Remote Control is disabled whenever the base URL is not `api.anthropic.com`.
