package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

type migrationState struct {
	FormatVersion int    `json:"format_version"`
	Version       *int64 `json:"version"`
	Dirty         bool   `json:"dirty"`
	Bootstrap     bool   `json:"bootstrap"`
}

var errTrackingMissing = errors.New("tracking missing on nonempty database")
var errTrackingMalformed = errors.New("tracking table malformed")

func runInspection(out io.Writer) int {
	signals, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	ctx, cancel := context.WithTimeout(signals, 2*time.Second)
	defer cancel()
	// Driver construction initializes tracking tables. Inspection must not do so.
	session, err := openSessionMode(ctx, os.Getenv("DATABASE_URL"), 2*time.Second, false)
	if err != nil {
		if errors.Is(err, errUnsupportedOption) {
			return report(out, "error=unsupported_option", 1)
		}
		return report(out, "error=database", failureStatus(err))
	}
	state, stateErr := session.inspect(ctx)
	cancelErr := session.finishCancellation()
	closeErr := session.close(nil)
	return reportInspection(out, state, stateErr, errors.Join(cancelErr, closeErr), ctx.Err())
}

// Emit a single JSON success record only after the exact session is confirmed
// stopped and closed. No caller may accept state followed by a cleanup failure.
func reportInspection(out io.Writer, state migrationState, stateErr, cleanupErr, contextErr error) int {
	if cleanupErr != nil {
		return report(out, "error=session_unconfirmed", 1)
	}
	if contextErr != nil {
		return report(out, "error=cancelled session_stopped=true", failureStatus(contextErr))
	}
	if errors.Is(stateErr, errTrackingMissing) {
		return report(out, "error=tracking_missing", 1)
	}
	if errors.Is(stateErr, errTrackingMalformed) {
		return report(out, "error=tracking_malformed", 1)
	}
	if stateErr != nil {
		return report(out, "error=state", failureStatus(stateErr))
	}
	if err := json.NewEncoder(out).Encode(state); err != nil {
		return 1
	}
	return 0
}

func (s *migrationSession) inspect(ctx context.Context) (state migrationState, result error) {
	state.FormatVersion = 1
	tx, err := s.conn.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return state, err
	}
	defer func() { result = errors.Join(result, tx.Rollback()) }()
	var exists bool
	// The canonical, safely quoted name lets PostgreSQL resolve identifier length
	// and escaping exactly as it does for migration-driver SQL.
	if err := tx.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", s.table.qualified()).Scan(&exists); err != nil {
		return state, err
	}
	if !exists {
		if err := tx.QueryRowContext(ctx, `SELECT NOT EXISTS (
			SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
			WHERE n.nspname <> 'information_schema' AND left(n.nspname,3) <> 'pg_'
			AND c.relkind IN ('r','p','v','m','S','f'))`).Scan(&state.Bootstrap); err != nil {
			return state, err
		}
		if !state.Bootstrap {
			return state, errTrackingMissing
		}
		return state, nil
	}
	return inspectTracking(ctx, tx, s.table)
}

func inspectTracking(ctx context.Context, tx *sql.Tx, table migrationTable) (migrationState, error) {
	state := migrationState{FormatVersion: 1}
	var count int
	var value sql.NullInt64
	var dirty sql.NullBool
	err := tx.QueryRowContext(ctx, "SELECT count(*), min(version), bool_or(dirty) FROM "+table.qualified()).Scan(&count, &value, &dirty)
	if err != nil {
		return state, err
	}
	if count != 1 || !value.Valid || value.Int64 < 1 || !dirty.Valid {
		return state, errTrackingMalformed
	}
	state.Version, state.Dirty = &value.Int64, dirty.Bool
	return state, nil
}
