# Wiki

The entry point. Every page under `wiki/pages/` has to be reachable from here —
directly, or through a page that is — because a page nothing links to is a page
nobody will find again.

Pages link to each other as `[[page-slug]]`, where the slug is the filename without
its extension and without its directory. A link that resolves to no page is reported,
and so is a page this index cannot reach.

This file and `changelog.md` live here rather than in `pages/`: they are the wiki's
fixed documents, not pages, and neither is ever an orphan.

## Pages

<!-- One line per page, as `- [[page-slug]] — what it covers`. Group them under
their own headings once there are enough that a flat list stops helping. -->

### Connecting Claude Code to other models

- [[claude-code-backend]] — the formats Claude Code speaks, the variables that redirect it, what that does to the subscription, and why a session has one backend
- [[litellm-gateway]] — translating Claude Code to an OpenAI-compatible API, with near.ai's abliterated DeepSeek V4.1 Flash as the worked example
- [[local-model-as-tool]] — keeping Opus on the subscription and calling a local Ollama model through MCP, the shell or a child session
