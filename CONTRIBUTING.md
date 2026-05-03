# Contributing to cogo

Thanks for your interest in contributing! This file is the table of contents — most of the detail lives in `dev/README.md` and the docs.

By participating in this project you agree to abide by the [Code of Conduct](./CODE_OF_CONDUCT.md).

## Reporting bugs and requesting features

- **Bugs:** [open an issue](https://github.com/go-steer/cogo/issues/new) and include the `cogo --version` output, your OS / Go version, the model + provider you're using, and the smallest set of steps that reproduces the problem. A `--debug` JSONL log (under `.agents/logs/`) is gold if you can attach a redacted snippet.
- **Feature requests:** check the [roadmap](./ROADMAP.md) and the [open milestones](https://github.com/go-steer/cogo/milestones) first — your idea may already be planned. If not, file an issue with the use case (what you're trying to do) before the proposed solution.
- **Questions / discussion:** [GitHub Discussions](https://github.com/go-steer/cogo/discussions).

## Pull requests

### Before you start

For anything beyond a typo fix or one-line bug, open an issue first so we can agree on the approach. PRs that are aligned upfront merge faster than ones that surface a design disagreement at review time.

### Workflow

1. Fork and branch from `dev` (not `main` — `main` is release-tagged HEAD).
2. Make your change. Keep the diff focused; unrelated cleanup belongs in a separate PR.
3. Run the full local CI before pushing:
   ```bash
   dev/tools/ci
   ```
   This is the same script that runs in GitHub Actions — green locally means green remotely. See [`dev/README.md`](./dev/README.md) for the full layout and how to add new checks.
4. Open the PR against `dev`. CI runs on the push; merging to `main` is done via release-cut PRs from `dev` and skips CI re-runs (the SHA already has green checks).

### Commit messages — Conventional Commits

The release changelog is auto-generated from commit messages by [goreleaser](https://goreleaser.com/), so the prefix matters:

- `feat:` — user-visible new functionality
- `fix:` — user-visible bug fix
- `docs:` — documentation only
- `test:` — tests only
- `refactor:` — code change that's neither a feature nor a fix
- `chore:` / `build:` / `ci:` — repo plumbing, not in the changelog

Optional scope in parens: `feat(tui): ...`, `fix(mcp): ...`. Keep the subject under ~70 chars; put detail in the body.

### Developer Certificate of Origin (DCO)

All commits must be **signed off** under the [Developer Certificate of Origin](https://developercertificate.org/). The DCO is a lightweight assertion that you wrote the patch (or have the right to submit it under the project's Apache-2.0 license) — it's a `Signed-off-by:` trailer in the commit message, not a cryptographic signature.

Sign off by passing `-s` to `git commit`:

```bash
git commit -s -m "feat(tui): add dim-the-rest scroll mode"
```

…which appends:

```
Signed-off-by: Your Name <you@example.com>
```

The name and email must match your `git config user.name` / `user.email`. If you forget, amend with `git commit --amend -s` (single commit) or rebase with `-x 'git commit --amend -s --no-edit'` (multiple). A DCO check on PRs blocks merge until every commit has a sign-off.

### License headers

Every Go source file must carry the SPDX header:

```
// Copyright 2026 The Cogo Authors.
// SPDX-License-Identifier: Apache-2.0
```

`golangci-lint` enforces this on `.go` files automatically. For new shell or YAML files, run `dev/tools/add-license-headers` once — it's idempotent and only touches files missing the header. See [`dev/README.md`](./dev/README.md#license-headers) for the full rules.

### Tests

- Unit tests live next to the code (`*_test.go`).
- TUI tests use [`teatest`](https://github.com/charmbracelet/x/tree/main/exp/teatest); see existing tests in `internal/tui/` for the pattern.
- End-to-end tests against Vertex are gated on `COGO_E2E=1` plus the auth env vars — they don't run in default CI. See the README's [Development section](./README.md#development) for the invocation.
- A new feature without a test is not done. A new bug fix without a regression test makes it easy for the bug to come back.

## Project layout

- `cmd/cogo/` — main binary entry point.
- `internal/` — implementation. `agent/`, `tui/`, `tools/`, `mcp/`, `skills/`, `models/`, etc.
- `docs/` — design and requirements documents.
- `dev/` — local + CI tooling (run from here, don't reinvent).
- `.github/workflows/` — thin delegators to `dev/ci/presubmits/`.

For deeper architecture, read [`docs/DESIGN.md`](./docs/DESIGN.md) and the project's own [`AGENTS.md`](./AGENTS.md).

## License

By contributing, you agree that your contributions will be licensed under the [Apache License 2.0](./LICENSE).
