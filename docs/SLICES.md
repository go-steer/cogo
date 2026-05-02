# V1 Implementation Slices

A roadmap for the remaining V1 work, layered as **vertical slices**: each slice ends in a runnable artifact that improves on the previous one, so we can stop, demo, or course-correct at any boundary.

The full V1 scope is in [`REQUIREMENTS.md`](./REQUIREMENTS.md); the architecture in [`DESIGN.md`](./DESIGN.md). This file is the *order of construction*.

| Slice | Status | Outcome |
|------:|--------|---------|
| 1 | ✅ shipped on `dev` | `cogo -p "<prompt>"` runs end-to-end against Gemini |
| 2 | next | `cogo` opens an interactive Bubble Tea TUI with streaming markdown |
| 3 |  | Agent gains tools (file / shell / web) gated by the permission system |
| 4 |  | Feature-complete V1 — MCP, skills, memory, slash commands, cost tracking, `cogo init` |
| 5 |  | Polish & ship — OTEL, transcripts, CI/release, delete the spike |

Each slice gets its own detailed plan written **at the start of that slice** (in the style of `~/.claude/plans/...md` for Slice 1), so plans reflect what we've actually learned by then. The sections below are scope outlines, not implementation plans.

---

## Slice 2 — Interactive TUI

**Goal.** `cogo` (no args) opens a Bubble Tea chat with streaming markdown rendering, multi-line input, and graceful interrupt.

**New / modified packages.**
- `internal/tui/` — Bubble Tea `Model` / `Update` / `View`, plus thin component wrappers around Bubbles' `viewport`, `textarea`, and `spinner`.
- `internal/tui/markdown.go` — Glamour renderer with terminal-theme detection.
- `internal/tui/cmd.go` — bridges `agent.Run`'s `iter.Seq2` into typed `tea.Msg` events (`streamChunkMsg`, `turnDoneMsg`, `errMsg`).
- `internal/tui/keys.go` — keybindings (Enter / Shift+Enter / Ctrl+C / Ctrl+L / `/` for command palette).
- `cmd/cogo/main.go` — when no `-p`, launch the TUI program instead of printing help-only.
- Minimal slash commands now: `/help`, `/clear`, `/quit` (full set in Slice 4).

**Decisions to lock at slice start.**
- How to bridge the agent's iterator to `tea.Msg`: goroutine + `program.Send`, or a `tea.Cmd` that drains and yields a "next" message recursively. Pick one and stick with it.
- Multi-line input affordance: `bubbles/textarea` (likely yes, supports Shift+Enter natively).
- Theme: rely on Lip Gloss adaptive colors with `auto` default; defer explicit light/dark switching.
- Message-history shape: tagged sum-type slice (`{user, assistant, system, error}`) so View can dispatch styling per-variant.
- Test approach: `bubbletea/teatest` for input dispatch, slash command parsing, and viewport scroll; `FakeModel` provides scripted responses.

**Acceptance.**
- `cogo` opens the TUI; user types prompt, hits Enter, assistant streams markdown; second prompt continues the session.
- Ctrl+C interrupts the current turn (cancels `ctx`); second press within ~1s exits the program.
- `/help`, `/clear`, `/quit` work; unknown `/foo` shows a friendly error in the chat.
- Window resize doesn't lose state.
- `go test ./internal/tui/...` passes via `teatest` with no real model.
- `cogo -p` headless mode unchanged.

**Deferred to later slices.** Tools, MCP, skills, `/model` picker, cost/token surfacing, permission modals, project memory loading.

---

## Slice 3 — Tools + Safety

**Goal.** Agent can read/edit files and run shell commands, with the permission system from REQUIREMENTS §3.10 fully wired (denylist, path scoping, ask/allow/yolo modes).

