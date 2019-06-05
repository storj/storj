// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"testing"

	"storj.io/storj/internal/testcontext"
)

// TODO: Start up test planet and call these from bash instead
func TestCCommonTests(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet := startTestPlanet(t, ctx)
	defer ctx.Check(planet.Shutdown)

	runCTest(t, ctx, "common_test.c")
}

