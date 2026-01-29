---
name: Go API and Worker E2E Review
overview: End-to-end review of the Go API and worker to reduce cyclomatic complexity (with gocyclo in CI), improve code structure and extensibility, fix compartmentalization issues, complete the RabbitMQ queue layer (including real DLQ GC), replace CORS with rs/cors (configurable via config tool + DB), explore Redis-backed rate limiting that preserves multi-node sharing, and ensure Docker builds and tests stay in sync.
todos:
  - id: gocyclo-ci
    content: Enable gocyclo (or cyclop) in .golangci.yml and verify local + GitHub CI
    status: completed
  - id: reduce-complexity
    content: Refactor analyzer, todos handler, auth middleware, health handler, openai service
    status: completed
  - id: compartmentalization
    content: Add internal/request, move ClientIP/UserFromContext, introduce Pinger for health
    status: completed
  - id: queue-complete
    content: Complete RabbitMQ queue layer (processor registry, GC decision, tests)
    status: completed
  - id: structure-extensibility
    content: TodoHandler options, processor registry, ensure cmd/server and cmd/worker exist
    status: completed
  - id: docker-builds
    content: Update server.Dockerfile and worker.Dockerfile if server/worker build or CMD changes
    status: completed
  - id: docs-ci
    content: Document complexity threshold in CONTRIBUTING or docs/TESTING; confirm pre-commit/CI
    status: completed
  - id: cors-rs-db
    content: Replace CORS with rs/cors; store rules in DB; configure cors list/set; server loads from DB; hot-reload
    status: completed
  - id: ratelimit-redis
    content: Replace custom rate limiter; Use ulule/limiter + Redis + sliding window; store config in DB; configure ratelimit list/set”
    status: completed
  - id: todo-1769638438346-j58ll8ndu
    content: Add/update unit and integration tests for all refactors
    status: pending
isProject: false
---

# Go API and Worker End-to-End Review Plan

## 1. Cyclomatic Complexity

### 1.1 Add gocyclo / cyclop to CI

**Current state:** [.golangci.yml](.golangci.yml) enables `govet`, `errcheck`, `staticcheck`, `unused`, `bidichk`, `contextcheck`, `ineffassign`. No cyclomatic-complexity linters. Pre-commit runs `golangci-lint` ([.pre-commit-config.yaml](.pre-commit-config.yaml)); [.github/workflows/ci.yml](.github/workflows/ci.yml) runs the lint job.

**Actions:**

- **Enable `gocyclo**` (or `cyclop`) in [.golangci.yml](.golangci.yml):
  - Add to `linters.enable` and set `min-complexity` (gocyclo) or `max-complexity` (cyclop). Start with **15** (gocyclo) or **10** (cyclop); tighten to **10** / **8** once hotspots are reduced.
  - Example gocyclo config:
    ```yaml
    linters:
      enable: [..., gocyclo]
    linters-settings:
      gocyclo:
        min-complexity: 15
    ```
- **Local CI:** [Makefile](Makefile) `lint` target already runs `golangci-lint`; no change needed.
- **GitHub CI:** Lint job uses `golangci-lint`; it will pick up the new linter once config is updated.

### 1.2 High-complexity hotspots to refactor