**New / modified packages.**
- `internal/tools/`
  - `file.go` — `read_file`, `write_file`, `edit_file`, `list_dir`.
  - `search.go` — `glob`, `grep` (use `io/fs` + ripgrep-like matching, no shell).
  - `bash.go` — single-command shell with timeout and built-in denylist.
  - `web.go` — `web_search`, `web_fetch` (likely Gemini's `GoogleSearch` for search; `net/http` for fetch).
  - `todo.go` — lightweight in-memory task list visible to the model.
  - `truncate.go` — output-cap decorator applied to every tool.
  - `register.go` — assembles the registry, applies truncation + permission gate.
- `internal/permissions/`
  - `gate.go` — central interception; consults policy chain.
  - `policy.go` — pattern matching for allow/deny lists.
  - `bash_denylist.go` — non-overridable dangerous patterns (`rm -rf /`, etc.).
  - `path_scope.go` — cwd + `~/.cogo/` check; 4-option modal escalation; persists "Always" choices.
  - `persist.go` — atomic temp-file + rename for config writes.
- `internal/agent/agent.go` — accept `tools []tool.Tool` and `gate permissions.Gate` Options.
- `internal/tui/` — permission modal (`y` / `n` / `a` and 4-option path-scope), tool badges (built-in vs MCP/skill styling), truncated-output indicator with "show full" expand.

**Decisions to lock at slice start.**
- Gate vs ADK confirmation: register every mutating tool with `RequireConfirmationProvider` that delegates to our gate. The gate produces the modal; provider returns true/false. Keeps ADK's HITL plumbing untouched.
- Path scoping placement: as a meta-decorator over file tools, so the per-tool implementations stay simple.
- Web search backend: prefer `geminitool.GoogleSearch` (free with the model) over external keys; revisit if quality is poor.
- Tool registry growth path: registration pattern (`tool.Register("read_file", ctor)`) so new tools land without editing `agent.New`.

**Acceptance.**
- Headless: `cogo -p "summarize file foo.md"` reads the file and answers.
- TUI: agent calls `bash git status`, modal pops, `y` runs it, `a` allows the exact call for the session.
- Path-scope modal: out-of-scope read pops 4-option modal; "Always allow this directory tree" persists to `.agents/config.json`.
- Tool output > 32 KB shows a `[truncated]` marker in model context AND collapsible UI badge.
- `bash rm -rf /` is denied in all modes (denylist non-overridable).
- `go test ./...` still green with `FakeModel`; new tool tests use temp dirs.

**Deferred to later slices.** MCP, skills, `/model`, cost surfacing, OTEL, transcript writes, subagents, plan mode.

---

## Slice 4 — Claude Code Parity

**Goal.** Feature-complete V1 per REQUIREMENTS.md.

**New / modified packages.**
- `internal/mcp/`
  - `config.go` — parse `.agents/mcp.json`.
  - `lifecycle.go` — spawn stdio servers, dial Streamable HTTP (POST + SSE).
  - `elicitation.go` — `mcp.ClientOptions{ElicitationHandler: ...}` bridges to TUI modal.
  - `register.go` — wrap each server's tools as an ADK toolset; namespace `<server>.<tool>`.
- `internal/skills/`
  - `discovery.go` — scan `.agents/skills/<name>/` and `~/.cogo/skills/`.
  - `schema.go` — parse Claude-compatible `SKILL.md` frontmatter (`name`, `description`, `allowed-tools`, etc.).
  - `register.go` — register each skill as an invocable tool whose body loads on call.
- `internal/memory/`
  - `load.go` — `AGENTS.md` → `CLAUDE.md` → `GEMINI.md` fallback chain (project + user-global).
- `internal/commands/`
  - Full slash-command dispatcher: `/help`, `/model` (`/models`), `/clear`, `/init`, `/mcp`, `/skills`, `/memory`, `/stats`, `/quit` (`/exit`).
- `internal/usage/`
  - `tracker.go` — per-turn + cumulative token + cost accounting.
  - `pricing.go` — built-in table for Gemini 3.x; overridable from `config.json`.
- `cmd/cogo/init.go` (or a small `cogo init` flow inside main) — silent default + `--interactive` wizard.
- `internal/tui/` — header gains running cost / token total; per-message footer with per-prompt usage; `/stats` rendered as a modal panel.

**Decisions to lock at slice start.**
- `cogo init --interactive` framing: Bubble Tea form (consistent with TUI) or stdlib `bufio.Scanner` prompts (simpler). Likely Bubble Tea since Bubbles already provide form components and it shares styling.
- `/model` picker UX: full-screen modal with arrow-key navigation, or inline `/model gemini-3.1-pro-preview` text command. Likely both: bare `/model` opens picker, `/model <id>` switches directly.
- Pricing source: built-in table with `pricing.go` as authority; explicit `model.pricing.{input_per_mtok, output_per_mtok}` in `config.json` wins. Refresh cadence: manual on each Gemini price change.
- Skill body load timing: lazy on first invocation vs eager on startup. Prefer lazy to keep cold-start fast.
- Whether to ship a starter `.agents/skills/example/` to make the format discoverable.

**Acceptance.**
- `cogo init` writes `.agents/{config.json, .gitignore, AGENTS.md}` with sensible defaults; `--interactive` walks through provider, model, permission mode.
- `/model` lists available Gemini 3.x models and switches mid-session; choice persists to `config.json`.
- `/mcp` shows configured servers + status; an in-process or stdio MCP server's tools become callable; elicitation works (modal pops, response returned).
- `/skills` lists discovered skills; agent invokes them when prompted.
- `/memory` shows which memory file loaded and from where; project memory is in the system prompt.
- `/stats` panel + per-message cost footer + exit summary all populated with real Gemini usage data.
- `go test ./...` still green; new `internal/mcp` tests use the in-memory MCP transport from the spike.

**Deferred to later slices.** OTEL, transcript persistence, JSONL debug logs, subagents, plan mode, hooks, user-defined commands, session resume, multimodal input.

---

## Slice 5 — Polish & Ship

**Goal.** A shippable `v0.1.0` binary with CI, releases, telemetry, and durable session artifacts.

**New / modified packages.**
- `internal/telemetry/` — OTEL setup that calls `providers.SetGlobalOtelProviders()` (per spike finding); env-driven exporter selection (`none` / `console` / `otlp`); `--otel=console` flag.
- `internal/session/` — transcript writer; persists `.agents/sessions/<timestamp>.json` on `/quit` and Ctrl+C; atomic temp + rename.
- `internal/log/` — `--debug` flips slog handler to a JSONL handler writing under `.agents/logs/<timestamp>.jsonl`.
- Top-level:
  - `.goreleaser.yaml` — cross-platform builds (linux/darwin × amd64/arm64); checksums; archive layout.
  - `.github/workflows/ci.yml` — `go test ./...` on push/PR across a small Go matrix.
  - `.github/workflows/release.yml` — runs goreleaser on tag push.
  - `AGENTS.md` at repo root — project memory for Cogo itself (so `cogo` in this repo loads sensible context).
  - README polish: install instructions, badges, gif/asciicast if practical.
- Removed: `cmd/spike/` and `cmd/probe-models/` (their findings live in docs and memory).

**Decisions to lock at slice start.**
- Distribution channels: GitHub Releases (definitely); Homebrew tap (likely yes — needs `homebrew-cogo` repo); Linux distro packaging (defer post-V1).
- Versioning: start at `v0.1.0`. Pre-1.0 means breaking changes allowed with documented changelog.
- CI matrix: Go 1.24 (floor) + 1.26 (current); OS: linux-amd64 + macos-latest. Windows best-effort, not blocking.
- Default OTEL exporter: `none`. `--otel=console` for dev. OTLP via `OTEL_EXPORTER_OTLP_ENDPOINT`.
- Session transcript schema: stable, versioned (`{"version": 1, ...}`); include user prompts, assistant text, tool calls + results, usage totals.

**Acceptance.**
- `goreleaser build --snapshot` produces working binaries for linux + macos.
- Tag push to GitHub triggers a release with cross-platform tarballs.
- Pull requests run CI; failures block merge.
- `cogo --debug -p "ping"` emits a JSONL debug log.
- `cogo` exit (via `/quit` or Ctrl+C) writes a transcript readable as JSON.
- `cogo --otel=console -p "ping"` prints OTEL spans to stderr; with `OTEL_EXPORTER_OTLP_ENDPOINT` set, spans go to the collector.
- An MCP server that calls `elicit` produces a TUI modal asking the user; on accept, the server gets the form data; on decline, it gets the same decline outcome it does today.
- `/reload` rebuilds the agent in place from a fresh read of `.agents/`; old MCP stdio child processes are torn down rather than leaked.
- `cmd/spike/` and `cmd/probe-models/` no longer exist.
- README showcases install + first prompt; links to docs.

**Out of V1 entirely (post-1.0 roadmap).** Subagents, plan mode, hooks, user-defined slash commands, session resume, persistent agent-managed memory, multimodal input (image paste / file drop), additional model providers (Anthropic / OpenAI / local Ollama). All scaffolded for in DESIGN §10 "Future Extensibility" — none touched in V1.
