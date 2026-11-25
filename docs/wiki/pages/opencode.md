# OpenCode

The harness abliterated models run in here (adr:0001-use-opencode-with-nanogpt-for-abliterated-models).
OpenCode is an open-source coding agent built to run any provider: it speaks the OpenAI format
NanoGPT and NEAR AI serve, so no [[litellm-gateway]] sits in between. This page covers what is
common to every provider; the setup for each is on [[nanogpt]] and [[near-ai]]. Written from the
OpenCode 1.18 documentation (1.18.15 is installed) on 2026-11-25; nothing here was run, the
session config below included.

OpenCode 2 changes the configuration format — `providers`, `package` and `settings` in place of
1.18's `provider`, `npm` and `options` ([v2 providers](https://opencode.ai/docs/providers.md)).
Everything below is the 1.18 format.

## Where the configuration lives

From [config](https://github.com/anomalyco/opencode/blob/v1.18.35/packages/web/src/content/docs/config.mdx),
later sources override earlier ones:

| Source | What it is |
|---|---|
| `~/.config/opencode/opencode.json` | The user's defaults. |
| The file named by `OPENCODE_CONFIG` | A custom config — still overridden by the project's. |
| `opencode.json` in the project | Found from the working directory up to the nearest Git root. |
| `.opencode/` in the project | Agents, commands, plugins. |
| `OPENCODE_CONFIG_CONTENT` | Inline JSON for one run — it overrides the project's files. |

**A project's `opencode.json` is code you run.** It overrides the user's config, so a repository
you did not write can turn permissions to `allow`, or give `nano-gpt` or `nearai` a different
`baseURL` — and the key then goes to that address. Read a cloned repository's `opencode.json` and
`.opencode/` before starting OpenCode in it, and pass the rules below through
`OPENCODE_CONFIG_CONTENT`, the one source that outranks the project. How OpenCode merges a
project's rules with inline ones was not checked; reading the project's files is the guard that
does not depend on it.

`{env:NAME}` in a config file is replaced by that environment variable, or by an empty string when
it is unset.

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

So a catalog model needs no provider block — only the key variable and the model.

**Set `small_model` to the same model.** OpenCode uses a separate `small_model` for light tasks
such as titling a session, and when it is unset it "tries to use a cheaper model if one is
available from your provider" ([config](https://github.com/anomalyco/opencode/blob/v1.18.35/packages/web/src/content/docs/config.mdx)).
The title is generated from the first message, which would then go to a model nobody chose — on
NEAR AI, possibly one outside the TEE.

## Where the key comes from

- **An environment variable** — the one the catalog names — set in the shell that launches
  OpenCode, read with `Read-Host -AsSecureString` so PowerShell's history never stores it. It
  dies with that shell. This is the way used here.
- **`opencode auth login`** saves the key in `~/.local/share/opencode/auth.json`
  ([CLI](https://github.com/anomalyco/opencode/blob/v1.18.35/packages/web/src/content/docs/cli.mdx)),
  where it stays on disk between sessions, readable by any process of the user.
- Never a literal key in `opencode.json` — NEAR's OpenCode guide warns against it too
  ([NEAR](https://docs.near.ai/cloud/guides/integrations/opencode.md)).

Either way, assume the agent can read its own key: a shell command can print the environment, and
the credentials file is just a file. Refuse any command that would print an environment variable
or read `auth.json`. What bounds a leak is the key itself — one made only for these sessions, with
a spend cap or a small balance (see [[nanogpt]], [[near-ai]]).

## Gating what the agent does

An abliterated model does what the text in front of it says, including instructions injected into
a file or a web page it reads. The permission rules are the one place to stop that before it runs.
From [permissions](https://github.com/anomalyco/opencode/blob/v1.18.35/packages/web/src/content/docs/permissions.mdx)
and the [option reference](https://github.com/anomalyco/opencode/blob/v1.18.35/packages/core/src/plugin/skill/customize-opencode.md):

- Each rule is `allow`, `ask` or `deny`, given flat or as `{ pattern: action }`.
- **The last matching rule wins**, so the catch-all `"*"` goes first and the exceptions after it.
- In a pattern, `*` matches any run of characters and `?` exactly one; everything else is literal.
  So `"git diff*"` also matches `git difftool` and `git diff --no-index <any file>` — allow exact
  commands, not prefixes.
- `read` allows everything but `.env` files by default. `webfetch` and `websearch` take only a flat
  action.
- An `ask` prompt offers `once`, `always` — every matching request for the rest of the session —
  and `reject`.

The session config, kept outside any project — say `~/.config/opencode/abliterated.json`, a name
OpenCode does not load on its own:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "bash": {
      "*": "ask",
      "git status": "allow",
      "git diff": "allow",
      "git log": "allow"
    },
    "edit": "ask",
    "read": {
      "*": "allow",
      "*.env": "deny",
      "*.env.*": "deny",
      "*.env.example": "allow",
      "*.pem": "deny",
      "*.key": "deny",
      "*.npmrc": "deny"
    },
    "webfetch": "deny",
    "websearch": "deny",
    "external_directory": "deny"
  }
}
```

What each rule is for:

- **`bash`** — only those three exact commands run unasked.
- **`edit: ask`** — an edit is a command the shell rule never sees: a written `.git/hooks/pre-commit`
  runs at the next commit, and a written `opencode.json` loosens the next session or redirects the
  key. Every edit asks.
- **`read`** — restates the `.env` defaults and adds common key files. `grep` can still return lines
  from any file, so an abliterated session belongs in a tree with no secrets in it.
- **`webfetch`, `websearch: deny`** — a URL or a search query is a way out for anything the model has
  read. Turn one to `ask` only for a session that needs it.
- **`external_directory: deny`** — nothing outside the project.

When a prompt comes: read the whole command or URL, answer `once`, never `always`, and `reject`
anything that prints the environment, reads a credentials file, or sends data to an address.

## Launching a session

The provider pages each give the model; the launch is the same:

```powershell
$cfg = Get-Content -Raw "$HOME\.config\opencode\abliterated.json" | ConvertFrom-Json
$cfg | Add-Member -Force -NotePropertyName model -NotePropertyValue '<provider id>/<model id>'
$cfg | Add-Member -Force -NotePropertyName small_model -NotePropertyValue $cfg.model
$env:OPENCODE_CONFIG_CONTENT = $cfg | ConvertTo-Json -Depth 10
opencode
```

Set the provider's key variable first, as its page shows.

## What leaves the machine besides the provider

NEAR's write-up of OpenCode says a session is synced to OpenCode's servers only when `/share` is
run ([NEAR blog](https://near.ai/blog/opencode-private-inference)). The provider receives everything
the session sends, and the model's host whatever the provider forwards.
