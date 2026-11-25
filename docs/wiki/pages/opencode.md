# OpenCode

The harness abliterated models run in here (adr:0001-use-opencode-with-nanogpt-for-abliterated-models).
OpenCode is an open-source coding agent built to run any provider: it speaks the OpenAI format
NanoGPT and NEAR AI serve, so no [[litellm-gateway]] sits in between. This page covers what is
common to every provider; the setup for each is on [[nanogpt]] and [[near-ai]]. Written from the
OpenCode 1.18 documentation (1.18.15 is installed) on 2026-11-25; nothing here was run.

OpenCode 2 changes the configuration format — `providers`, `package` and `settings` in place of
1.18's `provider`, `npm` and `options` ([v2 providers](https://opencode.ai/docs/providers.md)).
Everything below is the 1.18 format.

## Where the configuration lives

From [config](https://github.com/anomalyco/opencode/blob/v1.18.35/packages/web/src/content/docs/config.mdx),
later sources override earlier ones:

| Source | Use here |
|---|---|
| `~/.config/opencode/opencode.json` | The user's defaults. |
| The file named by `OPENCODE_CONFIG` | A config for one shell, without touching the others. |
| `opencode.json` in the project | Found from the working directory up to the nearest Git root. The docs call it safe to commit — true only while it holds no key. |
| `OPENCODE_CONFIG_CONTENT` | Inline JSON for one run. |

`{env:NAME}` anywhere in a config file is replaced by that environment variable, or by an empty
string when it is unset.

## How it picks the model

From [models](https://github.com/anomalyco/opencode/blob/v1.18.35/packages/web/src/content/docs/models.mdx),
first match wins: the `--model` (`-m`) flag, then `model` in the config, then the last model used,
then an internal default. The value is `<provider id>/<model id>`.

OpenCode's providers and models come from the [models.dev catalog](https://models.dev/api.json),
which already lists both providers used here, with their abliterated models and whether each
supports tool calling:

| Provider | Provider id | Key variable | Catalog base URL |
|---|---|---|---|
| NanoGPT | `nano-gpt` | `NANO_GPT_API_KEY` | `https://nano-gpt.com/api/v1` |
| NEAR AI | `nearai` | `NEARAI_API_KEY` | `https://cloud-api.near.ai/v1` |

So a catalog model needs no provider block at all: the key variable and a `--model` are enough.

## Where the key comes from

Two ways, and neither keeps the key from the agent:

- **An environment variable** — the one the catalog names. Set it in the shell that launches
  OpenCode, read with `Read-Host -AsSecureString` so PowerShell's history never stores it, and it
  dies with that shell. NEAR's OpenCode guide warns against a literal key in `opencode.json`
  ([NEAR](https://docs.near.ai/cloud/guides/integrations/opencode.md)).
- **`opencode auth login`** — saves the key in `~/.local/share/opencode/auth.json`
  ([CLI](https://github.com/anomalyco/opencode/blob/v1.18.35/packages/web/src/content/docs/cli.mdx)),
  where it stays between sessions.

The OpenCode docs do not say whether the shell commands the agent runs see OpenCode's
environment, and the credentials file is readable by any process of the user. Assume the model
can read its own key. What limits the damage is the key itself: a dedicated one, with a spend cap
set at the provider — see [[nanogpt]].

## Gating what the agent does

An abliterated model does what the text in front of it says, including instructions injected into
a file or a web page it reads. The permission rules are the one place to stop that before it runs.
From [permissions](https://github.com/anomalyco/opencode/blob/v1.18.35/packages/web/src/content/docs/permissions.mdx)
and the [option reference](https://github.com/anomalyco/opencode/blob/v1.18.35/packages/core/src/plugin/skill/customize-opencode.md):

- Each rule is `allow`, `ask` or `deny`, given flat or as `{ pattern: action }`.
- **The last matching rule wins**, so the catch-all `"*"` goes first and the exceptions after it.
- The keys include `read`, `edit`, `glob`, `grep`, `list`, `bash`, `task`, `external_directory`,
  `webfetch`, `websearch` and `skill`; `webfetch` and `websearch` take only a flat action.
- An agent's own `permission` overrides the top-level one.

A project `opencode.json` for a session with an abliterated model:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "bash": {
      "*": "ask",
      "git status*": "allow",
      "git diff*": "allow",
      "git log*": "allow"
    },
    "webfetch": "ask",
    "websearch": "ask",
    "external_directory": "ask"
  }
}
```

Every shell command but reading Git state asks first, and so does any fetch or search of the web
and any path outside the project. Edits inside the project keep OpenCode's default. Nothing in it
is a secret, so it can be committed.

## What leaves the machine besides the provider

NEAR's write-up of OpenCode says a session is synced to OpenCode's servers only when `/share` is
run ([NEAR blog](https://near.ai/blog/opencode-private-inference)). The provider still receives
everything the session sends.
