// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package certificates

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/peertls/tlsopts"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/server"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/storage"
)

var (
	idents = testidentity.NewPregeneratedIdentities(storj.LatestIDVersion())
	t1     = Token{
		UserID: "user@mail.test",
		Data:   [tokenDataLength]byte{1, 2, 3},
	}
	t2 = Token{
		UserID: "user2@mail.test",
		Data:   [tokenDataLength]byte{4, 5, 6},
	}
)

func TestCertSignerConfig_NewAuthDB(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB, err := newTestAuthDB(ctx)
	require.NoError(t, err)
	defer ctx.Check(authDB.Close)

	assert.NotNil(t, authDB)
	assert.NotNil(t, authDB.DB)
}

func TestAuthorizationDB_Create(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB, err := newTestAuthDB(ctx)
	require.NoError(t, err)
	defer ctx.Check(authDB.Close)

	cases := []struct {
		testID,
		email string
		startCount,
		incCount,
		newCount,
		endCount int
		errClass *errs.Class
		err      error
	}{
		{
			"first authorization",
			"user1@mail.test",
			0, 1, 1, 1,
			nil, nil,
		},
		{
			"second authorization",
			"user1@mail.test",
			1, 2, 2, 3,
			nil, nil,
		},
		{
			"large authorization",
			"user2@mail.test",
			0, 5, 5, 5,
			nil, nil,
		},
		{
			"authorization error",
			"user2@mail.test",
			5, -1, 0, 5,
			&ErrAuthorizationDB, ErrAuthorizationCount,
		},
	}

	for _, c := range cases {
		testCase := c
		t.Run(c.testID, func(t *testing.T) {
			emailKey := storage.Key(testCase.email)

			if testCase.startCount == 0 {
				_, err = authDB.DB.Get(ctx, emailKey)
				assert.Error(t, err)
			} else {
				v, err := authDB.DB.Get(ctx, emailKey)
				require.NoError(t, err)
				require.NotEmpty(t, v)

				var existingAuths Authorizations
				err = existingAuths.Unmarshal(v)
				require.NoError(t, err)
				require.Len(t, existingAuths, testCase.startCount)
			}

			expectedAuths, err := authDB.Create(ctx, testCase.email, testCase.incCount)
			if testCase.errClass != nil {
				assert.True(t, testCase.errClass.Has(err))
			}
			if testCase.err != nil {
				assert.Equal(t, testCase.err, err)
			}
			if testCase.errClass == nil && testCase.err == nil {
				assert.NoError(t, err)
			}
			assert.Len(t, expectedAuths, testCase.newCount)

			v, err := authDB.DB.Get(ctx, emailKey)
			assert.NoError(t, err)
			assert.NotEmpty(t, v)

			var actualAuths Authorizations
			err = actualAuths.Unmarshal(v)
			assert.NoError(t, err)
			assert.Len(t, actualAuths, testCase.endCount)
		})
	}
}

func TestAuthorizationDB_Get(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB, err := newTestAuthDB(ctx)
	require.NoError(t, err)
	defer ctx.Check(authDB.Close)

	var expectedAuths Authorizations
	for i := 0; i < 5; i++ {
		expectedAuths = append(expectedAuths, &Authorization{
			Token: t1,
		})
	}

	authsBytes, err := expectedAuths.Marshal()
	require.NoError(t, err)

	err = authDB.DB.Put(ctx, storage.Key("user@mail.test"), authsBytes)
	require.NoError(t, err)

	cases := []struct {
		testID,
		email string
		result Authorizations
	}{
		{
			"Non-existent email",
			"nouser@mail.test",
			nil,
		},
		{
			"Existing email",
			"user@mail.test",
			expectedAuths,
		},
	}

	for _, c := range cases {
		testCase := c
		t.Run(testCase.testID, func(t *testing.T) {
			auths, err := authDB.Get(ctx, testCase.email)
			require.NoError(t, err)
			if testCase.result != nil {
				assert.NotEmpty(t, auths)
				assert.Len(t, auths, len(testCase.result))
			} else {
				assert.Empty(t, auths)
			}
		})
	}
}

