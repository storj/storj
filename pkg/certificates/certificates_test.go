// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package certificates

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/gob"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/testidentity"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

var (
	idents = testplanet.NewPregeneratedIdentities()
	t1     = Token{
		UserID: "user@example.com",
		Data:   [tokenDataLength]byte{1, 2, 3},
	}
	t2 = Token{
		UserID: "user2@example.com",
		Data:   [tokenDataLength]byte{4, 5, 6},
	}
)

func TestCertSignerConfig_NewAuthDB(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	authDB, err := newTestAuthDB(ctx)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	defer ctx.Check(authDB.Close)

	assert.NotNil(t, authDB)
	assert.NotNil(t, authDB.DB)
}

func TestAuthorizationDB_Create(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()
	authDB, err := newTestAuthDB(ctx)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
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
			"user1@example.com",
			0, 1, 1, 1,
			nil, nil,
		},
		{
			"second authorization",
			"user1@example.com",
			1, 2, 2, 3,
			nil, nil,
		},
		{
			"large authorization",
			"user2@example.com",
			0, 5, 5, 5,
			nil, nil,
		},
		{
			"authorization error",
			"user2@example.com",
			5, -1, 0, 5,
			&ErrAuthorizationDB, ErrAuthorizationCount,
		},
	}

	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			emailKey := storage.Key(c.email)

			if c.startCount == 0 {
				_, err = authDB.DB.Get(emailKey)
				assert.Error(t, err)
			} else {
				v, err := authDB.DB.Get(emailKey)
				assert.NoError(t, err)
				assert.NotEmpty(t, v)

				var existingAuths Authorizations
				err = existingAuths.Unmarshal(v)
				assert.NoError(t, err)
				if !assert.Len(t, existingAuths, c.startCount) {
					t.FailNow()
				}
			}

			expectedAuths, err := authDB.Create(c.email, c.incCount)
			if c.errClass != nil {
				assert.True(t, c.errClass.Has(err))
			}
			if c.err != nil {
				assert.Equal(t, c.err, err)
			}
			if c.errClass == nil && c.err == nil {
				assert.NoError(t, err)
			}
			assert.Len(t, expectedAuths, c.newCount)

			v, err := authDB.DB.Get(emailKey)
			assert.NoError(t, err)
			assert.NotEmpty(t, v)

			var actualAuths Authorizations
			err = actualAuths.Unmarshal(v)
			assert.NoError(t, err)
			assert.Len(t, actualAuths, c.endCount)
		})
	}
}

func TestAuthorizationDB_Get(t *testing.T) {
	ctx := testcontext.New(t)
	authDB, err := newTestAuthDB(ctx)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	defer func() {
		err := authDB.Close()
		assert.NoError(t, err)
		ctx.Cleanup()
	}()

	var expectedAuths Authorizations
	for i := 0; i < 5; i++ {
		expectedAuths = append(expectedAuths, &Authorization{
			Token: t1,
		})
	}
	authsBytes, err := expectedAuths.Marshal()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	err = authDB.DB.Put(storage.Key("user@example.com"), authsBytes)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	cases := []struct {
		testID,
		email string
		result Authorizations
	}{
		{
			"Non-existant email",
			"nouser@example.com",
			nil,
		},
		{
			"Exising email",
			"user@example.com",
			expectedAuths,
		},
	}

	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			auths, err := authDB.Get(c.email)
			assert.NoError(t, err)
			if c.result != nil {
				assert.NotEmpty(t, auths)
				assert.Len(t, auths, len(c.result))
			} else {
				assert.Empty(t, auths)
			}
		})
	}
}

