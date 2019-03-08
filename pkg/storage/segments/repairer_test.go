// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package segments_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
)

func TestNewSegmentRepairer(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		oc, err := planet.Uplinks[0].DialOverlay(planet.Satellites[0])
		assert.NoError(t, err)
		ec := ecclient.NewClient(planet.Uplinks[0].Transport, 0)
		pdb := planet.Satellites[0].Metainfo.Service
		ss := segments.NewSegmentRepairer(oc, ec, pdb)
		assert.NotNil(t, ss)
	})
}

func TestSegmentStoreRepairRemote(t *testing.T) {
	//TODO
}
