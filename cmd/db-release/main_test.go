package main

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
)

func TestBoundedURL(t *testing.T) {
	t.Parallel()
	config := limits{Overall: 120, Statement: 10000, Lock: 2000, Connect: 5}
	got, err := boundedURL("postgres://user:secret@example.test/db?sslmode=verify-full&options=-c%20search_path%3Dmigration_meta%2Cpublic&connect_timeout=99&statement_timeout=0&lock_timeout=0", config)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("sslmode") != "verify-full" || parsed.Query().Get("connect_timeout") != "5" {
		t.Fatal("connection security must be preserved and timeout enforced")
	}
	if parsed.Query().Get("options") != "-c search_path=migration_meta,public -c statement_timeout=10000 -c lock_timeout=2000" {
		t.Fatal("statement and lock limits not enforced")
	}
	if parsed.Query().Get("statement_timeout") != "10000" || parsed.Query().Get("lock_timeout") != "2000" {
		t.Fatal("explicit startup parameters must not override configured limits")
	}
	for _, input := range []string{"", "host=example.test password=secret", "https://example.test/db"} {
		if _, err := boundedURL(input, config); err == nil {
			t.Errorf("accepted invalid URL %q", input)
		}
	}
}

func TestGuardedHelperKeepsConnectionOutOfArguments(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const secret = "postgres://fake_user:fake_password@invalid.example/fake"
	command := guardedMigrationCommand(ctx, "/app", secret)
	if len(command.Args) != 2 || command.Args[1] != "up" || strings.Contains(strings.Join(command.Args, " "), "fake_password") {
		t.Fatal("guarded helper accepts only up and must not receive credentials in argv")
	}
	if command.Env[len(command.Env)-1] != "DATABASE_URL="+secret || command.Cancel == nil || command.WaitDelay != 10*time.Second {
		t.Fatal("missing environment-only connection or cancellation cleanup allowance")
	}
}

func TestInspectionURLSeparatesMigrationOptions(t *testing.T) {
	t.Parallel()
	const raw = "postgres://fake_user:fake_password@example.test/db?sslmode=verify-full&sslrootcert=%2Ffake%2Fca.pem&x-statement-timeout=500&x-multi-statement=false"
	config := limits{Overall: 120, Statement: 10000, Lock: 2000, Connect: 5}
	helperURL, err := boundedURL(raw, config)
	if err != nil {
		t.Fatal(err)
	}
	inspection, err := inspectionURL(helperURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(inspection)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Has("x-statement-timeout") || parsed.Query().Has("x-multi-statement") {
		t.Fatal("migration options must not reach PostgreSQL startup")
	}
	if parsed.Query().Get("sslmode") != "verify-full" || parsed.Query().Get("sslrootcert") != "/fake/ca.pem" || parsed.Query().Get("connect_timeout") != "5" {
		t.Fatal("inspection must preserve TLS settings and connection bounds")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	command := guardedMigrationCommand(ctx, "/app", helperURL)
	if !strings.Contains(command.Env[len(command.Env)-1], "x-statement-timeout=500") {
		t.Fatal("guarded helper must retain migration-specific options")
	}
}

func TestDatabaseErrorsExcludeDriverMessages(t *testing.T) {
	t.Parallel()
	for _, err := range []error{&pq.Error{Code: "28P01", Message: "fake_password private SQL"}, errors.New("postgres://fake_user:fake_password@host/db"), context.Canceled} {
		message := databaseError("validate", err).Error()
		if strings.Contains(message, "fake_") || strings.Contains(message, "private SQL") || strings.Contains(message, "postgres://") {
			t.Fatalf("driver details leaked: %s", message)
		}
	}
}

func TestLimitsFailClosed(t *testing.T) {
	t.Parallel()
	valid := limits{Format: 1, Overall: 120, Statement: 10000, Lock: 2000, Connect: 5, Invariants: []string{"check.sql"}}
	if err := valid.validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*limits){
		func(c *limits) { c.Overall = 0 }, func(c *limits) { c.Statement = 0 },
		func(c *limits) { c.Lock = 0 }, func(c *limits) { c.Connect = 0 },
		func(c *limits) { c.Overall = 301 }, func(c *limits) { c.Invariants = nil },
		func(c *limits) { c.Invariants = []string{"../outside.sql"} },
		func(c *limits) { c.Format = 2 },
	} {
		config := valid
		mutate(&config)
		if config.validate() == nil {
			t.Errorf("accepted unsafe limits: %+v", config)
		}
	}
}

func TestMigrationStateProtocol(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{"format_version":1,"version":null,"dirty":false,"bootstrap":true}`,
		`{"format_version":1,"version":12,"dirty":false,"bootstrap":false}`,
		`{"format_version":1,"version":13,"dirty":true,"bootstrap":false}`,
	} {
		if _, err := decodeMigrationState([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{
		``, `{}`, `{"format_version":2,"version":12,"dirty":false,"bootstrap":false}`,
		`{"format_version":1,"dirty":false,"bootstrap":true}`,
		`{"format_version":1,"version":12,"bootstrap":false}`,
		`{"format_version":1,"version":null,"dirty":false,"bootstrap":false}`,
		`{"format_version":1,"version":12,"dirty":false,"bootstrap":true}`,
		`{"format_version":1,"version":null,"dirty":true,"bootstrap":true}`,
		`{"format_version":1,"version":-1,"dirty":false,"bootstrap":false}`,
		`{"format_version":1,"version":0,"dirty":false,"bootstrap":false}`,
		`{"format_version":1,"version":12,"dirty":false,"bootstrap":false,"unknown":true}`,
		`{"format_version":1,"version":12,"dirty":false,"bootstrap":false} {}`,
		`{"format_version":1,"version":12,"dirty":false,"bootstrap":false} error=session_unconfirmed`,
	} {
		if _, err := decodeMigrationState([]byte(body)); err == nil {
			t.Fatal("invalid migration state accepted")
		}
	}
}
