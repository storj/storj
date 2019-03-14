// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit_test

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/benchmark/latency"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/audit"
	"storj.io/storj/pkg/transport"
)

func TestAuditTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 10, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		err := planet.Satellites[0].Audit.Service.Close()
		assert.NoError(t, err)

		uplink := planet.Uplinks[0]
		testData := make([]byte, 5*memory.MiB)
		_, err = rand.Read(testData)
		assert.NoError(t, err)

		err = uplink.Upload(ctx, planet.Satellites[0], "test/bucket", "test/path", testData)
		assert.NoError(t, err)

		pointers := planet.Satellites[0].Metainfo.Service
		allocation := planet.Satellites[0].Metainfo.Allocation
		cursor := audit.NewCursor(pointers, allocation, planet.Satellites[0].Identity)

		var stripe *audit.Stripe
		for {
			stripe, err = cursor.NextStripe(ctx)
			if stripe != nil || err != nil {
				break
			}
		}
		require.NoError(t, err)
		require.NotNil(t, stripe)

		overlay := planet.Satellites[0].Overlay.Service
		tc := planet.Satellites[0].Transport
		slowtc := transport.NewClientWithLatency(tc, latency.Local)
		require.NotNil(t, slowtc)

		verifier := audit.NewVerifier(slowtc, overlay, planet.Satellites[0].Identity)
		require.NotNil(t, verifier)

		// We want this version of the verifier to be used for all auditing within the test
		// which should cause the test to fail because of the slowness.

		_, err = verifier.Verify(ctx, stripe)
		t.Error(err)
		assert.Error(t, err)
	})
}
