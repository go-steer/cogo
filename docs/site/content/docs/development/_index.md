---
title: Development
weight: 6
sidebar:
  open: true
---

How to hack on Cogo itself: local CI, contributing conventions, license headers.

## Quick start

```bash
git clone https://github.com/go-steer/cogo
cd cogo

# Run the same checks as GitHub Actions, in fast-fail order.
dev/tools/ci

# Auto-fix formatting (gofmt + goimports).
dev/tools/fix-go-format
```

Missing tools (`golangci-lint`, `goimports`, `govulncheck`) auto-install on first use; the only prerequisite is a Go 1.26+ toolchain.

## Repo layout

```
cogo/
├── cmd/cogo/                # main entrypoint + version flag
├── internal/
│   ├── agent/               # agent loop wrapper around Google ADK
│   ├── config/              # .agents/config.json schema + load/save
│   ├── headless/            # `cogo -p` mode
│   ├── tui/                 # Bubble Tea TUI
│   ├── tools/               # built-in tool implementations
│   ├── permissions/         # permission gate + path scope
│   ├── memory/              # AGENTS.md / CLAUDE.md / GEMINI.md loader
│   ├── mcp/                 # MCP server lifecycle + namespacing
│   ├── skills/              # SKILL.md discovery + loader
│   ├── models/              # provider abstraction (Gemini, Vertex)
│   ├── usage/               # per-turn + session token + cost tracking
│   ├── session/             # transcript writer
│   ├── telemetry/           # OpenTelemetry setup
│   └── initcmd/             # `cogo init` subcommand + wizard
├── dev/                     # local CI tooling (mirrors .github/workflows/ci.yml)
└── docs/
    ├── REQUIREMENTS.md      # internal V1 scope doc
    ├── DESIGN.md            # internal architecture doc
    └── site/                # this Hugo site
```

## CI checks

Both local (`dev/tools/ci`) and remote (GitHub Actions) run the same seven checks:

| Check               | Tool              |
|---------------------|-------------------|
| `format`            | `gofmt -s` + `goimports`  |
| `vet`               | `go vet ./...`    |
| `build`             | `go build ./...`  |
| `lint`              | `golangci-lint`   |
| `mod-tidy`          | `go mod tidy` clean check |
| `test`              | `go test -race -coverprofile` |
| `vuln`              | `govulncheck ./...` |

See [`dev/README.md`](https://github.com/go-steer/cogo/blob/main/dev/README.md) for the full layout.

## Conventions

- **Conventional Commits** — `feat:`, `fix:`, `chore:`, `docs:`, `ci:`, `build:`, `test:`, `refactor:`. Subject in imperative mood, ≤72 chars.
- **No `Co-Authored-By` trailer** — keep authorship clean.
- **License headers** — every Go / shell / YAML source carries a 2-line SPDX header. The `goheader` linter enforces it on `.go`. For new shell + YAML, run `dev/tools/add-license-headers` (idempotent).
- **Plan before non-trivial work** — significant changes get a brief design pass before implementation. Slice plans live in `docs/SLICES.md` (internal).
- **Tests with code** — every new package ships with tests; coverage shows up in CI summary.

## Branch model

- `main` is protected. CI must be green to merge.
- `dev` is the integration branch. Day-to-day work lands here first.
- PRs from `dev` → `main` skip CI (the same SHA already has green checks from the `dev` push).

See [`dev/README.md`](https://github.com/go-steer/cogo/blob/main/dev/README.md) for branch-protection details.

## E2E tests

The Vertex e2e suite is gated on env vars (no network in normal CI):

```bash
COGO_E2E=1 \
GOOGLE_GENAI_USE_VERTEXAI=true \
GOOGLE_CLOUD_PROJECT=... \
GOOGLE_CLOUD_LOCATION=... \
  go test ./internal/headless/... -run E2E -v
```

Skip locally unless you're explicitly debugging provider issues — they cost real tokens.
