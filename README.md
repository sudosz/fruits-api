# Fruits API

[![CI/CD](https://github.com/sudosz/fruits-api/actions/workflows/ci-cd.yml/badge.svg?branch=main)](https://github.com/sudosz/fruits-api/actions/workflows/ci-cd.yml)
[![CodeQL](https://img.shields.io/github/actions/workflow/status/sudosz/fruits-api/ci-cd.yml?branch=main&label=codeql&logo=github)](https://github.com/sudosz/fruits-api/security/code-scanning)
[![Go Report Card](https://goreportcard.com/badge/github.com/sudosz/fruits-api)](https://goreportcard.com/report/github.com/sudosz/fruits-api)
[![Go Version](https://img.shields.io/badge/go-1.26-00ADD8?logo=go&logoColor=white)](go.mod)
[![GHCR Image](https://img.shields.io/badge/ghcr.io-sudosz%2Ffruits--api-2496ED?logo=docker&logoColor=white)](https://github.com/sudosz/fruits-api/pkgs/container/fruits-api)
[![Platforms](https://img.shields.io/badge/platforms-linux%2Famd64%20%7C%20linux%2Farm64-informational)](https://github.com/sudosz/fruits-api/pkgs/container/fruits-api)
[![Signed with cosign](https://img.shields.io/badge/supply%20chain-cosign-4B32C3?logo=sigstore&logoColor=white)](https://docs.sigstore.dev/cosign/overview/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

RESTful service for managing fruits, written in Go with Gin and backed by PostgreSQL. It is organised as a three-tier application (handler → service → repository), runs versioned SQL migrations on startup, and ships with Swagger docs, a multi-arch container image, Docker Compose for local work, raw Kubernetes manifests, a Helm chart with per-environment values, and a GitHub Actions pipeline that lints, scans, tests, builds, scans the image, then pushes and signs it.

## Architecture

```
cmd/api            composition root: config → db → repository → service → handler → router
internal/config    all tunables as struct fields with env overrides and defaults
internal/database  pool setup, single-pass ping, embedded migration runner
internal/models    Fruit, FruitAttributes, CreateFruitRequest
internal/repository FruitRepository interface + postgres implementation (database/sql)
internal/service   FruitService interface + business rules (trimming, validation)
internal/handlers  FruitHandler: HTTP concerns only, depends on FruitService
internal/middleware security headers, body limit, per-IP rate limiting
migrations         versioned .up.sql/.down.sql, embedded with go:embed
charts/fruits-api  Helm chart with base, staging, and prod values
k8s                raw manifests for a plain kubectl apply flow
```

Each layer depends only on the interface of the layer beneath it, so the service can be tested with a stub repository and the handler with a stub service — no database required for unit tests.

| Layer | Interface | Constructor |
| ----- | --------- | ----------- |
| Repository | `FruitRepository{List,Get,Create,Health}` | `repository.NewFruitRepository(db *sql.DB)` |
| Service | `FruitService{List,Get,Create,Health}` | `service.NewFruitService(repo)` |
| Handler | `*FruitHandler{List,Get,Create,Health}` | `handlers.NewFruitHandler(svc)` |

Errors cross layers as sentinels: `repository.ErrNotFound` (aliased as `service.ErrNotFound`) and `service.ErrInvalidInput`.

## Features

- `GET`/`POST` endpoints over a single `fruits` table, no ORM
- Versioned SQL migrations embedded in the binary and applied on startup inside a transaction
- `/healthz` probe that pings the database, used by Docker, Compose, and Kubernetes
- Swagger UI generated from handler annotations with `swaggo`
- Graceful shutdown via `signal.NotifyContext` so rolling deploys don't cut requests
- Security middleware: hardening headers, request body limit, per-IP rate limiting
- Every timeout, pool size, and limit configurable — no magic numbers in code
- Static binary, non-root user, dropped capabilities, read-only root filesystem
- Multi-arch images (`linux/amd64`, `linux/arm64`) scanned before push and signed with cosign

**Stack**: Go 1.26 · Gin · PostgreSQL 17 · `lib/pq` · swaggo · Docker · Kubernetes · Helm · GitHub Actions

## API

| Method | Path             | Description                  | Success | Errors          |
| ------ | ---------------- | ---------------------------- | ------- | --------------- |
| GET    | `/fruits`        | List all fruits (`[]` empty) | 200     | 429, 500        |
| GET    | `/fruits/{id}`   | Get one fruit                | 200     | 400, 404, 429   |
| POST   | `/fruits`        | Create a fruit               | 201     | 400, 413, 429   |
| GET    | `/healthz`       | Readiness/liveness probe     | 200     | 503             |
| GET    | `/swagger/*any`  | Swagger UI                   | 200     | —               |

Fruit shape:

```json
{ "id": 1, "fruit": "apple", "color": "red" }
```

Errors are always `{"error": "..."}` and health is `{"status": "ok"}`; both are plain JSON objects rather than dedicated response structs.

### curl

```bash
# Create
curl -X POST http://localhost:8080/fruits \
  -H 'Content-Type: application/json' \
  -d '{"fruit":"banana","color":"yellow"}'
# -> 201 {"id":1,"fruit":"banana","color":"yellow"}

# List
curl http://localhost:8080/fruits
# -> 200 [{"id":1,"fruit":"banana","color":"yellow"}]

# Get by id
curl http://localhost:8080/fruits/1
# -> 200 {"id":1,"fruit":"banana","color":"yellow"}

# Missing fruit
curl -i http://localhost:8080/fruits/999
# -> 404 {"error":"fruit not found"}

# Health
curl http://localhost:8080/healthz
# -> 200 {"status":"ok"}
```

### Swagger UI

http://localhost:8080/swagger/index.html

Regenerate after changing annotations:

```bash
go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/api/main.go -o docs
```

## Running

### Docker Compose

```bash
docker compose up --build
```

Brings up Postgres 17 with a `pg_isready` healthcheck plus the API on `localhost:8080`; the API waits for the database to report healthy. Shared service settings (`restart`, `security_opt`, `logging`) come from the `x-service-defaults` and `x-security-opts` YAML anchors, so they are declared once. Postgres is not published to the host, and the API container runs read-only with all capabilities dropped and `no-new-privileges`.

### Locally

Needs a reachable Postgres.

```bash
go run ./cmd/api
```

### Container image

```bash
docker run --rm -p 8080:8080 ghcr.io/sudosz/fruits-api:latest
```

`ghcr.io/sudosz/fruits-api:latest` is the runnable multi-arch image (`linux/amd64`, `linux/arm64`).
Tags ending in `.sig` (for example `sha256-<digest>.sig`) are **Cosign signature artifacts, not images** — pulling or running them fails. Verify instead:

```bash
cosign verify ghcr.io/sudosz/fruits-api:latest \
  --certificate-identity-regexp 'https://github.com/sudosz/fruits-api/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

The publish step sets `provenance: false` on Buildx. Attestation entries inside the manifest list are what caused `mismatched image rootfs and manifest layers` on older Docker daemons and snapshotters; without them the manifest list contains only the two platform images and unpacks everywhere.

### Configuration

All configuration is environment variables with defaults in `internal/config`. `DATABASE_URL`, if set, overrides the individual `DB_*` values.

| Variable | Default | Purpose |
| -------- | ------- | ------- |
| `PORT` | `8080` | HTTP listen port |
| `DB_HOST` | `localhost` | Database host |
| `DB_PORT` | `5432` | Database port |
| `DB_USER` | `postgres` | Database user |
| `DB_PASSWORD` | `postgres` | Database password |
| `DB_NAME` | `fruitsdb` | Database name |
| `DB_SSLMODE` | `disable` | `sslmode` in the DSN |
| `DATABASE_URL` | — | Full DSN override |
| `DB_MAX_OPEN_CONNS` | `25` | Pool size |
| `DB_MAX_IDLE_CONNS` | `25` | Idle pool size |
| `DB_CONN_MAX_LIFETIME` | `5m` | Connection lifetime |
| `DB_CONN_MAX_IDLE_TIME` | `1m` | Idle connection lifetime |
| `DB_CONNECT_TIMEOUT` | `30s` | Bound on connect + ping |
| `DB_MIGRATE_TIMEOUT` | `15s` | Bound on startup migrations |
| `RATE_LIMIT_PER_SECOND` | `50` | Token refill rate per client IP |
| `RATE_LIMIT_BURST` | `100` | Bucket size per client IP |
| `RATE_LIMIT_REAP_INTERVAL` | `10m` | Idle bucket eviction interval |
| `MAX_BODY_BYTES` | `8192` | Request body limit |
| `SERVER_READ_HEADER_TIMEOUT` | `5s` | `http.Server` read header timeout |
| `SERVER_READ_TIMEOUT` | `15s` | `http.Server` read timeout |
| `SERVER_WRITE_TIMEOUT` | `15s` | `http.Server` write timeout |
| `SERVER_IDLE_TIMEOUT` | `60s` | `http.Server` idle timeout |
| `SERVER_MAX_HEADER_BYTES` | `1048576` | Max header size |
| `SERVER_SHUTDOWN_TIMEOUT` | `10s` | Graceful drain budget |

Setting `RATE_LIMIT_PER_SECOND` or `RATE_LIMIT_BURST` to `0` disables rate limiting.

## Migrations

Schema lives in `migrations/` as numbered pairs:

```
migrations/000001_create_fruits_table.up.sql
migrations/000001_create_fruits_table.down.sql
```

They are embedded with `go:embed` and applied by `database.Migrate` on startup: it creates `schema_migrations`, reads applied versions, and runs each pending `*.up.sql` in its own transaction, recording the version on success. Adding a migration means dropping in the next numbered pair — no code changes, and no `CREATE TABLE IF NOT EXISTS` constant in Go.

## Tests

```bash
go test -v -race ./...
```

- **Handler tests** (`internal/handlers`) drive the real router through `httptest` against a stub `FruitService`, covering status codes, error bodies, and the empty-list-not-null contract.
- **Service tests** (`internal/service`) use a stub `FruitRepository` to cover trimming, validation, and sentinel error propagation.
- **Repository tests** (`internal/repository`) are integration tests: they run migrations and exercise real SQL. They skip when neither `DB_HOST` nor `DATABASE_URL` is set, so the suite stays runnable on a laptop, and CI provides a `postgres:17-alpine` service container so they execute for real there.
- **Middleware tests** (`internal/middleware`) are pure unit tests.

## Linting and security scanning

```bash
gofmt -l .
go vet ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

`.golangci.yml` enables, beyond the defaults: `gosec`, `bodyclose`, `rowserrcheck`, `sqlclosecheck`, `noctx`, `errorlint`, `nilerr`, `gocritic`, `revive`, `misspell`, `unconvert`, `unparam`, `wastedassign`, `copyloopvar`, with `gofmt` and `goimports` as formatters. The workflow pins `golangci/golangci-lint-action@v7` with a v2-compatible linter version so the config schema and the action agree and Dependabot PRs stay green.

## Helm

The chart lives in `charts/fruits-api/`:

```
charts/fruits-api/
├── Chart.yaml
├── values.yaml            # base defaults
├── values-staging.yaml    # staging overrides
├── values-prod.yaml       # HA replicas, strict quotas, external secret
└── templates/
    ├── _helpers.tpl
    ├── configmap.yaml
    ├── deployment.yaml
    ├── hpa.yaml
    ├── networkpolicy.yaml
    ├── pdb.yaml
    ├── secret.yaml
    ├── service.yaml
    └── serviceaccount.yaml
```

Validate without a cluster, then install:

```bash
helm lint charts/fruits-api
helm template fruits-api charts/fruits-api

# staging
helm upgrade --install fruits-api charts/fruits-api \
  -f charts/fruits-api/values-staging.yaml -n staging --create-namespace

# production
helm upgrade --install fruits-api charts/fruits-api \
  -f charts/fruits-api/values-prod.yaml -n production --create-namespace
```

Every environment variable from the configuration table is rendered into the ConfigMap from `.Values.config`, so tuning timeouts or rate limits is a values change. Credentials come from a chart-managed Secret in dev/staging; `values-prod.yaml` sets `secret.create: false` and `secret.existingSecret`, so production credentials are managed by sealed-secrets, SOPS, or External Secrets. Production also enables the HPA (3–10 replicas), a `minAvailable: 2` PodDisruptionBudget, zone anti-affinity, and topology spread.

## Kubernetes (raw manifests)

```bash
kubectl apply -f k8s/serviceaccount.yaml
kubectl apply -f k8s/configmap.yaml

# Create the Secret out of band — no plaintext Secret is tracked in git.
kubectl create secret generic fruits-api-secret \
  --from-literal=DB_USER=postgres \
  --from-literal=DB_PASSWORD='<strong-password>'

kubectl apply -f k8s/postgres-pvc.yaml
kubectl apply -f k8s/postgres-deployment.yaml -f k8s/postgres-service.yaml
kubectl apply -f k8s/app-deployment.yaml -f k8s/app-service.yaml
kubectl apply -f k8s/networkpolicy.yaml -f k8s/poddisruptionbudget.yaml
```

The API runs 2 replicas of `ghcr.io/sudosz/fruits-api:latest` with probes on `/healthz` and is exposed on NodePort `30080`:

```bash
curl http://$(minikube ip):30080/fruits
```

Hardening applied to both the manifests and the chart:

- Pods run as non-root UID 10001 with `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`, all capabilities dropped, and the `RuntimeDefault` seccomp profile
- Dedicated ServiceAccounts with `automountServiceAccountToken: false`
- NetworkPolicies restrict both directions; the API may reach only Postgres `5432` and kube-DNS
- PodDisruptionBudget keeps replicas available during voluntary disruptions, with `maxUnavailable: 0` rolling updates

`k8s/secret.example.yaml` is a template only. Real Secret files (`k8s/secret.yaml`, `k8s/*secret*.local.yaml`) are git-ignored.

## CI/CD

`.github/workflows/ci-cd.yml` runs on pushes and pull requests to `main`, on `v*.*.*` tags, and on demand. Permissions are read-only at the top level and widened per job; runs are cancelled on new PR pushes via a concurrency group.

```
[ lint ]   [ sast ]   [ test ]   [ codeql ]   [ helm-lint ]
    └──────────┴──────────┴──────────┘              │
                      │ (gate)                      │
                      ▼                             │
              [ build container ]  (local, no push) │
                      │                             │
                      ▼                             │
              [ scan container ]   (Trivy)          │
                      │                             │
                      ▼◄────────────────────────────┘
              [ push & sign ]      (GHCR + cosign)
                      │
                      ▼
              [ package / dry-run ] (helm package + template)
```

1. **lint** — `gofmt` check, `go vet`, `golangci-lint-action@v7`.
2. **sast** — `govulncheck` and a Trivy filesystem scan.
3. **test** — `go test -race` with coverage against a `postgres:17-alpine` service container.
4. **codeql** — CodeQL analysis for Go.
5. **helm-lint** — `helm lint` and `helm template` for base, staging, and prod values, plus `kubeconform` over the rendered output and `k8s/`.
6. **build** — gated on lint, sast, test, and codeql; builds the image locally with `load: true, push: false` and uploads it as an artifact. Nothing is published yet.
7. **scan** — loads that exact image and fails on CRITICAL/HIGH Trivy findings, uploading SARIF to code scanning. The image is scanned *before* it can reach the registry.
8. **push & sign** — only on non-PR events: builds and pushes `linux/amd64` + `linux/arm64` to GHCR with `provenance: false`, then signs the digest keylessly with cosign.
9. **package / dry-run** — `helm package` plus a prod render against the published image, uploaded as an artifact.

Dependabot (`.github/dependabot.yml`) keeps Go modules, GitHub Actions, and base images current with grouped weekly PRs.

## AI Usage Disclosure

This project was built with an AI coding assistant driving the implementation end to end, with human review of the output.

**Where it was applied**

- Go source: models, config, database connection and migration runner, repository, service, handlers, middleware, and tests
- Swagger annotations on handlers and the generated `docs/` package
- Multi-stage `Dockerfile`, `.dockerignore`, `docker-compose.yml`
- Kubernetes manifests in `k8s/` and the Helm chart in `charts/fruits-api/`
- GitHub Actions pipeline, `.golangci.yml`, Dependabot config
- This README

**Architectural choices and why**

- **Three tiers, interface-bounded** — the handler owns HTTP, the service owns rules, the repository owns SQL. Because each depends on an interface, handler and service tests run in milliseconds against stubs while SQL is still covered for real against Postgres.
- **Gin** — small, fast HTTP router with a swaggo integration, so the OpenAPI spec is generated from the handlers themselves instead of drifting in a separate file.
- **`database/sql` + `lib/pq`, no ORM** — the data model is one table; parameterized SQL is clearer than ORM configuration and keeps queries injection-safe.
- **Sentinel errors across layers** — `ErrNotFound` and `ErrInvalidInput` let the handler map failures to 404/400 without importing SQL semantics or leaking driver errors to clients.
- **Embedded versioned migrations** — schema history lives in reviewable `.sql` files, ships inside the binary with no sidecar or init container, and each migration commits atomically.
- **Config struct instead of constants** — every timeout, pool bound, and limit is a field with a default and an env override, so staging and prod differ by values, not by rebuilds.
- **`signal.NotifyContext` + `run()`** — one context cancels the server, one error path returns to `main`, which prints to stderr and exits non-zero. No channel plumbing, and deferred cleanup still runs.
- **`/healthz` pings the database** — a probe that only proves the process is alive would keep routing traffic to a replica that cannot serve requests.
- **Scan before push** — building locally and scanning that exact image means a vulnerable image never reaches the registry, instead of being scanned after the fact.
- **`provenance: false`** — attestation manifests in the list broke unpacking on older daemons; dropping them keeps the multi-arch list portable, and cosign still provides signatures.
- **Helm on top of raw manifests** — `k8s/` stays readable for a plain `kubectl apply`, while the chart handles per-environment differences without copy-pasted YAML trees.
- **Defense in depth over a WAF** — security headers, a body limit, and a per-IP token-bucket rate limiter live in the application, so the guarantees hold regardless of what sits in front of it. Proxy trust is off by default so client IPs cannot be spoofed via `X-Forwarded-For`.
- **Secrets out of git** — only a `REPLACE_ME` template is tracked, and the prod values file requires an externally managed Secret.

## License

MIT. See `LICENSE`.
