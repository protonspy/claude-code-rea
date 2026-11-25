# NanoGPT

The provider abliterated models are reached through here
(adr:0001-use-opencode-with-nanogpt-for-abliterated-models), run in [[opencode]]. NanoGPT is an
aggregator: one key and one balance in front of models that other hosts serve, each named in the
model's record. Read on 2026-11-25 from NanoGPT's docs, its privacy page and its live catalog;
nothing here was run, and prices change — query the catalog before relying on one.

## Access and billing

- **API.** OpenAI-compatible at `https://api.nano-gpt.com/api/v1`, with
  `Authorization: Bearer <key>` or `x-api-key: <key>`; keys look like `sk-nano-<uuid>`
  ([authentication](https://docs.nano-gpt.com/authentication.md)). OpenCode's catalog lists the
  base URL as `https://nano-gpt.com/api/v1` ([models.dev](https://models.dev/api.json)) and uses
  that one unless a config overrides it.
- **Billing.** Pay as you go from a prepaid balance, or a PRO subscription that covers a weekly
  allowance of input tokens on the models it includes
  ([pricing](https://nano-gpt.com/pricing)). The default model below is included.
- **Per-key limits.** Each key can be capped per day, week or month on requests, dollars spent and
  input tokens, from the key dashboard
  ([rate limits](https://docs.nano-gpt.com/api-reference/miscellaneous/rate-limits.md)). Use a key
  made only for OpenCode, with a spend cap: the agent can read its own key (see [[opencode]]), and
  the cap is what bounds a leak. The same reasoning says to keep only a small balance in the
  account.
- NanoGPT's API page says abliterated models are for "defensive or authorized work only", under
  its Terms of Service ([API](https://nano-gpt.com/api)).

## The abliterated DeepSeek V4.1 Flash models

DeepSeek V4.1 Flash variants with tool calling, from the
[live catalog](https://api.nano-gpt.com/api/v1/models?detailed=true) on 2026-11-25, cheapest first.
The last row is Abliteration.ai's own model, which NanoGPT's [API page](https://nano-gpt.com/api)
describes as based on DeepSeek V4.1 Flash. Prices are per million tokens:

| Model id | Context | Max output | In / out |
|---|---|---|---|
| `pqhaz/DeepSeek-V4.1-Flash-1017-heretic` | 512,000 | 128,000 | $0.198 / $0.792 |
| `lyber/deepseek-v4.1-flash-1017-abliterated` | 1,048,576 | 126,976 | $0.29 / $0.99 |
| `deepseek/deepseek-v4.1-flash-abliterated` | 262,144 | 128,000 | $0.297 / $1.045 |
| `jcartu/deepseek-v4.1-flash-1017-abliterated-lossless-csf` | 1,048,576 | 943,718 | $0.315 / $1.05 |
| `deepseek/deepseek-v4.1-flash-1017-abliterated` | 1,000,000 | 131,072 | $0.375 / $1.50 |
| `abliteration-ai/abliterated-model-large-flash` | 1,048,576 | 262,144 | $1.00 / $2.00 |

The catalog also carries abliterated or heretic GLM 5.3, Kimi K3.1 Pro, MiMo V2.6 and Gemma 4
models, Abliteration.ai's larger models, and older ones without tool calling, which an agent
cannot use. Some cost less than the default — the Gemma 4 26B-A4B heretic,
`gemma-4-26b-a4b-it-chimeraxheretic`, is $0.12 / $0.38 — and are smaller models. The catalog answers without a key, and entries
marked `available: false` are paused
([models endpoint](https://docs.nano-gpt.com/api-reference/endpoint/models.md)):

```powershell
(Invoke-RestMethod 'https://api.nano-gpt.com/api/v1/models?detailed=true').data |
  Where-Object { $_.id -match 'ablit|heretic' -and $_.capabilities.tool_calling -and $_.available -ne $false } |
  Select-Object id, context_length, max_output_tokens, @{n='in'; e={$_.pricing.prompt}}, @{n='out'; e={$_.pricing.completion}}
```

## The default model

`pqhaz/DeepSeek-V4.1-Flash-1017-heretic`, per its own catalog record:

- A Heretic checkpoint of DeepSeek V4.1 Flash 1017, served by one host, **"sable"**.
- Tool calling and optional reasoning, with efforts from `none` to `max` — but **no parallel tool
  calls and no structured output**.
- Privacy: **zero data retention `unknown`, TEE `unknown`**, availability not verified; the host
  is marked eligible for NanoGPT's zero-retention policy.
- Included in the PRO subscription.
- A `:thinking` variant has the same limits and price.

One host means no failover: when sable is down, so is the model. The next one in the table above
is the fallback.

## What NanoGPT says about your data

From its [privacy page](https://nano-gpt.com/privacy):

- It "does not log or retain prompt and response content as part of normal inference", and keeps
  non-content metadata for billing, reliability and abuse prevention. Request logging is off by
  default and only the key owner can turn it on.
- **The host that serves the model receives the prompt.** What that host retains depends on the
  host.
- A **"ZDR only"** setting, on the account or on a key, sends requests only to hosts that state
  they keep no prompts or outputs, and fails them otherwise. NanoGPT calls it best effort. The
  default model's own retention is `unknown` while its host is marked eligible, so whether it is
  served under "ZDR only" is not stated; a refusal there is the setting working, not a fault.

NanoGPT's **Private Mode** — a local proxy that hides prompts from NanoGPT itself — covers only
its `private/...` models, and none of them is abliterated
([Private Mode](https://docs.nano-gpt.com/api-reference/miscellaneous/private-mode.md)). For
material no host may read, use [[near-ai]].

## Running it in OpenCode

The catalog already lists the provider and the model, so no provider block is needed — only the
key variable and the model. The key variable is `NANO_GPT_API_KEY` in both the catalog and
NanoGPT's own [OpenCode guide](https://docs.nano-gpt.com/integrations/opencode.md); NanoGPT's API
page spells it `NANOGPT_API_KEY`.

```powershell
$env:NANO_GPT_API_KEY = [Net.NetworkCredential]::new('', (Read-Host 'NanoGPT key' -AsSecureString)).Password
$cfg = Get-Content -Raw "$HOME\.config\opencode\abliterated.json" | ConvertFrom-Json
$cfg | Add-Member -Force -NotePropertyName model -NotePropertyValue 'nano-gpt/pqhaz/DeepSeek-V4.1-Flash-1017-heretic'
$cfg | Add-Member -Force -NotePropertyName small_model -NotePropertyValue $cfg.model
$env:OPENCODE_CONFIG_CONTENT = $cfg | ConvertTo-Json -Depth 10
opencode
```

`abliterated.json` is the session config from [[opencode]]: the permission rules, passed inline so
a project's own `opencode.json` cannot undo them, and `small_model` pinned to the same model so the
session's title is not generated by a cheaper model with other retention rules. NanoGPT
tested its OpenCode guide with OpenCode 1.19.5 and a DeepSeek V4.1 Flash model that is not
abliterated; whether the Heretic model's tool calls hold up through a whole session is the first
thing to try.

NanoGPT also documents a Claude Code setup on its Anthropic-format endpoint
([Claude Code guide](https://docs.nano-gpt.com/integrations/claude-code.md)). That route was ruled
out by the ADR; [[claude-code-backend]] says why.
