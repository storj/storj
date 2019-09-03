// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificates_test

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/certificates"
	"storj.io/storj/pkg/certificates/authorizations"
	"storj.io/storj/pkg/certificates/client"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/revocation"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

// TODO: test sad path
func TestCertificateSigner_Sign_E2E(t *testing.T) {
	testidentity.SignerVersionsTest(t, func(t *testing.T, _ storj.IDVersion, signer *identity.FullCertificateAuthority) {
		testidentity.CompleteIdentityVersionsTest(t, func(t *testing.T, _ storj.IDVersion, serverIdent *identity.FullIdentity) {
			testidentity.CompleteIdentityVersionsTest(t, func(t *testing.T, _ storj.IDVersion, clientIdent *identity.FullIdentity) {
				ctx := testcontext.New(t)
				defer ctx.Cleanup()

				caCert := ctx.File("ca.cert")
				caKey := ctx.File("ca.key")
				userID := "user@mail.test"
				signerCAConfig := identity.FullCAConfig{
					CertPath: caCert,
					KeyPath:  caKey,
				}
				err := signerCAConfig.Save(signer)
				require.NoError(t, err)
				config := certificates.Config{
					Authorizations: authorizations.Config{
						DBURL: "bolt://" + ctx.File("authorizations.db"),
					},
					CA: signerCAConfig,
				}

				authDB, err := authorizations.NewDBFromCfg(config.Authorizations)
				require.NoError(t, err)

				auths, err := authDB.Create(ctx, "user@mail.test", 1)
				require.NoError(t, err)
				require.NotEmpty(t, auths)

				err = authDB.Close()
				require.NoError(t, err)

				sc := server.Config{
					Config: tlsopts.Config{
						PeerIDVersions: "*",
					},
					Address:        "127.0.0.1:0",
					PrivateAddress: "127.0.0.1:0",
				}

				revocationDB, err := revocation.NewDBFromCfg(sc.Config)
				require.NoError(t, err)
				defer ctx.Check(revocationDB.Close)

				serverOpts, err := tlsopts.NewOptions(serverIdent, sc.Config, revocationDB)
				require.NoError(t, err)
				require.NotNil(t, serverOpts)

				peer, err := certificates.New(zaptest.NewLogger(t), serverIdent, signer, authDB, revocationDB, &config)
				require.NoError(t, err)
				require.NotNil(t, peer)

				ctx.Go(func() error {
					err := peer.Run(ctx)
					assert.NoError(t, err)
					return err
				})
				defer ctx.Check(peer.Close)

				clientOpts, err := tlsopts.NewOptions(clientIdent, tlsopts.Config{PeerIDVersions: "*"}, nil)
				require.NoError(t, err)

				clientTransport := transport.NewClient(clientOpts)

				client, err := client.NewClient(ctx, clientTransport, peer.Server.Addr().String())
				require.NoError(t, err)
				require.NotNil(t, client)

				signedChainBytes, err := client.Sign(ctx, auths[0].Token.String())
				require.NoError(t, err)
				require.NotEmpty(t, signedChainBytes)

				signedChain, err := pkcrypto.CertsFromDER(signedChainBytes)
				require.NoError(t, err)

				assert.Equal(t, clientIdent.CA.RawTBSCertificate, signedChain[0].RawTBSCertificate)
				assert.Equal(t, signer.Cert.Raw, signedChainBytes[1])
				// TODO: test scenario with rest chain
				//assert.Equal(t, signingCA.RawRestChain(), signedChainBytes[1:])

				err = signedChain[0].CheckSignatureFrom(signer.Cert)
				require.NoError(t, err)

				err = peer.Close()
				assert.NoError(t, err)

				// NB: re-open after closing for server
				authDB, err = authorizations.NewDBFromCfg(config.Authorizations)
				require.NoError(t, err)
				defer ctx.Check(authDB.Close)
				require.NotNil(t, authDB)

				updatedAuths, err := authDB.Get(ctx, userID)
				require.NoError(t, err)
				require.NotEmpty(t, updatedAuths)
				require.NotNil(t, updatedAuths[0].Claim)

				now := time.Now().Unix()
				claim := updatedAuths[0].Claim

				listenerHost, _, err := net.SplitHostPort(peer.Server.Addr().String())
				require.NoError(t, err)
				claimHost, _, err := net.SplitHostPort(claim.Addr)
				require.NoError(t, err)

				assert.Equal(t, listenerHost, claimHost)
				assert.Equal(t, signedChainBytes, claim.SignedChainBytes)
				assert.Condition(t, func() bool {
					return now-10 < claim.Timestamp &&
						claim.Timestamp < now+10
				})
			})
		})
	})
}

