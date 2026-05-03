# Cogo project memory

When `cogo` runs inside this repo, this file is loaded into the
agent's system prompt.

## What this project is

Cogo is a terminal-native agentic CLI written in Go — think
*Claude Code* but Go-native. It's built on the Google ADK
(`google.golang.org/adk`) and Google's GenAI Go SDK
(`google.golang.org/genai`), driving Gemini 3.x models. The TUI is
Bubble Tea + Bubbles + Lip Gloss + Glamour. Configuration lives in a
project-local `.agents/` directory.

The roadmap is in [`docs/SLICES.md`](./docs/SLICES.md); requirements
and design are in [`docs/REQUIREMENTS.md`](./docs/REQUIREMENTS.md) and
[`docs/DESIGN.md`](./docs/DESIGN.md).

## Layout

```
cmd/cogo/                Entry point (flag parsing, subcommand
                         dispatch, dispatching to TUI or headless).
internal/agent/          Thin wrapper over ADK's runner + llmagent.
internal/config/         .agents/config.json schema, discovery,
                         atomic Save.
internal/headless/       cogo -p "<prompt>" path.
internal/initcmd/        cogo init subcommand (silent + Bubble Tea
                         wizard).
internal/mcp/            MCP server lifecycle + namespace prefixing.
internal/memory/         AGENTS.md / CLAUDE.md / GEMINI.md fallback.
internal/models/         models.Provider interface + gemini provider
                         (public Gemini API + Vertex AI).
internal/permissions/    Permission gate (ask/allow/yolo modes,
                         bash denylist, path scope).
internal/session/        On-exit transcript writer.
internal/skills/         Claude-compatible SKILL.md loader.
internal/telemetry/      OpenTelemetry setup.
internal/testutil/       FakeModel for token-free tests.
internal/tools/          Built-in tool set (file, bash, todo) + the
                         GateToolset wrapper used by mcp + skills.
internal/tui/            Bubble Tea program: model/update/view,
                         palette overlays, modals.
internal/usage/          Token + cost tracking.
docs/                    REQUIREMENTS.md, DESIGN.md, SLICES.md.
```

## Build & test

```bash
go build ./cmd/cogo                # build the binary
go test ./...                      # full unit + teatest sweep, no network
```

Vertex e2e tests are gated behind `COGO_E2E=1` plus standard auth
env vars so the default test run never hits the network:

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=...
export GOOGLE_CLOUD_LOCATION=...
COGO_E2E=1 go test ./internal/headless/... -run E2E -v
```

## Conventions

- **Plan before non-trivial work.** Slices were designed in plan mode;
  features land as one focused commit per slice, with a brief plan
  documented in advance.
- **Small, self-contained commits with informative bodies.** Subject
  lines follow Conventional Commits (`feat:`, `fix:`, `chore:`,
  `docs:`). Bodies explain *why* and call out tests + verification.
  No `Co-Authored-By:` trailer.
- **Tests before merging.** Every new package ships with unit tests.
  `teatest` covers the TUI scenarios. `FakeModel` keeps the default
  test run network-free.
- **Errors flow to the user.** Agent / tool / config failures never
  panic — they surface as system / error messages in chat or as
  `cogo: ...` lines on stderr.
- **Gate everything that mutates.** Built-in mutating tools, MCP
  tools, and skill-invoked tools all pass through
  `permissions.Gate` so the same `ask` / `allow` / `yolo` semantics
  apply uniformly.

## Pitfalls & gotchas (real ones we've hit)

- ADK's `telemetry.New(...)` returns providers but does **not**
  install them as OTEL globals. Always call
  `providers.SetGlobalOtelProviders()`.
- ADK streaming requires `agent.RunConfig{StreamingMode: agent.StreamingModeSSE}`.
  The default `StreamingModeNone` produces no partial events.
- Glamour's `WithAutoStyle` fires an OSC-11 background-color query
  whose response races into the textarea once Bubble Tea owns stdin.
  Detect light/dark **once** before `tea.NewProgram`, then pass a
  fixed style name via `WithStandardStyle`.
- Don't enable `tea.WithMouseCellMotion` without a strong reason —
  capturing global mouse events breaks terminal-native text
  selection. We deliberately don't.
- Gemini function names must match `[A-Za-z0-9_]{1,64}` — no dots
  in MCP tool namespaces; use `<server>_<tool>` not `<server>.<tool>`.
- `t.Setenv` and `t.Parallel()` don't mix in Go's testing package.
- The MCP SDK's `Toolset.Tools(ctx)` requires an
  `agent.ReadonlyContext`, not a regular `context.Context`. We have a
  minimal stub in `internal/mcp/listctx.go`.

## Status

V1 (`v0.1.0`) shipped. Future work is tracked by GitHub milestones
`v0.2.0` through `v0.5.0`; see [`ROADMAP.md`](./ROADMAP.md) for the
release themes and links to the milestones.

## Branch workflow

Single long-lived branch: `main`. Work happens on short-lived feature
branches (`feat/...`, `fix/...`, `chore/...`, `docs/...`) → PR
against `main` → rebase merge (the only PR merge style enabled).
Branch protection on `main` requires the `test`, `lint`, `go mod tidy
is clean`, and `govulncheck` status checks; docs-only PRs satisfy
them via the companion `ci-docs.yml` workflow without running the
full Go pipeline. Commits must be DCO-signed off (`git commit -s`)
and follow Conventional Commits (the prefix drives the auto-generated
release changelog). See [`CONTRIBUTING.md`](./CONTRIBUTING.md) for
the full contributor flow.
