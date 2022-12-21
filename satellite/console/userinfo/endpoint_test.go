// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package userinfo_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
)

func TestEndpointGet_UnTrusted(t *testing.T) {
	t.Skip("disable until UserInfo is added to API. See issue #5363")

	// trusted identity
	ident, err := testidentity.NewTestIdentity(context.TODO())
	require.NoError(t, err)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				url, err := storj.ParseNodeURL(ident.ID.String() + "@")
				require.NoError(t, err)
				config.Userinfo.AllowedPeers = storj.NodeURLs{url}
			},
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]

			state := tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
			}
			peerCtx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
				Addr:  sat.API.Console.Listener.Addr(),
				State: state,
			})
			_, err = sat.Userinfo.Endpoint.Get(peerCtx, &pb.GetUserInfoRequest{})
			// a trusted peer should be able to get Userinfo
			require.NoError(t, err)

			// untrusted identity
			badIdent, err := testidentity.NewTestIdentity(ctx)
			require.NoError(t, err)

			state = tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{badIdent.Leaf, badIdent.CA},
			}
			peerCtx = rpcpeer.NewContext(ctx, &rpcpeer.Peer{
				Addr:  sat.API.Console.Listener.Addr(),
				State: state,
			})
			_, err = sat.Userinfo.Endpoint.Get(peerCtx, &pb.GetUserInfoRequest{})
			// an untrusted peer shouldn't be able to get Userinfo
			require.Error(t, err)
			require.EqualError(t, err, fmt.Sprintf("userinfo_endpoint: peer %q is untrusted", badIdent.ID))
		})
}
