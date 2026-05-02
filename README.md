# cogo

A terminal-native agentic CLI for Go developers — think *Claude Code* but Go-native, built on the Google ADK and Gemini 3.x. Configurable per project via a `.agents/` directory, with first-class support for MCP servers and Claude-compatible skills.

> **Status:** V1 in active development. The walking skeleton (Slice 1) is up — `cogo -p "..."` works end-to-end. Interactive TUI, tools, skills, MCP, permissions, slash commands, and project memory are landing in subsequent slices. See [`docs/REQUIREMENTS.md`](./docs/REQUIREMENTS.md) for the full V1 scope.

## Why

Existing AI CLIs are great but Python-heavy and tightly coupled to one vendor. Cogo aims to give Go developers a single static binary that:

- Drives a multi-turn, tool-using conversation in the terminal.
- Reads its configuration, skills, and MCP servers from a project-local `.agents/` directory.
- Plugs into either the public Gemini API (key auth) or Vertex AI (ADC + GCP project), with the model abstraction designed to admit other providers later.
- Is built and shipped as one fast, dependency-free binary.

## Quickstart

Requires Go 1.24+ (see `go.mod`).

```bash
git clone https://github.com/go-steer/cogo
cd cogo

# Pick one auth path:

# 1) Public Gemini API
export GOOGLE_API_KEY=...

# 2) Vertex AI
gcloud auth application-default login
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=...
export GOOGLE_CLOUD_LOCATION=us-central1   # or "global"

go run ./cmd/cogo -p "What is 2+2?"
```

See [`.env.example`](./.env.example) for a copy-pasteable template.

## What works today (Slice 1)

- `cogo -p "<prompt>"` runs a single turn and streams the assistant's response to stdout.
- Both auth paths work (public Gemini API + Vertex AI).
- `.agents/config.json` is auto-discovered (walks up from the working directory like `.git`); falls back to built-in defaults when absent.
- Provider auto-detection from environment variables when `model.provider` is not set in config.

Interactive TUI, tools, MCP, skills, slash commands, project memory, and the permission system are not yet wired — those land in Slices 2–5.

## Tests

```bash
go test ./...                              # unit only, no network

COGO_E2E=1 \
GOOGLE_GENAI_USE_VERTEXAI=true \
GOOGLE_CLOUD_PROJECT=... \
GOOGLE_CLOUD_LOCATION=... \
  go test ./internal/headless/... -run E2E -v   # hits real Vertex
```

## Design

- [`docs/REQUIREMENTS.md`](./docs/REQUIREMENTS.md) — V1 scope, FRs and NFRs, resolved-decisions log.
- [`docs/DESIGN.md`](./docs/DESIGN.md) — architecture, configuration sketches, module layout, testing strategy.
- [`cmd/spike/`](./cmd/spike/) — throwaway program that validated the architecture against real Gemini 3.x before V1 implementation began. Will be deleted in Slice 5.

## License

Apache 2.0 — see [`LICENSE`](./LICENSE).