func TestCertificateSigner_Sign(t *testing.T) {
	testidentity.SignerVersionsTest(t, func(t *testing.T, _ storj.IDVersion, ca *identity.FullCertificateAuthority) {
		testidentity.CompleteIdentityVersionsTest(t, func(t *testing.T, _ storj.IDVersion, ident *identity.FullIdentity) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			userID := "user@mail.test"
			// TODO: test with all types of authorization DBs (bolt, redis, etc.)
			authDB, err := authorizations.NewDB("bolt://"+ctx.File("authorizations.db"), false)
			require.NoError(t, err)
			defer ctx.Check(authDB.Close)
			require.NotNil(t, authDB)

			auths, err := authDB.Create(ctx, userID, 1)
			require.NoError(t, err)
			require.NotEmpty(t, auths)

			expectedAddr := &net.TCPAddr{
				IP:   net.ParseIP("1.2.3.4"),
				Port: 5,
			}
			grpcPeer := &peer.Peer{
				Addr: expectedAddr,
				AuthInfo: credentials.TLSInfo{
					State: tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
					},
				},
			}
			peerCtx := peer.NewContext(ctx, grpcPeer)

			srvIdent, err := testidentity.NewTestIdentity(ctx)
			require.NoError(t, err)

			certSigner := certificates.NewCertificatesServer(zaptest.NewLogger(t), srvIdent, ca, authDB, 0)
			req := pb.SigningRequest{
				Timestamp: time.Now().Unix(),
				AuthToken: auths[0].Token.String(),
			}
			res, err := certSigner.Sign(peerCtx, &req)
			require.NoError(t, err)
			require.NotNil(t, res)
			require.NotEmpty(t, res.Chain)

			signedChain, err := pkcrypto.CertsFromDER(res.Chain)
			require.NoError(t, err)

			assert.Equal(t, ident.CA.RawTBSCertificate, signedChain[0].RawTBSCertificate)
			assert.Equal(t, ca.Cert.Raw, signedChain[1].Raw)
			// TODO: test scenario with rest chain
			//assert.Equal(t, signingCA.RawRestChain(), res.Chain[1:])

			err = signedChain[0].CheckSignatureFrom(ca.Cert)
			require.NoError(t, err)

			updatedAuths, err := authDB.Get(ctx, userID)
			require.NoError(t, err)
			require.NotEmpty(t, updatedAuths)
			require.NotNil(t, updatedAuths[0].Claim)

			now := time.Now().Unix()
			claim := updatedAuths[0].Claim
			assert.Equal(t, expectedAddr.String(), claim.Addr)
			assert.Equal(t, res.Chain, claim.SignedChainBytes)
			assert.Condition(t, func() bool {
				return now-authorizations.MaxClaimDelaySeconds < claim.Timestamp &&
					claim.Timestamp < now+authorizations.MaxClaimDelaySeconds
			})
		})
	})
}

func newTestAuthDB(ctx *testcontext.Context) (*authorizations.DB, error) {
	dbURL := "bolt://" + ctx.File("authorizations.db")
	return authorizations.NewDB(dbURL, false)
}
