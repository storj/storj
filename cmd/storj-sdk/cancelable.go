// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"os"
	"os/signal"
)

// NewCLIContext creates a context that can be canceled with Ctrl-C
func NewCLIContext(root context.Context) (context.Context, func()) {
	// trap Ctrl+C and call cancel on the context
	ctx, cancel := context.WithCancel(root)
	signals := make(chan os.Signal, 1)

	signal.Notify(signals, os.Interrupt)

	go func() {
		select {
		case <-signals:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, func() {
		signal.Stop(signals)
		cancel()
	}
}
