# cogo

A terminal-native agentic CLI for Go developers — think *Claude Code* but Go-native, built on the Google ADK and Gemini 3.x. Configurable per project via a `.agents/` directory, with first-class support for MCP servers and Claude-compatible skills.

> **Status:** V1 in active development. All Slice 4 features are up: full Claude-Code-like surface — TUI + tools + permissions + project memory + cost surfacing + `/model` picker + MCP servers + Claude-compatible skills + `cogo init` (silent + interactive wizard). Polish (OTEL, transcript persistence, goreleaser/CI, deletion of the spike binaries) lands in Slice 5. See [`docs/REQUIREMENTS.md`](./docs/REQUIREMENTS.md) for the full V1 scope and [`docs/SLICES.md`](./docs/SLICES.md) for the build order.

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

## What works today (Slices 1–3)

- `cogo -p "<prompt>"` runs a single turn and streams the assistant's response to stdout (Slice 1).
- `cogo` (no args, on a TTY) opens an interactive Bubble Tea chat: streaming text in real time, markdown rendering on completion, multi-line input (Shift+Enter for newline), and `/help` / `/clear` / `/quit` slash commands (Slice 2).
- Type `/` at the start of an empty prompt to open a slash-command palette; type `@` anywhere to open a file picker. Selecting a file inserts `@<path>` and the file contents are inlined when you submit the message.
- Built-in tools the agent can call: `read_file`, `write_file`, `edit_file`, `list_dir`, `bash`, `todo`. Tool output is truncated when it exceeds the per-tool caps in `tool_output` config (Slice 3).
- Permission system with `ask` / `allow` / `yolo` modes: an in-TUI modal prompts before mutating ops with **y** (allow once) / **s** (allow this session) / **a** (always allow, persisted) / **n** or **esc** (deny). A non-overridable bash denylist refuses things like `rm -rf /`. Path scoping confines file tools to the project root + `~/.cogo/` + any explicit `path_scope.allow` entries (Slice 3).
- Project memory (`AGENTS.md` → `CLAUDE.md` → `GEMINI.md` fallback at the project root, plus `~/.cogo/AGENTS.md` user-global) is loaded into the agent's system prompt at startup. Inspect via `/memory` (Slice 4a).
- Per-prompt + session-total token / cost surfacing: each completed assistant turn shows `↑in · ↓out · $cost` underneath; the header carries a running session total; headless `cogo -p` prints a one-line exit summary on stderr. Use `/stats` for a full breakdown (Slice 4a).
- Mid-session model switching: bare `/model` opens a picker; `/model gemini-3-flash-preview` switches directly. Persisted to `.agents/config.json` when a project config exists (Slice 4a).
- **MCP server integration**: declare stdio or Streamable HTTP MCP servers in `.agents/mcp.json`; their tools become callable in the agent loop. `/mcp` shows server status. Elicitation handler is wired (declines with a notice; full schema-form modal lands in Slice 5 polish) (Slice 4b).
- **Skills**: drop a Claude-compatible `SKILL.md` bundle under `.agents/skills/<name>/` and the agent can invoke it. `/skills` lists what's discovered. Body markdown loads lazily (Slice 4b).
- **`cogo init`**: scaffolds `.agents/{config.json, .gitignore, AGENTS.md}` for a fresh project. Refuses to overwrite without `--force`. `cogo init --interactive` walks a Bubble Tea wizard for provider / model / permission mode (Slice 4b).
- Ctrl+C cancels the current turn while streaming; a second press while idle exits.
- Non-TTY invocation (piped stdin, CI) prints a hint pointing at `-p` and exits non-zero rather than hanging.
- Both auth paths work (public Gemini API + Vertex AI).
- `.agents/config.json` is auto-discovered (walks up from the working directory like `.git`); falls back to built-in defaults when absent.
- Provider auto-detection from environment variables when `model.provider` is not set in config.

OTEL wiring, transcript persistence, the schema-driven elicitation modal, and goreleaser/CI land in Slice 5.

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
