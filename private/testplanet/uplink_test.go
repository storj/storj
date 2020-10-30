// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information

package testplanet_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
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
	"storj.io/storj/satellite/metainfo/metabase"
	"storj.io/uplink"
	"storj.io/uplink/private/metainfo"
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
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 5),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		// first, upload some remote data
		ul := planet.Uplinks[0]
		satellite := planet.Satellites[0]

		testData := testrand.Bytes(memory.MiB)

		err := ul.Upload(ctx, satellite, "testbucket", "test/path", testData)
		require.NoError(t, err)

		// get a remote segment from pointerdb
		pdb := satellite.Metainfo.Service
		listResponse, _, err := pdb.List(ctx, metabase.SegmentKey{}, "", true, 0, 0)
		require.NoError(t, err)

		var path string
		var pointer *pb.Pointer
		for _, v := range listResponse {
			path = v.GetPath()
			pointer, err = pdb.Get(ctx, metabase.SegmentKey(path))
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

		for _, piece := range remotePieces[:toKill] {
			err := planet.StopNodeAndUpdate(ctx, planet.FindNode(piece.NodeId))
			require.NoError(t, err)
		}

		// confirm that we marked the correct number of storage nodes as offline
		nodes, err := satellite.Overlay.Service.Reliable(ctx)
		require.NoError(t, err)
		require.Len(t, nodes, len(planet.StorageNodes)-toKill)

		// we should be able to download data without any of the original nodes
		newData, err := ul.Download(ctx, satellite, "testbucket", "test/path")
		require.NoError(t, err)
		require.Equal(t, testData, newData)
	})
}

type piecestoreMock struct {
}

func (mock *piecestoreMock) Upload(server pb.DRPCPiecestore_UploadStream) error {
	return nil
}

func (mock *piecestoreMock) Download(server pb.DRPCPiecestore_DownloadStream) error {
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
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.ReconfigureRS(2, 3, 4, 5),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedData := testrand.Bytes(memory.MiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "testbucket", "test/path", expectedData)
		require.NoError(t, err)

		// get a remote segment from pointerdb
		pdb := planet.Satellites[0].Metainfo.Service
		listResponse, _, err := pdb.List(ctx, metabase.SegmentKey{}, "", true, 0, 0)
		require.NoError(t, err)

		var path string
		var pointer *pb.Pointer
		for _, v := range listResponse {
			path = v.GetPath()
			pointer, err = pdb.Get(ctx, metabase.SegmentKey(path))
			require.NoError(t, err)
			if pointer.GetType() == pb.Pointer_REMOTE {
				break
			}
		}

		// choose used storage node and replace it with fake listener
		storageNode := planet.FindNode(pointer.Remote.RemotePieces[0].NodeId)
		require.NotNil(t, storageNode)

		err = planet.StopPeer(storageNode)
		require.NoError(t, err)

		wl, err := planet.WriteWhitelist(storj.LatestIDVersion())
		require.NoError(t, err)
		tlscfg := tlsopts.Config{
			RevocationDBURL:     "bolt://" + ctx.File("fakestoragenode", "revocation.db"),
			UsePeerCAWhitelist:  true,
			PeerCAWhitelistPath: wl,
			PeerIDVersions:      "*",
			Extensions: extensions.Config{
				Revocation:          false,
				WhitelistSignedLeaf: false,
			},
		}

		revocationDB, err := revocation.OpenDBFromCfg(ctx, tlscfg)
		require.NoError(t, err)

		tlsOptions, err := tlsopts.NewOptions(storageNode.Identity, tlscfg, revocationDB)
		require.NoError(t, err)

		server, err := server.New(storageNode.Log.Named("mock-server"), tlsOptions, storageNode.Addr(), storageNode.PrivateAddr())
		require.NoError(t, err)

		err = pb.DRPCRegisterPiecestore(server.DRPC(), &piecestoreMock{})
		require.NoError(t, err)

		defer ctx.Check(server.Close)

		subctx, subcancel := context.WithCancel(ctx)
		defer subcancel()
		ctx.Go(func() error {
			if err := server.Run(subctx); err != nil {
				return errs.Wrap(err)
			}

			return errs.Wrap(revocationDB.Close())
		})

		data, err := planet.Uplinks[0].Download(ctx, planet.Satellites[0], "testbucket", "test/path")
		assert.NoError(t, err)

		assert.Equal(t, expectedData, data)
	})
}

func TestDeleteWithOfflineStoragenode(t *testing.T) {
	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1, StorageNodeCount: 6, UplinkCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: testplanet.MaxSegmentSize(1 * memory.MiB),
		},
	}, func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
		expectedData := testrand.Bytes(5 * memory.MiB)

		err := planet.Uplinks[0].Upload(ctx, planet.Satellites[0], "test-bucket", "test-file", expectedData)
		require.NoError(t, err)

		for _, node := range planet.StorageNodes {
			err = planet.StopPeer(node)
			require.NoError(t, err)
		}

		err = planet.Uplinks[0].DeleteObject(ctx, planet.Satellites[0], "test-bucket", "test-file")
		require.NoError(t, err)

		_, err = planet.Uplinks[0].Download(ctx, planet.Satellites[0], "test-bucket", "test-file")
		require.Error(t, err)
		require.True(t, errors.Is(err, uplink.ErrObjectNotFound))

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