func TestAuthorizationDB_Claim_Valid(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB, err := newTestAuthDB(ctx)
	require.NoError(t, err)
	defer ctx.Check(authDB.Close)

	userID := "user@mail.test"

	auths, err := authDB.Create(ctx, userID, 1)
	require.NoError(t, err)
	require.NotEmpty(t, auths)

	ident, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	require.NotNil(t, ident)

	addr := &net.TCPAddr{
		IP:   net.ParseIP("1.2.3.4"),
		Port: 5,
	}
	grpcPeer := &peer.Peer{
		Addr: addr,
		AuthInfo: credentials.TLSInfo{
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
			},
		},
	}

	now := time.Now().Unix()
	req := &pb.SigningRequest{
		AuthToken: auths[0].Token.String(),
		Timestamp: now,
	}
	difficulty, err := ident.ID.Difficulty()
	require.NoError(t, err)

	err = authDB.Claim(ctx, &ClaimOpts{
		Req:           req,
		Peer:          grpcPeer,
		ChainBytes:    [][]byte{ident.CA.Raw},
		MinDifficulty: difficulty,
	})
	require.NoError(t, err)

	updatedAuths, err := authDB.Get(ctx, userID)
	require.NoError(t, err)
	require.NotEmpty(t, updatedAuths)
	assert.Equal(t, auths[0].Token, updatedAuths[0].Token)

	require.NotNil(t, updatedAuths[0].Claim)

	claim := updatedAuths[0].Claim
	assert.Equal(t, grpcPeer.Addr.String(), claim.Addr)
	assert.Equal(t, [][]byte{ident.CA.Raw}, claim.SignedChainBytes)
	assert.Condition(t, func() bool {
		return now-MaxClaimDelaySeconds < claim.Timestamp &&
			claim.Timestamp < now+MaxClaimDelaySeconds
	})
}

