// db-release executes the finite database portion of the release handoff.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lib/pq"
)

type limits struct {
	Format     int      `json:"format_version"`
	Overall    int      `json:"overall_timeout_seconds"`
	Statement  int      `json:"statement_timeout_ms"`
	Lock       int      `json:"lock_timeout_ms"`
	Connect    int      `json:"connect_timeout_seconds"`
	Invariants []string `json:"invariants"`
}

type contract struct {
	Format int `json:"format_version"`
	From   int `json:"from_version"`
	To     int `json:"to_version"`
}

func (c limits) validate() error {
	if c.Format != 1 {
		return errors.New("unsupported database instruction format")
	}
	for _, limit := range [][2]int{{c.Overall, 300}, {c.Statement, 30000}, {c.Lock, 5000}, {c.Connect, 10}, {len(c.Invariants), 20}} {
		if limit[0] < 1 || limit[0] > limit[1] {
			return errors.New("invalid database validation limits (must be positive and bounded)")
		}
	}
	for _, name := range c.Invariants {
		if filepath.Base(name) != name || filepath.Ext(name) != ".sql" {
			return errors.New("invariants must be SQL files in the release instruction directory")
		}
	}
	return nil
}

func boundedURL(raw string, config limits) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Host == "" {
		return "", errors.New("DATABASE_URL must be a PostgreSQL URL")
	}
	query := u.Query()
	query.Set("connect_timeout", strconv.Itoa(config.Connect))
	query.Set("options", fmt.Sprintf("-c statement_timeout=%d -c lock_timeout=%d", config.Statement, config.Lock))
	u.RawQuery = query.Encode()
	return u.String(), nil
}

// Inspection uses ordinary PostgreSQL parameters. Migration options stay in the
// separate URL supplied to the guarded helper, which validates them itself.
func inspectionURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", errors.New("invalid inspection database URL")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return "", errors.New("invalid inspection database options")
	}
	for key := range query {
		if strings.HasPrefix(key, "x-") {
			query.Del(key)
		}
	}
	u.RawQuery = query.Encode()
	return u.String(), nil
}

func readJSON(path string, target any) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	if err := json.Unmarshal(contents, target); err != nil {
		return fmt.Errorf("parse %s: %w", filepath.Base(path), err)
	}
	return nil
}

func schemaVersion(ctx context.Context, db *sql.DB) (int, bool, error) {
	var exists bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass('public.schema_migrations') IS NOT NULL").Scan(&exists); err != nil {
		return 0, false, databaseError("inspect schema version", err)
	}
	if !exists {
		// A missing version table is a bootstrap only when there are no application tables.
		var empty bool
		err := db.QueryRowContext(ctx, "SELECT NOT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname='public')").Scan(&empty)
		if err != nil {
			return 0, false, databaseError("inspect empty database", err)
		}
		if !empty {
			return 0, false, errors.New("schema version missing from a nonempty database; operator resolution required")
		}
		return 0, false, nil
	}
	var version int
	var dirty bool
	var count int
	err := db.QueryRowContext(ctx, "SELECT count(*), COALESCE(min(version),0), COALESCE(bool_or(dirty),false) FROM public.schema_migrations").Scan(&count, &version, &dirty)
	if err != nil {
		return 0, false, databaseError("read schema version", err)
	}
	if count != 1 {
		return 0, false, fmt.Errorf("expected exactly one schema version record; found %d", count)
	}
	return version, dirty, nil
}

// Report only a stable error class, never PostgreSQL messages, SQL or DSNs.
func databaseError(operation string, err error) error {
	var serverError *pq.Error
	if errors.As(err, &serverError) {
		return fmt.Errorf("%s failed (SQLSTATE %s)", operation, serverError.Code)
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s stopped by deadline or cancellation", operation)
	}
	return fmt.Errorf("%s failed", operation)
}

func migrate(ctx context.Context, db *sql.DB, databaseURL, root string, plan contract) error {
	version, dirty, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("schema %d is dirty; operator resolution/roll-forward required", version)
	}
	if version != 0 && version != plan.From && version != plan.To {
		return fmt.Errorf("unsupported starting schema %d; expected empty, %d or %d", version, plan.From, plan.To)
	}
	if version == plan.To {
		fmt.Printf("schema %d clean; no pending migrations\n", version)
		return nil
	}
	command := guardedMigrationCommand(ctx, root, databaseURL)
	// Only the guarded helper owns migration sessions and their cancellation.
	// Connection details remain in the environment, never argv or diagnostics.
	if _, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("migration from schema %d to %d failed (%w); rollout stopped; inspect schema version/dirty state and resolve or roll forward", version, plan.To, err)
	}
	return validateVersion(ctx, db, plan.To)
}

