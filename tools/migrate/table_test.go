package main

import (
	"net/url"
	"testing"
	"time"
)

func TestMigrationTableConfiguration(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, raw, schema, table string
		quoted                   bool
	}{
		{"default", "", "", "schema_migrations", false},
		{"custom", "release_history", "", "release_history", false},
		{"literal dot", "release.history", "", "release.history", false},
		{"literal quote", `"Release"`, "", `"Release"`, false},
		{"quoted", `"Release History"`, "", "Release History", true},
		{"qualified", `"Migration Meta"."Release.History"`, "Migration Meta", "Release.History", true},
		{"qualified whitespace", `"Migration Meta" . "Release.History"`, "Migration Meta", "Release.History", true},
		{"escaped", `"Meta""Schema"."Release""History"`, `Meta"Schema`, `Release"History`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			query := url.Values{"x-migrations-table": {tt.raw}}
			if tt.quoted {
				query.Set("x-migrations-table-quoted", "true")
			}
			_, config, err := connectionOptions("postgres://fake:fake@example.test/db?"+query.Encode(), "test", time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if config.MigrationsTable != tt.table || config.SchemaName != tt.schema || config.MigrationsTableQuoted {
				t.Fatalf("migration configuration not normalized once: schema=%q table=%q quoted=%t", config.SchemaName, config.MigrationsTable, config.MigrationsTableQuoted)
			}
		})
	}
}

func TestMalformedMigrationTableRejectedBeforeConnection(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"", `""`, `"a"junk"b"`, `"a"."b"."c"`, `"unterminated`, `plain`, `"trailing" `, "\"a\x00b\""} {
		query := url.Values{"x-migrations-table": {raw}, "x-migrations-table-quoted": {"true"}}
		if _, _, err := connectionOptions("postgres://fake:fake@example.test/db?"+query.Encode(), "test", time.Second); err == nil {
			t.Fatal("malformed quoted migration table accepted")
		}
	}
}
