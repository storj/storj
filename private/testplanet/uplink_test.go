// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/peertls/extensions"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/common/testrand"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/pkg/server"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite/overlay"
	"storj.io/storj/uplink"
	"storj.io/storj/uplink/metainfo"
)

func TestUplinksParallel(t *testing.T) {
	const uplinkCount = 2
	const parallelCount = 2

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: uplinkCount,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		satellite := planet.Satellites[0]

		var group errgroup.Group
		for i := range planet.Uplinks {
			uplink := planet.Uplinks[i]

			for p := 0; p < parallelCount; p++ {
				suffix := fmt.Sprintf("-%d-%d", i, p)
				group.Go(func() error {
					data := testrand.Bytes(memory.Size(100+testrand.Intn(500)) * memory.KiB)

					err := uplink.Upload(ctx, satellite, "testbucket"+suffix, "test/path"+suffix, data)
					if err != nil {
						return err
					}

					downloaded, err := uplink.Download(ctx, satellite, "testbucket"+suffix, "test/path"+suffix)
					if err != nil {
						return err
					}

					if !bytes.Equal(data, downloaded) {
						return fmt.Errorf("upload != download data: %s", suffix)
					}

					return nil
				})
			}
		}
		err := group.Wait()
		require.NoError(t, err)
	})
}

func TestDownloadWithSomeNodesOffline(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		testData := testrand.Bytes(memory.MiB)

		err := ul.UploadWithConfig(ctx, satellite, &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: 4,
			MaxThreshold:     5,
		}, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// get a remote segment from pointerdb
		pdb := satellite.Metainfo.Service
		listResponse, _, err := pdb.List(ctx, "", "", true, 0, 0)
		require.NoError(t, err)

		var path string
		var pointer *pb.Pointer
		for _, v := range listResponse {
			path = v.GetPath()
			pointer, err = pdb.Get(ctx, path)
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

				// mark node as offline in overlay
				info := overlay.NodeCheckInInfo{
					NodeID: node.ID(),
					IsUp:   true,
					Address: &pb.NodeAddress{
						Address: node.Addr(),
					},
					Version: &pb.NodeVersion{
						Version:    "v0.0.0",
						CommitHash: "",
						Timestamp:  time.Time{},
						Release:    false,
					},
				}
				err = satellite.Overlay.Service.UpdateCheckIn(ctx, info, time.Now().UTC().Add(-4*time.Hour))
				require.NoError(t, err)
			}

		}
		// confirm that we marked the correct number of storage nodes as offline
		nodes, err := satellite.Overlay.Service.Reliable(ctx)
		require.NoError(t, err)
		require.Len(t, nodes, len(planet.StorageNodes)-len(nodesToKill))

		// we should be able to download data without any of the original nodes
		newData, err := ul.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, testData, newData)
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

func (mock *piecestoreMock) DeletePiece(ctx context.Context, delete *pb.PieceDeletePieceRequest) (_ *pb.PieceDeletePieceResponse, err error) {
	return nil, nil
}

func (mock *piecestoreMock) DeletePieces(ctx context.Context, delete *pb.DeletePiecesRequest) (_ *pb.DeletePiecesResponse, err error) {
	return nil, nil
}

func (mock *piecestoreMock) Retain(ctx context.Context, retain *pb.RetainRequest) (_ *pb.RetainResponse, err error) {
	return nil, nil
}
func (mock *piecestoreMock) RestoreTrash(context.Context, *pb.RestoreTrashRequest) (*pb.RestoreTrashResponse, error) {
	return nil, nil
}

func TestDownloadFromUnresponsiveNode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 5, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedData := testrand.Bytes(memory.MiB)

		err := planet.Uplinks[0].UploadWithConfig(ctx, planet.Satellites[0], &uplink.RSConfig{
			MinThreshold:     2,
			RepairThreshold:  3,
			SuccessThreshold: 4,
			MaxThreshold:     5,
		}, "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		// get a remote segment from pointerdb
		pdb := planet.Satellites[0].Metainfo.Service
		listResponse, _, err := pdb.List(ctx, "", "", true, 0, 0)
		require.NoError(t, err)

		var path string
		var pointer *pb.Pointer
		for _, v := range listResponse {
			path = v.GetPath()
			pointer, err = pdb.Get(ctx, path)
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
				tlscfg := tlsopts.Config{
					RevocationDBURL:     "bolt://" + filepath.Join(ctx.Dir("fakestoragenode"), "revocation.db"),
					UsePeerCAWhitelist:  true,
					PeerCAWhitelistPath: wl,
					PeerIDVersions:      "*",
					Extensions: extensions.Config{
						Revocation:          false,
						WhitelistSignedLeaf: false,
					},
				}

				revocationDB, err := revocation.NewDBFromCfg(tlscfg)
				require.NoError(t, err)

				tlsOptions, err := tlsopts.NewOptions(storageNode.Identity, tlscfg, revocationDB)
				require.NoError(t, err)

				server, err := server.New(storageNode.Log.Named("mock-server"), tlsOptions, storageNode.Addr(), storageNode.PrivateAddr(), nil)
				require.NoError(t, err)
				pb.RegisterPiecestoreServer(server.GRPC(), &piecestoreMock{})
				go func() {
					// TODO: get goroutine under control
					err := server.Run(ctx)
					require.NoError(t, err)

					err = revocationDB.Close()
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

func TestDeleteWithOfflineStoragenode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedData := testrand.Bytes(5 * memory.MiB)

		config := planet.Uplinks[0].GetConfig(planet.Satellites[0])
		config.Client.SegmentSize = 1 * memory.MiB
		err := planet.Uplinks[0].UploadWithClientConfig(ctx, planet.Satellites[0], config, "test-bucket", "test-file", expectedData)
		require.NoError(t, err)

		for _, node := range planet.StorageNodes {
			err = planet.StopPeer(node)
			require.NoError(t, err)
		}

		err = planet.Uplinks[0].Delete(ctx, planet.Satellites[0], "test-bucket", "test-file")
		require.Error(t, err)

		key := planet.Uplinks[0].APIKey[planet.Satellites[0].ID()]

		metainfoClient, err := planet.Uplinks[0].DialMetainfo(ctx, planet.Satellites[0], key)
		require.NoError(t, err)
		defer ctx.Check(metainfoClient.Close)

		objects, _, err := metainfoClient.ListObjects(ctx, metainfo.ListObjectsParams{
			Bucket: []byte("test-bucket"),
		})
		require.NoError(t, err)
		require.Equal(t, 0, len(objects))
	})
}
