package main

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/golang-migrate/migrate/v4/database"
)

type recordingDriver struct {
	database.Driver
	calls int
}

func (d *recordingDriver) Run(io.Reader) error         { d.calls++; return nil }
func (d *recordingDriver) SetVersion(int, bool) error  { d.calls++; return nil }
func (d *recordingDriver) Drop() error                 { d.calls++; return nil }
func (d *recordingDriver) Lock() error                 { d.calls++; return nil }
func (d *recordingDriver) Version() (int, bool, error) { d.calls++; return 1, false, nil }
func TestNoSQLOperationAfterCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	raw := &recordingDriver{}
	driver := &guardedDriver{Driver: raw, ctx: ctx}
	if err := driver.Run(strings.NewReader("first migration")); err != nil {
		t.Fatal(err)
	}
	cancel() // A deterministic idle interval between the two migrations.
	operations := []func() error{
		func() error { return driver.Run(strings.NewReader("next migration")) },
		func() error { return driver.SetVersion(2, false) },
		driver.Lock,
		driver.Drop,
		func() error { _, _, err := driver.Version(); return err },
	}
	for _, operation := range operations {
		if err := operation(); !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want cancellation", err)
		}
	}
	if raw.calls != 1 {
		t.Fatalf("SQL ran after cancellation: calls=%d", raw.calls)
	}
}

type observedReader struct {
	io.Reader
	entered chan struct{}
	once    sync.Once
}

func (r *observedReader) Read(data []byte) (int, error) {
	r.once.Do(func() { close(r.entered) })
	return r.Reader.Read(data)
}

type readingDriver struct {
	database.Driver
	sqlCalls int
}

func (d *readingDriver) Run(reader io.Reader) error {
	if _, err := io.ReadAll(reader); err != nil {
		return err
	}
	d.sqlCalls++
	return nil
}
func TestBufferedBodyCannotStartSQLAfterCancellation(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reader, writer := io.Pipe()
	entered := make(chan struct{})
	raw := &readingDriver{}
	driver := &guardedDriver{Driver: raw, ctx: ctx}
	done := make(chan error, 1)
	go func() { done <- driver.Run(&observedReader{Reader: reader, entered: entered}) }()
	<-entered
	cancel()
	if _, err := io.WriteString(writer, "CREATE TABLE must_not_start(id integer)"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation", err)
	}
	if raw.sqlCalls != 0 {
		t.Fatal("buffered SQL started after cancellation")
	}
}
