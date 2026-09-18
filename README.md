# Fruits API

[![CI/CD](https://github.com/sudosz/fruits-api/actions/workflows/ci-cd.yml/badge.svg?branch=main)](https://github.com/sudosz/fruits-api/actions/workflows/ci-cd.yml)
[![CodeQL](https://img.shields.io/github/actions/workflow/status/sudosz/fruits-api/ci-cd.yml?branch=main&label=codeql&logo=github)](https://github.com/sudosz/fruits-api/security/code-scanning)
[![Go Report Card](https://goreportcard.com/badge/github.com/sudosz/fruits-api)](https://goreportcard.com/report/github.com/sudosz/fruits-api)
[![Go Version](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white)](go.mod)
[![GHCR Image](https://img.shields.io/badge/ghcr.io-sudosz%2Ffruits--api-2496ED?logo=docker&logoColor=white)](https://github.com/sudosz/fruits-api/pkgs/container/fruits-api)
[![Platforms](https://img.shields.io/badge/platforms-linux%2Famd64%20%7C%20linux%2Farm64-informational)](https://github.com/sudosz/fruits-api/pkgs/container/fruits-api)
[![Signed with cosign](https://img.shields.io/badge/supply%20chain-cosign%20%7C%20SBOM%20%7C%20provenance-4B32C3?logo=sigstore&logoColor=white)](https://docs.sigstore.dev/cosign/overview/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

RESTful service for managing fruits, written in Go with Gin and backed by PostgreSQL. Ships with Swagger docs, a multi-stage multi-arch container image, Docker Compose for local work, hardened Kubernetes manifests, and a GitHub Actions pipeline that lints, scans, tests against a real Postgres, and publishes signed images with SBOM and provenance to GHCR.

## Features

- `GET`/`POST` endpoints over a single `fruits` table, no ORM
- Table auto-created on startup (`CREATE TABLE IF NOT EXISTS`)
- `/healthz` probe that pings the database, used by Docker and Kubernetes
- Swagger UI generated from handler annotations with `swaggo`
- Graceful shutdown on `SIGTERM` so rolling deploys don't cut requests
- Security middleware: hardening headers, request body limit, per-IP rate limiting
- Hardened HTTP server timeouts and no implicit proxy trust
- Distroless-style runtime: static binary, non-root user, dropped capabilities, read-only root filesystem
- Multi-arch images (`linux/amd64`, `linux/arm64`) signed with cosign, published with SBOM and build provenance

**Stack**: Go 1.25 · Gin · PostgreSQL 16 · `lib/pq` · swaggo · Docker · Kubernetes · GitHub Actions

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

Errors are always `{"error": "..."}`.

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

Brings up Postgres with a `pg_isready` healthcheck plus the API on `localhost:8080`; the API waits for the database to report healthy. Postgres is not published to the host, and the API container runs read-only with all capabilities dropped and `no-new-privileges`.

### Locally

Needs a reachable Postgres.

```bash
go run ./cmd/api
```

### Configuration

All configuration is environment variables. `DATABASE_URL`, if set, overrides the individual `DB_*` values.

| Variable      | Default     |
| ------------- | ----------- |
| `PORT`        | `8080`      |
| `DB_HOST`     | `localhost` |
| `DB_PORT`     | `5432`      |
| `DB_USER`     | `postgres`  |
| `DB_PASSWORD` | `postgres`  |
| `DB_NAME`     | `fruitsdb`  |
| `DB_SSLMODE`  | `disable`   |

## Tests

```bash
go test -v ./...
go test -v -race ./...
```

Handler tests are integration tests: they drive the real router through `httptest` against a real Postgres, rather than mocking the database. When no database is reachable they skip, so the suite stays runnable on a laptop; CI provides a `postgres:16-alpine` service container and **fails the build if any integration test skips**, so they always execute for real there. They truncate the `fruits` table before each test, so point them at a throwaway database. Middleware has its own unit tests that need no database.

## Linting and security scanning

```bash
gofmt -l .
go vet ./...
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2 run
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
```

`.golangci.yml` enables, beyond the defaults: `gosec`, `bodyclose`, `rowserrcheck`, `sqlclosecheck`, `noctx`, `errorlint`, `nilerr`, `gocritic`, `revive`, `misspell`, `unconvert`, `unparam`, `wastedassign`, `copyloopvar`, with `gofmt` and `goimports` as formatters.

CI additionally runs `govulncheck` (Go module CVEs), `gosec`, Trivy (filesystem and image, vulnerabilities/secrets/misconfiguration), `gitleaks` (committed secrets), `hadolint` (Dockerfile), and CodeQL. Findings are uploaded as SARIF to the repository's code scanning tab.

## Kubernetes

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

The API runs 2 replicas of `ghcr.io/sudosz/fruits-api:latest` with startup, liveness, and readiness probes on `/healthz`, requests of 64Mi/50m and limits of 128Mi/200m, and is exposed on NodePort `30080`:

```bash
curl http://$(minikube ip):30080/fruits
```

Hardening applied to the manifests:

- Pods run as non-root UID 10001 with `readOnlyRootFilesystem`, `allowPrivilegeEscalation: false`, all capabilities dropped, and the `RuntimeDefault` seccomp profile
- Dedicated ServiceAccounts with `automountServiceAccountToken: false`
- NetworkPolicies default-deny both directions; the API may reach only Postgres `5432` and kube-DNS, and Postgres accepts connections only from API pods
- PodDisruptionBudget keeps at least one replica during voluntary disruptions, plus `maxUnavailable: 0` rolling updates and topology spread across nodes

`k8s/secret.example.yaml` is a template only. Real Secret files (`k8s/secret.yaml`, `k8s/*secret*.local.yaml`) are git-ignored; for production use sealed-secrets, SOPS, or External Secrets rather than a committed manifest.

## CI/CD

`.github/workflows/ci-cd.yml` runs on pushes and pull requests to `main`, on `v*.*.*` tags, and weekly. Permissions are read-only at the top level and widened per job; runs are cancelled on new PR pushes via a concurrency group.

1. **lint** — `gofmt` check, `go vet`, `golangci-lint`, `hadolint`.
2. **security** — `govulncheck`, `gosec`, Trivy filesystem scan, `gitleaks`; results uploaded as SARIF.
3. **codeql** — CodeQL `security-and-quality` analysis for Go.
4. **test** — `go test -race` with coverage against a `postgres:16-alpine` service container; fails if integration tests skip.
5. **build-and-push** — only on pushes, after all of the above: builds `linux/amd64` and `linux/arm64` with Buildx and QEMU, pushes `latest`, a long commit SHA tag, and semver tags to GHCR, attaches an SBOM and `mode=max` provenance, attests build provenance, signs the digest keylessly with cosign, and fails on Trivy image findings.

Dependabot (`.github/dependabot.yml`) keeps Go modules, GitHub Actions, and base images current with grouped weekly PRs.

## AI Usage Disclosure

This project was built with an AI coding assistant (Claude Code) driving the implementation end to end, with human review of the output.

**Where it was applied**

- Go source: models, env config, database connection and migration, Gin handlers, security middleware, integration and unit tests
- Swagger annotations on handlers and the generated `docs/` package
- Multi-stage `Dockerfile`, `.dockerignore`, `docker-compose.yml`
- Kubernetes manifests in `k8s/`
- GitHub Actions pipeline, `.golangci.yml`, Dependabot config
- This README

**Architectural choices and why**

- **Gin** — small, fast HTTP router with first-class swaggo integration, so the OpenAPI spec is generated from the handlers themselves instead of drifting in a separate file.
- **`database/sql` + `lib/pq`, no ORM** — the data model is one table and three columns; parameterized SQL is clearer than ORM configuration and keeps queries injection-safe.
- **Handlers talk to the database directly** — a repository or service layer would add indirection without a second consumer or second storage backend to justify it.
- **Context-aware database calls** — `Connect`, `Migrate`, and every query take a context, so startup retries and shutdown cancel cleanly instead of hanging a pod through its probe deadlines.
- **`run()` instead of `log.Fatal` in `main`** — fatal logging in `main` skips deferred cleanup, so errors bubble up to a single exit point that still closes the database pool.
- **Integration tests over mocks** — mocking `database/sql` verifies the mock, not the SQL. Running against a real Postgres catches schema and query errors, which is why CI provisions one as a service container and treats a skip as a failure.
- **Startup migration** — `CREATE TABLE IF NOT EXISTS` is idempotent and safe across replicas, and removes a separate migration step for a schema this small.
- **Multi-stage build on a static binary** — the runtime image carries no Go toolchain, compiler, or source, which shrinks it and minimizes attack surface. It runs as non-root UID 10001, with a read-only root filesystem and all capabilities dropped in Kubernetes.
- **Cross-compilation instead of emulation for multi-arch** — the builder stage stays on the native platform and Go cross-compiles to `TARGETARCH`, so arm64 images build in seconds rather than under QEMU.
- **Defense in depth over a WAF** — security headers, an 8 KiB body limit, and a per-IP token-bucket rate limiter live in the application, so the guarantees hold regardless of what sits in front of it. Proxy trust is off by default so client IPs cannot be spoofed via `X-Forwarded-For`.
- **Secrets out of git** — a committed Secret manifest is a credential leak even in a demo, so only a `REPLACE_ME` template is tracked and real Secrets are created out of band.
- **Signed images with SBOM and provenance** — consumers can verify what they are running and where it came from, which is the supply-chain half of security that scanners alone do not cover.
- **`/healthz` pings the database** — a probe that only proves the process is alive would keep routing traffic to a replica that cannot serve requests.
- **Graceful shutdown** — draining in-flight requests on `SIGTERM` avoids dropped connections during rolling deploys.

## License

MIT — see `LICENSE`.
