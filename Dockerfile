# Dockerfile for goreleaser-managed multi-arch builds.
#
# goreleaser builds the static linux/{amd64,arm64} cogo binary first
# (CGO_ENABLED=0, -trimpath, ldflags-injected version), drops the
# binary in the build context, then invokes `docker buildx build` with
# this Dockerfile per arch.
#
# Base: gcr.io/distroless/static (the minimal "static" variant — no
# shell, no package manager, no glibc — paired with the :nonroot tag
# so the container runs as uid 65532 by default rather than root.

FROM gcr.io/distroless/static:nonroot

COPY cogo /usr/local/bin/cogo

ENTRYPOINT ["/usr/local/bin/cogo"]
