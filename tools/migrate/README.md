# Migration helper compatibility

This candidate helper reads `DATABASE_URL` from the environment and accepts
`version`, `up`, and read-only `inspect`. Driver errors, SQL, credentials, and connection URLs are never
printed. A successful result requires the helper's exact database session to be
closed and checked. An unconfirmed cleanup always fails permanently, even when
its original error was a retryable readiness deadline or network failure.

The helper supports PostgreSQL's default execution path: an entire migration file
is passed unchanged to the upstream PostgreSQL driver's `Run`. Ordinary SQL files
containing several statements remain supported. It buffers the body and checks
cancellation before delegating, guards subsequent driver operations, and cancels
only its captured session identity on TERM or deadline.

`x-multi-statement=true`, its true aliases, malformed values, duplicate values,
and unknown `x-` driver options are rejected before any database connection with
`error=unsupported_option` and exit 1. The optional upstream statement-splitting
parser has private cancellation boundaries that this helper cannot safely guard.
This is an explicit candidate compatibility restriction. Before first release,
the trusted runtime must classify configured option keys/values without printing
the connection URL and block promotion if an unsupported option is present.
No production configuration change is implied by this candidate.

Supported migration options are `x-migrations-table`,
`x-migrations-table-quoted`, and `x-statement-timeout`. The timeout must be a
nonnegative 32-bit integer in milliseconds and is capped by the helper's budget.
`x-multi-statement=false` and its false aliases are accepted. A parsed
`x-multi-statement-max-size` is accepted for compatibility but has no effect while
the optional splitting parser is disabled, matching upstream behavior.

Only `postgres` and `postgresql` URLs are accepted. Other ordinary PostgreSQL
connection options are forwarded to lib/pq, which may reject unsupported values;
arbitrary driver or DSN compatibility is not promised. TLS parameters are
preserved. The helper overrides `application_name` with a random per-session
identity, `connect_timeout` with 2 seconds, and server `statement_timeout` with its
2-second readiness or 90-second migration budget. These candidate limits need
release review; they do not authorize production limits. The shell supplies an
additional 100-second migration guard with a 5-second kill allowance and forwards
TERM to its tracked child before exiting.

## Migration state ownership

The helper owns migration-option interpretation for both migration and inspection. It normalizes the configured tracking schema/table once, then supplies those same identifiers to the migration driver and inspector. Without quoted mode, a table option is a literal name, including dots. Quoted mode accepts one quoted table identifier or a quoted schema and table; doubled quotes escape a quote. Malformed or ambiguous quoting is rejected before connecting. The default schema follows the connection's search path. Identifiers are safely quoted for SQL, and PostgreSQL resolves their actual catalog identity.

`inspect` opens a read-only transaction without constructing the migration driver, so it never initializes tracking tables. It emits one format-version-1 JSON record with `version` (integer or null), `dirty`, and `bootstrap`, only after its dedicated session has been closed and verified. Missing tracking permits bootstrap only when no user relations exist across user schemas. Existing tracking must contain exactly one valid positive version and boolean dirty flag. Missing tracking on a nonempty database, malformed records, invalid options, cancellation and unconfirmed cleanup fail without emitting a success record. Database, schema, table and credential strings are excluded from the protocol and diagnostics.

Inspection uses the same two-second session budget and exact-session cancellation/cleanup safeguards as readiness checks. The release command consumes this protocol for all pre-migration, post-migration and no-change checks; it does not independently choose a tracking table.
