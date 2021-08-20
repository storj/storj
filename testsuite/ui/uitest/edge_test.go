// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package uitest_test

import (
	"testing"

	"github.com/go-rod/rod"
	"github.com/stretchr/testify/require"

	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/testsuite/ui/uitest"
)

func TestRunWithEdge(t *testing.T) {
	for x := 0; x < 100; x++ {
		uitest.RunWithEdge(t, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet, browser *rod.Browser, edgeInfo uitest.EdgeInfo) {
			require.NotEmpty(t, edgeInfo.AccessKey)
			require.NotEmpty(t, edgeInfo.SecretKey)
			require.NotEmpty(t, edgeInfo.GatewayAddr)
			require.NotEmpty(t, edgeInfo.AuthSvcAddr)
			t.Log("working")
		})
	}
}