func TestAuthorizationDB_Claim_Invalid(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB, err := newTestAuthDB(ctx)
	require.NoError(t, err)
	defer ctx.Check(authDB.Close)

	userID := "user@mail.test"
	claimedTime := int64(1000000)
	claimedAddr := "6.7.8.9:0"

	ident1, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	require.NotNil(t, ident1)

	claimedIdent := &identity.PeerIdentity{
		CA:   ident1.CA,
		Leaf: ident1.Leaf,
	}

	auths, err := authDB.Create(ctx, userID, 2)
	require.NoError(t, err)
	require.NotEmpty(t, auths)

	claimedIndex, unclaimedIndex := 0, 1

	auths[claimedIndex].Claim = &Claim{
		Timestamp:        claimedTime,
		Addr:             claimedAddr,
		Identity:         claimedIdent,
		SignedChainBytes: [][]byte{claimedIdent.CA.Raw},
	}
	err = authDB.put(ctx, userID, auths)
	require.NoError(t, err)

	ident2, err := testidentity.NewTestIdentity(ctx)
	require.NoError(t, err)
	require.NotNil(t, ident2)

	addr := &net.TCPAddr{
		IP:   net.ParseIP("1.2.3.4"),
		Port: 5,
	}
	grpcPeer := &peer.Peer{
		Addr: addr,
		AuthInfo: credentials.TLSInfo{
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{ident2.Leaf, ident2.CA},
			},
		},
	}

	difficulty2, err := ident2.ID.Difficulty()
	require.NoError(t, err)

	t.Run("double claim", func(t *testing.T) {
		err = authDB.Claim(ctx, &ClaimOpts{
			Req: &pb.SigningRequest{
				AuthToken: auths[claimedIndex].Token.String(),
				Timestamp: time.Now().Unix(),
			},
			Peer:          grpcPeer,
			ChainBytes:    [][]byte{ident2.CA.Raw},
			MinDifficulty: difficulty2,
		})
		if assert.Error(t, err) {
			assert.True(t, ErrAuthorization.Has(err))
			// NB: token string shouldn't leak into error message
			assert.NotContains(t, err.Error(), auths[claimedIndex].Token.String())
		}

		updatedAuths, err := authDB.Get(ctx, userID)
		require.NoError(t, err)
		require.NotEmpty(t, updatedAuths)

		assert.Equal(t, auths[claimedIndex].Token, updatedAuths[claimedIndex].Token)

		claim := updatedAuths[claimedIndex].Claim
		assert.Equal(t, claimedAddr, claim.Addr)
		assert.Equal(t, [][]byte{ident1.CA.Raw}, claim.SignedChainBytes)
		assert.Equal(t, claimedTime, claim.Timestamp)
	})

	t.Run("invalid timestamp", func(t *testing.T) {
		err = authDB.Claim(ctx, &ClaimOpts{
			Req: &pb.SigningRequest{
				AuthToken: auths[unclaimedIndex].Token.String(),
				// NB: 1 day ago
				Timestamp: time.Now().Unix() - 86400,
			},
			Peer:          grpcPeer,
			ChainBytes:    [][]byte{ident2.CA.Raw},
			MinDifficulty: difficulty2,
		})
		if assert.Error(t, err) {
			assert.True(t, ErrAuthorization.Has(err))
			// NB: token string shouldn't leak into error message
			assert.NotContains(t, err.Error(), auths[unclaimedIndex].Token.String())
		}

		updatedAuths, err := authDB.Get(ctx, userID)
		require.NoError(t, err)
		require.NotEmpty(t, updatedAuths)

		assert.Equal(t, auths[unclaimedIndex].Token, updatedAuths[unclaimedIndex].Token)
		assert.Nil(t, updatedAuths[unclaimedIndex].Claim)
	})

	t.Run("invalid difficulty", func(t *testing.T) {
		err = authDB.Claim(ctx, &ClaimOpts{
			Req: &pb.SigningRequest{
				AuthToken: auths[unclaimedIndex].Token.String(),
				Timestamp: time.Now().Unix(),
			},
			Peer:          grpcPeer,
			ChainBytes:    [][]byte{ident2.CA.Raw},
			MinDifficulty: difficulty2 + 1,
		})
		if assert.Error(t, err) {
			assert.True(t, ErrAuthorization.Has(err))
			// NB: token string shouldn't leak into error message
			assert.NotContains(t, err.Error(), auths[unclaimedIndex].Token.String())
		}

		updatedAuths, err := authDB.Get(ctx, userID)
		require.NoError(t, err)
		require.NotEmpty(t, updatedAuths)

		assert.Equal(t, auths[unclaimedIndex].Token, updatedAuths[unclaimedIndex].Token)
		assert.Nil(t, updatedAuths[unclaimedIndex].Claim)
	})
}

func TestNewAuthorization(t *testing.T) {
	userID := "user@mail.test"
	auth, err := NewAuthorization(userID)
	require.NoError(t, err)
	require.NotNil(t, auth)

	assert.NotZero(t, auth.Token)
	assert.Equal(t, userID, auth.Token.UserID)
	assert.NotEmpty(t, auth.Token.Data)
}

func TestAuthorizations_Marshal(t *testing.T) {
	expectedAuths := Authorizations{
		{Token: t1},
		{Token: t2},
	}

	authsBytes, err := expectedAuths.Marshal()
	require.NoError(t, err)
	require.NotEmpty(t, authsBytes)

	var actualAuths Authorizations
	decoder := gob.NewDecoder(bytes.NewBuffer(authsBytes))
	err = decoder.Decode(&actualAuths)
	assert.NoError(t, err)
	assert.NotNil(t, actualAuths)
	assert.Equal(t, expectedAuths, actualAuths)
}

func TestAuthorizations_Unmarshal(t *testing.T) {
	expectedAuths := Authorizations{
		{Token: t1},
		{Token: t2},
	}

	authsBytes, err := expectedAuths.Marshal()
	require.NoError(t, err)
	require.NotEmpty(t, authsBytes)

	var actualAuths Authorizations
	err = actualAuths.Unmarshal(authsBytes)
	assert.NoError(t, err)
	assert.NotNil(t, actualAuths)
	assert.Equal(t, expectedAuths, actualAuths)
}

