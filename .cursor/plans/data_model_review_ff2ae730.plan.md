---
name: Data model review
overview: Review the Postgres-backed data model against your four concerns (data consistency, performance, deduplication, supportability), document current state vs best practice, and recommend changes only where justified—with clear “leave as-is” justifications where the current design is sound.
todos: []
isProject: false
---

# Data Model and Database Layout Review

This plan documents the current schema and access patterns, compares them to Postgres best practice, and gives concrete recommendations—including when to **leave things as-is** and why.

---

## Current schema (summary)


| Table                | Purpose                                                  | User scoping               |
| -------------------- | -------------------------------------------------------- | -------------------------- |
| **users**            | Identity (OIDC); id, email, provider_id, name            | N/A                        |
| **todos**            | User tasks; `user_id` FK → users, ON DELETE CASCADE      | `user_id` column           |
| **oidc_config**      | OIDC provider config (global)                            | None (correct)             |
| **cors_config**      | CORS settings (global)                                   | None (correct)             |
| **ratelimit_config** | Rate limits (global)                                     | None (correct)             |
| **user_activity**    | Last API interaction, reprocessing pause; PK = `user_id` | By design one row per user |
| **ai_context**       | AI context summary + preferences; UNIQUE(user_id)        | By design one row per user |
| **tag_statistics**   | Aggregated tag stats per user; PK = `user_id`            | By design one row per user |


All user-scoped tables have `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`. Migrations live under [internal/database/migrations/](internal/database/migrations/).

---

## 1. Data consistency (no “bleeding” between users)

### Current behavior

- **todos**: [TodoRepository.GetByID](internal/database/todos.go) and Update/Delete use only `id` in the WHERE clause (e.g. `WHERE id = $1`). They do **not** filter by `user_id`. Isolation is enforced in application code:
  - Handlers: after `GetByID`, they check `todo.UserID != user.ID` and return 404/403 ([handlers/todos.go](internal/handlers/todos.go) at GetTodo, UpdateTodo, DeleteTodo, CompleteTodo, AnalyzeTodo).
  - Task analyzer: after `GetByID`, it checks `todo.UserID != job.UserID` ([analyzer.go](internal/workers/analyzer.go) ~line 158).
- **user_activity, ai_context, tag_statistics**: All access is by `user_id` only (GetByUserID, Upsert by user_id, etc.). There is no “get by id” that could return another user’s row. **Good.**
- **Global config** (oidc_config, cors_config, ratelimit_config): No user_id; correctly shared.

So today, isolation is **application-enforced**. The only place where “forgetting a check” could leak data is todo Get/Update/Delete: if a new handler or worker called `GetByID`/`Update`/`Delete` without verifying ownership, another user’s todo could be read or modified.

### Best practice and recommendation

- **Defense in depth**: Prefer enforcing tenant scope at the **database layer** as well, so a single missed check in app code cannot cross user boundaries.
- **Postgres options**: (1) Row Level Security (RLS) with a `current_setting('app.user_id')`-style tenant, or (2) repository methods that always include `user_id` in the predicate.

**Recommendation — enforce todo ownership in the repository (no RLS required for this step):**

- Add **GetByUserIDAndID(ctx, userID, todoID)** (or equivalent) that runs `SELECT ... WHERE user_id = $1 AND id = $2`. Use it everywhere today that does “GetByID then check user” (handlers + task analyzer). Then treat the old **GetByID** as internal/legacy and don’t use it for request-scoped access.
- Change **Update** and **Delete** to require `user_id` and use `WHERE id = $1 AND user_id = $2`. That way, even if someone passes the wrong todo, the DB won’t update/delete another user’s row.

**Leave as-is (with a short doc note):**

- **user_activity, ai_context, tag_statistics**: Already accessed only by `user_id`; no change needed. Optional later step: add RLS for defense in depth across all user-scoped tables.
- **Global config tables**: No user_id; leave as-is.
- **Cascades**: `ON DELETE CASCADE` from todos, user_activity, ai_context, tag_statistics → users is correct and should stay.

**Documentation:** Add a short “Data isolation” subsection (e.g. in [docs/](docs/) or README) stating: (1) all todo access must be scoped by the authenticated user; (2) repository methods that take `user_id` enforce this at the DB layer; (3) workers must only process jobs that carry the correct UserID and must validate todo ownership when loading by id.

---

## 2. Performance

### Current indexes (relevant to your concerns)

- **todos**: `idx_todos_user_id`, `idx_todos_time_horizon`, `idx_todos_status`, `idx_todos_metadata` (GIN), `idx_todos_due_date`.
- **user_activity**: `idx_user_activity_last_api_interaction`, `idx_user_activity_reprocessing_paused`.
- **ai_context**: `UNIQUE(user_id)`, `idx_ai_context_user_id`.
- **tag_statistics**: `idx_tag_statistics_user_id`, `idx_tag_statistics_tainted`, `idx_tag_statistics_last_analyzed_at`.
- **users**: `idx_users_email`, `idx_users_provider_id`.

### List-todos workload

The main list query is: **WHERE user_id = $1** [AND time_horizon = $2] [AND status = $3] **ORDER BY created_at DESC** with LIMIT/OFFSET ([todos.go](internal/database/todos.go) `GetByUserIDPaginated`). Postgres can use `idx_todos_user_id` and then sort by `created_at`. For large per-user lists, a composite index can avoid a separate sort.

