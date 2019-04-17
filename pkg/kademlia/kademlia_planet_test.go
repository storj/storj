// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia_test

import (
	"context"
	"io"
	"net"
	"path/filepath"
	"testing"
	"time"

	"storj.io/storj/pkg/storj"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/bootstrap"
	"storj.io/storj/bootstrap/bootstrapdb"
	"storj.io/storj/bootstrap/bootstrapweb/bootstrapserver"
	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storagenode"
)

func TestFetchPeerIdentity(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		sat := planet.Satellites[0]
		peerID, err := planet.StorageNodes[0].Kademlia.Service.FetchPeerIdentity(ctx, sat.ID())
		require.NoError(t, err)
		require.Equal(t, sat.ID(), peerID.ID)
		require.True(t, sat.Identity.Leaf.Equal(peerID.Leaf))
		require.True(t, sat.Identity.CA.Equal(peerID.CA))
	})
}

func TestRequestInfo(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 1, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		node := planet.StorageNodes[0]
		info, err := planet.Satellites[0].Kademlia.Service.FetchInfo(ctx, node.Local())
		require.NoError(t, err)
		require.Equal(t, node.Local().Type, info.GetType())
		require.Equal(t, node.Local().Metadata.GetEmail(), info.GetOperator().GetEmail())
		require.Equal(t, node.Local().Metadata.GetWallet(), info.GetOperator().GetWallet())
		require.Equal(t, node.Local().Restrictions.GetFreeDisk(), info.GetCapacity().GetFreeDisk())
		require.Equal(t, node.Local().Restrictions.GetFreeBandwidth(), info.GetCapacity().GetFreeBandwidth())
	})
}

func TestPingTimeout(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 4, UplinkCount: 0,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		self := planet.StorageNodes[0]
		routingTable := self.Kademlia.RoutingTable

		tlsOpts, err := tlsopts.NewOptions(self.Identity, tlsopts.Config{})
		require.NoError(t, err)

		self.Transport = transport.NewClientWithTimeout(tlsOpts, 1*time.Millisecond)

		network := &transport.SimulatedNetwork{
			DialLatency:    300 * time.Second,
			BytesPerSecond: 1 * memory.KB,
		}

		slowClient := network.NewClient(self.Transport)
		require.NotNil(t, slowClient)

		node := pb.Node{
			Id: self.ID(),
			Address: &pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
			},
		}

		newService, err := kademlia.NewService(zaptest.NewLogger(t), node, slowClient, routingTable, kademlia.Config{})
		require.NoError(t, err)

		target := pb.Node{
			Id: planet.StorageNodes[2].ID(),
			Address: &pb.NodeAddress{
				Transport: pb.NodeTransport_TCP_TLS_GRPC,
				Address:   planet.StorageNodes[2].Addr(),
			},
		}

		_, err = newService.Ping(ctx, target)
		require.Error(t, err, context.DeadlineExceeded)
		require.True(t, kademlia.NodeErr.Has(err) && transport.Error.Has(err))

	})
}

func TestBootstrapBackoff(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	planet, err := testplanet.NewCustom(zaptest.NewLogger(t), testplanet.Config{
		SatelliteCount:   1,
		StorageNodeCount: 1,
		Reconfigure: testplanet.Reconfigure{
			// Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
			// 	config.Kademlia.BootstrapAddr = "127.0.0.1:9999"
			// },
			StorageNode: func(index int, config *storagenode.Config) {
				config.Kademlia.BootstrapAddr = "127.0.0.1:9999"
			},
		},
	})
	// planet, err := testplanet.New(t, 1, 5, 1)
	require.NoError(t, err)
	defer ctx.Check(planet.Shutdown)

	planet.Start(ctx)

	whitelistPath, err := planet.WriteWhitelist(storj.LatestIDVersion())
	require.NoError(t, err)

	config := bootstrap.Config{
		Server: server.Config{
			Address:        "127.0.0.1:9999",
			PrivateAddress: "127.0.0.1:0",

			Config: tlsopts.Config{
				RevocationDBURL:     "bolt://" + filepath.Join("/temp/", "revocation.db"),
				UsePeerCAWhitelist:  true,
				PeerCAWhitelistPath: whitelistPath,
				PeerIDVersions:      "latest",
				Extensions: extensions.Config{
					Revocation:          false,
					WhitelistSignedLeaf: false,
				},
			},
		},
		Kademlia: kademlia.Config{
			Alpha:    5,
			DBPath:   "/temp/kaddb", // TODO: replace with master db
			Operator: kademlia.OperatorConfig{
				// Email:  prefix + "@example.com",
				// Wallet: "0x" + strings.Repeat("00", 20),
			},
		},
		Web: bootstrapserver.Config{
			Address:   "127.0.0.1:0",
			StaticDir: "./web/bootstrap", // TODO: for development only
		},
		Version: planet.NewVersionConfig(),
	}
	// if planet.config.Reconfigure.Bootstrap != nil {
	// 	planet.config.Reconfigure.Bootstrap(0, &config)
	// }

	var verInfo version.Info
	verInfo = planet.NewVersionInfo()
	id, err := planet.NewIdentity()
	require.NoError(t, err)

	db, err := bootstrapdb.NewInMemory("/temp/")
	require.NoError(t, err)

	peer, err := bootstrap.New(zaptest.NewLogger(t), id, db, config, verInfo)
	require.NoError(t, err)

	var group errgroup.Group
	group.Go(func() error {
		return peer.Run(ctx)
	})
	defer peer.Close()
	require.NoError(t, err)

	// err = peer.Run(ctx)
	// require.NoError(t, err)

}

func badBootstrapProxy(done chan bool) (err error) {
	l, err := net.Listen("tcp", ":9999")
	if err != nil {
		return err
	}
	connCount := 0

	defer func() {
		done <- true
		l.Close()
	}()

	for {
		c, err := l.Accept()
		if err != nil {
			return err
		}
		connCount++

		go func() {
			if connCount < 3 {
				c.Close()
				return
			}
			c2, err := net.Dial("tcp", "127.0.0.1:9990")
			if err != nil {
				c.Close()
				return
			}
			connCount++
			go io.Copy(c, c2)
			io.Copy(c2, c)
		}()
	}
}
