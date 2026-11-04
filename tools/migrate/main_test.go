package main

import (
	"bytes"
	"errors"
	"net"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/lib/pq"
)

// Only typed transient connection errors should earn retry status 2.
func TestFailureStatus(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"connection", &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("refused")}, 2},
		{"starting", &pq.Error{Code: "57P03"}, 2},
		{"auth", &pq.Error{Code: "28P01"}, 1},
		{"dirty", migrate.ErrDirty{Version: 2}, 1},
		{"invalid", errors.New("postgres://fake:fake@invalid.example/fake"), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := failureStatus(tt.err); got != tt.want {
				t.Fatalf("got %d want %d", got, tt.want)
			}
		})
	}
}

func TestInvalidInputIsSanitized(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://fake_user:fake_password@%invalid/fake")
	for _, args := range [][]string{{}, {"force"}, {"version"}} {
		var out bytes.Buffer
		if status := run(args, &out); status != 1 {
			t.Fatalf("status=%d", status)
		}
		if strings.Contains(out.String(), "fake_") || strings.Contains(out.String(), "postgres://") {
			t.Fatal("credential-bearing error leaked")
		}
	}
}
