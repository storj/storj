// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/common/identity"
	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode/blobstore"
	"storj.io/storj/storagenode/blobstore/filestore"
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
	t.Skip("The tests and the behavior is currently flaky. See https://github.com/storj/storj/issues/6465")

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.StorageNodes[0].Server.PrivateAddr().String()
		db := planet.StorageNodes[0].DB
		log := zaptest.NewLogger(t)

		store, err := filestore.NewAt(log, db.Config().Pieces, filestore.DefaultConfig)
		require.NoError(t, err)
		defer ctx.Check(store.Close)

		satelliteID := planet.Satellites[0].ID()

		blobSize := memory.KB
		blobRef := blobstore.BlobRef{
			Namespace: satelliteID.Bytes(),
			Key:       testrand.PieceID().Bytes(),
		}
		w, err := store.Create(ctx, blobRef, -1)
		require.NoError(t, err)
		_, err = w.Write(testrand.Bytes(blobSize))
		require.NoError(t, err)
		require.NoError(t, w.Commit(ctx))

		// create a new satellite reputation
		timestamp := time.Now().UTC()
		reputationDB := db.Reputation()

		stats := reputation.Stats{
			SatelliteID: satelliteID,
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
		rstats, err := reputationDB.Get(ctx, satelliteID)
		require.NoError(t, err)
		require.NotNil(t, rstats)
		require.Equal(t, stats, *rstats)

		// insert a new untrusted satellite in the database
		err = db.Satellites().SetAddressAndStatus(ctx, satelliteID, address, satellites.Untrusted)
		require.NoError(t, err)
		// test that the satellite was inserted correctly
		satellite, err := db.Satellites().GetSatellite(ctx, satelliteID)
		require.NoError(t, err)
		require.Equal(t, satellites.Untrusted, satellite.Status)

		// set up the identity
		ident := planet.StorageNodes[0].Identity
		identConfig := identity.Config{
			CertPath: ctx.File("identity", "identity.cert"),
			KeyPath:  ctx.File("identity", "identity.Key"),
		}
		err = identConfig.Save(ident)
		require.NoError(t, err)
		planet.StorageNodes[0].Config.Identity = identConfig

		//  run the forget satellite command with All flag
		err = cmdForgetSatellite(ctx, log, &forgetSatelliteCfg{
			AllUntrusted: true,
			Config:       planet.StorageNodes[0].Config,
		})
		require.NoError(t, err)

		// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
		// TODO: this is for reproducing the bug,
		// remove it once it's fixed.
		// !!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!
		time.Sleep(10 * time.Second)

		// check that the blob was deleted
		blobInfo, err := store.Stat(ctx, blobRef)
		require.Error(t, err)
		require.True(t, errs.Is(err, os.ErrNotExist))
		require.Nil(t, blobInfo)
		// check that the reputation was deleted
		rstats, err = reputationDB.Get(ctx, satelliteID)
		require.NoError(t, err)
		require.Equal(t, &reputation.Stats{SatelliteID: satelliteID}, rstats)
		// check that the satellite info was deleted from the database
		satellite, err = db.Satellites().GetSatellite(ctx, satelliteID)
		require.NoError(t, err)
		require.True(t, satellite.SatelliteID.IsZero())
	})
}

func Test_cmdForgetSatellite_Exclusions(t *testing.T) {
	t.Skip("The tests and the behavior is currently flaky. See https://github.com/storj/storj/issues/6465")

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 2, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		address := planet.StorageNodes[0].Server.PrivateAddr().String()
		db := planet.StorageNodes[0].DB
		log := zaptest.NewLogger(t)

		store, err := filestore.NewAt(log, db.Config().Pieces, filestore.DefaultConfig)
		require.NoError(t, err)
		defer ctx.Check(store.Close)

		satelliteID := planet.Satellites[0].ID()

		blobSize := memory.KB
		blobRef := blobstore.BlobRef{
			Namespace: satelliteID.Bytes(),
			Key:       testrand.PieceID().Bytes(),
		}
		w, err := store.Create(ctx, blobRef, -1)
		require.NoError(t, err)
		_, err = w.Write(testrand.Bytes(blobSize))
		require.NoError(t, err)
		require.NoError(t, w.Commit(ctx))

		// create a new satellite reputation
		timestamp := time.Now().UTC()
		reputationDB := db.Reputation()

		stats := reputation.Stats{
			SatelliteID: satelliteID,
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
		rstats, err := reputationDB.Get(ctx, satelliteID)
		require.NoError(t, err)
		require.NotNil(t, rstats)
		require.Equal(t, stats, *rstats)

		// set up the identity
		ident := planet.StorageNodes[0].Identity
		identConfig := identity.Config{
			CertPath: ctx.File("identity", "identity.cert"),
			KeyPath:  ctx.File("identity", "identity.Key"),
		}
		err = identConfig.Save(ident)
		require.NoError(t, err)
		planet.StorageNodes[0].Config.Identity = identConfig

		// add the satellite to the exclusion list
		err = planet.StorageNodes[0].Config.Storage2.Trust.Exclusions.Set(satelliteID.String() + "@" + address)
		require.NoError(t, err)
		//  run the forget satellite command with All flag
		err = cmdForgetSatellite(ctx, log, &forgetSatelliteCfg{
			AllUntrusted: true,
			Config:       planet.StorageNodes[0].Config,
		})
		require.NoError(t, err)

		// check that the blob was deleted
		blobInfo, err := store.Stat(ctx, blobRef)
		require.Error(t, err)
		require.True(t, errs.Is(err, os.ErrNotExist))
		require.Nil(t, blobInfo)
		// check that the reputation was deleted
		rstats, err = reputationDB.Get(ctx, satelliteID)
		require.NoError(t, err)
		require.Equal(t, &reputation.Stats{SatelliteID: satelliteID}, rstats)
		// check that the satellite info was deleted from the database
		satellite, err := db.Satellites().GetSatellite(ctx, satelliteID)
		require.NoError(t, err)
		require.True(t, satellite.SatelliteID.IsZero())
	})
}