func guardedMigrationCommand(ctx context.Context, root, databaseURL string) *exec.Cmd {
	command := exec.CommandContext(ctx, filepath.Join(root, "migrate"), "up")
	command.Env = append(os.Environ(), "DATABASE_URL="+databaseURL)
	// Give the helper time to cancel and verify its exact database session.
	command.Cancel = func() error { return command.Process.Signal(syscall.SIGTERM) }
	command.WaitDelay = 10 * time.Second
	return command
}

func validateVersion(ctx context.Context, db *sql.DB, target int) error {
	version, dirty, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if dirty || version != target {
		return fmt.Errorf("expected clean schema %d; found version=%d dirty=%t", target, version, dirty)
	}
	return nil
}

func validateInvariant(ctx context.Context, db *sql.DB, path string) (result error) {
	query, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read invariant %s: %w", filepath.Base(path), err)
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return databaseError("begin invariant transaction", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) && result == nil {
			result = databaseError("close invariant transaction", err)
		}
	}()
	rows, err := tx.QueryContext(ctx, string(query))
	if err != nil {
		return databaseError("execute invariant "+filepath.Base(path), err)
	}
	defer func() {
		if err := rows.Close(); err != nil && result == nil {
			result = databaseError("close invariant result", err)
		}
	}()
	return checkInvariantRows(rows)
}

func checkInvariantRows(rows *sql.Rows) error {
	var valid bool
	if !rows.Next() {
		return errors.New("invariant returned no row")
	}
	if err := rows.Scan(&valid); err != nil {
		return databaseError("read one invariant boolean", err)
	}
	if !valid || rows.Next() || rows.NextResultSet() {
		return errors.New("invariant must return exactly one true boolean")
	}
	if err := rows.Err(); err != nil {
		return databaseError("read invariant result", err)
	}
	return nil
}

func run(mode, root string) (result error) {
	config, plan, err := loadInstructions(root)
	if err != nil {
		return err
	}
	databaseURL, err := boundedURL(os.Getenv("DATABASE_URL"), config)
	if err != nil {
		return err
	}
	inspection, err := inspectionURL(databaseURL)
	if err != nil {
		return err
	}
	signals, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stopSignals()
	ctx, cancel := context.WithTimeout(signals, time.Duration(config.Overall)*time.Second)
	defer cancel()
	db, err := sql.Open("postgres", inspection)
	if err != nil {
		return errors.New("open database connection failed")
	}
	defer func() {
		if err := db.Close(); err != nil {
			result = errors.Join(result, errors.New("close database connection failed"))
		}
	}()
	return execute(ctx, db, mode, root, databaseURL, config, plan)
}

func loadInstructions(root string) (limits, contract, error) {
	var config limits
	var plan contract
	if err := readJSON(filepath.Join(root, "release", "validation.json"), &config); err != nil {
		return config, plan, err
	}
	if err := config.validate(); err != nil {
		return config, plan, err
	}
	if err := readJSON(filepath.Join(root, "release", "contract.json"), &plan); err != nil {
		return config, plan, err
	}
	if plan.Format != 1 || plan.From < 1 || plan.To < plan.From {
		return config, plan, errors.New("invalid migration contract")
	}
	return config, plan, nil
}

func execute(ctx context.Context, db *sql.DB, mode, root, databaseURL string, config limits, plan contract) error {
	if mode == "migrate" {
		return migrate(ctx, db, databaseURL, root, plan)
	}
	if mode != "validate" {
		return errors.New("usage: db-release migrate|validate")
	}
	if err := validateVersion(ctx, db, plan.To); err != nil {
		return err
	}
	for _, name := range config.Invariants {
		if err := validateInvariant(ctx, db, filepath.Join(root, "release", name)); err != nil {
			return fmt.Errorf("invariant %s failed: %w", name, err)
		}
	}
	fmt.Printf("schema %d clean; all database invariants passed\n", plan.To)
	return nil
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: db-release migrate|validate")
		os.Exit(1)
	}
	root := os.Getenv("RELEASE_ROOT")
	if root == "" {
		root = "/app"
	}
	if err := run(os.Args[1], root); err != nil {
		message := err.Error()
		// Driver errors can contain user-supplied URL fragments. Keep secrets out of diagnostics.
		if raw := os.Getenv("DATABASE_URL"); raw != "" {
			message = strings.ReplaceAll(message, raw, "[database URL redacted]")
			if parsed, parseErr := url.Parse(raw); parseErr == nil && parsed.User != nil {
				if password, ok := parsed.User.Password(); ok && password != "" {
					message = strings.ReplaceAll(message, password, "[redacted]")
				}
			}
		}
		fmt.Fprintln(os.Stderr, "database release step stopped:", message)
		os.Exit(1)
	}
}