| Location                                                         | Issue                                                                                                                  | Refactor                                                                                                                                    |
| ---------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- |
| [internal/workers/analyzer.go](internal/workers/analyzer.go)     | `handleJobError` (~430–519): many branches (quota, rate limit, retry, DLQ, ack/nack).                                  | Extract helpers: `handleQuotaError`, `handleRateLimitError`, `handleGenericRetry`, `sendToDLQ`. Keep `handleJobError` as a thin dispatcher. |
| [internal/workers/analyzer.go](internal/workers/analyzer.go)     | `ProcessTaskAnalysisJob` and `ProcessReprocessUserJob`: duplicated “call AI (with/without due date, tag stats)” logic. | Extract `analyzeTodoWithProvider(ctx, provider, todo, userContext, tagStats) (tags, timeHorizon, err)`, use in both.                        |
| [internal/workers/analyzer.go](internal/workers/analyzer.go)     | `ProcessJob`: switch + ack/nack handling.                                                                              | Consider small helpers per job type if cyclop still flags it after the above.                                                               |
| [internal/handlers/todos.go](internal/handlers/todos.go)         | `UpdateTodo`: many optional fields, validation branches.                                                               | Extract `parseAndValidateUpdateRequest`, `applyUpdatesToTodo`; keep handler thin.                                                           |
| [internal/handlers/todos.go](internal/handlers/todos.go)         | `ListTodos`: query parsing + validation.                                                                               | Extract `parseListParams(r)` returning `(page, pageSize, timeHorizon, status, err)`.                                                        |
| [internal/middleware/auth.go](internal/middleware/auth.go)       | Auth middleware: token validation, get-or-create user, update user.                                                    | Extract `getOrCreateUser(ctx, repo, claims)`, `maybeUpdateUser(ctx, repo, user, claims)`; keep middleware as orchestration only.            |
| [internal/services/ai/openai.go](internal/services/ai/openai.go) | `AnalyzeTaskWithDueDate`: prompt build, API call, JSON parse, validation.                                              | Split: `buildAndSendAnalysisRequest`, `parseAndValidateAnalysisResponse`.                                                                   |
| [internal/handlers/health.go](internal/handlers/health.go)       | `HealthCheck`: mode branch, multiple checks, response build.                                                           | Extract `runExtendedChecks(ctx) (checks map[string]string, status string)`, `writeHealthResponse(w, status, checks)`.                       |


**Additional cleanup:** Remove duplicated `if err != nil` block in [internal/services/ai/openai.go](internal/services/ai/openai.go) (lines 165–182 are unreachable after the earlier return).

---

## 2. Code Model / Structure and Extensibility

### 2.1 Current layout

- **API:** [api/openapi/openapi.yaml](api/openapi/openapi.yaml) defines contract. Handlers live in [internal/handlers/](internal/handlers/); each handler has `RegisterRoutes(r)` and is wired from the server entrypoint (`cmd/server` per [Makefile](Makefile), [ci.yml](.github/workflows/ci.yml), Dockerfiles).
- **Worker:** [internal/workers/](internal/workers/) (`TaskAnalyzer`, `TagAnalyzer`, `Reprocessor`). Job types in [internal/queue/job.go](internal/queue/job.go). Processing switch in `TaskAnalyzer.ProcessJob` and consumer wiring in `cmd/worker`.

**Note:** `cmd/server` and `cmd/worker` are referenced by Makefile, CI, Dockerfiles, and README but do not appear under `cmd/` in the current layout (only `cmd/configure`). Ensure those entrypoints exist and that all routing/worker registration lives there.

### 2.2 Docker builds

- **[server.Dockerfile](server.Dockerfile)** builds `./cmd/server` and `./cmd/configure`, copies binaries and migrations, and runs `start_server.sh`. **[worker.Dockerfile](worker.Dockerfile)** builds `./cmd/worker` and runs `/app/worker`.
- **Whenever server or worker entrypoints, build paths, or CMD change:** update both Dockerfiles accordingly. That includes:
  - Changing `go build -o ... ./cmd/server` or `./cmd/worker` paths if you move or rename those packages.
  - Updating `CMD` or `start_server.sh` if you change how the server or worker process is invoked.
  - Adjusting any `COPY` paths for new binaries or assets.
- Keep Docker builds in lockstep with Makefile and CI (both already use `./cmd/server` and `./cmd/worker`).

### 2.3 Extensibility improvements

- **New API endpoints:** Add a new handler (e.g. `internal/handlers/foo.go`), implement `RegisterRoutes`, and register in `cmd/server` next to existing handlers. Update OpenAPI spec. No structural change required.
- **New job types / workers:**
  - Add `JobTypeX` in [internal/queue/job.go](internal/queue/job.go).
  - Add processor (e.g. `ProcessXJob`) and call it from `ProcessJob` (or a dedicated consumer if you split by queue). Consider a **processor registry** (map `JobType` → handler func) to avoid a single growing `switch` and make new job types additive.
