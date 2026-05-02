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

Every [GitHub release](https://github.com/go-steer/cogo/releases/latest) attaches platform-specific tarballs. Each tarball contains the `cogo` binary plus `LICENSE`, `README.md`, and the `docs/` directory. Pick the one for your OS + architecture:

| Platform        | Asset                                       | Direct link |
|-----------------|---------------------------------------------|-------------|
| Linux amd64     | `cogo_<version>_linux_amd64.tar.gz`         | [Latest releases →](https://github.com/go-steer/cogo/releases/latest) |
| Linux arm64     | `cogo_<version>_linux_arm64.tar.gz`         | [Latest releases →](https://github.com/go-steer/cogo/releases/latest) |
| macOS Intel     | `cogo_<version>_darwin_amd64.tar.gz`        | [Latest releases →](https://github.com/go-steer/cogo/releases/latest) |
| macOS Apple Silicon | `cogo_<version>_darwin_arm64.tar.gz`    | [Latest releases →](https://github.com/go-steer/cogo/releases/latest) |

Each release also includes a `checksums.txt` with SHA256s for every asset.

### Auto-detect installer

A one-liner that picks the right asset for the current host:

```bash
VERSION=$(curl -fsSL https://api.github.com/repos/go-steer/cogo/releases/latest \
  | grep -o '"tag_name": *"v[^"]*"' | head -1 | sed 's/.*"v\([^"]*\)".*/\1/')
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
ARCHIVE="cogo_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/go-steer/cogo/releases/download/v${VERSION}/${ARCHIVE}"

curl -fsSL -o "$ARCHIVE" "$URL"
tar -xzf "$ARCHIVE"
sudo install cogo /usr/local/bin/cogo
cogo --version
```

### Verify with checksums

```bash
# After downloading the tarball:
curl -fsSL -O "https://github.com/go-steer/cogo/releases/download/v${VERSION}/checksums.txt"
sha256sum --check --ignore-missing checksums.txt
# cogo_<version>_<os>_<arch>.tar.gz: OK
```

### Manual install

If you'd rather click through the UI:

1. Open the [latest release](https://github.com/go-steer/cogo/releases/latest).
2. Under **Assets**, click the tarball for your platform.
3. Extract: `tar -xzf cogo_<version>_<os>_<arch>.tar.gz`.
4. Move the binary onto your `$PATH`: `sudo install cogo /usr/local/bin/cogo` (or any directory on `$PATH`).
5. Verify: `cogo --version`.

### Verify the install

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
