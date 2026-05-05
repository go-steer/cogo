# cogo

A terminal-native agentic CLI for Go developers — like Claude Code, but built in Go on the [Google ADK](https://github.com/google/adk-go) and Gemini 3.x. One static binary, no Python runtime, project-scoped configuration, first-class support for [MCP](https://modelcontextprotocol.io) servers and Claude-compatible skills.

**📚 Full documentation: [go-steer.github.io/cogo](https://go-steer.github.io/cogo/)**

[![CI](https://github.com/go-steer/cogo/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/go-steer/cogo/actions/workflows/ci.yml)
[![Docs](https://github.com/go-steer/cogo/actions/workflows/docs.yml/badge.svg?branch=main)](https://go-steer.github.io/cogo/)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](./LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-steer/cogo.svg)](https://pkg.go.dev/github.com/go-steer/cogo)

## Highlights

- **Two modes, one binary.** `cogo -p "..."` for shell pipelines and CI; `cogo` on a TTY for an interactive Bubble Tea chat with streaming markdown, slash and `@`-file palettes, and prompt history.
- **Project-local configuration.** A `.agents/` directory holds the model, tool settings, MCP servers, skills, and project memory. Auto-discovered like `.git`.
- **Built-in tools.** `read_file`, `write_file`, `edit_file`, `list_dir`, `bash`, `todo` — the same surface a coding agent expects, with per-tool output caps.
- **MCP-native.** Drop a `.agents/mcp.json` describing stdio or HTTP MCP servers and their tools become callable. Schema-driven elicitation modal for servers that prompt the user.
- **Claude-compatible skills.** `SKILL.md` bundles under `.agents/skills/<name>/` are loaded lazily and exposed as agent tools.
- **Permissions you can trust.** Three modes (`ask` / `allow` / `yolo`), an in-TUI approval modal, a non-overridable bash denylist (no `rm -rf /`), and path-scope confinement for file tools.
- **Project memory.** `AGENTS.md` / `CLAUDE.md` / `GEMINI.md` at the project root + `~/.cogo/AGENTS.md` user-global, all merged into the system prompt.
- **Cost surfacing.** Per-turn `↑in · ↓out · $cost` footer, running session total in the header, headless one-line summary on stderr.
- **Telemetry + transcripts.** OpenTelemetry support (`--otel=console` or OTLP) and an automatically-persisted JSON transcript per session under `.agents/sessions/`.

## Install

**Homebrew** (recommended):

```bash
brew install go-steer/cogo/cogo
```

**Docker / OCI** (multi-arch image on GHCR):

```bash
docker pull ghcr.io/go-steer/cogo:latest
docker run --rm -it -v "$PWD:/work" -w /work ghcr.io/go-steer/cogo:latest -p "hello"
```

**`go install`** (HEAD from main):

```bash
go install github.com/go-steer/cogo/cmd/cogo@latest
```

**Pre-built binaries** — every [GitHub release](https://github.com/go-steer/cogo/releases/latest) attaches a tarball per platform:

| Platform              | Asset                                           |
|-----------------------|-------------------------------------------------|
| Linux amd64           | `cogo_<version>_linux_amd64.tar.gz`             |
| Linux arm64           | `cogo_<version>_linux_arm64.tar.gz`             |
| macOS Intel           | `cogo_<version>_darwin_amd64.tar.gz`            |
| macOS Apple Silicon   | `cogo_<version>_darwin_arm64.tar.gz`            |

A `checksums.txt` (SHA256) ships in every release. The [install docs](https://go-steer.github.io/cogo/docs/getting-started/install/#pre-built-binaries) include an auto-detect installer one-liner and a verification snippet.

Verify after install: `cogo --version`.

## Quick start

Cogo speaks Gemini through one of two auth paths:

```bash
# Option A — Vertex AI
gcloud auth application-default login
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=your-project
export GOOGLE_CLOUD_LOCATION=us-central1   # or "global"

# Option B — Public Gemini API
export GOOGLE_API_KEY=...
```

(See [`.env.example`](./.env.example) for a copy-pasteable template.)

Then either:

```bash
# Interactive TUI
cogo

# One-shot, pipeable
cogo -p "Summarize internal/agent in three bullets."
```

Want a fresh project bootstrapped? `cogo init` writes `.agents/{config.json, .gitignore, AGENTS.md}` with sensible defaults; `cogo init --interactive` walks a wizard for provider / model / permission mode.

## Configuration

Cogo looks for a `.agents/` directory by walking up from the current working directory (same way Git finds `.git`). Layout:

```
.agents/
├── config.json          # model, provider, permission mode, path scope, OTEL
├── mcp.json             # MCP server definitions (stdio + Streamable HTTP)
├── skills/              # one subdir per skill, each with a SKILL.md
│   └── example/SKILL.md
├── AGENTS.md            # project memory (CLAUDE.md / GEMINI.md also accepted)
├── sessions/            # transcripts, written on /quit and Ctrl+C
└── logs/                # JSONL debug logs (when --debug is set)
```

When no `.agents/` is present Cogo runs with built-in defaults — useful for quick one-offs in arbitrary directories.

## Slash commands

| Command    | Effect                                                                      |
|------------|-----------------------------------------------------------------------------|
| `/help`    | Full keymap + command list.                                                 |
| `/memory`  | Show which memory files were loaded and from where.                         |
| `/stats`   | Per-session token + cost breakdown.                                         |
| `/model`   | Open the model picker, or `/model <id>` to switch directly.                 |
| `/mcp`     | List configured MCP servers + each server's tools.                          |
| `/skills`  | List discovered skills.                                                     |
| `/reload`  | Re-read `.agents/` from disk and rebuild the agent in place (no chat reset).|
| `/clear`   | Clear chat history (with confirmation).                                     |
| `/quit`    | Exit cleanly, persisting a session transcript.                              |

Type `/` at the start of an empty prompt to open the slash palette; type `@` anywhere to inline a file by path.

## Permissions

Three modes set in `.agents/config.json` under `permissions.mode`:

- **`ask`** — prompt before any mutating tool call (default).
- **`allow`** — pre-approve the listed tool patterns; prompt for everything else.
- **`yolo`** — no prompts. Use sparingly; the bash denylist still blocks the truly dangerous commands.

When the modal appears: **`y`** allow once, **`s`** allow this session, **`a`** always allow (persisted to `.agents/config.json`), **`n`** or **Esc** deny.

File-touching tools are scoped to the project root + `~/.cogo/` plus any `path_scope.allow` glob patterns you add. Out-of-scope reads via `@<path>` still work but surface a one-line warning so private files don't end up in the model context by accident.

## MCP + skills

Drop a JSON file at `.agents/mcp.json`:

```json
{
  "version": 1,
  "servers": {
    "filesystem": {
      "transport": "stdio",
      "command": "npx",
      "args": ["-y", "@modelcontextprotocol/server-filesystem", "/Users/me/work"]
    },
    "github": {
      "transport": "http",
      "url": "https://mcp.example.com/v1",
      "headers": { "Authorization": "Bearer ${env:GITHUB_TOKEN}" }
    }
  }
}
```

Tools are namespaced as `<server>_<tool>` so they never collide with built-ins. `/mcp` shows status and tool inventory.

Skills follow Claude Code's bundle layout — drop `.agents/skills/<name>/SKILL.md` (with frontmatter `name:` + `description:`) and the agent will discover and invoke it. Body markdown is loaded on demand.

## Telemetry & observability

- **OpenTelemetry** — set `cfg.OTEL.Exporter` to `none` (default), `console`, or `otlp`. With `otlp`, point `OTEL_EXPORTER_OTLP_ENDPOINT` at your collector. Spans cover the agent loop, tool calls, and provider requests.
- **Session transcripts** — every interactive session writes `.agents/sessions/<timestamp>.json` on exit, with the full message log and usage totals.
- **Debug logs** — `cogo --debug -p "..."` swaps the slog handler to a JSONL writer under `.agents/logs/<timestamp>.jsonl` for after-the-fact inspection.

## Troubleshooting

### VS Code integrated terminal: random characters disappear

If you run cogo inside VS Code's integrated terminal and see characters dropping out of typed input or rendered output — most often the CSI command terminators (`D`, `b`, `n` are the usual suspects, but other letters can vanish too) — you're hitting a known interaction between Bubble Tea's escape sequences and VS Code's WebGL terminal renderer.

The fix is one VS Code setting, not a cogo config:

1. `⌘+,` (or `Ctrl+,`) to open Settings.
2. Search for `gpuAcceleration`.
3. Change **Terminal › Integrated: Gpu Acceleration** from `auto` to `canvas`.
4. Close and reopen the terminal panel and rerun cogo.

If `canvas` doesn't fully resolve it, try `off`. The perf cost in either case is invisible for an interactive TUI like cogo. This isn't cogo-specific — most Bubble Tea TUIs hit the same WebGL renderer bug.

## Development

The full local CI pipeline lives under [`dev/`](./dev/) and is the same source of truth as the GitHub Actions workflow. Quick start:

```bash
dev/tools/ci              # run every check (format, vet, build, lint, mod-tidy, tests, vuln)
dev/tools/fix-go-format   # auto-fix formatting
```

Missing tools (`golangci-lint`, `goimports`, `govulncheck`) auto-install on first use; only prereq is a Go toolchain. See [`dev/README.md`](./dev/README.md) for the full layout, contributor checklist, and license-header rules.

End-to-end Vertex tests are gated on `COGO_E2E=1` plus the same auth env vars used by the runtime:

```bash
COGO_E2E=1 \
GOOGLE_GENAI_USE_VERTEXAI=true \
GOOGLE_CLOUD_PROJECT=... \
GOOGLE_CLOUD_LOCATION=... \
  go test ./internal/headless/... -run E2E -v
```

## Documentation

- [`ROADMAP.md`](./ROADMAP.md) — release themes (v0.2.0 → v0.5.0) and links to the GitHub milestones.
- [`CONTRIBUTING.md`](./CONTRIBUTING.md) — how to file issues, the PR workflow, DCO sign-off, commit conventions.
- [`CODE_OF_CONDUCT.md`](./CODE_OF_CONDUCT.md) — Contributor Covenant v2.1.
- [`docs/REQUIREMENTS.md`](./docs/REQUIREMENTS.md) — V1 scope, functional & non-functional requirements.
- [`docs/DESIGN.md`](./docs/DESIGN.md) — architecture, configuration, module layout, testing strategy.
- [`AGENTS.md`](./AGENTS.md) — project memory that `cogo` itself loads when run inside this repo.

## License

Apache 2.0 — see [`LICENSE`](./LICENSE).
