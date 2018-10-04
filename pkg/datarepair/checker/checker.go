// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package checker

import (
	"context"
)

// Config contains configurable values for checker
type Config struct {
	queueAddress string `help:"data repair queue address" default:"localhost:6379"`
	queuePass    string `help:"data repair queue password" default:""`
}

// Run runs the checker with configured values
func (c *Config) Run(ctx context.Context) (err error) {

	// TODO: start checker server

	return err
}