- **Handler construction:** `TodoHandler` has multiple constructors (`NewTodoHandler`, `NewTodoHandlerWithQueue`, `NewTodoHandlerWithQueueAndTagStats`). Prefer a **single constructor + optional config struct** (or functional options) to keep wiring clear and avoid constructor sprawl as dependencies grow.
- **Health checks:** `HealthChecker` takes optional `RedisRateLimiter` and `JobQueue`. The pattern is good. Keep health-specific types (e.g. `Pinger`) behind interfaces so handlers need not depend on middleware concretions (see §3).

---

## 3. Code Compartmentalization and Dependencies

### 3.1 Dependency overview

```
handlers → database, logger, middleware, models, queue, validation
middleware → database, logger, models, services/oidc
workers → database, logger, models, queue, services/ai
services/oidc → database, models
services/ai → models
validation → models
database → models
```

### 3.2 Issues and changes


| Issue                          | Location                                                                                                                                                   | Change                                                                                                                                                                                                                                                                                                                                                                           |
| ------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Handlers import middleware** | [handlers/todos.go](internal/handlers/todos.go), [handlers/health.go](internal/handlers/health.go), etc.                                                   | Handlers use `middleware.UserFromContext` and `middleware.RedisRateLimiter`. Move `UserFromContext` (and shared request helpers) to a small `**internal/request**` (or `internal/context`) package used by both handlers and middleware. Handlers then depend on `request` instead of `middleware` for context helpers.                                                          |
| **Health → middleware**        | [handlers/health.go](internal/handlers/health.go)                                                                                                          | `HealthChecker` depends on `*middleware.RedisRateLimiter` for Redis ping. Introduce a `**Pinger` interface** (e.g. `Ping(ctx) error`) in `handlers` or a shared `health` package. Implement it for Redis (e.g. in middleware or a `ratelimit` package). `HealthChecker` accepts `Pinger` instead of `*RedisRateLimiter`. Same idea for queue: keep using `JobQueue.HealthCheck`. |
| **Duplicate client-IP logic**  | [middleware/auth.go](internal/middleware/auth.go) (`getClientIP`), [middleware/ratelimit.go](internal/middleware/ratelimit.go) (`getClientIPForRateLimit`) | Both implement the same X-Forwarded-For / X-Real-IP / RemoteAddr logic. Extract to `**internal/request**` (or `internal/network`) as `ClientIP(r *http.Request) string` and use it from auth and ratelimit.                                                                                                                                                                      |
| **Middleware → database**      | auth, activity                                                                                                                                             | Auth and activity middleware reasonably perform DB lookups. No change required; keep via interfaces where it helps testing.                                                                                                                                                                                                                                                      |


### 3.3 Desired dependency flow

- `**internal/request**`: HTTP helpers (`ClientIP`, `UserFromContext` if moved here). No imports from `handlers` or `middleware`.
- `**internal/models**`: Domain types only; remains a leaf.
- `**validation**`, `**database**`: Depend on `models`; no circular deps.
- `**handlers**`: Use `request`, `validation`, `database`, `queue`, `models`, `logger`. Use `Pinger` (and `JobQueue`) for health, not concrete middleware types.
- `**middleware**`: Use `request`, `database`, `logger`, `services/oidc`, `models`. Implement `Pinger` for Redis if that stays in middleware.

---

## 4. Home-Grown vs Third-Party Libraries

### 4.1 Keep as-is