func TestAuthorizations_Group(t *testing.T) {
	auths := make(Authorizations, 10)
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			auths[i] = &Authorization{
				Token: t1,
				Claim: &Claim{
					Timestamp: time.Now().Unix(),
				},
			}
		} else {
			auths[i] = &Authorization{
				Token: t2,
			}
		}
	}

	claimed, open := auths.Group()
	for _, a := range claimed {
		assert.NotNil(t, a.Claim)
	}
	for _, a := range open {
		assert.Nil(t, a.Claim)
	}
}

func TestAuthorizationDB_Emails(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB, err := newTestAuthDB(ctx)
	require.NoError(t, err)
	defer ctx.Check(authDB.Close)

	var authErrs errs.Group
	for i := 0; i < 5; i++ {
		_, err := authDB.Create(ctx, fmt.Sprintf("user%d@mail.test", i), 1)
		if err != nil {
			authErrs.Add(err)
		}
	}
	require.NoError(t, authErrs.Err())

	userIDs, err := authDB.UserIDs(ctx)
	assert.NoError(t, err)
	assert.NotEmpty(t, userIDs)
}

func TestParseToken_Valid(t *testing.T) {
	userID := "user@mail.test"
	data := [tokenDataLength]byte{1, 2, 3}

	cases := []struct {
		testID string
		userID string
	}{
		{
			"valid token",
			userID,
		},
		{
			"multiple delimiters",
			"us" + tokenDelimiter + "er@mail.test",
		},
	}

	for _, c := range cases {
		testCase := c
		t.Run(testCase.testID, func(t *testing.T) {
			b58Data := base58.CheckEncode(data[:], tokenVersion)
			tokenString := testCase.userID + tokenDelimiter + b58Data
			token, err := ParseToken(tokenString)
			require.NoError(t, err)
			require.NotNil(t, token)

			assert.Equal(t, testCase.userID, token.UserID)
			assert.Equal(t, data[:], token.Data[:])
		})
	}
}

func TestParseToken_Invalid(t *testing.T) {
	userID := "user@mail.test"
	data := [tokenDataLength]byte{1, 2, 3}

	cases := []struct {
		testID      string
		tokenString string
	}{
		{
			"no delimiter",
			userID + base58.CheckEncode(data[:], tokenVersion),
		},
		{
			"missing userID",
			tokenDelimiter + base58.CheckEncode(data[:], tokenVersion),
		},
		{
			"not enough data",
			userID + tokenDelimiter + base58.CheckEncode(data[:len(data)-10], tokenVersion),
		},
		{
			"too much data",
			userID + tokenDelimiter + base58.CheckEncode(append(data[:], []byte{0, 0, 0}...), tokenVersion),
		},
		{
			"data checksum/format error",
			userID + tokenDelimiter + base58.CheckEncode(data[:], tokenVersion)[:len(base58.CheckEncode(data[:], tokenVersion))-4] + "0000",
		},
	}

	for _, c := range cases {
		testCase := c
		t.Run(testCase.testID, func(t *testing.T) {
			token, err := ParseToken(testCase.tokenString)
			assert.Nil(t, token)
			assert.True(t, ErrInvalidToken.Has(err))
		})
	}
}

