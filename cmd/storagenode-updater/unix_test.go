// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !windows

package main_test

import (
	"testing"

	"storj.io/storj/internal/testcontext"
)

// NB: noop
func createTestService(ctx *testcontext.Context, t *testing.T, name, binPath string) (cleanup func()) {
	return func() {}
}
