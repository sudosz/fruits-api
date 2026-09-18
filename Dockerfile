# syntax=docker/dockerfile:1

# ---- Builder ----
# gin v1.12 and the swaggo toolchain require Go 1.25, so the builder tracks it.
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src

# Copy manifests first so dependency download is cached across code changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /out/fruits-api ./cmd/api

# ---- Runtime ----
FROM alpine:3.20

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