func TestToken_Equal(t *testing.T) {
	assert.True(t, t1.Equal(&t1))
	assert.False(t, t1.Equal(&t2))
}

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
				config := CertServerConfig{
					AuthorizationDBURL: "bolt://" + ctx.File("authorizations.db"),
					CA:                 signerCAConfig,
				}

				authDB, err := config.NewAuthDB()
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
				serverOpts, err := tlsopts.NewOptions(serverIdent, sc.Config)
				require.NoError(t, err)
				require.NotNil(t, serverOpts)

				service, err := server.New(zaptest.NewLogger(t), serverOpts, sc.Address, sc.PrivateAddress, nil, config)
				require.NoError(t, err)
				require.NotNil(t, service)

				ctx.Go(func() error {
					err := service.Run(ctx)
					assert.NoError(t, err)
					return err
				})
				defer ctx.Check(service.Close)

				clientOpts, err := tlsopts.NewOptions(clientIdent, tlsopts.Config{PeerIDVersions: "*"})
				require.NoError(t, err)

				clientTransport := transport.NewClient(clientOpts)

				client, err := NewClient(ctx, clientTransport, service.Addr().String())
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

				err = service.Close()
				assert.NoError(t, err)

				// NB: re-open after closing for server
				authDB, err = config.NewAuthDB()
				require.NoError(t, err)
				defer ctx.Check(authDB.Close)
				require.NotNil(t, authDB)

				updatedAuths, err := authDB.Get(ctx, userID)
				require.NoError(t, err)
				require.NotEmpty(t, updatedAuths)
				require.NotNil(t, updatedAuths[0].Claim)

				now := time.Now().Unix()
				claim := updatedAuths[0].Claim

				listenerHost, _, err := net.SplitHostPort(service.Addr().String())
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

func TestNewClient(t *testing.T) {
	t.Skip("needs proper grpc listener to work")

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ident, err := idents.NewIdentity()
	require.NoError(t, err)
	require.NotNil(t, ident)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NotNil(t, listener)

	defer ctx.Check(listener.Close)
	ctx.Go(func() error {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return nil
			}
			if err := conn.Close(); err != nil {
				return err
			}
		}
	})

	tlsOptions, err := tlsopts.NewOptions(ident, tlsopts.Config{})
	require.NoError(t, err)

	clientTransport := transport.NewClient(tlsOptions)

	t.Run("Basic", func(t *testing.T) {
		client, err := NewClient(ctx, clientTransport, listener.Addr().String())
		assert.NoError(t, err)
		assert.NotNil(t, client)

		defer ctx.Check(client.Close)
	})

	t.Run("ClientFrom", func(t *testing.T) {
		conn, err := clientTransport.DialAddress(ctx, listener.Addr().String())
		require.NoError(t, err)
		require.NotNil(t, conn)

		defer ctx.Check(conn.Close)

		pbClient := pb.NewCertificatesClient(conn)
		require.NotNil(t, pbClient)

		client, err := NewClientFrom(pbClient)
		assert.NoError(t, err)
		assert.NotNil(t, client)

		defer ctx.Check(client.Close)
	})
}

func TestCertificateSigner_Sign(t *testing.T) {
	testidentity.SignerVersionsTest(t, func(t *testing.T, _ storj.IDVersion, signer *identity.FullCertificateAuthority) {
		testidentity.CompleteIdentityVersionsTest(t, func(t *testing.T, _ storj.IDVersion, ident *identity.FullIdentity) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			userID := "user@mail.test"
			// TODO: test with all types of authorization DBs (bolt, redis, etc.)
			config := CertServerConfig{
				AuthorizationDBURL: "bolt://" + ctx.File("authorizations.db"),
			}

			authDB, err := config.NewAuthDB()
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

			certSigner := NewServer(zaptest.NewLogger(t), signer, authDB, 0)
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
			assert.Equal(t, signer.Cert.Raw, signedChain[1].Raw)
			// TODO: test scenario with rest chain
			//assert.Equal(t, signingCA.RawRestChain(), res.Chain[1:])

			err = signedChain[0].CheckSignatureFrom(signer.Cert)
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
				return now-MaxClaimDelaySeconds < claim.Timestamp &&
					claim.Timestamp < now+MaxClaimDelaySeconds
			})
		})
	})
}

func newTestAuthDB(ctx *testcontext.Context) (*AuthorizationDB, error) {
	dbPath := "bolt://" + ctx.File("authorizations.db")
	config := CertServerConfig{
		AuthorizationDBURL: dbPath,
	}
	return config.NewAuthDB()
}
