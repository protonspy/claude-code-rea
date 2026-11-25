# Glossary

One canonical term per concept, and the synonyms nobody should use for it. Domain
vocabulary drifts by default — three names for one thing appear within a week of two
people working in parallel — so this file picks the name and the rest become
findings.

One entry per line:

```markdown
- **order total** — the amount charged, in minor units. Avoid: grand total, sum
```

The term is bold, the definition follows an em dash, and `Avoid:` lists the
synonyms it replaces, comma-separated and optional. Every synonym listed here is
reported wherever it appears as a whole word under `docs/`, so list one only when
you mean it.

Reach for the `glossary` skill when a term is coined, contested, or renamed. Define
what the thing *is*, not what it is called, and do not define what is obvious to
anyone in the domain.

<!-- Entries go below this line. Delete the guidance above once they read for
themselves. -->

- **abliterated model** — a model whose refusal direction was removed from its weights without retraining, so it answers what the original would decline.
- **backend** — the endpoint Claude Code sends its model requests to; Anthropic's API unless `ANTHROPIC_BASE_URL` or a cloud provider variable moves it.
- **credential variable** — `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_API_KEY` or `apiKeyHelper`; any one of them replaces the subscription login for the whole session.
- **gateway** — a proxy that speaks Anthropic Messages to Claude Code and calls a provider in that provider's own format; LiteLLM here. Avoid: router
- **subscription login** — the claude.ai login Claude Code uses when no credential variable is set.
