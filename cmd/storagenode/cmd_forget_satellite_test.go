// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/reputation"
	"storj.io/storj/storagenode/satellites"
)

func Test_newForgetSatelliteCmd_Error(t *testing.T) {
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

func Test_cmdForgetSatellite(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// pause the forget satellite chore
		planet.StorageNodes[0].ForgetSatellite.Chore.Loop.Pause()

		address := planet.StorageNodes[0].Server.PrivateAddr().String()
		log := zaptest.NewLogger(t)

		store := planet.StorageNodes[0].Storage2.BlobsCache
		defer ctx.Check(store.Close)

		satellite := planet.Satellites[0]

		blobSize := memory.KB
		blobRef := blobstore.BlobRef{
			Namespace: satellite.ID().Bytes(),
			Key:       testrand.PieceID().Bytes(),
		}
		w, err := store.Create(ctx, blobRef, -1)
		require.NoError(t, err)
		_, err = w.Write(testrand.Bytes(blobSize))
		require.NoError(t, err)
		require.NoError(t, w.Commit(ctx))

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
		require.NotNil(t, rstats)
		require.Equal(t, stats, *rstats)

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
		blobInfo, err := store.Stat(ctx, blobRef)
		require.Error(t, err)
		require.True(t, errs.Is(err, os.ErrNotExist))
		require.Nil(t, blobInfo)
		// check that the reputation was deleted
		rstats, err = reputationDB.Get(ctx, satellite.ID())
		require.NoError(t, err)
		require.Equal(t, &reputation.Stats{SatelliteID: satellite.ID()}, rstats)
	})
}
