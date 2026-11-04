# Migration helper compatibility

This candidate helper reads `DATABASE_URL` from the environment and accepts only
`version` and `up`. Driver errors, SQL, credentials, and connection URLs are never
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
