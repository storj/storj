// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenode_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/storj/private/server"
	"storj.io/storj/shared/debug"
	"storj.io/storj/shared/mudplanet"
	"storj.io/storj/shared/mudplanet/sntest"
	"storj.io/storj/storagenode"
)

func TestDebugServer(t *testing.T) {
	mudplanet.Run(t, mudplanet.Config{
		Components: []mudplanet.Component{
			mudplanet.NewComponent("storagenode",
				mudplanet.WithModule(storagenode.Module),
				mudplanet.WithRunning[debug.Wrapper]()),
		},
	}, func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		wrapper := mudplanet.FindFirst[debug.Wrapper](t, run, "storagenode", 0)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+wrapper.Listener.Addr().String()+"/debug/vars", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer func() {
			_ = resp.Body.Close()
		}()
		require.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestUploadPiecestore(t *testing.T) {
	mudplanet.Run(t, mudplanet.Config{
		Components: []mudplanet.Component{
			mudplanet.NewComponent("storagenode", sntest.Storagenode, mudplanet.WithRunning[*storagenode.EndpointRegistration]()),
		},
	}, func(t *testing.T, ctx context.Context, run mudplanet.RuntimeEnvironment) {
		srv := mudplanet.FindFirst[*server.Server](t, run, "storagenode", 0)
		nodeID := mudplanet.FindFirst[*identity.FullIdentity](t, run, "storagenode", 0)

		signer := signing.SignerFromFullIdentity(testidentity.MustPregeneratedIdentity(149, storj.LatestIDVersion()))
		url := storj.NodeURL{
			ID:      nodeID.ID,
			Address: srv.Addr().String(),
		}

		// Listener is up by this point, looks like it works, even if the Server.Run() is not executed fully.
		pieceID := sntest.UploadPiece(ctx, t,
			signer,
			url,
			[]byte{1, 2, 3, 4})

		data := sntest.DownloadPiece(ctx, t, signer, url, pieceID, 4)
		require.Equal(t, []byte{1, 2, 3, 4}, data)

	})
}
