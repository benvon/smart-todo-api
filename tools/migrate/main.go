// The migration helper reads the existing DATABASE_URL environment contract so
// connection details never enter argv. Driver output and SQL are never logged.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/lib/pq"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

func run(args []string, out io.Writer) (status int) {
	if len(args) != 1 || (args[0] != "version" && args[0] != "up") {
		return report(out, "error=usage", 1)
	}
	budget := 90 * time.Second
	if args[0] == "version" {
		budget = 2 * time.Second
	}
	signals, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSignals()
	ctx, cancel := context.WithTimeout(signals, budget)
	defer cancel()
	session, err := openSession(ctx, os.Getenv("DATABASE_URL"), budget)
	if err != nil {
		return report(out, "error=database", failureStatus(err))
	}
	finishCancellation := session.finishCancellation
	m, err := migrate.NewWithDatabaseInstance("file:///app/migrations", "postgres", session.driver)
	defer func() {
		cancelErr := finishCancellation()
		closeErr := session.close(m)
		if cancelErr != nil || closeErr != nil || ctx.Err() != nil {
			status = report(out, "error=session_unconfirmed_or_cancelled", 1)
		}
	}()
	if err != nil {
		return report(out, "error=configuration", 1)
	}
	if args[0] == "version" {
		return version(m, out)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return report(out, "error=migration", 1)
	}
	return report(out, "result=success", 0)
}

func version(m *migrate.Migrate, out io.Writer) int {
	value, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return report(out, "version=none clean=true", 0)
	}
	if err != nil {
		return report(out, "error=version", failureStatus(err))
	}
	if dirty {
		return report(out, "error=dirty clean=false", 1)
	}
	return report(out, fmt.Sprintf("version=%d clean=true", value), 0)
}

func report(out io.Writer, message string, status int) int {
	if _, err := fmt.Fprintln(out, message); err != nil {
		return 1
	}
	return status
}

func failureStatus(err error) int {
	var networkErr *net.OpError
	if errors.As(err, &networkErr) {
		return 2
	}
	var postgresErr *pq.Error
	if errors.As(err, &postgresErr) && (postgresErr.Code.Class() == "08" || postgresErr.Code == "57P03") {
		return 2
	}
	return 1
}
