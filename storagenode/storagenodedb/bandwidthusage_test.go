// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/storagenode/storagenodedb"
)

func TestBandwidthUsages(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	log := zaptest.NewLogger(t)

	db, err := storagenodedb.NewInfoInMemory()
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	require.NoError(t, db.CreateTables(log))

	bandwidthusages := db.BandwidthUsage()

	satellite0 := testplanet.MustPregeneratedSignedIdentity(0).ID
	satellite1 := testplanet.MustPregeneratedSignedIdentity(1).ID

	now := time.Now()

	// ensure zero queries work
	usage, err := bandwidthusages.Summary(ctx, now, now)
	require.NoError(t, err)
	require.Equal(t, &storagenodedb.BandwidthUsage{}, usage)

	usageBySatellite, err := bandwidthusages.SummaryBySatellite(ctx, now, now)
	require.NoError(t, err)
	require.Equal(t, map[storj.NodeID]*storagenodedb.BandwidthUsage{}, usageBySatellite)

	actions := []pb.Action{
		pb.Action_INVALID,

		pb.Action_PUT,
		pb.Action_GET,
		pb.Action_GET_AUDIT,
		pb.Action_GET_REPAIR,
		pb.Action_PUT_REPAIR,
		pb.Action_DELETE,

		pb.Action_PUT,
		pb.Action_GET,
		pb.Action_GET_AUDIT,
		pb.Action_GET_REPAIR,
		pb.Action_PUT_REPAIR,
		pb.Action_DELETE,
	}

	expectedUsage := &storagenodedb.BandwidthUsage{}
	expectedUsageTotal := &storagenodedb.BandwidthUsage{}

	// add bandwidth usages
	for _, action := range actions {
		expectedUsage.Include(action, int64(action))
		expectedUsageTotal.Include(action, int64(2*action))

		err := bandwidthusages.Add(ctx, satellite0, action, int64(action), now)
		require.NoError(t, err)

		err = bandwidthusages.Add(ctx, satellite1, action, int64(action), now.Add(2*time.Hour))
		require.NoError(t, err)
	}

	// test summarizing
	usage, err = bandwidthusages.Summary(ctx, now.Add(-10*time.Hour), now.Add(10*time.Hour))
	require.NoError(t, err)
	require.Equal(t, expectedUsageTotal, usage)

	expectedUsageBySatellite := map[storj.NodeID]*storagenodedb.BandwidthUsage{
		satellite0: expectedUsage,
		satellite1: expectedUsage,
	}
	usageBySatellite, err = bandwidthusages.SummaryBySatellite(ctx, now.Add(-10*time.Hour), now.Add(10*time.Hour))
	require.NoError(t, err)
	require.Equal(t, expectedUsageBySatellite, usageBySatellite)

	// only range capturing second satellite
	usage, err = bandwidthusages.Summary(ctx, now.Add(time.Hour), now.Add(10*time.Hour))
	require.NoError(t, err)
	require.Equal(t, expectedUsage, usage)

	// only range capturing second satellite
	expectedUsageBySatellite = map[storj.NodeID]*storagenodedb.BandwidthUsage{
		satellite1: expectedUsage,
	}
	usageBySatellite, err = bandwidthusages.SummaryBySatellite(ctx, now.Add(time.Hour), now.Add(10*time.Hour))
	require.NoError(t, err)
	require.Equal(t, expectedUsage, usage)
}
