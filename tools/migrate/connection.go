package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"strconv"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
)

type migrationSession struct {
	driver             database.Driver
	db                 *sql.DB
	control            *sql.DB
	pid                int
	started            time.Time
	name               string
	finishCancellation func() error
}

// connectionOptions preserves upstream PostgreSQL options while adding server
// bounds. TLS settings remain exactly as supplied by DATABASE_URL.
func connectionOptions(raw, name string, budget time.Duration) (string, *postgres.Config, error) {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
		return "", nil, errors.New("invalid database URL")
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil {
		return "", nil, err
	}
	config := &postgres.Config{DatabaseName: parsed.Path, MigrationsTable: query.Get("x-migrations-table"), StatementTimeout: budget, MultiStatementMaxSize: postgres.DefaultMultiStatementMaxSize}
	for key, target := range map[string]*bool{"x-migrations-table-quoted": &config.MigrationsTableQuoted, "x-multi-statement": &config.MultiStatementEnabled} {
		if value := query.Get(key); value != "" {
			*target, err = strconv.ParseBool(value)
			if err != nil {
				return "", nil, err
			}
		}
	}
	if value := query.Get("x-statement-timeout"); value != "" {
		milliseconds, err := strconv.ParseInt(value, 10, 32)
		if err != nil || milliseconds < 0 {
			return "", nil, errors.New("invalid statement timeout")
		}
		if duration := time.Duration(milliseconds) * time.Millisecond; duration > 0 && duration < budget {
			config.StatementTimeout = duration
		}
	}
	if value := query.Get("x-multi-statement-max-size"); value != "" {
		size, err := strconv.Atoi(value)
		if err != nil {
			return "", nil, err
		}
		if size > 0 {
			config.MultiStatementMaxSize = size
		}
	}
	table := config.MigrationsTable
	if config.MigrationsTableQuoted && table != "" && (table[0] != '"' || table[len(table)-1] != '"') {
		return "", nil, errors.New("invalid quoted migration table")
	}
	query.Set("application_name", name)
	query.Set("statement_timeout", strconv.FormatInt(budget.Milliseconds(), 10))
	query.Set("connect_timeout", "2")
	parsed.RawQuery = query.Encode()
	return migrate.FilterCustomQuery(parsed).String(), config, nil
}

func openSession(ctx context.Context, raw string, budget time.Duration) (*migrationSession, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, err
	}
	name := "smart-todo-migrate-" + hex.EncodeToString(random[:])
	dsn, config, err := connectionOptions(raw, name, budget)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	session := &migrationSession{db: db, name: name}
	err = conn.QueryRowContext(ctx, "SELECT pg_backend_pid(), backend_start FROM pg_stat_activity WHERE pid=pg_backend_pid()").Scan(&session.pid, &session.started)
	if err != nil {
		return nil, errors.Join(err, conn.Close(), db.Close())
	}
	controlDSN, _, err := connectionOptions(raw, name+"-control", 2*time.Second)
	if err == nil {
		session.control, err = sql.Open("postgres", controlDSN)
	}
	if err != nil {
		return nil, errors.Join(err, conn.Close(), db.Close())
	}
	session.finishCancellation = session.watch(ctx)
	session.driver, err = postgres.WithConnection(ctx, conn, config)
	if err != nil {
		cancelErr := session.finishCancellation()
		return nil, errors.Join(err, cancelErr, conn.Close(), db.Close(), session.verifyStopped(), session.control.Close())
	}
	return session, nil
}

// Cancellation targets the captured identity of this helper's dedicated
// connection. PID reuse or another workload's session cannot match this query.
func (s *migrationSession) cancel() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var cancelled bool
	err := s.control.QueryRowContext(ctx, "SELECT pg_cancel_backend(pid) FROM pg_stat_activity WHERE pid=$1 AND backend_start=$2 AND application_name=$3 AND usename=current_user", s.pid, s.started, s.name).Scan(&cancelled)
	if err != nil {
		return err
	}
	if !cancelled {
		return errors.New("database cancellation unconfirmed")
	}
	return nil
}

func (s *migrationSession) watch(ctx context.Context) func() error {
	done := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() { done <- s.cancel() })
	return func() error {
		if stop() {
			return nil
		}
		return <-done
	}
}

func (s *migrationSession) close(m *migrate.Migrate) error {
	var migrationErr error
	if m != nil {
		sourceErr, dbErr := m.Close()
		migrationErr = errors.Join(sourceErr, dbErr)
	} else {
		migrationErr = s.driver.Close()
	}
	closeErr := s.db.Close()
	verifyErr := s.verifyStopped()
	return errors.Join(migrationErr, closeErr, verifyErr, s.control.Close())
}

func (s *migrationSession) verifyStopped() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		var active int
		err := s.control.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE pid=$1 AND backend_start=$2 AND application_name=$3 AND state='active'", s.pid, s.started, s.name).Scan(&active)
		if err != nil {
			return err
		}
		if active == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
