---
autonomy: auto
ci: wait
---

# Readme

Give the repository a `README.md` and fill in its GitHub "About": a description and topics.

## Why

The repository has neither a README nor a description, so a visitor to GitHub sees a list of
directories and no word on what `rea-launcher` is, how to get it or what a session risks. Done
when the README answers those three from what `docs/` already says, and `gh repo view` shows the
description and topics.

## Paths

- `README.md`

## References

- `docs/wiki/pages/rea-launcher.md` — the source of the README's install, setup and run sections; linked, not copied whole
- `docs/adr/0001-use-opencode-with-nanogpt-for-abliterated-models.md`, `docs/adr/0002-launch-opencode-with-rea-from-a-go-launcher.md`, `docs/adr/0003-approve-launcher-sessions-automatically.md` — the decisions the README summarizes
- `docs/glossary.md` — the terms the README uses

## Out of scope

- A new release tag: `v0.3.0` already points at `main`, and a README alone is no release.
- Renaming the repository, which is `opencode-rea` on GitHub and `claude-code-rea` locally.

## Tasks

- [x] 1.1 (Unit) Write `README.md`: what the launcher is, getting and verifying a release, setup, running a session, the security warning of adr:0003, and links into `docs/`
- [x] 1.2 (Unit) Set the GitHub repository description and topics with `gh repo edit`
  _Depends 1.1_

## Done when

- `scc validate` and `scc check` exit 0.
- `gh repo view --json description,repositoryTopics` shows a description and the topics.
