# Roadmap

This is the public direction for `cogo`. The canonical backlog lives in [GitHub milestones](https://github.com/go-steer/cogo/milestones) and individual issues — this file is the high-level shape so you can see where things are headed without scrolling 50 issues.

No dates. Order can shift based on what people actually use and what blockers turn up. Pre-1.0 means breaking changes are allowed, with a documented changelog at each release.

## Vision

`cogo` aims for broad parity with Claude Code's feature surface, expressed as a single static Go binary. The bet is that a TUI agent built on the [Google ADK](https://github.com/google/adk-go) and Gemini, with first-class MCP and Claude-compatible skills, fills a real gap for developers who want one binary in `$PATH` instead of a Python runtime — and who'd rather configure an agent through `.agents/` checked into the repo than through a hosted product. Long-term, the same surface should work against multiple model backends, not just Gemini.

## Current state

**Shipped:** [`v0.1.0`](https://github.com/go-steer/cogo/releases/tag/v0.1.0) — the V1 MVP.

What's in it: interactive TUI + headless `-p` mode, the built-in tool surface (`read_file` / `write_file` / `edit_file` / `list_dir` / `bash` / `todo`) with a permission system, MCP servers (stdio + Streamable HTTP) with schema-driven elicitation, Claude-compatible skills, project memory (`AGENTS.md`), `/model` picker, cost surfacing, OTEL spans, and persisted session transcripts. See the [README highlights](./README.md#highlights) for the full feature list.

## Releases

Each row links to the live milestone — that's where the up-to-date issue list, progress, and any new tickets land.

| Release | Theme | Milestone |
|---|---|---|
| **v0.2.0** | Workflow & customization | [milestone](https://github.com/go-steer/cogo/milestone/1) |
| **v0.3.0** | Multi-provider | [milestone](https://github.com/go-steer/cogo/milestone/2) |
| **v0.4.0** | Power features | [milestone](https://github.com/go-steer/cogo/milestone/3) |
| **v0.5.0** | Platform reach | [milestone](https://github.com/go-steer/cogo/milestone/4) |

---

### v0.2.0 — Workflow & customization → [milestone](https://github.com/go-steer/cogo/milestone/1)

The everyday-use polish that V1 deliberately deferred. Plan mode lets the agent propose before it mutates; user-defined slash commands turn repeated prompts into first-class commands; `/resume` reopens a previous session instead of starting fresh; user-global skills and MCP servers (`~/.cogo/`) merge with project-local config.

Major slices:
- **Plan mode** — config + agent state, mutating-tool gate, `/plan` command + Shift+Tab toggle, approve-plan UI, transcript persistence.
- **User-defined commands** — discovery loader for `.agents/commands/`, frontmatter parsing, slash dispatch + palette autocomplete.
- **Resume** — session loader, interactive `/resume` picker, `cogo --resume` CLI flag.
- **User-global config** — `~/.cogo/skills/` and `~/.cogo/mcp.json` merge layer.
- **Transcript controls** — `--no-transcript` flag and `session.persist` config.

### v0.3.0 — Multi-provider → [milestone](https://github.com/go-steer/cogo/milestone/2)

Cogo's core wasn't allowed to depend on Gemini-specific types outside the model adapter; v0.3.0 makes that promise real. Anthropic, OpenAI, and Ollama land behind the same `models.Provider` interface, with a published feature matrix so users know what each backend supports.

Major slices:
- **Provider-agnostic core** — audit `models.Provider` for Gemini-specific assumptions, extract neutral message + tool-call types, publish a feature matrix doc.
- **Anthropic** — auth, streaming, tool-use translation, pricing table, tests + docs.
- **OpenAI** — skeleton + streaming + function calling, pricing, tests + docs.
- **Ollama** — local HTTP backend, tests + docs.

### v0.4.0 — Power features → [milestone](https://github.com/go-steer/cogo/milestone/3)

The deeper Claude Code parity items that need the multi-provider work to be useful. Subagents let the parent delegate scoped work; hooks open the lifecycle to user shell commands; persistent agent-managed memory gives the agent a writable store across sessions. OTEL gains metrics on top of the spans shipped in v0.1.0.

Major slices:
- **Subagents** — `.agents/agents/<name>.md` discovery, the `subagent` tool, scoped tool allowlist, `/agents` command.
- **Hooks** — config schema, `PreToolUse`, `PostToolUse`, `OnSessionStart` executors.
- **Persistent memory** — `.agents/memory/` schema, `memory_set` / `memory_get` / `memory_list` tools, auto-load into the system prompt.
- **OTEL metrics** — counters (turns, tool calls, errors) and histograms (latency, tokens) on top of the existing spans.

### v0.5.0 — Platform reach → [milestone](https://github.com/go-steer/cogo/milestone/4)

Cogo's V1 ships Linux + macOS binaries and is text-only. v0.5.0 widens that: image input through the TUI, `.deb` / `.rpm` / Nix packages alongside the Homebrew tap, and Windows promoted from "best-effort" to a tier-2 supported target.

Major slices:
- **Multimodal** — image paste detection in the TUI, file-drop / `@`-image handling, provider passthrough for image parts.
- **Linux packaging** — Debian/Ubuntu `.deb` via goreleaser, Fedora/RHEL `.rpm`, a Nix flake.
- **Windows** — include in goreleaser builds, fix PTY + alt-screen quirks, PowerShell fallback for the `bash` tool with an updated denylist.

## Beyond v0.5.0

Aspirational, explicitly unscoped. Listed so the direction is visible, not as a commitment:

- A web UI that drives the same agent core, for share-a-session and remote-work scenarios.
- IDE integrations (VS Code first) over the same headless transport.
- A hosted MCP relay so users don't have to run every server locally.
- Team features: shared skills, shared memory, audit log of agent actions.
- Richer evaluation harness so provider/model swaps are quantitatively comparable.

## How priorities shift

The version-themed milestones are the working plan, not a contract. Order within a release is liquid — a piece of v0.4.0 may move forward if a user blocks on it, and a low-signal v0.2.0 item may slide. The truth is always the open issues in the relevant milestone; the table above is the rough shape.

## Contribute / suggest

- **Bugs and feature requests:** [open an issue](https://github.com/go-steer/cogo/issues/new/choose).
- **Discussion / questions:** [GitHub Discussions](https://github.com/go-steer/cogo/discussions).
- **Code:** see [`dev/README.md`](./dev/README.md) for the local CI pipeline, license-header rules, and contributor checklist. The same `dev/tools/ci` script that runs in GitHub Actions runs locally.

If you're using cogo and a particular roadmap item is the difference between adopting it and not, say so on the relevant issue — that's the strongest signal we have.
