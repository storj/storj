// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package sntest

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testrand"
	"storj.io/storj/shared/mudplanet"
	"storj.io/storj/storagenode"
	piecestore2 "storj.io/storj/storagenode/piecestore"
	"storj.io/uplink/private/piecestore"
)

// UploadPiece uploads a piece of data to a storage node.
// It creates an order limit, signs it, and uploads the data to the specified node URL.
// Returns the generated piece ID.
func UploadPiece(ctx context.Context, t *testing.T, signer signing.Signer, nodeURL storj.NodeURL, data []byte) (id storj.PieceID) {
	dialer, err := CreateRPCDialer()
	require.NoError(t, err)

	store, err := piecestore.Dial(ctx, dialer, nodeURL, piecestore.DefaultConfig)
	require.NoError(t, err)

	defer func() {
		_ = store.Close()
	}()

	pub, pk, err := storj.NewPieceKey()
	require.NoError(t, err)

	pieceID := testrand.PieceID()

	limit := &pb.OrderLimit{
		PieceId:         pieceID,
		SerialNumber:    testrand.SerialNumber(),
		SatelliteId:     signer.ID(),
		StorageNodeId:   nodeURL.ID,
		Action:          pb.PieceAction_PUT,
		Limit:           int64(len(data)),
		OrderCreation:   time.Now(),
		OrderExpiration: time.Now().Add(24 * time.Hour),
		UplinkPublicKey: pub,
	}

	signedOrderLimit, err := signing.SignOrderLimit(ctx, signer, limit)
	require.NoError(t, err)

	_, err = store.UploadReader(ctx, signedOrderLimit, pk, bytes.NewBuffer(data))
	require.NoError(t, err)

	return pieceID
}

// DownloadPiece downloads a piece from a storage node.
// It creates an order limit for downloading, signs it, and retrieves the data.
// Returns the downloaded data as a byte slice.
func DownloadPiece(ctx context.Context, t *testing.T, signer signing.Signer, nodeURL storj.NodeURL, pieceID storj.PieceID, size int64) (data []byte) {
	dialer, err := CreateRPCDialer()
	require.NoError(t, err)

	store, err := piecestore.Dial(ctx, dialer, nodeURL, piecestore.DefaultConfig)
	require.NoError(t, err)

	defer func() {
		_ = store.Close()
	}()

	pub, pk, err := storj.NewPieceKey()
	require.NoError(t, err)

	limit := &pb.OrderLimit{
		PieceId:         pieceID,
		SerialNumber:    testrand.SerialNumber(),
		SatelliteId:     signer.ID(),
		StorageNodeId:   nodeURL.ID,
		Action:          pb.PieceAction_GET,
		Limit:           size,
		OrderCreation:   time.Now(),
		OrderExpiration: time.Now().Add(24 * time.Hour),
		UplinkPublicKey: pub,
	}

	signedOrderLimit, err := signing.SignOrderLimit(ctx, signer, limit)
	require.NoError(t, err)

	download, err := store.Download(ctx, signedOrderLimit, pk, 0, size)
	require.NoError(t, err)
	defer func() {
		_ = download.Close()
	}()
	all, err := io.ReadAll(download)
	require.NoError(t, err)

	return all
}

// CreateRPCDialer creates an RPC dialer with a new identity and TLS configuration.
// It configures the dialer with minimal security settings suitable for testing.
func CreateRPCDialer() (rpc.Dialer, error) {
	ident, err := identity.NewFullIdentity(context.Background(), identity.NewCAOptions{
		Difficulty:  0,
		Concurrency: 1,
	})
	if err != nil {
		return rpc.Dialer{}, errs.Wrap(err)
	}

	tlsConfig := tlsopts.Config{
		UsePeerCAWhitelist: false,
		PeerIDVersions:     "0",
	}

	tlsOptions, err := tlsopts.NewOptions(ident, tlsConfig, nil)
	if err != nil {
		return rpc.Dialer{}, err
	}

	dialer := rpc.NewDefaultDialer(tlsOptions)
	//lint:ignore SA1019 deprecated is fine here.
	dialer.Connector = rpc.NewDefaultTCPConnector(nil)

	return dialer, nil
}

// InitStoragenodeDirs initializes the directory structure required for a storage node.
// It creates the necessary directories and writes a verification file with the node ID.
func InitStoragenodeDirs(t *testing.T, id storj.NodeID, config *piecestore2.OldConfig) {
	for _, directory := range []string{"blobs", "temp", "trash"} {
		d := filepath.Join(config.Path, directory)
		err := os.MkdirAll(d, 0755)
		if err != nil {
			t.Log("Couldn't create storagenode directories " + d)
		}
	}
	v, err := os.Create(filepath.Join(config.Path, "storage-dir-verification"))
	require.NoError(t, err)
	defer func() {
		_ = v.Close()
	}()
	_, err = v.Write(id.Bytes())
	require.NoError(t, err)
}

// Storagenode is a predefined customization, just enough to run a storage node with mudplanet.
var Storagenode = mudplanet.Customization{
	Modules: mudplanet.Modules{
		storagenode.Module,
		mudplanet.TrustAll,
	},
	PreInit: []any{
		InitStoragenodeDirs,
	},
}
