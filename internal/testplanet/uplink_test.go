// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"context"
	"crypto/rand"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/extensions"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/uplink"
)

func TestUploadDownload(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedData := make([]byte, 1*memory.MiB)
		_, err := rand.Read(expectedData)
		assert.NoError(t, err)

		err = planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		assert.NoError(t, err)

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path")
		assert.NoError(t, err)

		assert.Equal(t, expectedData, data)
	})
}

func TestDownloadWithSomeNodesOffline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		// stop discovery service so that we do not get a race condition when we delete nodes from overlay cache
		satellite.Discovery.Service.Discovery.Stop()

		testData := make([]byte, 1*memory.MiB)
		_, err := rand.Read(testData)
		require.NoError(t, err)

		err = ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: 4,
			MaxThreshold:     5,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// get a remote segment from pointerdb
		pdb := satellite.Metainfo.Service
		listResponse, _, err := pdb.List("", "", "", true, 0, 0)
		require.NoError(t, err)

		var path string
		var pointer *pb.Pointer
		for _, v := range listResponse {
			path = v.GetPath()
			pointer, err = pdb.Get(path)
			require.NoError(t, err)
			if pointer.GetType() == pb.Pointer_REMOTE {
				break
			}
		}

		// calculate how many storagenodes to kill
		redundancy := pointer.GetRemote().GetRedundancy()
		remotePieces := pointer.GetRemote().GetRemotePieces()
		minReq := redundancy.GetMinReq()
		numPieces := len(remotePieces)
		toKill := numPieces - int(minReq)

		nodesToKill := make(map[storj.NodeID]bool)
		for i, piece := range remotePieces {
			if i >= toKill {
				continue
			}
			nodesToKill[piece.NodeId] = true
		}

		for _, node := range planet.StorageNodes {
			if nodesToKill[node.ID()] {
				err = planet.StopPeer(node)
				require.NoError(t, err)

				// mark node as offline in overlay cache
				_, err = satellite.Overlay.Service.UpdateUptime(ctx, node.ID(), false)
				require.NoError(t, err)
			}
		}

		// we should be able to download data without any of the original nodes
		newData, err := ul.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, testData, newData)
	})
}

func TestUploadDownloadOneUplinksInParallel(t *testing.T) {
	t.Skip("flaky")

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		dataToUpload := make([][]byte, 5)
		for i := 0; i < len(dataToUpload); i++ {
			dataToUpload[i] = make([]byte, 100*memory.KiB.Int()+(i*100*memory.KiB.Int()))
			_, err := rand.Read(dataToUpload[i])
			require.NoError(t, err)
		}

		var group errgroup.Group
		for i, data := range dataToUpload {
			index := strconv.Itoa(i)
			uplink := planet.Uplinks[0]
			satellite := planet.Satellites[0]

			data := data
			group.Go(func() error {
				return uplink.Upload(ctx, satellite, "testbucket"+index, "test/path"+index, data)
			})
		}
		err := group.Wait()
		require.NoError(t, err)

		for i, data := range dataToUpload {
			index := strconv.Itoa(i)
			uplink := planet.Uplinks[0]
			satellite := planet.Satellites[0]

			expectedData := data
			group.Go(func() error {
				data, err := uplink.Download(ctx, satellite, "testbucket"+index, "test/path"+index)
				require.Equal(t, expectedData, data)
				return err
			})
		}
		err = group.Wait()
		require.NoError(t, err)
	})
}

func TestUploadDownloadMultipleUplinksInParallel(t *testing.T) {
	numberOfUplinks := 5

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: numberOfUplinks,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		dataToUpload := make([][]byte, numberOfUplinks)
		for i := 0; i < len(dataToUpload); i++ {
			dataToUpload[i] = make([]byte, 100*memory.KiB.Int()+(i*100*memory.KiB.Int()))
			_, err := rand.Read(dataToUpload[i])
			require.NoError(t, err)
		}

		var group errgroup.Group
		for i, data := range dataToUpload {
			index := strconv.Itoa(i)
			uplink := planet.Uplinks[i]
			satellite := planet.Satellites[0]

			data := data
			group.Go(func() error {
				return uplink.Upload(ctx, satellite, "testbucket"+index, "test/path"+index, data)
			})
		}
		err := group.Wait()
		require.NoError(t, err)

		for i, data := range dataToUpload {
			index := strconv.Itoa(i)
			uplink := planet.Uplinks[i]
			satellite := planet.Satellites[0]

			expectedData := data
			group.Go(func() error {
				data, err := uplink.Download(ctx, satellite, "testbucket"+index, "test/path"+index)
				require.Equal(t, expectedData, data)
				return err
			})
		}
		err = group.Wait()
		require.NoError(t, err)
	})
}

type piecestoreMock struct {
}

func (mock *piecestoreMock) Upload(server pb.Piecestore_UploadServer) error {
	return nil
}
func (mock *piecestoreMock) Download(server pb.Piecestore_DownloadServer) error {
	timoutTicker := time.NewTicker(30 * time.Second)
	defer timoutTicker.Stop()

	select {
	case <-timoutTicker.C:
		return nil
	case <-server.Context().Done():
		return nil
	}
}
func (mock *piecestoreMock) Delete(ctx context.Context, delete *pb.PieceDeleteRequest) (_ *pb.PieceDeleteResponse, err error) {
	return nil, nil
}

func TestDownloadFromUnresponsiveNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {

		expectedData := make([]byte, 1*memory.MiB)
		_, err := rand.Read(expectedData)
		assert.NoError(t, err)

		err = planet.Uplinks[0].UploadWithConfig(ctx, planet.Satellites[0], &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: 4,
			MaxThreshold:     5,
		}, "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		// get a remote segment from pointerdb
		pdb := planet.Satellites[0].Metainfo.Service
		listResponse, _, err := pdb.List("", "", "", true, 0, 0)
		require.NoError(t, err)

		var path string
		var pointer *pb.Pointer
		for _, v := range listResponse {
			path = v.GetPath()
			pointer, err = pdb.Get(path)
			require.NoError(t, err)
			if pointer.GetType() == pb.Pointer_REMOTE {
				break
			}
		}

		stopped := false
		// choose used storage node and replace it with fake listener
		unresponsiveNode := pointer.Remote.RemotePieces[0].NodeId
		for _, storageNode := range planet.StorageNodes {
			if storageNode.ID() == unresponsiveNode {
				err = planet.StopPeer(storageNode)
				require.NoError(t, err)

				wl, err := planet.WriteWhitelist(storj.LatestIDVersion())
				require.NoError(t, err)
				options, err := tlsopts.NewOptions(storageNode.Identity, tlsopts.Config{
					RevocationDBURL:     "bolt://" + filepath.Join(ctx.Dir("fakestoragenode"), "revocation.db"),
					UsePeerCAWhitelist:  true,
					PeerCAWhitelistPath: wl,
					PeerIDVersions:      "*",
					Extensions: extensions.Config{
						Revocation:          false,
						WhitelistSignedLeaf: false,
					},
				})
				require.NoError(t, err)

				server, err := server.New(options, storageNode.Addr(), storageNode.PrivateAddr(), nil)
				require.NoError(t, err)
				pb.RegisterPiecestoreServer(server.GRPC(), &piecestoreMock{})
				go func() {
					err := server.Run(ctx)
					require.NoError(t, err)
				}()
				stopped = true
				break
			}
		}
		assert.True(t, stopped, "no storage node was altered")

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path")
		assert.NoError(t, err)

		assert.Equal(t, expectedData, data)
	})
}
