// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package preflight_test

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/testcontext"
	"storj.io/storj/private/server"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/preflight"
	"storj.io/storj/storagenode/storagenodedb/storagenodedbtest"
	"storj.io/storj/storagenode/trust"
)

type mockServer struct {
	localTime time.Time
	pb.DRPCNodeServer
}

func TestLocalTime_InSync(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
		Reconfigure: testplanet.Reconfigure{
			StorageNode: func(index int, config *storagenode.Config) {
				config.Preflight.LocalTimeCheck = true
			},
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		storagenode := planet.StorageNodes[0]
		err := storagenode.Preflight.LocalTime.Check(ctx)
		require.NoError(t, err)
	})
}

func TestLocalTime_OutOfSync(t *testing.T) {
	storagenodedbtest.Run(t, func(ctx *testcontext.Context, t *testing.T, db storagenode.DB) {

		log := zaptest.NewLogger(t)

		// set up mock satellite server configuration
		mockSatID, err := testidentity.NewTestIdentity(ctx)
		require.NoError(t, err)
		config := server.Config{
			Address:        "127.0.0.1:0",
			PrivateAddress: "127.0.0.1:0",

			Config: tlsopts.Config{
				PeerIDVersions: "*",
				Extensions: extensions.Config{
					Revocation:          false,
					WhitelistSignedLeaf: false,
				},
			},
		}
		mockSatTLSOptions, err := tlsopts.NewOptions(mockSatID, config.Config, nil)
		require.NoError(t, err)

		t.Run("Less than 30m", func(t *testing.T) {
			// register mock GetTime endpoint to mock server
			var group errgroup.Group
			defer ctx.Check(group.Wait)

			contactServer, err := server.New(log, mockSatTLSOptions, config)
			require.NoError(t, err)
			defer ctx.Check(contactServer.Close)

			err = pb.DRPCRegisterNode(contactServer.DRPC(), &mockServer{
				localTime: time.Now().Add(-25 * time.Minute),
			})
			require.NoError(t, err)

			group.Go(func() error {
				return contactServer.Run(ctx)
			})

			// get mock server address
			_, portStr, err := net.SplitHostPort(contactServer.Addr().String())
			require.NoError(t, err)
			port, err := strconv.Atoi(portStr)
			require.NoError(t, err)
			url := trust.SatelliteURL{
				ID:   mockSatID.ID,
				Host: "127.0.0.1",
				Port: port,
			}
			require.NoError(t, err)

			// set up storagenode client
			source, err := trust.NewStaticURLSource(url.String())
			require.NoError(t, err)

			identity, err := testidentity.NewTestIdentity(ctx)
			require.NoError(t, err)
			tlsOptions, err := tlsopts.NewOptions(identity, config.Config, nil)
			require.NoError(t, err)
			dialer := rpc.NewDefaultDialer(tlsOptions)
			pool, err := trust.NewPool(log, trust.Dialer(dialer), trust.Config{
				Sources:   []trust.Source{source},
				CachePath: ctx.File("trust-cache.json"),
			}, db.Satellites())
			require.NoError(t, err)
			err = pool.Refresh(ctx)
			require.NoError(t, err)

			// should not return any error when node's clock is off no more than 30m
			localtime := preflight.NewLocalTime(log, preflight.Config{
				LocalTimeCheck: true,
			}, pool, dialer)
			err = localtime.Check(ctx)
			require.NoError(t, err)

		})

		t.Run("More than 30m", func(t *testing.T) {
			// register mock GetTime endpoint to mock server
			var group errgroup.Group
			defer ctx.Check(group.Wait)

			contactServer, err := server.New(log, mockSatTLSOptions, config)
			require.NoError(t, err)
			defer ctx.Check(contactServer.Close)

			err = pb.DRPCRegisterNode(contactServer.DRPC(), &mockServer{
				localTime: time.Now().Add(-31 * time.Minute),
			})
			require.NoError(t, err)

			group.Go(func() error {
				return contactServer.Run(ctx)
			})

			// get mock server address
			_, portStr, err := net.SplitHostPort(contactServer.Addr().String())
			require.NoError(t, err)
			port, err := strconv.Atoi(portStr)
			require.NoError(t, err)
			url := trust.SatelliteURL{
				ID:   mockSatID.ID,
				Host: "127.0.0.1",
				Port: port,
			}
			require.NoError(t, err)

			// set up storagenode client
			source, err := trust.NewStaticURLSource(url.String())
			require.NoError(t, err)

			identity, err := testidentity.NewTestIdentity(ctx)
			require.NoError(t, err)
			tlsOptions, err := tlsopts.NewOptions(identity, config.Config, nil)
			require.NoError(t, err)
			dialer := rpc.NewDefaultDialer(tlsOptions)
			pool, err := trust.NewPool(log, trust.Dialer(dialer), trust.Config{
				Sources:   []trust.Source{source},
				CachePath: ctx.File("trust-cache.json"),
			}, db.Satellites())
			require.NoError(t, err)
			err = pool.Refresh(ctx)
			require.NoError(t, err)

			// should return an error when node's clock is off by more than 30m with all trusted satellites
			localtime := preflight.NewLocalTime(log, preflight.Config{
				LocalTimeCheck: true,
			}, pool, dialer)
			err = localtime.Check(ctx)
			require.Error(t, err)
		})
	})
}

func (mock *mockServer) GetTime(ctx context.Context, req *pb.GetTimeRequest) (*pb.GetTimeResponse, error) {
	return &pb.GetTimeResponse{
		Timestamp: mock.localTime,
	}, nil
}
