---
title: Install
weight: 1
---

Cogo ships as a single static binary. Pick the install method that matches your environment.

## Homebrew (macOS, Linux)

```bash
brew install go-steer/cogo/cogo
```

The formula lives in the [`go-steer/homebrew-cogo`](https://github.com/go-steer/homebrew-cogo) tap. `brew upgrade cogo` picks up new releases.

## Docker / OCI

Multi-arch image (amd64 + arm64) on GitHub Container Registry:

```bash
docker pull ghcr.io/go-steer/cogo:latest

# One-shot run, mounting the current dir as the working directory
docker run --rm -it -v "$PWD:/work" -w /work \
  ghcr.io/go-steer/cogo:latest -p "Summarize this directory."
```

The image is built `FROM gcr.io/distroless/static:nonroot` — minimal surface, no shell.

## go install

```bash
go install github.com/go-steer/cogo/cmd/cogo@latest
```

Drops the binary into `$GOBIN` (or `$(go env GOPATH)/bin`). Make sure that's on your `$PATH`.

## Pre-built binaries

Each [GitHub release](https://github.com/go-steer/cogo/releases) attaches tarballs for:

- `linux-amd64`, `linux-arm64`
- `darwin-amd64`, `darwin-arm64`

Download, extract, and drop `cogo` somewhere on your `$PATH`. Verify the install:

```bash
cogo --version
# cogo v0.1.0 (commit abcdef12, built 2026-05-02T...)
```

## Build from source

Requires Go 1.26+ (see `go.mod`):

```bash
git clone https://github.com/go-steer/cogo
cd cogo
go install ./cmd/cogo
```

Or run without installing:

```bash
go run ./cmd/cogo -p "hello"
```

## Next

→ [Authenticate](../authenticate/) — set up Gemini API or Vertex AI credentials.
