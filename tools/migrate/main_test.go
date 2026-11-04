package main

import (
	"bytes"
	"context"
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
		{"probe deadline", context.DeadlineExceeded, 2},
		{"SIGTERM cancellation", context.Canceled, 1},
		{"probe SQL timeout", &pq.Error{Code: "57014"}, 2},
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

func TestUnconfirmedCleanupOverridesRetry(t *testing.T) {
	t.Parallel()
	for _, primary := range []error{context.DeadlineExceeded, &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("unavailable")}} {
		err := sessionFailure(primary, errors.New("cleanup unconfirmed"))
		if got := failureStatus(err); got != 1 {
			t.Fatalf("unconfirmed cleanup got status%d, want1", got)
		}
		if !errors.Is(err, primary) {
			t.Fatal("primary error context lost")
		}
	}
	if got := failureStatus(sessionFailure(context.DeadlineExceeded, nil)); got != 2 {
		t.Fatalf("confirmed cleanup got status%d, want2", got)
	}
}

func TestUnsupportedOptionHasStableSafeOutcome(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://fake_user:fake_password@invalid.example/fake?x-multi-statement=true")
	var out bytes.Buffer
	if status := run([]string{"version"}, &out); status != 1 {
		t.Fatalf("got%d want1", status)
	}
	if out.String() != "error=unsupported_option\n" {
		t.Fatalf("unexpected safe outcome: %q", out.String())
	}
}
