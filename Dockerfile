# syntax=docker/dockerfile:1

FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS builder

ARG TARGETOS
ARG TARGETARCH
ENV GOPROXY=https://goproxy.io,direct

# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-w -s" -o /out/fruits-api ./cmd/api

FROM alpine:3.20 AS runtime

# hadolint ignore=DL3018
RUN apk add --no-cache ca-certificates curl \
    && adduser -D -u 10001 appuser

WORKDIR /app

COPY --from=builder /out/fruits-api /app/fruits-api

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -fsS http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/fruits-api"]