func TestAuthorizationDB_Claim_Valid(t *testing.T) {
	ctx := testcontext.New(t)
	userID := "user@example.com"
	authDB, err := newTestAuthDB(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, authDB) {
		t.Fatal(err)
	}
	defer func() {
		err := authDB.Close()
		assert.NoError(t, err)
		ctx.Cleanup()
	}()

	auths, err := authDB.Create(userID, 1)
	if !assert.NoError(t, err) || !assert.NotEmpty(t, auths) {
		t.Fatal(err)
	}

	ident, err := testidentity.NewTestIdentity(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, ident) {
		t.Fatal(err)
	}

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
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	err = authDB.Claim(&ClaimOpts{
		Req:           req,
		Peer:          grpcPeer,
		ChainBytes:    [][]byte{ident.CA.Raw},
		MinDifficulty: difficulty,
	})
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	updatedAuths, err := authDB.Get(userID)
	if !assert.NoError(t, err) || !assert.NotEmpty(t, updatedAuths) {
		t.Fatal(err)
	}
	assert.Equal(t, auths[0].Token, updatedAuths[0].Token)

	if !assert.NotNil(t, updatedAuths[0].Claim) {
		t.FailNow()
	}
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
	userID := "user@example.com"
	claimedTime := int64(1000000)
	claimedAddr := "6.7.8.9:0"
	ident1, err := testidentity.NewTestIdentity(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, ident1) {
		t.Fatal(err)
	}
	claimedIdent := &provider.PeerIdentity{
		CA:   ident1.CA,
		Leaf: ident1.Leaf,
	}

	authDB, err := newTestAuthDB(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, authDB) {
		t.Fatal(err)
	}
	defer func() {
		err := authDB.Close()
		assert.NoError(t, err)
		ctx.Cleanup()
	}()

	auths, err := authDB.Create(userID, 2)
	if !assert.NoError(t, err) || !assert.NotEmpty(t, auths) {
		t.Fatal(err)
	}

	claimedIndex, unclaimedIndex := 0, 1

	auths[claimedIndex].Claim = &Claim{
		Timestamp:        claimedTime,
		Addr:             claimedAddr,
		Identity:         claimedIdent,
		SignedChainBytes: [][]byte{claimedIdent.CA.Raw},
	}
	err = authDB.put(userID, auths)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	ident2, err := testidentity.NewTestIdentity(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, ident2) {
		t.Fatal(err)
	}

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
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	t.Run("double claim", func(t *testing.T) {
		err = authDB.Claim(&ClaimOpts{
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

		updatedAuths, err := authDB.Get(userID)
		if !assert.NoError(t, err) || !assert.NotEmpty(t, updatedAuths) {
			t.Fatal(err)
		}
		assert.Equal(t, auths[claimedIndex].Token, updatedAuths[claimedIndex].Token)

		claim := updatedAuths[claimedIndex].Claim
		assert.Equal(t, claimedAddr, claim.Addr)
		assert.Equal(t, [][]byte{ident1.CA.Raw}, claim.SignedChainBytes)
		assert.Equal(t, claimedTime, claim.Timestamp)
	})

	t.Run("invalid timestamp", func(t *testing.T) {
		err = authDB.Claim(&ClaimOpts{
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

		updatedAuths, err := authDB.Get(userID)
		if !assert.NoError(t, err) || !assert.NotEmpty(t, updatedAuths) {
			t.Fatal(err)
		}

		assert.Equal(t, auths[unclaimedIndex].Token, updatedAuths[unclaimedIndex].Token)
		assert.Nil(t, updatedAuths[unclaimedIndex].Claim)
	})

	t.Run("invalid difficulty", func(t *testing.T) {
		err = authDB.Claim(&ClaimOpts{
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

		updatedAuths, err := authDB.Get(userID)
		if !assert.NoError(t, err) || !assert.NotEmpty(t, updatedAuths) {
			t.Fatal(err)
		}

		assert.Equal(t, auths[unclaimedIndex].Token, updatedAuths[unclaimedIndex].Token)
		assert.Nil(t, updatedAuths[unclaimedIndex].Claim)
	})
}

func TestNewAuthorization(t *testing.T) {
	userID := "user@example.com"
	auth, err := NewAuthorization(userID)
	assert.NoError(t, err)
	if !assert.NotNil(t, auth) {
		t.FailNow()
	}
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
	assert.NoError(t, err)
	assert.NotEmpty(t, authsBytes)

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
	assert.NoError(t, err)
	assert.NotEmpty(t, authsBytes)

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
	authDB, err := newTestAuthDB(ctx)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}
	defer func() {
		err = authDB.Close()
		assert.NoError(t, err)
		ctx.Cleanup()
	}()

	var authErrs utils.ErrorGroup
	for i := 0; i < 5; i++ {
		_, err := authDB.Create(fmt.Sprintf("user%d@example.com", i), 1)
		if err != nil {
			authErrs.Add(err)
		}
	}
	err = authErrs.Finish()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	userIDs, err := authDB.UserIDs()
	assert.NoError(t, err)
	assert.NotEmpty(t, userIDs)
}

func TestParseToken_Valid(t *testing.T) {
	userID := "user@example.com"
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
			"us" + tokenDelimiter + "er@example.com",
		},
	}

	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {
			b58Data := base58.CheckEncode(data[:], tokenVersion)
			tokenString := c.userID + tokenDelimiter + b58Data
			token, err := ParseToken(tokenString)

			assert.NoError(t, err)
			if !assert.NotNil(t, token) {
				t.FailNow()
			}
			assert.Equal(t, c.userID, token.UserID)
			assert.Equal(t, data[:], token.Data[:])
		})
	}
}

func TestParseToken_Invalid(t *testing.T) {
	userID := "user@example.com"
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
		t.Run(c.testID, func(t *testing.T) {
			token, err := ParseToken(c.tokenString)
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
	ctx := testcontext.New(t)
	tmp := ctx.Dir()
	defer ctx.Cleanup()
	caCert := filepath.Join(tmp, "ca.cert")
	caKey := filepath.Join(tmp, "ca.key")
	userID := "user@example.com"
	caSetupConfig := provider.CASetupConfig{
		CertPath: caCert,
		KeyPath:  caKey,
	}
	caConfig := provider.FullCAConfig{
		CertPath: caCert,
		KeyPath:  caKey,
	}
	config := CertServerConfig{
		AuthorizationDBURL: "bolt://" + filepath.Join(tmp, "authorizations.db"),
		CA:                 caConfig,
	}
	signingCA, err := caSetupConfig.Create(ctx)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	authDB, err := config.NewAuthDB()
	if !assert.NoError(t, err) || !assert.NotNil(t, authDB) {
		t.Fatal(err)
	}

	auths, err := authDB.Create("user@example.com", 1)
	if !assert.NoError(t, err) || !assert.NotEmpty(t, auths) {
		t.Fatal(err)
	}
	err = authDB.Close()
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	// TODO(bryanchriswhite): figure out why pregenerated
	//  identities change issuers when signed
	//
	//   Issuer: {
	//     Names: null => [],
	//     Organization: null => [],
	//   RawIssue": "MAA=" => "MBAxDjAMBgNVBAoTBVN0b3Jq",
	//------
	//serverIdent, err := idents.NewIdentity()
	//------
	serverCA, err := testidentity.NewTestCA(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, serverCA) {
		t.Fatal(err)
	}
	serverIdent, err := serverCA.NewIdentity()
	//------
	if !assert.NoError(t, err) || !assert.NotNil(t, serverIdent) {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) || !assert.NotNil(t, listener) {
		t.Fatal(err)
	}

	serverConfig := provider.ServerConfig{Address: listener.Addr().String()}
	opts, err := provider.NewServerOptions(serverIdent, serverConfig)
	if !assert.NoError(t, err) || !assert.NotNil(t, opts) {
		t.Fatal(err)
	}

	service, err := provider.NewProvider(opts, listener, nil, config)
	if !assert.NoError(t, err) || !assert.NotNil(t, service) {
		t.Fatal(err)
	}

	ctx.Go(func() error {
		err := service.Run(ctx)
		assert.NoError(t, err)
		return err
	})
	defer func() {
		err := service.Close()
		assert.NoError(t, err)
	}()

	// TODO(bryanchriswhite): figure out why pregenerated
	//  identities change issuers when signed
	//
	//   Issuer: {
	//     Names: null => [],
	//     Organization: null => [],
	//   RawIssue": "MAA=" => "MBAxDjAMBgNVBAoTBVN0b3Jq",
	//------
	//clientIdent, err := idents.NewIdentity()
	//------
	clientCA, err := testidentity.NewTestCA(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, clientCA) {
		t.Fatal(err)
	}
	clientIdent, err := clientCA.NewIdentity()
	//------
	if !assert.NoError(t, err) || !assert.NotNil(t, clientIdent) {
		t.Fatal(err)
	}

	client, err := NewClient(ctx, clientIdent, listener.Addr().String())
	if !assert.NoError(t, err) || !assert.NotNil(t, client) {
		t.Fatal(err)
	}

	signedChainBytes, err := client.Sign(ctx, auths[0].Token.String())
	if !assert.NoError(t, err) || !assert.NotEmpty(t, signedChainBytes) {
		t.Fatal(err)
	}

	signedChain, err := identity.ParseCertChain(signedChainBytes)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	assert.Equal(t, clientIdent.CA.RawTBSCertificate, signedChain[0].RawTBSCertificate)
	assert.Equal(t, signingCA.Cert.Raw, signedChainBytes[1])
	// TODO: test scenario with rest chain
	//assert.Equal(t, signingCA.RestChainRaw(), signedChainBytes[1:])

	err = signedChain[0].CheckSignatureFrom(signingCA.Cert)
	assert.NoError(t, err)

	err = service.Close()
	assert.NoError(t, err)

	// NB: re-open after closing for server
	authDB, err = config.NewAuthDB()
	if !assert.NoError(t, err) || !assert.NotNil(t, authDB) {
		t.Fatal(err)
	}
	defer func() {
		err := authDB.Close()
		assert.NoError(t, err)
	}()

	updatedAuths, err := authDB.Get(userID)
	if !assert.NoError(t, err) ||
		!assert.NotEmpty(t, updatedAuths) ||
		!assert.NotNil(t, updatedAuths[0].Claim) {
		t.Fatal(err)
	}

	now := time.Now().Unix()
	claim := updatedAuths[0].Claim
	assert.Equal(t,
		strings.Split(listener.Addr().String(), ":")[0],
		strings.Split(claim.Addr, ":")[0])
	assert.Equal(t, signedChainBytes, claim.SignedChainBytes)
	assert.Condition(t, func() bool {
		return now-10 < claim.Timestamp &&
			claim.Timestamp < now+10
	})
}

func TestNewClient(t *testing.T) {
	ctx := testcontext.New(t)
	ident, err := idents.NewIdentity()
	if !assert.NoError(t, err) || !assert.NotNil(t, ident) {
		t.Fatal(err)
	}

	client, err := NewClient(ctx, ident, "")
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestNewClientFrom(t *testing.T) {
	ctx := testcontext.New(t)
	ident, err := idents.NewIdentity()
	if !assert.NoError(t, err) || !assert.NotNil(t, ident) {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) || !assert.NotNil(t, listener) {
		t.Fatal(err)
	}

	tc := transport.NewClient(ident)
	conn, err := tc.DialAddress(ctx, listener.Addr().String())
	if !assert.NoError(t, err) || !assert.NotNil(t, conn) {
		t.Fatal(err)
	}

	pbClient := pb.NewCertificatesClient(conn)
	if !assert.NotNil(t, pbClient) {
		t.FailNow()
	}

	client, err := NewClientFrom(pbClient)
	assert.NoError(t, err)
	assert.NotNil(t, client)
}

func TestCertificateSigner_Sign(t *testing.T) {
	ctx := testcontext.New(t)
	tmp := ctx.Dir()
	defer ctx.Cleanup()
	caCert := filepath.Join(tmp, "ca.cert")
	caKey := filepath.Join(tmp, "ca.key")
	userID := "user@example.com"
	caSetupConfig := provider.CASetupConfig{
		CertPath: caCert,
		KeyPath:  caKey,
	}
	config := CertServerConfig{
		AuthorizationDBURL: "bolt://" + filepath.Join(tmp, "authorizations.db"),
	}
	signingCA, err := caSetupConfig.Create(ctx)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	authDB, err := config.NewAuthDB()
	if !assert.NoError(t, err) || !assert.NotNil(t, authDB) {
		t.Fatal(err)
	}
	defer func() {
		err := authDB.Close()
		assert.NoError(t, err)
	}()

	auths, err := authDB.Create(userID, 1)
	if !assert.NoError(t, err) || !assert.NotEmpty(t, auths) {
		t.Fatal(err)
	}

	// TODO(bryanchriswhite): figure out why pregenerated
	//  identities change issuers when signed
	//
	//   Issuer: {
	//     Names: null => [],
	//     Organization: null => [],
	//   RawIssue": "MAA=" => "MBAxDjAMBgNVBAoTBVN0b3Jq",
	//------
	//clientIdent, err := idents.NewIdentity()
	//------
	clientCA, err := testidentity.NewTestCA(ctx)
	if !assert.NoError(t, err) || !assert.NotNil(t, clientCA) {
		t.Fatal(err)
	}
	clientIdent, err := clientCA.NewIdentity()
	//------
	if !assert.NoError(t, err) || !assert.NotNil(t, clientIdent) {
		t.Fatal(err)
	}

	expectedAddr := &net.TCPAddr{
		IP:   net.ParseIP("1.2.3.4"),
		Port: 5,
	}
	grpcPeer := &peer.Peer{
		Addr: expectedAddr,
		AuthInfo: credentials.TLSInfo{
			State: tls.ConnectionState{
				PeerCertificates: []*x509.Certificate{clientIdent.Leaf, clientIdent.CA},
			},
		},
	}
	peerCtx := peer.NewContext(ctx, grpcPeer)

	certSigner := &CertificateSigner{
		Log:    zap.L(),
		Signer: signingCA,
		AuthDB: authDB,
	}
	req := pb.SigningRequest{
		Timestamp: time.Now().Unix(),
		AuthToken: auths[0].Token.String(),
	}
	res, err := certSigner.Sign(peerCtx, &req)
	if !assert.NoError(t, err) || !assert.NotNil(t, res) || !assert.NotEmpty(t, res.Chain) {
		t.Fatal(err)
	}

	signedChain, err := identity.ParseCertChain(res.Chain)
	if !assert.NoError(t, err) {
		t.Fatal(err)
	}

	assert.Equal(t, clientIdent.CA.RawTBSCertificate, signedChain[0].RawTBSCertificate)
	assert.Equal(t, signingCA.Cert.Raw, signedChain[1].Raw)
	// TODO: test scenario with rest chain
	//assert.Equal(t, signingCA.RestChainRaw(), res.Chain[1:])

	err = signedChain[0].CheckSignatureFrom(signingCA.Cert)
	assert.NoError(t, err)

	updatedAuths, err := authDB.Get(userID)
	if !assert.NoError(t, err) ||
		!assert.NotEmpty(t, updatedAuths) ||
		!assert.NotNil(t, updatedAuths[0].Claim) {
		t.Fatal(err)
	}

	now := time.Now().Unix()
	claim := updatedAuths[0].Claim
	assert.Equal(t, expectedAddr.String(), claim.Addr)
	assert.Equal(t, res.Chain, claim.SignedChainBytes)
	assert.Condition(t, func() bool {
		return now-MaxClaimDelaySeconds < claim.Timestamp &&
			claim.Timestamp < now+MaxClaimDelaySeconds
	})
}

func newTestAuthDB(ctx *testcontext.Context) (*AuthorizationDB, error) {
	dbPath := "bolt://" + filepath.Join(ctx.Dir(), "authorizations.db")
	config := CertServerConfig{
		AuthorizationDBURL: dbPath,
	}
	return config.NewAuthDB()
}
