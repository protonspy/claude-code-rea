# Local model as a tool

The way to keep Opus on the subscription and still use a local model: the session's
backend stays Anthropic's, and the local model is something Opus *calls*, like any other
tool, instead of something Claude Code *runs on*. Nothing about where Claude Code sends
its own requests changes, so none of the trade-offs in [[claude-code-backend]] apply to the
main session. Read on 2026-11-25; not yet built.

## Why not a subagent

A subagent cannot have its own backend — no frontmatter field sets a base URL, and its
`model:` only renames the model on the session's one backend (see [[claude-code-backend]]).
A gateway that routes by model name could split the traffic, but it needs a credential
variable, and that takes Opus off the subscription. Calling the model as a tool avoids both.

## Three ways to call it

The local server here is Ollama on `127.0.0.1:11434`.

1. **An MCP server** ([MCP](https://code.claude.com/docs/en/mcp.md)) exposing one tool — say
   `ask_local_model(prompt)` — that posts to Ollama and returns the answer. Opus decides when
   to delegate, and the tool's description is where that decision is steered. The cleanest
   of the three.
2. **A shell call** — Opus runs `curl` against Ollama from Bash, pre-approved with a
   permission rule ([permission rule syntax](https://code.claude.com/docs/en/settings-reference.md#permission-rule-syntax))
   so it does not prompt each time. No code to write; the prompt and the answer pass
   through the shell, quoting included.
3. **A child Claude Code** — `claude --bare -p "<prompt>" --settings <file>` started from
   Bash, with its own `ANTHROPIC_BASE_URL` ([headless](https://code.claude.com/docs/en/headless.md)).
   The child's environment is its own, so the parent keeps Opus. Bare mode reads no login,
   so the child needs a credential variable, and the child is itself a Claude Code session
   on a non-Claude backend — outside documented support, like the [[litellm-gateway]] route.

The Claude Code documentation is silent on this pattern as a whole. Its gateway disclaimer
is about where Claude Code sends its own requests; a tool that happens to call a model is a
different thing it does not discuss.

## What the local model can and cannot do here

- It sees only what Opus puts in the prompt — the code, the disassembly, the file. It has
  no tools of its own and no web access.
- It is far weaker than Opus. The case for it is a question Opus would decline, or
  material that must not leave the machine — not a better answer.
- The answer comes back to Opus as tool output, and Opus decides what to do with it.
