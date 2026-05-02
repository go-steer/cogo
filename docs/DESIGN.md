This `DESIGN.md` describes the architecture for **Cogo**, a high-performance, visually polished agentic CLI built with the Go ADK and the Charm Bracelet ecosystem. It mirrors the structural patterns of modern tools like Claude Code and is the implementation companion to [REQUIREMENTS.md](./REQUIREMENTS.md).

---

# Technical Design: Cogo

## 1. Overview
Cogo is a terminal-native agent: a single Go binary that launches a TUI in the user's working directory and drives a multi-turn, tool-using conversation with a Gemini 3.x model via the Google ADK. It is configured per-project through a `.agents/` directory that holds the model selection, MCP server definitions, skills, and project memory.

The design goals are:
- **Responsive TUI** — every long-running operation runs off the UI thread.
- **Composable tool surface** — built-in tools, MCP servers, and skills all funnel into one ADK tool registry.
- **Project-local config** — the `.agents/` directory is the unit of agent behavior; the binary itself is stateless.
- **Provider-pluggable** — Gemini 3.1 first, but the model layer is an interface so other backends can slot in later.

## 2. Tech Stack
*   **Language:** Latest stable Go (1.24+ floor; final minimum pinned by the ADK spike).
*   **License:** Apache 2.0.
*   **Build:** `go build` for development, `goreleaser` for cross-platform release artifacts. No Makefile unless one becomes necessary.
*   **TUI Framework:** [Bubble Tea](https://github.com/charmbracelet/bubbletea) (The Elm Architecture in Go).
*   **Styling:** [Lip Gloss](https://github.com/charmbracelet/lipgloss) for layout and adaptive colors.
*   **Components:** [Bubbles](https://github.com/charmbracelet/bubbles) for viewports, text inputs, and spinners.
*   **Agent Logic:** [Google ADK for Go](https://github.com/google/adk-go) (`github.com/google/adk-go` v1.2.0+; vanity `google.golang.org/adk`) for model orchestration and tool calling, layered over the [Google GenAI Go SDK](https://pkg.go.dev/google.golang.org/genai) (`google.golang.org/genai`).
*   **Markdown Rendering:** [Glamour](https://github.com/charmbracelet/glamour) for syntax-highlighted agent responses.
*   **Default Model:** Gemini 3.1 Pro (model ID `gemini-3.1-pro-preview`) with Gemini 3 Flash (`gemini-3-flash-preview`) as the fast-mode alternative. Accessible via **either** the public Gemini API (API key) or Vertex AI (Google Cloud project + ADC). Both providers are first-class at V1 and selected via `model.provider` in `config.json`. Note: all Gemini 3.x models are currently in *preview* — switch to GA IDs (drop the `-preview` suffix) when Google publishes them. Additional model providers can be added later behind the same `models.Provider` interface.

---

## 3. System Architecture

### 3.1. The Model-View-Update (MVU) Loop
The system operates as a state machine where every event (keypress, agent response, tool result, window resize) produces a new state.

*   **Model:** Holds the application state — chat history, input buffer, current loading/streaming status, tool execution log, active permission prompt (if any), session metadata.
*   **Update:** Processes messages. When an agent loop is triggered, it returns a `tea.Cmd` that runs the ADK call in a goroutine; tool calls and stream chunks come back as `tea.Msg` values that the Update function folds into state.
*   **View:** Renders the current state. Layout uses a Bubbles `viewport` for scrollable history and a fixed `textinput` at the bottom; modal overlays (permission prompts, model picker) are rendered above the chat.

### 3.2. Agentic Loop & Tool Integration
The **Google ADK** owns the reasoning cycle. Cogo wires its tool registry, model adapter, and streaming sink into ADK and lets ADK drive.

1.  **Reasoning:** The model (Gemini 3.x by default) decides whether to call a tool or respond directly.
2.  **Execution:** ADK invokes the resolved tool — a built-in Go function, an MCP-server-backed tool, or a skill invocation.
3.  **Observation:** Tool results are appended to the model context.
4.  **Loop:** Repeats until the model emits a terminal response or the configured max-step cap (default 50) is hit.
5.  **Final Response:** Streamed into the chat viewport as it arrives.

The agent loop is wrapped in a `tea.Cmd` so the UI thread stays free for input, scrolling, and Ctrl+C interrupts.

### 3.3. Tool Registry
A single registry presents a unified view of tools to ADK, populated from three sources at startup:

| Source       | Where defined                          | Namespacing             |
|--------------|----------------------------------------|-------------------------|
| Built-ins    | Go code in `internal/tools/`           | bare names (`read_file`, `bash`, …) |
| MCP servers  | `.agents/mcp.json`                     | `<server>.<tool>`       |
| Skills       | `.agents/skills/<name>/SKILL.md`       | `skill.<name>`          |

The registry is the single chokepoint where the **permission system** intercepts calls (see 3.6).

### 3.4. Configuration Layer
On startup, Cogo:
1. Walks up from the CWD to find the nearest `.agents/` directory (analogous to `.git` discovery). If none exists, it falls back to user-global `~/.cogo/` and runs without project config.
2. Loads `.agents/config.json` (validated against a versioned schema).
3. Loads `.agents/mcp.json` and spawns/connects to each server in parallel.
4. Scans `.agents/skills/` and registers each skill as a tool.
5. Loads `AGENTS.md` (project memory) and `~/.cogo/AGENTS.md` (user memory) into the system prompt.

User-global defaults in `~/.cogo/` are merged with project-local; project-local always wins on conflicts.

### 3.5. Slash Command Dispatcher
Input lines beginning with `/` are intercepted before being sent to the agent. The dispatcher:
- Resolves the command name against a built-in command table.
- Falls back to user-defined commands in `.agents/commands/` (post-V1) — these expand into prompt templates.
- Returns a `tea.Cmd` that mutates state directly (e.g. `/clear`, `/model`) or emits a system message into the chat.

Built-in V1 commands: `/help`, `/model` (`/models`), `/clear`, `/init`, `/mcp`, `/skills`, `/memory`, `/stats`, `/quit` (`/exit`).

### 3.6. Permission System
The permission gate hooks into ADK's built-in human-in-the-loop ("HITL") mechanism rather than wrapping tool calls externally. Tools register with `functiontool.Config{RequireConfirmation: true}` (or `RequireConfirmationProvider` for dynamic decisions); when the agent calls such a tool, the runner emits an event with the tool's call ID listed in `event.LongRunningToolIDs` instead of executing it. Cogo intercepts that event, runs the gate below, and (on approval) supplies the result via `runner.WithStateDelta` on the next iteration.

Every tool call dispatched by ADK passes through this gate before execution. The gate consults, in order:
1. The configured mode (`ask` / `allow` / `yolo`).
2. **Bash denylist** (built-in patterns plus user-config additions) — always rejects matches with no prompt. Denylist patterns: `rm -rf /`, `rm -rf ~`, `rm -rf $HOME`, `dd if=/dev/`, `:(){:|:&};:`, `mkfs`, `chmod -R 777 /`, `curl ... | sh`, `wget ... | sh`, and similar shell-pipeline-execution footguns. The denylist is non-overridable by config (a deliberately paternalistic floor).
3. **Path scope check** for file tools (`read_file`, `write_file`, `edit_file`, `list_dir`, `glob`, `grep`): the resolved absolute path must lie inside the project root or `~/.cogo/`, OR match a pattern in `path_scope.allow` from `config.json`. Out-of-scope access is escalated to a prompt (in `ask`) with four choices: *Allow once / Always allow this exact path / Always allow this directory tree / Deny*. "Always" choices append to `path_scope.allow` and are persisted by atomic config rewrite.
4. The denylist in `config.json` (always rejects).
5. The allowlist in `config.json` (auto-approves).
6. A built-in default policy (read-only ops auto-approved; mutating ops require approval in `ask` mode).
7. If still unresolved in `ask` mode: pop a modal in the TUI, await `y` / `n` / `a` (always-allow-this-call).

In `yolo` mode steps 3–7 collapse to "approve" — but step 2 (bash denylist) still applies. In headless mode any prompt is unanswerable, so unresolved checks fail with a clear error that names the missing allowlist entry the user could add.

The gate runs in the goroutine driving the ADK loop. Modal prompts are dispatched as `tea.Msg` values so the UI thread renders them.

### 3.7. Streaming Pipeline
The ADK `runner.Runner.Run` returns an `iter.Seq2[*session.Event, error]`. Streaming is **opt-in** via `agent.RunConfig{StreamingMode: agent.StreamingModeSSE}` (the default `StreamingModeNone` only emits the final consolidated event). With SSE on, intermediate events arrive with `event.Partial == true` carrying token chunks; the final event has `Partial == false` and `TurnComplete == true`.

Cogo's TUI runner wraps the iterator: each event becomes a `tea.Msg`. Partial-text events are appended to the in-progress assistant message; tool-call/tool-result events become tool badges; long-running events (`len(event.LongRunningToolIDs) > 0`) trigger the permission modal flow.

### 3.8. MCP Client & Elicitation
Cogo uses the official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk/mcp` v1.2+), wrapped through ADK's `tool/mcptoolset` package. Each MCP server runs as either a child process (stdio) or an HTTP client connection (Streamable HTTP — POST + SSE). Both transports are bidirectional, which is what enables MCP **elicitation**: a server may, mid-tool-execution, request structured input from the user.

Elicitation is wired by constructing the MCP client manually and passing it via `mcptoolset.Config{Client: ...}`:

```go
client := mcp.NewClient(impl, &mcp.ClientOptions{
    ElicitationHandler: func(ctx context.Context, req *mcp.ElicitRequest) (*mcp.ElicitResult, error) {
        // Render TUI modal, validate against req.Schema, return Action="accept"+Content,
        // or "decline" / "cancel".
    },
})
ts, _ := mcptoolset.New(mcptoolset.Config{Client: client, Transport: ...})
```

When an elicitation request arrives, the handler emits an `elicitationMsg` into the Bubble Tea program; the Update function pushes a modal onto the UI, awaits the user's choice, and returns the `*ElicitResult` synchronously back to the SDK. The agent loop pauses on that tool call until the response is delivered or the user cancels.

WebSocket transport is **not** part of the current MCP spec and is not implemented; if a future MCP transport spec adds it, the same `mcp.Client` abstraction can absorb it without changes elsewhere.

### 3.9. Tool Output Handling
A wrapper around every tool result enforces size caps before the result is fed back to the model:

1. The wrapper looks up `tool_output.per_tool.<name>` in `config.json`, falling back to the global `tool_output.{max_bytes,max_lines}` defaults.
2. If the result exceeds either cap, the wrapper truncates and appends a marker:
   `... [truncated: 1840 more lines / 142 KB more — narrow your query or read the file in chunks]`
3. The truncated payload is what the model sees. The **full** unredacted payload is written to the in-memory transcript so it ends up in `.agents/sessions/<timestamp>.json`, and the UI offers a "show full output" expand action.
4. Truncation events are exported as OTEL span attributes (`tool.output.truncated=true`, `tool.output.original_bytes=N`).

Default caps: 32 KB / 500 lines globally; `bash` 64 KB / 2000 lines; `read_file` 256 KB / 5000 lines; `grep` 16 KB / 200 lines. All overridable.

### 3.10. Cost & Usage Tracking
Every model response carries usage metadata (input/output token counts) which ADK surfaces. Cogo:

- Multiplies tokens by the active model's `pricing.input_per_mtok` / `pricing.output_per_mtok` (built-in defaults shipped for Gemini 3.1 Pro/Flash; overridable in `config.json`).
- Renders a small footer under each completed assistant turn: `↑ 1,247 in · ↓ 384 out · $0.0019`.
- Maintains a running session total in the model state, displayed in the header.
- On exit (`/quit` or Ctrl+C), prints a final summary line to the terminal **after** the TUI tears down, so it remains in scrollback.
- The `/stats` command re-renders the breakdown on demand: per-turn averages, per-tool call counts, total cost, session duration.

### 3.11. Headless Mode
When invoked as `cogo -p "prompt"` (or `--prompt`), Cogo skips the Bubble Tea program entirely. The bootstrap path is:

1. Load config and discover `.agents/` (same as TUI mode).
2. Initialize the model adapter, tool registry, MCP clients, and skills.
3. Construct the agent with the supplied prompt as the first user message.
4. Stream the assistant response to `stdout` via the same model-adapter `io.Writer` used in TUI mode (no markdown rendering — raw text plus a brief one-line summary per tool call to `stderr`).
5. On agent loop completion: write the session transcript, print the cost/usage summary to `stderr`, exit `0`. On error, exit non-zero with a clear message.

Permission resolution in headless mode behaves as if `ask` had no TTY available: out-of-scope access without a matching allowlist entry fails fast rather than blocking forever. The recommended pattern for CI is to set `permissions.mode = "allow"` plus an explicit allowlist in the project's `config.json`.

### 3.12. Observability (OpenTelemetry)
Cogo wires the Go ADK's built-in OpenTelemetry instrumentation into the standard `go.opentelemetry.io/otel` global tracer. Initialization requires **two steps**: `telemetry.New(ctx, opts...)` constructs the providers, and `providers.SetGlobalOtelProviders()` installs them as globals so ADK's instrumentation can find them. ADK does not auto-install — calling only `New` produces no spans.

ADK emits spans named (observed in the spike):
- `invoke_agent` — one per agent run, attrs include the agent name.
- `generate_content` — one per model call, attrs include the model ID.
- Transport-level spans (`HTTP POST` etc.) from the underlying genai SDK.

Cogo wraps **its own** spans on top of ADK's to express the structure documented in REQUIREMENTS NFR-12:
- `cogo.session` (root, lifetime of the process)
  - `cogo.turn` (one per user message)
    - ADK's `invoke_agent` and `generate_content` nest underneath
    - `cogo.tool_call` (attrs: `tool.name`, `tool.namespace` ∈ `{builtin,mcp,skill}`, `tool.output.bytes`, `tool.output.truncated`, `permission.outcome`)

By default, the OTEL SDK is initialized with **no exporter** — no spans leave the process. Users opt in by:
- Setting standard OTEL env vars (`OTEL_EXPORTER_OTLP_ENDPOINT`, `OTEL_SERVICE_NAME`, etc.), or
- Setting `otel.exporter` in `config.json` (`"otlp"`, `"console"`, `"none"`), or
- Passing `--otel=console` to dump spans to stderr.

Cogo never ships traces to any Cogo-operated endpoint. Telemetry is strictly under user control.

---

## 4. Configuration & File Layout

### 4.1 The `.agents/` directory
```
.agents/
├── .gitignore           # written by `cogo init`; excludes sessions/ + logs/
├── config.json          # model, theme, permissions, max steps   [V1]
├── mcp.json             # MCP server definitions                 [V1]
├── AGENTS.md            # project memory (loaded into system prompt) [V1]
├── skills/              # one subdir per skill                   [V1]
│   └── <skill-name>/
│       ├── SKILL.md
│       └── ...assets
├── sessions/            # transcripts written on exit (gitignored) [V1]
│   └── 2026-05-02T10-00-00.json
├── commands/            # user-defined slash commands            [Future]
│   └── <name>.md
├── agents/              # subagent definitions                   [Future]
│   └── <name>.md
└── logs/                # debug logs, --debug (gitignored)       [V1]
    └── 2026-05-02T10-00-00.jsonl
```

**Versioning convention:** `.agents/` is intended to be committed — it is the shared per-project unit of agent behavior. The `.gitignore` written by `cogo init` keeps per-user state (`sessions/`, `logs/`) out of version control.

User-global mirror: `~/.cogo/` with the same layout, merged underneath project-local.

### 4.2 `config.json` (sketch)

Public Gemini API:
```json
{
  "version": 1,
  "model": {
    "provider": "gemini",
    "name": "gemini-3.1-pro-preview",
    "pricing": {
      "input_per_mtok":  1.25,
      "output_per_mtok": 5.00
    }
  },
  "permissions": {
    "mode": "ask",
    "allow": ["bash:git status", "bash:git diff*"],
    "deny":  ["bash:rm -rf*"]
  },
  "agent": {
    "max_steps": 50
  },
  "tool_output": {
    "max_bytes": 32768,
    "max_lines": 500,
    "per_tool": {
      "bash":      { "max_bytes": 65536,  "max_lines": 2000 },
      "read_file": { "max_bytes": 262144, "max_lines": 5000 },
      "grep":      { "max_bytes": 16384,  "max_lines": 200 }
    }
  },
  "otel": {
    "exporter": "none",
    "endpoint": ""
  },
  "ui": {
    "theme": "auto"
  }
}
```

Vertex AI variant (only the `model` block changes):
```json
{
  "model": {
    "provider": "vertex",
    "name": "gemini-3.1-pro-preview",
    "vertex": {
      "project":  "my-gcp-project",
      "location": "us-central1"
    }
  }
}
```

API keys are never written to `config.json` by Cogo itself — they are read from `GOOGLE_API_KEY` (public API) or resolved via Application Default Credentials (Vertex). Users may set `model.api_key` in `config.json` manually if they want, but the file should then be excluded from version control.

**Provider auto-detection** (when `model.provider` is unset): if `GOOGLE_GENAI_USE_VERTEXAI=true` or ADC is configured for a project, use Vertex; else if `GOOGLE_API_KEY` is set, use the public API; else fail fast with a setup-instructions error.

### 4.3 `mcp.json` (sketch)
```json
{
  "version": 1,
  "servers": {
    "github": {
      "transport": "stdio",
      "command": "mcp-github",
      "args": ["--read-only"],
      "env": { "GITHUB_TOKEN": "${env:GITHUB_TOKEN}" }
    },
    "internal-api": {
      "transport": "http",
      "url": "https://internal.example.com/mcp",
      "headers": { "Authorization": "Bearer ${env:INTERNAL_TOKEN}" }
    }
  }
}
```

### 4.4 Skill layout (sketch)
```
.agents/skills/db-migrations/
├── SKILL.md     # frontmatter: name, description, when_to_use
├── reference/
│   └── migration-style-guide.md
└── scripts/
    └── verify.sh
```
The `SKILL.md` body is loaded into context only when the agent decides to invoke the skill — registration cost is just the frontmatter.

---

## 5. UI/UX Design

### 5.1. Visual Layout
Three persistent zones:
*   **Header:** Working directory, active model + provider, permission mode, **running session token + cost total**.
*   **Chat Viewport:** Scrollable. Renders user input, streamed assistant markdown, tool-call badges, tool results (collapsible, with "truncated" indicator when applicable), system notices, and errors. Each completed assistant turn carries a small footer line with per-prompt usage (`↑ in · ↓ out · $cost`). Permission prompts, MCP elicitation prompts, and the model picker render as modal overlays.
*   **Input Area:** Bordered box with a multi-line text input, a "Thinking" spinner slot, and the active mode indicator.

### 5.2. Feedback Loops
*   **Spinners:** Active during model inference and tool execution; the label tells the user *what* is happening (`Calling read_file`, `Streaming…`).
*   **Tool Badges:** Each tool call gets a one-line badge (icon + name + collapsed args). MCP tools have a distinct accent so the user knows the call left the local process.
*   **Streaming Markdown:** Glamour renders incrementally as tokens arrive.
*   **Permission Prompts:** Modal, default-deny on bare Enter, single-key answers (`y` / `n` / `a`).
*   **Slash Commands:** Tab-complete, with inline help text.

### 5.3 Keybindings (V1)
| Key            | Action                              |
|----------------|-------------------------------------|
| `Enter`        | Submit input                        |
| `Shift+Enter`  | Newline in input                    |
| `Up`/`Down`    | Cycle command history (when input empty) |
| `Ctrl+C`       | Interrupt current turn / second press exits |
| `Ctrl+L`       | Clear viewport (history preserved)  |
| `Ctrl+T`       | Toggle tool-result expansion        |
| `Esc`          | Dismiss active modal                |

---

## 6. Data Flow (Sequence)

1.  **User Input:** User types `List files in /src` and hits `Enter`.
2.  **Slash check:** Dispatcher confirms it's not a slash command, hands off to the agent path.
3.  **Event:** `Update()` receives the submission. State sets `streaming = true`, appends a user message, dispatches `runAgentLoop` as a `tea.Cmd`.
4.  **ADK cycle (goroutine):**
    *   ADK asks the model for the next step. Model proposes `list_files(path="/src")`.
    *   Permission gate auto-approves (read-only).
    *   Tool runs, returns the directory listing.
    *   Result is appended to context; ADK loops.
    *   Model emits a streamed natural-language summary; chunks flow back as `streamChunkMsg`.
5.  **Render:** Each chunk updates the in-progress assistant message; the viewport re-renders the tail.
6.  **Completion:** A `turnDoneMsg` clears `streaming`, re-enables input, and persists the turn into the in-memory transcript.
7.  **On exit:** Transcript is flushed to `.agents/sessions/<timestamp>.json`.

---

## 7. Performance Considerations
*   **Concurrency:** All AI calls are wrapped in `tea.Cmd`. The TUI never blocks on I/O.
*   **Parallel startup:** MCP server connections, skill scanning, and config loading happen concurrently; the prompt appears as soon as `config.json` is parsed (other init failures are surfaced as system messages later).
*   **Memory:** Chat history is kept in memory for the active session; for sessions exceeding ~100 turns the viewport switches to a windowed renderer. Persisted transcripts go to disk on exit.
*   **Streaming:** The model adapter implements `io.Writer` and pushes chunks straight to the Bubble Tea program — no buffering between provider and viewport beyond a small batching window for render coalescing.
*   **Cold start:** Target < 200 ms to first prompt with no MCP servers configured.

---

## 8. Module Layout (proposed)

```
cmd/cogo/                # main: flag parsing, bootstrap, dispatches to TUI or headless
internal/
  tui/                   # Bubble Tea model, update, view, components
  headless/              # -p mode runner: same agent loop, plain stdout, no Bubble Tea
  agent/                 # ADK wiring, agent loop, streaming sink (shared by tui + headless)
  tools/                 # built-in tools (fs, bash, web, todo) + path scoping + output truncation
  mcp/                   # MCP client, server lifecycle, elicitation
  skills/                # skill discovery & registration (Claude-compatible SKILL.md)
  config/                # .agents/ discovery, schema, merging, init scaffolding
  permissions/           # permission gate, bash denylist, path scope policy
  commands/              # slash command dispatcher (incl. /stats, /mcp, /skills, /memory)
  session/               # transcript read/write, usage accounting
  models/                # provider adapters (gemini API + vertex; pluggable for others)
  telemetry/             # OTEL setup, exporter wiring, span helpers
  testutil/              # FakeModel, fake MCP server fixture, teatest helpers
```

---

## 9. Testing Strategy

### 9.1 Layers
- **Unit tests** for pure logic: permission gate, path-scope evaluation, bash denylist, config merging, slash command parsing, truncation policy, pricing math. No I/O, no fakes — fast.
- **Integration tests** for the agent loop: a `FakeModel` (in `internal/testutil/`) implements `models.Provider` and returns scripted responses, including tool-call sequences. Tests assert that the agent invokes the right tools in the right order, that truncation fires, that permission outcomes propagate, and that usage accounting matches.
- **MCP tests**: a fake MCP server fixture backed by an in-memory bidirectional pipe exercises the client, including the elicitation round-trip.
- **TUI tests** via `teatest`: input dispatch, slash command parsing, permission modal interaction (`y` / `n` / `a` / 4-option path-scope modal), viewport scrolling, model picker, exit summary.
- **Headless tests**: `cogo -p` invocations against a `FakeModel` asserting exit codes, stdout content, and stderr cost summaries.
- **End-to-end tests** against real Gemini gated behind `COGO_E2E=1`. Default `go test ./...` never hits the network.

### 9.2 Test conventions
- Table-driven tests where the matrix is meaningful.
- `t.TempDir()` for any test that touches disk; never write into the real `.agents/` directory.
- All tests deterministic — no real time, no real network, no real model calls in the default suite.

---

## 10. Future Extensibility
*   **Multi-provider models:** Adapters for Anthropic, OpenAI, and local Ollama behind the `models.Provider` interface — selectable in `config.json`.
*   **Subagents:** `subagent` tool plus `.agents/agents/<name>.md` definitions, mirroring Claude Code's Task pattern.
*   **Plan mode:** A toggle (keybinding + slash command) that disables mutating tools until the user approves an emitted plan.
*   **Hooks:** `PreToolUse`, `PostToolUse`, `OnSessionStart` shell hooks declared in `config.json`, executed by Cogo, able to block or transform tool calls.
*   **User-defined slash commands:** Markdown files in `.agents/commands/` that expand into prompt templates with argument substitution.
*   **Session resume:** `cogo --resume` and `/resume` to reopen a transcript from `.agents/sessions/`.
*   **Persistent agent memory:** A `.agents/memory/` store the agent itself can read/write, mirroring Claude Code's auto-memory.
*   **Progress bars:** Bubbles' `progress` component for long-running tool executions like `npm install`.
