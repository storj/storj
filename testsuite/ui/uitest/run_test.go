// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest_test

import (
	"testing"

	"github.com/go-rod/rod"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestRun(t *testing.T) {
	uitest.Run(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser) {
		t.Log("working")
	})
}
