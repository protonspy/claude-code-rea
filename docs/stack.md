# Stack

Every adopted technology, with one line on why it earned its place. **Technology
that is not listed here is an open decision, never something adopted silently** —
and because this project's dependency file is structured data rather than source,
that rule is checkable: a direct dependency declared there and absent from here is
reported.

So adding a dependency is a two-step act: add it, and say here why.

```markdown
## Runtime

- **the-http-router** — routing with the standard library's handler contract, no
  framework underneath it.

## Development

- **the-test-runner** — what CI runs; the suite is the gate for a feature.
```

Group however the project actually thinks about it. What matters is that the name
appears and the reason is one line a reader can disagree with.

<!-- Entries go below this line. Delete the guidance above once they read for
themselves. -->

## The launcher

- **Go** — `rea-launcher` is one static binary per platform, with nothing to install beside it
  (adr:0002-launch-opencode-with-rea-from-a-go-launcher).
- **github.com/zalando/go-keyring** — the provider key in Windows Credential Manager, macOS
  Keychain or the Secret Service, without cgo, so the release builds cross-compile.
- **golang.org/x/term** — reads NEAR AI's key without echoing it; the standard library cannot.

## What the launcher installs and runs

Pinned and hash-checked in `internal/launcher/pins.go`; changing one is a release of the launcher.

- **OpenCode** — the harness (adr:0001-use-opencode-with-nanogpt-for-abliterated-models).
- **REA** (`rea-agents` on npm) — the reverse-engineering MCP server, its skills and its CLI.
- **Node.js** — REA's runtime, and the npm that installs REA from its verified tarball; the
  system's Node.js is not used.

## Delivery

- **GitHub Actions** — builds, vets and tests every pull request on Linux and Windows, and builds
  the release binaries with `SHA256SUMS` and a build provenance attestation.