- **Validation:** [go-playground/validator](https://github.com/go-playground/validator) plus thin custom validators (`ValidateTimeHorizon`, `ValidateTodoStatus`) and `SanitizeText` in [internal/validation/validators.go](internal/validation/validators.go). Good.
- **Logger:** [zap](https://go.uber.org/zap) + [internal/logger/sanitize.go](internal/logger/sanitize.go). Sanitization is minimal and logging-specific; keep.
- **OIDC/JWT:** [lestrrat-go/jwx](https://github.com/lestrrat-go/jwx), [golang.org/x/oauth2](https://pkg.go.dev/golang.org/x/oauth2). Custom OIDC client/JWKS in [internal/services/oidc/](internal/services/oidc/). Works; optional future swap to `coreos/go-oidc/v3` only if you standardize on that ecosystem.

### 4.2 Queue: make it more complete (stay on RabbitMQ)

**Scope:** Improve the existing RabbitMQ-based queue layer; **do not** switch to Redis, asynq, or another broker.

- **Processor registry:** Introduce a `JobType` → handler registry (e.g. in `internal/queue` or `internal/workers`) so new job types are additive. Replace the central `switch` in `TaskAnalyzer.ProcessJob` with registry lookup. Register `task_analysis`, `reprocess_user`, `tag_analysis` (and any other existing types) there.
- **GC:** [internal/queue/gc.go](internal/queue/gc.go) is a placeholder. **Implement real DLQ garbage collection** for RabbitMQ: query the DLQ, expire or purge messages older than a configurable retention period, and optionally archive before purge. Wire GC into the existing `GarbageCollector` and document behavior.
- **Tests:** Add or extend unit tests for queue wiring, job marshaling, registry lookup, and GC. Ensure existing worker tests (e.g. `analyzer_test`, `tag_analyzer_test`, `reprocessor_test`) still pass and cover new paths.
- **Interfaces:** Keep `JobQueue` and `MessageInterface`; add or adjust as needed for registry-based dispatch. Retain DLQ and delayed-exchange setup.

### 4.3 CORS: replace with `rs/cors` and configure via config tool + DB

- **Replace** the custom [internal/middleware/cors.go](internal/middleware/cors.go) implementation with `**github.com/rs/cors**`. Use `cors.Options` (AllowedOrigins, AllowedMethods, AllowedHeaders, AllowCredentials, MaxAge, etc.) and `cors.New(options).Handler(next)`.
- **Store CORS rules in the database** (similar to OIDC config). Add a migration for a `cors_config` table (e.g. allowed origins, optionally methods/headers overrides). Introduce `CORSConfigRepository` and models, and a service or loader that reads CORS config from DB.
- **Configure CORS via the `config` tool** ([cmd/configure](cmd/configure)). Add subcommands (e.g. `configure cors list`, `configure cors set`) to list and update CORS rules in the DB, mirroring the pattern used for OIDC (`configure oidc`, `configure list`, `configure test`).
- **Server wiring:** On startup, load CORS config from DB (with env-based fallback if needed, e.g. `FRONTEND_URL`), build `cors.Options`, and wrap the API with `rs/cors`.
- **Hot-reload:** Implement **CORS config hot-reload**. The server periodically re-reads CORS rules from the DB (e.g. background ticker with configurable interval) and updates the CORS middleware in place. Use a lock or atomic swap so the handler always serves the latest config without restart. Changes made via `configure cors set` thus take effect across server nodes after the next reload cycle.
- **Tests:** Add tests for the CORS loader, config tool CORS commands, middleware behavior with `rs/cors`, and hot-reload (e.g. update DB, trigger reload, verify behavior).

### 4.4 Rate limiting: keep Redis-backed, multi-node sharing

- **Requirement:** Rate limiting must **continue to use Redis** (or another shared store) so that limits are shared across **multiple server nodes**. In-process-only solutions (e.g. `golang.org/x/time/rate` alone) do not satisfy this.
- **Explore options** that preserve Redis-backed, distributed rate limiting. For example:
  - `**github.com/ulule/limiter**` – Redis-backed, supports multiple algorithms (fixed window, sliding), well-used. Evaluate whether its API and behavior (e.g. sliding vs fixed window) match your needs.
  - **Keep the custom Redis implementation** in [internal/middleware/ratelimit.go](internal/middleware/ratelimit.go) but refactor for clarity and testability (extract store interface, add tests). You already have Redis sliding-window logic; preserving it is valid if you prefer full control.
  - Other Redis-based limiters (e.g. `go-redis/redis` rate limiter patterns, or small libraries that use Redis) — assess fit.
- **Do not** switch to an in-process-only limiter unless it is used in addition to a Redis-backed one (e.g. per-node + global), and the latter remains the source of truth for cross-node limits.

### 4.5 Optional / nice-to-have

- **Router:** [gorilla/mux](https://github.com/gorilla/mux) is fine. **chi** or **echo** are alternatives if you later want a different DX; not required for this review.

---

## 5. Testing

- **Update or add tests for every refactor.** Preserve and extend coverage for:
  - **Workers:** [internal/workers/analyzer_test.go](internal/workers/analyzer_test.go), [reprocessor_test.go](internal/workers/reprocessor_test.go), [tag_analyzer_test.go](internal/workers/tag_analyzer_test.go), [tag_analyzer_pagination_test.go](internal/workers/tag_analyzer_pagination_test.go). After extracting `handleQuotaError` / `handleRateLimitError` / `analyzeTodoWithProvider` and introducing a processor registry, add or adapt tests for new units and ensure existing scenarios still pass.
  - **Handlers:** [internal/handlers/todos_test.go](internal/handlers/todos_test.go), [health_test.go](internal/handlers/health_test.go), and others. Cover new helpers (`parseListParams`, `parseAndValidateUpdateRequest`, `applyUpdatesToTodo`, `runExtendedChecks`, `writeHealthResponse`) and any changed handler behavior.
  - **Middleware:** [internal/middleware/](internal/middleware/) tests (e.g. `usercontext_test`, `error_test`, `logging_test`). After moving `UserFromContext` to `internal/request`, update middleware tests and add `internal/request` tests for `ClientIP` and `UserFromContext`.
  - **Queue:** [internal/queue/job_test.go](internal/queue/job_test.go) and any new queue or registry code. Add tests for registry-based dispatch, job marshaling, and **GC**.
  - **CORS:** Loader that reads from DB, config tool CORS commands (`configure cors list` / `cors set`), middleware behavior with `rs/cors`, and **hot-reload** (update DB, trigger reload, assert config applies).
  - **Rate limiting:** If you replace or refactor the Redis rate limiter, add tests for the new implementation (store interface, multi-node behavior where feasible).
  - **AI service:** [internal/services/ai/](internal/services/ai/) tests. Ensure `buildAndSendAnalysisRequest` / `parseAndValidateAnalysisResponse` and related paths are covered.
- **Run the full test suite** (`make test` / `go test ./...`) and fix any regressions before considering each phase done.
- **CI:** Tests already run in GitHub Actions; keep that passing and add any new test targets or coverage goals only if you choose to.

---

## 6. Implementation Order

1. **CI and complexity tooling**
  - Enable `gocyclo` (or `cyclop`) in `.golangci.yml` with a threshold of 15 (or 10 for cyclop).  
  - Run `golangci-lint run` and fix any new failures that are low-effort.  
  - Add a brief “Complexity” section to [CONTRIBUTING.md](CONTRIBUTING.md) or [docs/TESTING.md](docs/TESTING.md) documenting the threshold and how to fix violations.
2. **Reduce complexity**
  - Refactor [internal/workers/analyzer.go](internal/workers/analyzer.go) (`handleJobError`, AI-call dedup).  
  - Refactor [internal/handlers/todos.go](internal/handlers/todos.go) and [internal/middleware/auth.go](internal/middleware/auth.go) as in §1.2.  
  - Fix [internal/services/ai/openai.go](internal/services/ai/openai.go) (remove dead code, split analysis flow).  
  - Refactor [internal/handlers/health.go](internal/handlers/health.go).  
  - **Add/update tests** for all of the above (§5).  
  - Lower gocyclo/cyclop threshold toward 10 (or 8) and fix remaining violations.
3. **Compartmentalization**
  - Add `internal/request`, move `ClientIP` and `UserFromContext`, switch handlers/middleware to use it.  
  - Introduce `Pinger` for health and decouple `HealthChecker` from `*middleware.RedisRateLimiter`.  
  - **Add/update tests** for `request` and health wiring (§5).
4. **Queue completion (RabbitMQ-only)**
  - Introduce processor registry, wire existing job types, replace `ProcessJob` switch.  
  - **Implement real DLQ GC** in [internal/queue/gc.go](internal/queue/gc.go): query DLQ, expire/purge by retention, optionally archive; wire into `GarbageCollector`.  
  - **Add/update queue and worker tests** (§5), including GC.
5. **Structure and extensibility**
  - Option struct (or functional options) for `TodoHandler`; ensure processor registry is used.  
  - Ensure `cmd/server` and `cmd/worker` exist and that all route and worker registration lives there.  
  - **If server/worker build or CMD changes:** update [server.Dockerfile](server.Dockerfile) and [worker.Dockerfile](worker.Dockerfile) (§2.2), and adjust Makefile/CI if needed.  
  - **Add/update tests** for new handler options and worker wiring (§5).
6. **CORS: `rs/cors` + config tool + DB + hot-reload**
  - Replace custom CORS middleware with `github.com/rs/cors`.  
  - Add `cors_config` migration, repo, and loader; store CORS rules in DB.  
  - Add `configure cors list` / `configure cors set` (or similar) to [cmd/configure](cmd/configure).  
  - Server loads CORS from DB at startup and configures `rs/cors`.  
  - **Implement CORS hot-reload:** background ticker (configurable interval) re-reads from DB and updates the CORS handler (lock or atomic swap).  
  - **Add/update tests** for CORS loader, config commands, middleware, and hot-reload (§5).
7. **Rate limiting:  Redis-backed**
  - Use ulule/limiter with Redis and sliding window, config in DB, configure via configure tool, and server loading from DB. Libraries row updated to “adopt ulule/limiter (Redis, sliding window)
  - Add `configure ratelimit list` / `configure ratelimit set` (or similar) to [cmd/configure](cmd/configure)
  - Server loads ratelimit from DB at startup. If no configuration exists, set a default of 5 requests / 1 second and save to the database.
  - Implement chosen approach and **add/update tests** (§5).

---

## 7. Summary


| Area                     | Actions                                                                                                                                                                                                               |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Complexity**           | Enable gocyclo/cyclop in golangci-lint (local + GitHub CI); refactor analyzer, todos, auth, health, openai; lower threshold over time.                                                                                |
| **Structure**            | Centralize routing/workers in `cmd/server` / `cmd/worker`; optional config for handlers; processor registry for jobs.                                                                                                 |
| **Compartmentalization** | Add `internal/request`, move shared HTTP helpers, introduce `Pinger` for health, remove handlers → middleware and health → middleware concretions.                                                                    |
| **Queue (RabbitMQ)**     | Processor registry, **implement real DLQ GC** (query DLQ, retention-based purge, optional archive), tests. Stay on RabbitMQ.                                                                                          |
| **CORS**                 | Replace with `github.com/rs/cors`; store rules in DB; configure via `configure` tool (`cors list` / `cors set`); server loads from DB at startup; **hot-reload** (periodic re-read from DB, update handler in place). |
| **Rate limiting**        | use ulule/limiter with Redis and sliding window, config in DB, configure via configure tool, and server loading from DB. Libraries row updated to “adopt ulule/limiter (Redis, sliding window)                        |
| **Docker**               | Update [server.Dockerfile](server.Dockerfile) and [worker.Dockerfile](worker.Dockerfile) whenever server/worker build paths or CMD change.                                                                            |
| **Testing**              | Add/update tests for all refactors (workers, handlers, middleware, request, queue, GC, CORS, rate limiting, AI). Run `make test` and fix regressions.                                                                 |
| **Libraries**            | Keep validation, logger, OIDC as-is; adopt `rs/cors`; rate limiter per §4.4.                                                                                                                                          |


After refactors, keep golangci-lint (including gocyclo/cyclop) in both pre-commit and CI.