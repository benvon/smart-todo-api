package main

import (
	"bytes"
	"context"
	"io"

	"github.com/golang-migrate/migrate/v4/database"
)

// Guard every operation that can start new migration SQL. Unlock and Close
// remain available so cancellation can release the original driver's resources.
type guardedDriver struct {
	database.Driver
	ctx context.Context
}

func (d *guardedDriver) Run(reader io.Reader) error {
	if err := d.ctx.Err(); err != nil {
		return err
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	if err := d.ctx.Err(); err != nil {
		return err
	}
	return d.Driver.Run(bytes.NewReader(body))
}
func (d *guardedDriver) SetVersion(value int, dirty bool) error {
	if err := d.ctx.Err(); err != nil {
		return err
	}
	return d.Driver.SetVersion(value, dirty)
}
func (d *guardedDriver) Lock() error {
	if err := d.ctx.Err(); err != nil {
		return err
	}
	return d.Driver.Lock()
}
func (d *guardedDriver) Version() (int, bool, error) {
	if err := d.ctx.Err(); err != nil {
		return 0, false, err
	}
	return d.Driver.Version()
}
func (d *guardedDriver) Drop() error {
	if err := d.ctx.Err(); err != nil {
		return err
	}
	return d.Driver.Drop()
}
