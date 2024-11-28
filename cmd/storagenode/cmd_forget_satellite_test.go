// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"io/fs"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/satellites"
)

func TestNewForgetSatelliteCmd_Error(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr string
	}{
		{
			name:    "no args",
			args:    "",
			wantErr: "must specify either satellite ID(s) as arguments or --all-untrusted flag",
		},
		{
			name:    "Both satellite ID and --all-untrusted flag specified",
			args:    "--all-untrusted 1234567890123456789012345678901234567890123456789012345678901234",
			wantErr: "cannot specify both satellite IDs and --all-untrusted",
		},
		{
			name:    "--all-untrusted and --force specified",
			args:    "--all-untrusted --force",
			wantErr: "cannot specify both --all-untrusted and --force",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newForgetSatelliteCmd(&Factory{})
			cmd.SetArgs(strings.Fields(tt.args))
			err := cmd.ExecuteContext(testcontext.New(t))
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Equal(t, tt.wantErr, err.Error())
		})
	}
}

func TestCmdForgetSatellite(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// pause the forget satellite chore
		planet.StorageNodes[0].ForgetSatellite.Chore.Loop.Pause()

		// TODO(clement): remove this once I figure out why it's flaky
		planet.StorageNodes[0].Reputation.Chore.Loop.Pause()

		address := planet.StorageNodes[0].Server.PrivateAddr().String()
		log := zaptest.NewLogger(t)

		backend := planet.StorageNodes[0].Storage2.PieceBackend
		satellite := planet.Satellites[0]

		pieceID := testrand.PieceID()
		w, err := backend.Writer(ctx, satellite.ID(), pieceID, pb.PieceHashAlgorithm_BLAKE3, time.Time{})
		require.NoError(t, err)
		_, err = w.Write(testrand.Bytes(memory.KB))
		require.NoError(t, err)
		require.NoError(t, w.Commit(ctx, &pb.PieceHeader{}))

		// create a new satellite reputation
		timestamp := time.Now().UTC()
		reputationDB := planet.StorageNodes[0].DB.Reputation()

		stats := reputation.Stats{
			SatelliteID: satellite.ID(),
			Audit: reputation.Metric{
				TotalCount:   6,
				SuccessCount: 7,
				Alpha:        8,
				Beta:         9,
				Score:        10,
				UnknownAlpha: 11,
				UnknownBeta:  12,
				UnknownScore: 13,
			},
			OnlineScore: 14,
			UpdatedAt:   timestamp,
			JoinedAt:    timestamp,
		}
		err = reputationDB.Store(ctx, stats)
		require.NoError(t, err)
		// test that the reputation was stored correctly
		rstats, err := reputationDB.Get(ctx, satellite.ID())
		require.NoError(t, err)
		// just making sure we have reputation stats for the satellite.
		// We can't compare the stats directly because the nodestats cache service
		// can update the reputation stats for the satellite until it is no longer in the trust cache.
		// This is fine because we haven't run the forget satellite command yet.
		require.NotNil(t, rstats)
		require.Equal(t, satellite.ID(), rstats.SatelliteID)
		require.False(t, rstats.JoinedAt.IsZero())
		require.False(t, rstats.UpdatedAt.IsZero())

		satelliteDB := planet.StorageNodes[0].DB.Satellites()
		// insert a new untrusted satellite in the database
		err = satelliteDB.SetAddressAndStatus(ctx, satellite.ID(), satellite.URL(), satellites.Untrusted)
		require.NoError(t, err)
		// test that the satellite was inserted correctly
		satelliteInfo, err := satelliteDB.GetSatellite(ctx, satellite.ID())
		require.NoError(t, err)
		require.Equal(t, satellites.Untrusted, satelliteInfo.Status)

		//  run the forget satellite command with All flag
		var stdout bytes.Buffer

		client, err := dialForgetSatelliteClient(ctx, address)
		require.NoError(t, err)
		defer ctx.Check(client.close)

		cmdCtx, cmdCancel := context.WithCancel(ctx)
		defer cmdCancel()
		err = startForgetSatellite(cmdCtx, log, client, ForgetSatelliteOptions{
			AllUntrusted: true,
			Stdout:       &stdout,
		})
		require.NoError(t, err)
		require.Contains(t, stdout.String(), satellite.ID().String())
		require.Contains(t, stdout.String(), "In Progress")

		// trigger the chore to run
		planet.StorageNodes[0].ForgetSatellite.Chore.Loop.TriggerWait()

		// confirm cleanup succeeded
		satelliteInfo, err = satelliteDB.GetSatellite(ctx, satellite.ID())
		require.NoError(t, err)
		require.Equal(t, satellites.CleanupSucceeded, satelliteInfo.Status)

		// check that the blob was deleted
		_, err = backend.Reader(ctx, satellite.ID(), pieceID)
		require.Error(t, err)
		require.True(t, errs.Is(err, fs.ErrNotExist))
		// check that the reputation was deleted
		rstats, err = reputationDB.Get(ctx, satellite.ID())
		require.NoError(t, err)
		require.Equal(t, &reputation.Stats{SatelliteID: satellite.ID()}, rstats)
	})
}
