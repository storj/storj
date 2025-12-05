// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package userinfo_test

import (
	"crypto/tls"
	"crypto/x509"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/common/identity/testidentity"
	"storj.io/common/macaroon"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/rpc/rpcstatus"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/private/testplanet"
	"storj.io/storj/satellite"
	"storj.io/storj/satellite/console"
)

func TestEndpointGet(t *testing.T) {

	// trusted identity
	ident, err := testidentity.NewTestIdentity(t.Context())
	require.NoError(t, err)

	testplanet.Run(t, testplanet.Config{
		SatelliteCount: 1,
		Reconfigure: testplanet.Reconfigure{
			Satellite: func(log *zap.Logger, index int, config *satellite.Config) {
				url, err := storj.ParseNodeURL(ident.ID.String() + "@")
				require.NoError(t, err)

				config.Userinfo.Enabled = true
				config.Userinfo.AllowedPeers = storj.NodeURLs{url}
			},
		},
	},
		func(t *testing.T, ctx *testcontext.Context, planet *testplanet.Planet) {
			sat := planet.Satellites[0]
			endpoint := sat.Userinfo.Endpoint
			addr := sat.API.Console.Listener.Addr()

			t.Run("reject untrusted peer", func(t *testing.T) {
				// untrusted identity.
				badIdent, err := testidentity.NewTestIdentity(ctx)
				require.NoError(t, err)

				state := tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{badIdent.Leaf, badIdent.CA},
				}
				peerCtx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
					Addr:  addr,
					State: state,
				})
				_, err = endpoint.Get(peerCtx, &pb.GetUserInfoRequest{})
				// an untrusted peer shouldn't be able to get Userinfo.
				require.Error(t, err)
				require.Equal(t, rpcstatus.PermissionDenied, rpcstatus.Code(err))
			})

			t.Run("allow trusted peer", func(t *testing.T) {
				// using trusted ident.
				state := tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
				}
				peerCtx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
					Addr:  addr,
					State: state,
				})
				_, err = endpoint.Get(peerCtx, &pb.GetUserInfoRequest{})
				// trusted peer should not get an untrusted error
				// but an error for not adding API key to the request.
				require.Error(t, err)
				require.Equal(t, rpcstatus.InvalidArgument, rpcstatus.Code(err))
			})

			t.Run("get userinfo", func(t *testing.T) {
				newUser := console.CreateUser{
					FullName:  "username",
					ShortName: "",
					Email:     "userinfo@test.test",
				}

				user, err := sat.AddUser(ctx, newUser, 1)
				require.NoError(t, err)
				require.Equal(t, console.FreeUser, user.Kind)

				project, err := sat.AddProject(ctx, user.ID, "info")
				require.NoError(t, err)

				secret, err := macaroon.NewSecret()
				require.NoError(t, err)

				key, err := macaroon.NewAPIKey(secret)
				require.NoError(t, err)

				keyInfo := console.APIKeyInfo{
					Name:      "test",
					ProjectID: project.ID,
					Secret:    secret,
				}

				_, err = sat.DB.Console().APIKeys().Create(ctx, key.Head(), keyInfo)
				require.NoError(t, err)

				state := tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
				}
				peerCtx := rpcpeer.NewContext(ctx, &rpcpeer.Peer{
					Addr:  addr,
					State: state,
				})

				response, err := endpoint.Get(peerCtx, &pb.GetUserInfoRequest{
					Header: &pb.RequestHeader{
						ApiKey: key.SerializeRaw(),
					},
				})
				// a trusted peer should be able to get Userinfo.
				require.NoError(t, err)
				require.Equal(t, false, response.PaidTier)

				userCtx, err := sat.UserContext(ctx, user.ID)
				require.NoError(t, err)
				// add a credit card to put the user in the paid tier.
				_, err = sat.API.Console.Service.Payments().AddCreditCard(userCtx, "test-cc-token")
				require.NoError(t, err)

				// get user info again
				response, err = endpoint.Get(peerCtx, &pb.GetUserInfoRequest{
					Header: &pb.RequestHeader{
						ApiKey: key.SerializeRaw(),
					},
				})
				require.NoError(t, err)
				// user should now be in paid tier.
				require.Equal(t, true, response.PaidTier)

				kind := console.NFRUser
				err = sat.API.DB.Console().Users().Update(ctx, user.ID, console.UpdateUserRequest{Kind: &kind})
				require.NoError(t, err)

				response, err = endpoint.Get(peerCtx, &pb.GetUserInfoRequest{
					Header: &pb.RequestHeader{
						ApiKey: key.SerializeRaw(),
					},
				})
				require.NoError(t, err)
				// user should now be in paid tier.
				require.Equal(t, true, response.PaidTier)
			})
		})
}
