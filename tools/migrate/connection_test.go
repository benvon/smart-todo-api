package main

import (
	"errors"
	"net/url"
	"testing"
	"time"
)

func TestConnectionOptionsPreserveTLSAndBoundStatements(t *testing.T) {
	t.Parallel()
	dsn, config, err := connectionOptions("postgres://fake:fake@invalid.example/fake?sslmode=verify-full&sslrootcert=/fake/ca.crt&x-migrations-table=custom&x-statement-timeout=500", "helper-test", 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Query().Get("sslmode") != "verify-full" || parsed.Query().Get("sslrootcert") != "/fake/ca.crt" {
		t.Fatal("TLS options changed")
	}
	if parsed.Query().Get("application_name") != "helper-test" || parsed.Query().Get("statement_timeout") != "2000" {
		t.Fatal("session bounds missing")
	}
	if config.MigrationsTable != "custom" || config.StatementTimeout != 500*time.Millisecond {
		t.Fatal("migration options changed")
	}
}

func TestConnectionOptionsRejectInvalidInput(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", "mysql://fake/fake", "postgres://fake/fake?sslmode=%invalid", "postgres://fake/fake?x-multi-statement=invalid"} {
		if _, _, err := connectionOptions(raw, "helper-test", time.Second); err == nil {
			t.Fatal("invalid input accepted")
		}
	}
}

func TestUnsupportedMigrationOptionsFailBeforeConnection(t *testing.T) {
	t.Parallel()
	for _, option := range []string{"x-multi-statement=true", "x-multi-statement=1", "x-multi-statement=invalid", "x-unknown-option=value"} {
		_, _, err := connectionOptions("postgres://fake:fake@invalid.example/fake?"+option, "helper-test", time.Second)
		if !errors.Is(err, errUnsupportedOption) {
			t.Fatalf("option %s: got %v, want unsupported option", option, err)
		}
	}
}
