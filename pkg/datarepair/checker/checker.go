// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
)

// Config contains configurable values for checker
type Config struct {
	// queueAddress string `help:"data repair queue address" default:"redis://localhost:6379?db=5&password=123"`
}

// Run runs the checker with configured values
func (c *Config) Run(ctx context.Context) (err error) {

	// TODO: start checker server

	return err
}
