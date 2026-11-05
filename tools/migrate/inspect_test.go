package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestInspectionSuccessRequiresConfirmedCleanup(t *testing.T) {
	t.Parallel()
	value := int64(12)
	state := migrationState{FormatVersion: 1, Version: &value}
	var out bytes.Buffer
	if status := reportInspection(&out, state, nil, nil, nil); status != 0 {
		t.Fatalf("status=%d", status)
	}
	var decoded migrationState
	if err := json.Unmarshal(out.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Version == nil || *decoded.Version != 12 || decoded.Dirty || decoded.Bootstrap {
		t.Fatal("wrong inspection state")
	}
	for _, tt := range []struct {
		stateErr, cleanupErr, contextErr error
		want                             string
	}{
		{nil, errors.New("fake_password"), nil, "error=session_unconfirmed\n"},
		{context.DeadlineExceeded, errors.New("fake_password"), context.DeadlineExceeded, "error=session_unconfirmed\n"},
		{nil, nil, context.Canceled, "error=cancelled session_stopped=true\n"},
		{errTrackingMissing, nil, nil, "error=tracking_missing\n"},
		{errTrackingMalformed, nil, nil, "error=tracking_malformed\n"},
		{errors.New("private SQL fake_password"), nil, nil, "error=state\n"},
	} {
		out.Reset()
		if status := reportInspection(&out, state, tt.stateErr, tt.cleanupErr, tt.contextErr); status == 0 {
			t.Fatal("failed inspection reported success")
		}
		if out.String() != tt.want || strings.Contains(out.String(), "format_version") {
			t.Fatal("inspection emitted state before verified cleanup")
		}
	}
}

func TestInspectionRejectsUnsupportedOptionsBeforeConnecting(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://fake_user:fake_password@invalid.example/fake?x-multi-statement=true")
	var out bytes.Buffer
	if status := run([]string{"inspect"}, &out); status != 1 || out.String() != "error=unsupported_option\n" {
		t.Fatal("inspection must use the same option policy as migrations")
	}
}
