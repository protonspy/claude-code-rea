# LiteLLM gateway

LiteLLM's proxy is the translator that lets Claude Code use a model whose API is not
Anthropic's. It answers Claude Code in Anthropic Messages — the only format a
[[claude-code-backend]] over plain HTTP can speak — and calls the real provider in that
provider's own format. Read on 2026-11-25 from the LiteLLM docs; not yet run end to end.

## How Claude Code reaches it

LiteLLM's [Claude Code quick reference](https://docs.litellm.ai/docs/proxy/client_setup/claude_code):

| Setting | Value |
|---|---|
| `ANTHROPIC_BASE_URL` | The proxy, e.g. `http://127.0.0.1:4000` |
| `ANTHROPIC_AUTH_TOKEN` | A LiteLLM virtual key, or the master key |
| `ANTHROPIC_MODEL` | A `model_name` from the proxy's `config.yaml` |

Setting `ANTHROPIC_AUTH_TOKEN` is a credential variable, so the session leaves the
subscription — which is also what keeps the claude.ai login from being sent to the proxy
(see [[claude-code-backend]]).

## An OpenAI-compatible provider

Per [OpenAI-compatible endpoints](https://docs.litellm.ai/docs/providers/openai_compatible),
a `model_list` entry with the `openai/` prefix routes to any server that speaks OpenAI
Chat Completions. `api_base` has to end in `/v1`, or the proxy answers `Not Found`.

```yaml
model_list:
  - model_name: my-model
    litellm_params:
      model: openai/<upstream-model-id>
      api_base: https://<host>/v1
      api_key: os.environ/<KEY_VARIABLE>
```

`os.environ/<NAME>` makes the proxy read the key from its own environment, so the key is
never written into the file.

## Worked example: DeepSeek V4.1 Flash abliterated on near.ai

[near.ai's model page](https://near.ai/models/deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated)
lists `deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated`, the October 17, 2026 snapshot of
DeepSeek V4.1 Flash with the refusal direction removed. Hosted only — the page links no
weights — on an OpenAI-compatible endpoint, `https://cloud-api.near.ai/v1/chat/completions`,
with a 1M-token context, at $0.30 per million input tokens and $1.20 per million output.
The page says it runs in a TEE where "not even NEAR AI can see your prompts", with an
attestation report; that is near.ai's claim, not something this repository checked. The
base model is 284B parameters with 13B active per token
([NVIDIA's model reference](https://docs.api.nvidia.com/nim/reference/deepseek-ai-deepseek-v4-flash)),
far beyond a 12 GB GPU, so hosted is the only way to run it here.

`config.yaml`:

```yaml
model_list:
  - model_name: deepseek-abl
    litellm_params:
      model: openai/deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated
      api_base: https://cloud-api.near.ai/v1
      api_key: os.environ/NEARAI_API_KEY
general_settings:
  master_key: os.environ/LITELLM_MASTER_KEY
```

First shell — the proxy:

```powershell
pip install "litellm[proxy]"
$env:NEARAI_API_KEY = '<near.ai key>'
$env:LITELLM_MASTER_KEY = '<any local secret>'
litellm --config config.yaml --port 4000
```

Second shell — the redirected Claude Code session:

```powershell
$env:ANTHROPIC_BASE_URL = 'http://127.0.0.1:4000'
$env:ANTHROPIC_AUTH_TOKEN = '<the same local secret>'
$env:ANTHROPIC_MODEL = 'deepseek-abl'
$env:ANTHROPIC_DEFAULT_HAIKU_MODEL = 'deepseek-abl'
$env:CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS = '1'
claude
```

Why each line, beyond the quick reference:

- `ANTHROPIC_DEFAULT_HAIKU_MODEL` — background calls use the Haiku alias, and this proxy
  knows only `deepseek-abl`; left unset, they would ask it for a Claude model it does not
  have. This follows from the documented variables, not from a test.
- `CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS` — the documented fix for 400s about extra
  inputs or `anthropic-beta` headers on a non-Anthropic backend.
- Two shells, variables set with `$env:` — they die with the shell, so no other Claude Code
  session leaves the subscription, and no key lands in a file.

## What is not known yet

- Whether Claude Code's tool calls — reading, editing, running commands — survive the
  translation to OpenAI tool calls and back with this model. Nothing here has been run.
- How a local Ollama model fits this proxy: not covered in these pages. Keeping Opus and
  calling a local model as a tool is the other route: [[local-model-as-tool]].
