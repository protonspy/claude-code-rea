# NEAR AI

The private alternative to [[nanogpt]] (adr:0001-use-opencode-with-nanogpt-for-abliterated-models):
its abliterated models run inside TEEs whose attestation can be checked, instead of on a host whose
retention is a promise. It costs more and offers two abliterated models instead of dozens. Read on
2026-11-25 from NEAR AI Cloud's docs and live catalog; the attestation request below was run once,
its verification was not.

## Access and billing

- **API.** OpenAI-compatible at `https://cloud-api.near.ai/v1`, with
  `Authorization: Bearer <key>` ([quickstart](https://docs.near.ai/cloud/quickstart.md)).
- **Billing.** Credits are bought first, then a key is created in the dashboard at
  `cloud.near.ai`; usage is charged per token with no minimum
  ([uncensored API](https://near.ai/uncensored-llm-api)).
- **The key.** The agent can read its own key (see [[opencode]]), and the docs read here describe
  no per-key spend cap. Make a key only for these sessions, and keep only the credits you can lose
  in the account: the prepaid balance is what bounds a leak.

## The abliterated models

From the [live catalog](https://cloud-api.near.ai/v1/models) on 2026-11-25; prices per million
tokens. Both are described as "for red teaming, security research and evaluations" and "served
privately from NEAR AI's attested TEE infrastructure":

| Model id | Context | Max output | In / out / cache read | Supports |
|---|---|---|---|---|
| `deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated` | 1,048,576 | not listed | $0.30 / $1.20 / $0.006 | tools, reasoning; datacenter US |
| `Qwen/Qwen3.8-27B-abliterated` | 262,144 | 65,536 | $0.04 / $1.35 / $0.018 | tools, structured outputs, reasoning; FP8; US and IN |

OpenCode's catalog ([models.dev](https://models.dev/api.json)) gives different output limits —
384,000 for the DeepSeek model, 32,768 for the Qwen one — and OpenCode goes by its catalog. If
requests fail on output length, that mismatch is the first suspect.

## What the TEE claim covers

- **Private TEE models** — both abliterated ones — run in NEAR's own enclaves, where "nobody, not
  even NEAR, can see your prompts or outputs" ([models](https://docs.near.ai/cloud/models.md)).
- **Incognito models** are only routed through NEAR's own proxy and run at the model's host,
  outside a TEE; the guarantee does not reach them. None of the abliterated models is one — which
  is why the session pins `small_model` to the same model (see [[opencode]]).
- NEAR's [privacy policy](https://near.ai/privacy-policy) covers account, usage and billing data
  and meters tokens for billing; it says nothing about retaining prompts.
- **An attestation is a check of the endpoint at one moment.** OpenCode does not verify it, and
  nothing ties the requests of a later session to the evidence fetched today. What a session rests
  on is NEAR's claim, plus whatever was verified by hand — and only for the Private TEE models.

## Fetching and verifying the attestation

Fetching the evidence is not verifying it, and a `True` from the snippet below proves only that the
server echoed the nonce back. The browser way to verify is the
[Verification Center](https://cloud.near.ai/verify). The evidence itself can be fetched without a
key ([verification](https://docs.near.ai/cloud/verification/index.md)):

```powershell
$b = New-Object byte[] 32
[Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b)
$nonce = ($b | ForEach-Object { $_.ToString('x2') }) -join ''
$model = [uri]::EscapeDataString('deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated')
$r = Invoke-RestMethod "https://cloud-api.near.ai/v1/attestation/report?signing_algo=ecdsa&nonce=$nonce&model=$model&provider=near" `
  -Headers @{ 'x-no-aliasing' = 'true' }
$r.gateway_attestation.request_nonce -eq $nonce   # echo only: proves nothing until steps 1 and 2 pass
```

Run on 2026-11-25, it returned `gateway_attestation` with the nonce echoed and two
`model_attestations`, each with an Intel quote and an NVIDIA payload. Per NEAR's docs, verifying
means doing this for the gateway attestation **and for every entry in `model_attestations`**:

1. Verify the `intel_quote` with an Intel DCAP verifier under your own TCB policy.
2. Check that the nonce and the `signing_address` match the verified quote.
3. Replay the `event_log` and match the quote's RTMR3.
4. Hash the raw `app_compose` and match it to MRCONFIGID.
5. Read the verified `app_compose`: it says which image, and so which model, the enclave runs. A
   valid quote of code nobody has looked at proves nothing.

NEAR names two example verifiers, `near-examples/nearai-cloud-verification-example` and
`nearai/nearai-cloud-verifier` on GitHub. Its docs add a limit: a matching signing key does not
prove the same serving instance produced a given reply.

## Running it in OpenCode

OpenCode's catalog lists the provider as `nearai`, with both models, so the key variable and the
model are enough. NEAR's own [OpenCode guide](https://docs.near.ai/cloud/guides/integrations/opencode.md)
uses the same variable and warns against putting a literal key in `opencode.json`:

```powershell
$env:NEARAI_API_KEY = [Net.NetworkCredential]::new('', (Read-Host 'NEAR AI key' -AsSecureString)).Password
$cfg = Get-Content -Raw "$HOME\.config\opencode\abliterated.json" | ConvertFrom-Json
$cfg | Add-Member -Force -NotePropertyName model -NotePropertyValue 'nearai/deepseek-ai/DeepSeek-V4.1-Flash-1017-abliterated'
$cfg | Add-Member -Force -NotePropertyName small_model -NotePropertyValue $cfg.model
$env:OPENCODE_CONFIG_CONTENT = $cfg | ConvertTo-Json -Depth 10
opencode
```

`abliterated.json` is the session config from [[opencode]]. For Claude Code instead of OpenCode,
this same model is the worked example of the [[litellm-gateway]].