**Recommendation:** Add a composite index for the list pattern, e.g.:

- `(user_id, created_at DESC)` — supports “my todos” ordered by created_at; optional filter on time_horizon/status can be applied on the fly.

If you usually filter by time_horizon and status, consider later (only if you see sort cost in EXPLAIN):

- `(user_id, time_horizon, status, created_at DESC)`.

One index is enough to start; the single composite on `(user_id, created_at DESC)` is a good default and aligns with “supportability” (fewer indexes to maintain).

### Redundant indexes

- **tag_statistics**: PK is `user_id`; `idx_tag_statistics_user_id` is redundant (PK already provides a unique index on `user_id`). **Recommendation:** Drop `idx_tag_statistics_user_id` in a migration to reduce write cost and catalog size.
- **ai_context**: `UNIQUE(user_id)` already implies an index on `user_id`; `idx_ai_context_user_id` is redundant. **Recommendation:** Drop `idx_ai_context_user_id`.
- **user_activity**: PK is `user_id`; there is no separate index on `user_id` in the migration list—only on `last_api_interaction` and `reprocessing_paused`. So no redundant user_id index here. **Leave as-is.**

**Leave as-is:** GIN on `todos.metadata` for JSONB, indexes on `users.email`/`provider_id`, and the reprocessing/activity indexes are appropriate for current query patterns.

---

## 3. Deduplication

- **Tags:**  
  - **Source of truth:** Per-todo tags live once in `todos.metadata` (e.g. `category_tags`, `tag_sources`).  
  - **Derived data:** `tag_statistics.tag_stats` is an **aggregate** over those todos (computed by the worker when tainted).  
  So you do not store “the same fact” in two places: you have one canonical place (todos) and one derived cache (tag_statistics). That’s a standard, supportable pattern (similar to a materialized view). **Leave as-is;** no deduplication change needed.
- **User identity:** Only in `users`. **Good.**  
- **Per-user singleton data:** `user_activity`, `ai_context`, and `tag_statistics` are one row per user, keyed by `user_id` (PK or UNIQUE). No duplication across tables. **Leave as-is.**  
- **Config:** Global config (OIDC, CORS, ratelimit) stored once in their respective tables. **Good.**

No structural deduplication recommendations; only the earlier note that tag_statistics is intentionally derived from todos and recomputed when tainted.

---

## 4. Supportability

- **Migrations:** Sequential, reversible (up/down), and one migration (000007) uses an idempotent enum add. This is good practice. **Leave as-is.**  
- **Naming:** Snake_case, consistent `user_id` for tenant key. **Good.**  
- **Types:** ENUMs for `time_horizon` and `todo_status` keep the schema clear. **Good.**  
- **JSONB:** Used for flexible but queryable fields (todos.metadata, tag_statistics.tag_stats, ai_context.preferences) with GIN where needed. **Good.**  
- **Foreign keys and cascades:** Clear FKs and ON DELETE CASCADE from user-scoped tables to users improve long-term supportability. **Leave as-is.**

**Recommendation:** Add a **data model / schema** section to the docs (e.g. in [docs/](docs/) or a new `docs/DATA_MODEL.md`). Include:

- Short description of each table and its role.
- Statement that user-scoped data is keyed by `user_id` and that todo access must always be user-scoped (and that repository methods will enforce this once the consistency changes are in).
- Note that tag_statistics is derived from todos and recomputed by the worker when tainted.
- List of migrations and how to run them (or point to existing migration docs).

That gives future maintainers (and “non–data people”) a single place to understand the layout and the “why.”

---

## Summary of recommendations


| Area                 | Action                                                                                                                                                                                                                             |
| -------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Data consistency** | Add todo access by (user_id, id): e.g. GetByUserIDAndID; Update/Delete with WHERE id AND user_id. Use these in handlers and task analyzer. Optionally deprecate public GetByID for request-scoped use. Document isolation in docs. |
| **Performance**      | Add composite index on todos (user_id, created_at DESC). Drop redundant indexes: idx_tag_statistics_user_id, idx_ai_context_user_id.                                                                                               |
| **Deduplication**    | No schema or storage changes; document that tag_statistics is derived from todos.                                                                                                                                                  |
| **Supportability**   | Keep migrations and naming as-is. Add a short data model doc (tables, user scoping, tag_statistics derivation, migration pointer).                                                                                                 |


Optional (not in initial scope): Postgres RLS on user-scoped tables for extra defense in depth; add only if you want a second enforcement layer after repository-level checks.

---

## Diagram (current user-scoped layout)

```mermaid
erDiagram
    users ||--o{ todos : "user_id"
    users ||--o| user_activity : "user_id PK"
    users ||--o| ai_context : "user_id UNIQUE"
    users ||--o| tag_statistics : "user_id PK"
    users {
        uuid id PK
        string email
        string provider_id
    }
    todos {
        uuid id PK
        uuid user_id FK
        text text
        enum time_horizon
        enum status
        jsonb metadata
    }
    user_activity {
        uuid user_id PK_FK
        timestamp last_api_interaction
        bool reprocessing_paused
    }
    ai_context {
        uuid id PK
        uuid user_id UNIQUE_FK
        text context_summary
        jsonb preferences
    }
    tag_statistics {
        uuid user_id PK_FK
        jsonb tag_stats
        bool tainted
    }
```



All relationships use `ON DELETE CASCADE` from the child to `users`.