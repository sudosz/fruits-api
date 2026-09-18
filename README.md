# Fruits API

RESTful service for managing fruits, written in Go with Gin and backed by PostgreSQL. Ships with Swagger docs, a multi-stage container image, Docker Compose for local work, Kubernetes manifests, and a GitHub Actions pipeline that tests against a real Postgres and publishes to GHCR.

## Features

- `GET`/`POST` endpoints over a single `fruits` table, no ORM
- Table auto-created on startup (`CREATE TABLE IF NOT EXISTS`)
- `/healthz` probe that pings the database, used by Docker and Kubernetes
- Swagger UI generated from handler annotations with `swaggo`
- Graceful shutdown on `SIGTERM` so rolling deploys don't cut requests
- Distroless-style runtime: static binary, non-root user, dropped capabilities, read-only root filesystem

**Stack**: Go 1.25 · Gin · PostgreSQL 16 · `lib/pq` · swaggo · Docker · Kubernetes · GitHub Actions

## API

| Method | Path             | Description                  | Success | Errors     |
| ------ | ---------------- | ---------------------------- | ------- | ---------- |
| GET    | `/fruits`        | List all fruits (`[]` empty) | 200     | 500        |
| GET    | `/fruits/{id}`   | Get one fruit                | 200     | 400, 404   |
| POST   | `/fruits`        | Create a fruit               | 201     | 400        |
| GET    | `/healthz`       | Readiness/liveness probe     | 200     | 503        |
| GET    | `/swagger/*any`  | Swagger UI                   | 200     | —          |

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
go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/api/main.go -o docs
```

## Running

### Docker Compose

```bash
docker compose up --build
```

Brings up Postgres with a `pg_isready` healthcheck plus the API on `localhost:8080`; the API waits for the database to report healthy.

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

Handler tests are integration tests: they drive the real router through `httptest` against a real Postgres, rather than mocking the database. When no database is reachable they skip, so the suite stays runnable on a laptop; CI provides a `postgres:16-alpine` service container, so the tests execute for real there. They truncate the `fruits` table before each test, so point them at a throwaway database.

## Kubernetes

```bash
kubectl apply -f k8s/postgres-pvc.yaml
kubectl apply -f k8s/configmap.yaml -f k8s/secret.yaml
kubectl apply -f k8s/postgres-deployment.yaml -f k8s/postgres-service.yaml
kubectl apply -f k8s/app-deployment.yaml -f k8s/app-service.yaml
```

The API runs 2 replicas of `ghcr.io/sudosz/fruits-api:latest` with liveness and readiness probes on `/healthz`, requests of 64Mi/50m and limits of 128Mi/200m, and is exposed on NodePort `30080`:

```bash
curl http://$(minikube ip):30080/fruits
```

`k8s/secret.yaml` holds demo credentials. Replace it before any real deployment:

```bash
kubectl create secret generic fruits-api-secret \
  --from-literal=DB_USER=postgres \
  --from-literal=DB_PASSWORD='<strong-password>'
```

## CI/CD

`.github/workflows/ci-cd.yml` runs on pushes and pull requests to `main`:

1. **test** — `gofmt` check, `go vet`, `go test -v -race ./...` against a Postgres service container.
2. **build-and-push** — only on pushes to `main`, after tests pass: logs in to `ghcr.io` with `GITHUB_TOKEN` and pushes `latest` plus a commit-SHA tag to `ghcr.io/sudosz/fruits-api`.

## AI Usage Disclosure

This project was built with an AI coding assistant (Claude Code) driving the implementation end to end, with human review of the output.

**Where it was applied**

- Go source: models, env config, database connection and migration, Gin handlers, integration tests
- Swagger annotations on handlers and the generated `docs/` package
- Multi-stage `Dockerfile`, `.dockerignore`, `docker-compose.yml`
- Kubernetes manifests in `k8s/`
- GitHub Actions pipeline
- This README

**Architectural choices and why**

- **Gin** — small, fast HTTP router with first-class swaggo integration, so the OpenAPI spec is generated from the handlers themselves instead of drifting in a separate file.
- **`database/sql` + `lib/pq`, no ORM** — the data model is one table and three columns; parameterized SQL is clearer than ORM configuration and keeps queries injection-safe.
- **Handlers talk to the database directly** — a repository or service layer would add indirection without a second consumer or second storage backend to justify it.
- **Integration tests over mocks** — mocking `database/sql` verifies the mock, not the SQL. Running against a real Postgres catches schema and query errors, which is why CI provisions one as a service container.
- **Startup migration** — `CREATE TABLE IF NOT EXISTS` is idempotent and safe across replicas, and removes a separate migration step for a schema this small.
- **Multi-stage build on a static binary** — the runtime image carries no Go toolchain, compiler, or source, which shrinks it and minimizes attack surface. It runs as non-root UID 10001, with a read-only root filesystem and all capabilities dropped in Kubernetes.
- **`/healthz` pings the database** — a probe that only proves the process is alive would keep routing traffic to a replica that cannot serve requests.
- **Graceful shutdown** — draining in-flight requests on `SIGTERM` avoids dropped connections during rolling deploys.

## License

MIT — see `LICENSE`.
