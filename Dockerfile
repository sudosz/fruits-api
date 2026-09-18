# syntax=docker/dockerfile:1

# gin v1.12 and the swaggo toolchain require Go 1.25, so the builder tracks it.
# TARGETOS/TARGETARCH come from buildx, which is how the multi-arch images
# (linux/amd64 + linux/arm64) are produced without emulating the compiler.
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS builder

ARG TARGETOS
ARG TARGETARCH

# hadolint ignore=DL3018 # Alpine repos do not retain old package versions, so pinning breaks rebuilds
RUN apk add --no-cache ca-certificates git

WORKDIR /src

# Copy manifests first so dependency download stays cached across source edits.
COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

# Static, reproducible binary: no libc dependency and no build paths embedded,
# so the runtime image needs no toolchain.
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-w -s" -o /out/fruits-api ./cmd/api

FROM alpine:3.20 AS runtime

# hadolint ignore=DL3018 # Alpine repos do not retain old package versions, so pinning breaks rebuilds
RUN apk add --no-cache ca-certificates curl \
    && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/fruits-api /app/fruits-api
COPY --from=builder /src/docs /app/docs

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/fruits-api"]
