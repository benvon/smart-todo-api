# Data Model and Schema

This document describes the database layout, user scoping, and supportability practices. It is intended for maintainers and operators who are not primarily "data people."

## Tables

| Table | Purpose |
|-------|---------|
| **users** | Identity (OIDC). Columns: id, email, provider_id, name, email_verified, created_at, updated_at. |
| **todos** | User tasks. Each row has `user_id` referencing users(id). Columns include text, time_horizon, status, metadata (JSONB), due_date, completed_at. |
| **oidc_config** | OIDC provider configuration (global, not per-user). |
| **cors_config** | CORS settings (global). |
| **ratelimit_config** | Rate limit settings (global). |
| **user_activity** | One row per user: last API interaction, reprocessing pause flag. Primary key is `user_id`. |
| **ai_context** | One row per user: AI context summary and preferences (JSONB). Unique on `user_id`. |
| **tag_statistics** | One row per user: aggregated tag stats (JSONB) and tainted/version fields. Primary key is `user_id`. |

All user-scoped tables have `user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE`, so deleting a user removes their related rows.

## Data isolation

- **Todo access must always be scoped by the authenticated user.** The API uses repository methods that take `user_id` and enforce scope at the database layer:
  - **GetByUserIDAndID(ctx, userID, todoID)** — fetches a todo only if it belongs to that user (`WHERE user_id = $1 AND id = $2`).
  - **Update** — updates only when the todo’s `user_id` matches (`WHERE id = $1 AND user_id = $2`).
  - **Delete(ctx, userID, id)** — deletes only when the row belongs to that user (`WHERE id = $1 AND user_id = $2`).
- **user_activity, ai_context, tag_statistics** are accessed only by `user_id` (e.g. GetByUserID, Upsert by user_id). There is no "get by id" that could return another user’s row.
- **Workers** must only process jobs that carry the correct `UserID` and must load todos via user-scoped methods (e.g. GetByUserIDAndID) so the database never returns another user’s data.

Using these patterns ensures a single missed check in application code cannot cause data to "bleed" between users.

## Tag statistics and deduplication

- **Source of truth for tags:** Per-todo tags live in `todos.metadata` (e.g. `category_tags`, `tag_sources`).
- **Derived data:** `tag_statistics.tag_stats` is an **aggregate** over those todos. It is computed by the worker when a user’s stats are "tainted" (e.g. after tag changes). So tag statistics are not duplicated facts—they are a derived cache (similar to a materialized view) and are recomputed from todos when needed.

## Migrations

Migrations are in `internal/database/migrations/` and are applied with [golang-migrate](https://github.com/golang-migrate/migrate). See the [README Database Migrations section](../README.md#database-migrations) for how to run them:

```bash
# Apply migrations
migrate -path internal/database/migrations -database "$DATABASE_URL" up

# Rollback one migration
migrate -path internal/database/migrations -database "$DATABASE_URL" down 1

# Check current version
migrate -path internal/database/migrations -database "$DATABASE_URL" version
```

Migrations are sequential, reversible (up/down), and use consistent naming (e.g. `000012_todos_list_index_and_drop_redundant.up.sql`).
