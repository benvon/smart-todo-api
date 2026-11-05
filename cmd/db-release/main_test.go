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
	got, err := boundedURL("postgres://user:secret@example.test/db?sslmode=verify-full&options=unsafe&connect_timeout=99", config)
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
	if parsed.Query().Get("options") != "-c statement_timeout=10000 -c lock_timeout=2000" {
		t.Fatal("statement and lock limits not enforced")
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
