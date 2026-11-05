package main

import (
	"errors"
	"strings"

	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/lib/pq"
)

type migrationTable struct{ schema, name string }

func (t migrationTable) qualified() string {
	return pq.QuoteIdentifier(t.schema) + "." + pq.QuoteIdentifier(t.name)
}

// Resolve the URL option once. Both the driver and inspection receive the same
// identifiers; neither path reparses a SQL-like string independently.
func configuredTable(raw string, quoted bool) (migrationTable, error) {
	if strings.ContainsRune(raw, 0) {
		return migrationTable{}, errors.New("invalid migration table")
	}
	if !quoted {
		if raw == "" {
			raw = postgres.DefaultMigrationsTable
		}
		return migrationTable{name: raw}, nil
	}
	if !strings.HasSuffix(raw, `"`) {
		return migrationTable{}, errors.New("invalid quoted migration table")
	}
	first, rest, err := quotedIdentifier(raw)
	if err != nil {
		return migrationTable{}, err
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return migrationTable{name: first}, nil
	}
	if !strings.HasPrefix(rest, ".") {
		return migrationTable{}, errors.New("invalid migration table")
	}
	second, rest, err := quotedIdentifier(strings.TrimSpace(rest[1:]))
	if err != nil || rest != "" {
		return migrationTable{}, errors.New("invalid migration table")
	}
	return migrationTable{schema: first, name: second}, nil
}

func quotedIdentifier(raw string) (string, string, error) {
	if !strings.HasPrefix(raw, `"`) {
		return "", "", errors.New("invalid quoted identifier")
	}
	var value strings.Builder
	for i := 1; i < len(raw); i++ {
		if raw[i] != '"' {
			value.WriteByte(raw[i])
			continue
		}
		if i+1 < len(raw) && raw[i+1] == '"' {
			value.WriteByte('"')
			i++
			continue
		}
		if value.Len() == 0 {
			return "", "", errors.New("empty quoted identifier")
		}
		return value.String(), raw[i+1:], nil
	}
	return "", "", errors.New("unterminated quoted identifier")
}
