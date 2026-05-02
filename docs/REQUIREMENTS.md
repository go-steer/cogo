# Requirements: Cogo

**Cogo** is a terminal-native agentic CLI written in Go. It aims to deliver a "Claude Code"–class experience — a multi-turn, tool-aware, visually polished TUI agent — backed by Google's Gemini 3.x models via the Go ADK, with first-class support for skills, MCP servers, and project-local configuration.

This document captures the functional and non-functional requirements. The **long-term vision** is broad parity with Claude Code's feature surface; the **V1 MVP** is a deliberately smaller slice marked inline with `[V1]` tags. Items marked `[Future]` are post-V1.

---

## 1. Vision & Goals

### 1.1 Vision
Give a developer a single-binary, terminal-first agent that:
- Reasons in a tool-using loop (file ops, shell, web, custom tools).
- Feels native to the terminal (Bubble Tea TUI, streamed responses, markdown).
- Is **configurable per project** via a `.agents/` directory checked into the repo (or git-ignored, user's choice).
- Is **extensible** through skills (declarative agent capabilities) and MCP servers (external tool providers).
- Is **portable across model providers** — Gemini 3.x first, with a clean abstraction for other backends later.

### 1.2 Primary Goals
1. Match the conversational fluency and tool-use loop that users expect from Claude Code.
2. Make the agent's behavior **inspectable and steerable** — visible tool calls, permission prompts, slash commands.
3. Keep the binary small, the startup fast, and the TUI responsive even during long-running tool calls.
4. Make project-level configuration the **primary unit of agent behavior** (so different repos can have different tools, skills, and models).

### 1.3 Non-Goals
- Web UI or hosted service. Cogo is a local CLI, period.
- A full IDE integration (LSP, debugger). The terminal *is* the UI.
- Reimplementing the model provider — we wrap ADK, not Gemini directly.
- Building a marketplace for skills/MCP servers (filesystem discovery is enough).

---

## 2. User Personas

- **Solo developer (primary):** Wants to invoke `cogo` in a project directory and chat with an agent that can read code, run commands, and edit files with permission.
- **Small team:** Wants to commit a `.agents/` directory so every teammate gets the same skill set, MCP servers, and model defaults.
- **OSS contributor (future):** Wants to write a custom skill or MCP server and drop it into a published `.agents/` directory.

---

## 3. Functional Requirements

### 3.1 Conversational TUI `[V1]`
- **FR-1.1** Render a chat-style interface with a scrollable history viewport, a fixed input area, and a header showing session info (model, working directory, token usage if available).
- **FR-1.2** Stream model output token-by-token into the viewport.
- **FR-1.3** Render assistant messages as markdown (code blocks syntax-highlighted via Glamour).
- **FR-1.4** Show distinct visual treatment for: user messages, assistant text, tool calls (with arguments), tool results, system notices, and errors.
- **FR-1.5** Support multi-line input (Shift+Enter for newline, Enter to submit) and command history (Up/Down arrows in the input).
- **FR-1.6** Show a spinner / "thinking" indicator while the agent loop is active; allow Ctrl+C to interrupt the current turn cleanly.
- **FR-1.7** Adapt to terminal resize events without losing state.
- **FR-1.8** Support a **non-interactive / headless mode** invoked as `cogo -p "prompt"` (or `--prompt`). In this mode Cogo skips the Bubble Tea TUI entirely, runs one full agent loop against the supplied prompt, streams the assistant response (and any tool-call summaries) to stdout, then exits with status 0 on success / non-zero on error. Designed for use in shell pipelines, CI, and scripts. Permission mode in headless runs defaults to `allow` (config-driven) — interactive prompts cannot be answered without a TTY, so `ask` falls back to deny-with-error.

### 3.2 Agent Loop (Google ADK) `[V1]`
- **FR-2.1** Use Google ADK for Go to drive the reason → act → observe loop.
- **FR-2.2** Surface every tool invocation in the chat as a "tool badge" with the tool name and a collapsed view of arguments. The result is shown beneath, also collapsible.
- **FR-2.3** Support arbitrary tool call depth in a single user turn (subject to a configurable max-step safety cap; default 50).
- **FR-2.4** When the agent loop completes, mark the turn done and re-enable input.
- **FR-2.5** All ADK calls run in a goroutine wrapped by a `tea.Cmd` so the UI stays responsive.

### 3.3 Model Management `[V1]`
- **FR-3.1** Default to Gemini 3.1 Pro (model ID `gemini-3.1-pro-preview`) on first run; allow Gemini 3 Flash (`gemini-3-flash-preview`) as a fast alternative. All Gemini 3.x models are currently in *preview*; drop the `-preview` suffix once Google publishes GA IDs.
- **FR-3.2** Support **two access paths to Gemini, both at V1**:
  - **Public Gemini API** — authenticated via API key (`GOOGLE_API_KEY` env var or `model.api_key` in config).
  - **Vertex AI** — authenticated via Google Cloud Application Default Credentials (`gcloud auth application-default login`, service-account key file, or workload identity), with the GCP project and location configured in `.agents/config.json` (or via `GOOGLE_CLOUD_PROJECT` / `GOOGLE_CLOUD_LOCATION` env vars).
- **FR-3.3** Provider is selected explicitly via `model.provider` in `config.json` (`"gemini"` for the public API, `"vertex"` for Vertex AI). When unset, Cogo auto-detects: if Vertex env vars or ADC are present, use Vertex; else if an API key is present, use the public API; else fail fast with a setup-instructions error.
- **FR-3.4** Provide a `/model` (alias `/models`) slash command that lists available models, shows the active provider, and lets the user switch model interactively. Provider switching is also supported from the same command.
- **FR-3.5** Persist the chosen model and provider to `.agents/config.json` so the next session starts with the same defaults.
- **FR-3.6** Credential & config resolution precedence (highest wins): explicit CLI flag → env var → `.agents/config.json` → user-global `~/.cogo/config.json` → built-in default.
- **FR-3.7 [Future]** Allow registering non-Gemini backends (Anthropic, OpenAI, local Ollama) behind the same model adapter interface used by the Gemini and Vertex providers. Cogo's core must not depend on Gemini-specific types outside the model adapter.

### 3.4 Configuration: the `.agents/` Directory `[V1]`
- **FR-4.1** Cogo looks for `.agents/` in the current working directory; if absent, it walks up to the nearest ancestor containing one (similar to `.git`).
- **FR-4.2** A `cogo init` command scaffolds `.agents/config.json` with sensible defaults and writes a `.agents/.gitignore` that excludes `sessions/` and `logs/`. The `.agents/` directory itself is intended to be committed — it is the shared, per-project unit of agent behavior — but per-user transcripts and debug logs stay local.
  - **Default (silent):** `cogo init` writes baseline files using built-in defaults (provider auto-detected, model = `gemini-3.1-pro`, permission mode = `ask`). Suitable for scripting and CI.
  - **Interactive:** `cogo init --interactive` walks the user through provider selection (Gemini API vs Vertex AI, with credential discovery hints), model choice, default permission mode, and whether to commit `.agents/` or add it to the repo `.gitignore`.
- **FR-4.3** Files Cogo reads from `.agents/`:
  - `config.json` — model, theme, permission defaults, max steps.
  - `mcp.json` — MCP server definitions (see 3.6).
  - `skills/` — directory of skill bundles (see 3.7).
  - `commands/` — directory of user-defined slash commands `[Future]`.
  - `sessions/` — saved conversation transcripts `[V1: write only]`.
  - `AGENTS.md` (project memory) — auto-loaded into the system prompt.
- **FR-4.4** A user-global `~/.cogo/` mirror provides fallback defaults; project-local always wins.
- **FR-4.5** All config files are JSON for V1 (TOML/YAML deferred). Schema versioned via a top-level `"version"` key.

### 3.5 Built-in Tools `[V1]`
Cogo ships a baseline tool set, all of them gated by the permission system (3.10):

- **FR-5.1** File system: `read_file`, `write_file`, `edit_file` (string-replace), `list_dir`, `glob`, `grep`.
- **FR-5.2** Shell: `bash` (single command, captured stdout/stderr, configurable timeout).
- **FR-5.3** Web: `web_search`, `web_fetch` — backed by ADK-provided implementations where available, otherwise pluggable.
- **FR-5.4** Task tracking: a lightweight `todo` tool the agent can call to surface its plan to the user.
- **FR-5.5** **Path scoping (file tools).** `read_file`, `write_file`, `edit_file`, `list_dir`, `glob`, and `grep` apply soft path scoping by default: any access outside the resolved project root *and* `~/.cogo/` is intercepted. The user is prompted (in TUI mode) with: *Allow once / Always allow this exact path / Always allow this directory tree / Deny*. "Always" choices append a pattern to `path_scope.allow` in `config.json` and persist for future sessions. A `path_scope.allow` allowlist can also be authored manually. In headless mode, out-of-scope access without an existing allowlist match fails with an error.
- **FR-5.6** **Bash safety.** The `bash` tool ships with a built-in **denylist** of patterns it always refuses (e.g. `rm -rf /`, `rm -rf ~`, `dd if=`, `:(){:|:&};:`, `curl ... | sh`, `wget ... | sh`, `chmod -R 777 /`). Anything not denylisted still runs through the standard permission gate (FR-10). No allowlist is required by default; users can opt into stricter allowlist-only mode via `permissions.bash_mode = "allowlist"` in `config.json`.
- **FR-5.7 [Future]** `subagent` tool that spawns a child agent with a scoped tool set (the Claude Code "Task" pattern).

### 3.6 MCP Server Integration `[V1]`
- **FR-6.1** Read MCP server definitions from `.agents/mcp.json`. Schema mirrors the conventional MCP config format (server name → stdio command + args + env, or HTTP endpoint + headers).
- **FR-6.2** Support transports at V1: **stdio** (child process) and **Streamable HTTP** (POST + Server-Sent Events response stream). Both transports are bidirectional, which is required for elicitation (FR-6.7).
- **FR-6.3** Discover the tools each server exposes and register them into the ADK tool registry, namespaced as `<server>.<tool>` to avoid collisions with built-ins.
- **FR-6.4** Surface MCP tool calls in the UI with a distinct badge style so the user knows the call left the local process.
- **FR-6.5** A `/mcp` slash command lists configured servers, their status (connected / failed), and the tools each exposes.
- **FR-6.6** Failures to start an MCP server must not crash Cogo — log, surface in `/mcp`, continue.
- **FR-6.7** Support **MCP elicitation** — when an MCP server requests structured input from the user mid-tool-call, Cogo renders a modal prompt in the TUI, validates the response against the server's schema, and returns it. Works over stdio and Streamable HTTP without additional transports. *Confirmed buildable in spike: `mcp.ClientOptions{ElicitationHandler: ...}` is supported by `github.com/modelcontextprotocol/go-sdk` v1.2+ and reachable through ADK's `mcptoolset` by passing a custom `mcp.Client` via `Config.Client`.*
- **FR-6.8 [Future]** WebSocket transport — not part of the current MCP spec. Add only if a published MCP server requires it.

### 3.7 Skills System `[V1]`
A "skill" is a self-contained capability bundle the agent can invoke. (Same conceptual model as Claude Code skills.)

- **FR-7.1** Skills live in `.agents/skills/<skill-name>/` and consist of at minimum a `SKILL.md` with YAML frontmatter. **The frontmatter schema mirrors Claude Code's skill format** (`name`, `description`, plus optional fields like `allowed-tools`, `model`, `version`) so skills authored for Claude Code work in Cogo without modification. The body is plain markdown; any references to Claude-specific tool names degrade gracefully (the skill still loads, the agent uses the closest equivalent or skips unknown tools).
- **FR-7.2** A skill may also bundle: scripts (executed via the shell tool), reference docs (loaded into context when invoked), and helper data files.
- **FR-7.3** On startup, Cogo scans the skills directory and registers each skill as an invocable tool whose description is the SKILL.md frontmatter. The skill body is loaded into context only when the agent decides to invoke it.
- **FR-7.4** A `/skills` slash command lists available skills with descriptions.
- **FR-7.5 [Future]** User-global skills in `~/.cogo/skills/` merge with project-local; project-local wins on name collision.

### 3.8 Slash Commands `[V1]`
Slash commands are typed at the input prompt and intercepted before the message reaches the agent.

- **FR-8.1** Built-in commands for V1:
  - `/help` — list all commands.
  - `/model` (alias `/models`) — list and switch models and providers.
  - `/clear` — clear conversation history (with confirmation).
  - `/init` — scaffold `.agents/` in the current directory.
  - `/mcp` — show MCP server status.
  - `/skills` — list available skills.
  - `/memory` — show which project/user memory file(s) were loaded.
  - `/stats` — show session token / cost / tool-call breakdown.
  - `/quit` (alias `/exit`).
- **FR-8.2 [Future]** User-defined commands as markdown files in `.agents/commands/` (filename = command name, body = prompt template).
- **FR-8.3 [Future]** A `/resume` command to load a previous session from `.agents/sessions/`.

### 3.9 Project Memory `[V1]`
- **FR-9.1** On startup, look for a project memory file in the resolved project root using this fallback chain (first match wins): `AGENTS.md` → `CLAUDE.md` → `GEMINI.md`. Load the first one found into the system prompt. The chain exists so Cogo can be dropped into a repo that already has memory authored for Claude Code or Gemini-based tools.
- **FR-9.2** Also load `~/.cogo/AGENTS.md` (user-global memory), prepended to the project memory in the system prompt. (User-global uses only the `AGENTS.md` name — no fallback chain.)
- **FR-9.3** A `/memory` slash command shows which memory file(s) Cogo loaded and from where.
- **FR-9.4 [Future]** A persistent agent-managed memory store (writable from the agent, similar to Claude Code's auto memory) under `.agents/memory/`.

### 3.10 Permission & Safety Model `[V1]`
- **FR-10.1** Three permission modes, switchable via a slash command or CLI flag: `ask` (prompt before every non-read tool call), `allow` (allowlist-driven), `yolo` (auto-approve everything; explicit opt-in).
- **FR-10.2** Default mode is `ask`. Read-only file ops and `web_search` are auto-approved by default; mutating ops (write, edit, bash, MCP tool calls) require approval in `ask` mode.
- **FR-10.3** Allowlists in `.agents/config.json` follow a simple pattern syntax (e.g. `bash:git status`, `bash:git diff*`, `write_file:src/**`).
- **FR-10.4** Denylists override allowlists.
- **FR-10.5** Permission prompts in the TUI are modal and answered with single keys (`y` / `n` / `a` for "always allow this exact call").
- **FR-10.6 [Future]** Hooks (`PreToolUse`, `PostToolUse`, `OnSessionStart`, etc.) configurable in `config.json`, executed as shell commands, able to block or modify tool calls.

### 3.11 Session Management & Usage Tracking
- **FR-11.1 [V1]** On `/quit`, Ctrl+C, or any graceful exit, write the session transcript to `.agents/sessions/<timestamp>.json` (atomic write via temp file + rename).
- **FR-11.2 [V1]** Surface per-prompt cost/token usage in the chat — each completed assistant turn shows a small footer with input tokens, output tokens, and estimated cost (when pricing is known for the active model).
- **FR-11.3 [V1]** Maintain a running session total in the header (cumulative tokens, cumulative cost).
- **FR-11.4 [V1]** On exit (`/quit` or Ctrl+C), print a final summary to the terminal: total turns, total tokens (input/output), total estimated cost, session duration.
- **FR-11.5 [V1]** A `/stats` slash command shows the current session breakdown on demand without exiting — totals, per-turn averages, per-tool call counts.
- **FR-11.6** Pricing tables for known models (Gemini 3.1 Pro/Flash) ship with Cogo and can be overridden in `config.json` (`model.pricing.{input_per_mtok, output_per_mtok}`) for custom backends or new models.
- **FR-11.7 [Future]** `cogo --resume` and `/resume` to reopen a previous session.

### 3.12 Subagents `[Future]`
- **FR-12.1** A `subagent` tool the parent agent can call to delegate a scoped task. The child runs with its own tool allowlist and returns a single textual result.
- **FR-12.2** Subagent definitions (system prompt, allowed tools) live in `.agents/agents/<name>.md`.

### 3.13 Plan Mode `[Future]`
- **FR-13.1** A mode (toggled by keybinding or slash command) where the agent is forbidden from mutating tools and must produce an approvable plan before being allowed to execute.

### 3.14 Tool Output Handling `[V1]`
Applies uniformly to built-in tools, MCP tools, and skill-invoked tools.

- **FR-14.1** Every tool result is subject to a size cap before it enters the model context. Caps are expressed as `max_bytes` and `max_lines` (whichever is hit first triggers truncation).
- **FR-14.2** A **global default** cap lives in `config.json` (`tool_output.max_bytes`, `tool_output.max_lines`). Reasonable defaults: 32 KB / 500 lines.
- **FR-14.3** Per-tool overrides live in `config.json` (`tool_output.per_tool.<tool_name>.{max_bytes,max_lines}`). Sensible built-in defaults: `bash` → 64 KB / 2000 lines; `read_file` → 256 KB / 5000 lines; `grep` → 16 KB / 200 lines.
- **FR-14.4** When truncation occurs, the model sees a clear marker (`... [truncated: N more lines / M more bytes; use offset/limit or a more specific query]`) and the UI shows a "truncated" badge on the tool result with an option to expand the full output locally.
- **FR-14.5** The unredacted full output is preserved on disk in the session transcript so post-hoc review (and `/stats` accounting) is accurate.

### 3.15 Slash Commands — additions
The full `/` command list including all V1 additions: `/help`, `/model` (`/models`), `/clear`, `/init`, `/mcp`, `/skills`, `/memory`, `/stats`, `/quit` (`/exit`).

---

## 4. Non-Functional Requirements

### 4.1 Performance
- **NFR-1** TUI must remain interactive (input, scroll, quit) during any long-running tool call or model stream.
- **NFR-2** Cold start to first prompt: < 200 ms on a modern laptop with no MCP servers configured.
- **NFR-3** MCP server startup is parallelized; a slow MCP server must not block the prompt from appearing.
- **NFR-4** For sessions exceeding ~100 turns, history rendering switches to a windowed view (don't re-render the whole transcript on every keystroke).

### 4.2 Reliability
- **NFR-5** A panic in any tool call must be caught and surfaced as a tool error, not crash the TUI.
- **NFR-6** A dropped network / model API failure must surface a clear error in the chat and leave the user able to retry the turn.
- **NFR-7** All file writes go through a temp-file + rename pattern so a crash mid-write can't corrupt config or session files.

### 4.3 Portability
- **NFR-8** Single static binary for Linux (amd64, arm64) and macOS (amd64, arm64). Windows is best-effort for V1.
- **NFR-9** No external runtime dependencies (Go static link). MCP servers are the user's problem.

### 4.4 Observability
- **NFR-10** A `--debug` flag writes a structured log (JSONL) of every tool call, model request, and error to `.agents/logs/<timestamp>.jsonl`.
- **NFR-11** A `/debug` toggle in the TUI shows the raw last model response (useful for prompt iteration).
- **NFR-12** Cogo emits **OpenTelemetry traces** for the agent loop, tool calls, MCP server interactions, and model requests, leveraging the Go ADK's built-in OTEL instrumentation. Spans cover: per-turn span (root), per-step span (model call + tool call), per-tool span (with attributes for tool name, namespace, truncation flag, byte size).
- **NFR-13** OTEL is **off by default** (no exporter configured = no network egress). Users opt in by setting standard OTEL env vars (`OTEL_EXPORTER_OTLP_ENDPOINT`, etc.) or via `config.json` (`otel.exporter`, `otel.endpoint`). Cogo never ships traces to any Cogo-operated endpoint — telemetry is strictly under user control.
- **NFR-14** A `--otel=console` shortcut writes spans to stderr for local debugging without needing a collector.

### 4.5 Security
- **NFR-15** API keys are never written to session transcripts, debug logs, or OTEL spans.
- **NFR-16** Permission prompts default to "no" if the user just hits Enter.
- **NFR-17** MCP servers run as child processes with the same OS-level privileges as Cogo; we do not sandbox further in V1, but document this clearly.

### 4.6 Testability
- **NFR-18** A `FakeModel` adapter implements the `models.Provider` interface and returns scripted responses, enabling agent-loop and tool-sequencing tests without burning real tokens.
- **NFR-19** A fake MCP server fixture (in-memory bidirectional pipe) supports MCP client tests including elicitation flows.
- **NFR-20** TUI behavior is testable via Bubble Tea's `teatest` package; at minimum, the input dispatch, slash command parsing, permission modal, and viewport scrolling have integration tests.
- **NFR-21** Real-Gemini integration tests exist but are gated behind `COGO_E2E=1` so the default `go test ./...` run never hits the network.
- **NFR-22** Internal logging uses `log/slog` (Go stdlib). The `--debug` flag swaps the slog handler to a JSONL file handler writing to `.agents/logs/<timestamp>.jsonl`. OTEL spans are emitted independently and are not duplicated as log records.

---

## 5. V1 Scope Summary

The MVP that exits "private dogfood" status:

- Bubble Tea TUI with streaming, markdown, tool badges (3.1).
- ADK-driven agent loop with Gemini 3.1 Pro/Flash, accessible via **both** the public Gemini API and Vertex AI, switchable via `/model` (3.2, 3.3).
- `.agents/` directory with `config.json`, `mcp.json`, `skills/`, `AGENTS.md`, and write-only `sessions/` (3.4).
- Built-in tool set: file I/O, bash, web search, web fetch, todo (3.5).
- MCP server support — stdio and Streamable HTTP, with elicitation, with `/mcp` (3.6).
- Skills loaded from `.agents/skills/` with Claude-compatible `SKILL.md` schema, with `/skills` (3.7).
- Slash commands listed in 3.8 FR-8.1, including `/stats`.
- Project memory via `AGENTS.md` with `CLAUDE.md` / `GEMINI.md` fallback (3.9).
- Permission system with `ask` / `allow` / `yolo` modes (3.10).
- Per-prompt and session-total cost/token tracking, exit summary, `/stats` command (3.11).
- Tool output truncation with global default + per-tool overrides (3.14).
- OpenTelemetry instrumentation, off by default, user-controlled exporter (NFR-12 to 14).
- Non-interactive headless mode `cogo -p "prompt"` (FR-1.8).
- Soft path scoping for file tools with prompt-to-allowlist UX (FR-5.5).
- Built-in bash denylist for obvious footguns (FR-5.6).
- Hybrid `cogo init` (silent default + `--interactive` wizard) (FR-4.2).

Everything tagged `[Future]` is explicitly out of V1.

---

## 6. Resolved Decisions

No outstanding open questions for V1 scope. Decisions made during requirements review:

- **Provider support:** Both public Gemini API and Vertex AI are first-class at V1 (FR-3.2).
- **`.agents/` versioning:** Committed by default; `cogo init` writes a `.gitignore` for `sessions/` and `logs/` (FR-4.2).
- **Skill format:** `SKILL.md` frontmatter mirrors Claude Code's schema; existing Claude skills work in Cogo unmodified (FR-7.1).
- **MCP transports:** stdio + Streamable HTTP at V1, with elicitation support; WebSocket deferred until a real server requires it (FR-6.2, FR-6.7, FR-6.8).
- **Project memory naming:** `AGENTS.md` primary, with fallback chain `CLAUDE.md` → `GEMINI.md`; first match wins (FR-9.1).
- **Telemetry:** Strictly local — no Cogo-operated endpoint. OpenTelemetry instrumentation via ADK, off by default, user-configured exporter (NFR-12 to 14).
- **Tool output truncation:** Global defaults in `config.json` with per-tool overrides; truncation marker visible to model and a "truncated" badge in UI; full output preserved in session transcript (FR-14).
- **Cost/token surfacing:** V1 — per-prompt footer, header running total, exit summary, `/stats` command (FR-11.2 to 11.5).
- **Multimodal input:** Deferred from V1; revisit when a concrete use case appears.
- **License:** Apache 2.0. Source headers and `LICENSE` file added to the repo before V1.
- **Go version:** Latest stable Go (1.24+ floor; pin exact minimum to whatever the ADK spike validates).
- **Build tooling:** `go build` for dev, `goreleaser` for cross-platform release artifacts. No Makefile unless one becomes necessary.
- **MCP elicitation:** Confirmed buildable in V1 — MCP SDK v1.2+ exposes `ElicitationHandler` via `mcp.ClientOptions`; reachable through `mcptoolset.Config{Client: ...}`. (Validated in cmd/spike on 2026-05-02.)
- **Non-interactive mode permission default:** `allow` (config-driven), since `ask` cannot be answered without a TTY.
- **`cogo init` mode:** Hybrid — silent by default, `--interactive` flag for the wizard.
