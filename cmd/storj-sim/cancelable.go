// Copyright (C) 2019 Storj Labs, Inc.
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

	stop := func() {
		signal.Stop(signals)
		cancel()
	}

	go func() {
		select {
		case <-signals:
			stop()
		case <-ctx.Done():
		}
	}()

	return ctx, stop
}
