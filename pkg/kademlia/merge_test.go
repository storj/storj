// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/bootstrap"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/storagenode"
)

func TestMergePlanets(t *testing.T) {
	t.Skip("flaky")

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	alpha, err := testplanet.NewCustom(log.Named("A"), testplanet.Config{
		SatelliteCount:   2,
		StorageNodeCount: 5,
	})
	require.NoError(t, err)

	beta, err := testplanet.NewCustom(log.Named("B"), testplanet.Config{
		SatelliteCount:   2,
		StorageNodeCount: 5,
		Identities:       alpha.Identities(), // avoid using the same pregenerated identities
		Reconfigure: testplanet.Reconfigure{
			Bootstrap: func(index int, config *bootstrap.Config) {
				config.Kademlia.BootstrapAddr = alpha.Bootstrap.Addr()

			},
		},
	})
	require.NoError(t, err)

	defer ctx.Check(alpha.Shutdown)
	defer ctx.Check(beta.Shutdown)

	alpha.Start(ctx)
	beta.Start(ctx)

	time.Sleep(10 * time.Second)

	test := func(tag string, satellites []*satellite.Peer, storageNodes []*storagenode.Peer) string {
		found, missing := 0, 0
		for _, satellite := range satellites {
			for _, storageNode := range storageNodes {
				node, err := satellite.Overlay.Service.Get(ctx, storageNode.ID())
				if assert.NoError(t, err, tag) {
					found++
					assert.Equal(t, storageNode.Addr(), node.Address.Address, tag)
				} else {
					missing++
				}
			}
		}
		return fmt.Sprintf("%s: Found %v out of %v (missing %v)", tag, found, found+missing, missing)
	}

	sumAA := test("A-A", alpha.Satellites, alpha.StorageNodes)
	sumAB := test("A-B", alpha.Satellites, beta.StorageNodes)
	sumBB := test("B-B", beta.Satellites, beta.StorageNodes)
	sumBA := test("B-A", beta.Satellites, alpha.StorageNodes)

	t.Log(sumAA)
	t.Log(sumAB)
	t.Log(sumBB)
	t.Log(sumBA)
}
