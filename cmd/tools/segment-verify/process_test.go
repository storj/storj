// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main_test

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"storj.io/common/memory"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/common/uuid"
	segmentverify "storj.io/storj/cmd/tools/segment-verify"
	"storj.io/storj/private/testplanet"
)

func TestProcess(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		verifier := segmentverify.NewVerifier(
			planet.Log().Named("verifier"),
			satellite.Dialer,
			satellite.Orders.Service,
			segmentverify.VerifierConfig{
				PerPieceTimeout:    time.Second,
				OrderRetryThrottle: 500 * time.Millisecond,
				RequestThrottle:    500 * time.Millisecond,
			})

		config := segmentverify.ServiceConfig{
			NotFoundPath:      ctx.File("not-found.csv"),
			RetryPath:         ctx.File("retry.csv"),
			ProblemPiecesPath: ctx.File("problem-pieces.csv"),
			Check:             2,
			BatchSize:         4,
			Concurrency:       2,
			MaxOffline:        3,
		}

		service, err := segmentverify.NewService(
			planet.Log().Named("process"),
			satellite.Metabase.DB,
			verifier,
			satellite.Overlay.Service,
			config)
		require.NoError(t, err)

		// upload some data
		data := testrand.Bytes(8 * memory.KiB)
		for _, up := range planet.Uplinks {
			for i := 0; i < 10; i++ {
				err := up.Upload(ctx, satellite, "bucket1", strconv.Itoa(i), data)
				require.NoError(t, err)
			}
		}

		err = service.ProcessRange(ctx, uuid.UUID{}, uuid.Max())
		require.NoError(t, err)

		require.NoError(t, service.Close())

		retryCSV, err := os.ReadFile(config.RetryPath)
		require.NoError(t, err)
		require.Equal(t, "stream id,position,created_at,required,found,not found,retry\n", string(retryCSV))

		notFoundCSV, err := os.ReadFile(config.NotFoundPath)
		require.NoError(t, err)
		require.Equal(t, "stream id,position,created_at,required,found,not found,retry\n", string(notFoundCSV))
	})
}
