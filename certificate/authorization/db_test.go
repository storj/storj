// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/rpc/rpcpeer"
	"storj.io/common/testcontext"
	"storj.io/storj/storage"
)

func TestNewDB(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	dbURL := "bolt://" + ctx.File("authorizations.db")
	db, err := NewDB(dbURL, false)
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	require.NotNil(t, db)
	require.NotNil(t, db.db)
}

func TestNewDBFromCfg(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	db, err := NewDBFromCfg(DBConfig{
		URL:       "bolt://" + ctx.File("authorizations.db"),
		Overwrite: false,
	})
	require.NoError(t, err)
	defer ctx.Check(db.Close)

	require.NotNil(t, db)
	require.NotNil(t, db.db)
}

func TestAuthorizationDB_Create(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB := newTestAuthDB(t, ctx)
	defer ctx.Check(authDB.Close)

	cases := []struct {
		testID,
		email string
		startCount,
		incCount,
		newCount,
		endCount int
	}{
		{
			"first authorization",
			"user1@mail.test",
			0, 1, 1, 1,
		},
		{
			"second authorization",
			"user1@mail.test",
			1, 2, 2, 3,
		},
		{
			"large authorization",
			"user2@mail.test",
			0, 5, 5, 5,
		},
	}

	for _, c := range cases {
		testCase := c
		t.Run(c.testID, func(t *testing.T) {
			emailKey := storage.Key(testCase.email)

			if testCase.startCount == 0 {
				_, err := authDB.db.Get(ctx, emailKey)
				require.Error(t, err, ErrNotFound)
			} else {
				v, err := authDB.db.Get(ctx, emailKey)
				require.NoError(t, err)
				require.NotEmpty(t, v)

				var existingAuths Group
				err = existingAuths.Unmarshal(v)
				require.NoError(t, err)
				require.Len(t, existingAuths, testCase.startCount)
			}

			expectedAuths, err := authDB.Create(ctx, testCase.email, testCase.incCount)
			require.NoError(t, err)
			require.Len(t, expectedAuths, testCase.newCount)

			v, err := authDB.db.Get(ctx, emailKey)
			require.NoError(t, err)
			require.NotEmpty(t, v)

			var actualAuths Group
			err = actualAuths.Unmarshal(v)
			require.NoError(t, err)
			require.Len(t, actualAuths, testCase.endCount)
		})
	}
}

func TestAuthorizationDB_Create_error(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB := newTestAuthDB(t, ctx)
	defer ctx.Check(authDB.Close)

	cases := []struct {
		testID,
		email string
		count    int
		errClass *errs.Class
		err      error
	}{
		{
			"empty userID",
			"", 1,
			&ErrDB,
			ErrEmptyUserID,
		},
		{
			"negative count",
			"user@mail.test", -1,
			&ErrDB,
			ErrCount,
		},
	}

	for _, c := range cases {
		testCase := c
		t.Run(c.testID, func(t *testing.T) {
			auths, err := authDB.Create(ctx, testCase.email, testCase.count)
			assert.Truef(t, testCase.errClass.Has(err), "error: %s", err)
			assert.Equal(t, testCase.err, err)
			assert.Empty(t, auths)
		})
	}
}

func TestAuthorizationDB_Get(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB := newTestAuthDB(t, ctx)
	defer ctx.Check(authDB.Close)

	var expectedAuths Group
	for i := 0; i < 5; i++ {
		expectedAuths = append(expectedAuths, &Authorization{
			Token: t1,
		})
	}

	authsBytes, err := expectedAuths.Marshal()
	require.NoError(t, err)

	err = authDB.db.Put(ctx, storage.Key("user@mail.test"), authsBytes)
	require.NoError(t, err)

	{
		t.Log("Non-existent email")
		auths, err := authDB.Get(ctx, "nouser@mail.test")
		require.Error(t, err, ErrNotFound)
		require.Empty(t, auths)
	}

	{
		t.Log("Existing email")
		auths, err := authDB.Get(ctx, "user@mail.test")
		require.NoError(t, err)
		assert.NotEmpty(t, auths)
		assert.Len(t, auths, len(expectedAuths))
		assert.Equal(t, expectedAuths, auths)
	}
}

func TestAuthorizationDB_Claim_Valid(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB := newTestAuthDB(t, ctx)
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
	peer := &rpcpeer.Peer{
		Addr: addr,
		State: tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{ident.Leaf, ident.CA},
		},
	}

	now := time.Now()
	req := &pb.SigningRequest{
		AuthToken: auths[0].Token.String(),
		Timestamp: now.Unix(),
	}
	difficulty, err := ident.ID.Difficulty()
	require.NoError(t, err)

	err = authDB.Claim(ctx, &ClaimOpts{
		Req:           req,
		Peer:          peer,
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
	assert.Equal(t, peer.Addr.String(), claim.Addr)
	assert.Equal(t, [][]byte{ident.CA.Raw}, claim.SignedChainBytes)

	claimTime := time.Unix(claim.Timestamp, 0)
	assert.Condition(t, func() bool {
		return now.Sub(claimTime) < MaxClockSkew &&
			claimTime.Sub(now) < MaxClockSkew
	})
}

func TestAuthorizationDB_Claim_Invalid(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB := newTestAuthDB(t, ctx)
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
	peer := &rpcpeer.Peer{
		Addr: addr,
		State: tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{ident2.Leaf, ident2.CA},
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
			Peer:          peer,
			ChainBytes:    [][]byte{ident2.CA.Raw},
			MinDifficulty: difficulty2,
		})
		if assert.Error(t, err) {
			assert.True(t, ErrAlreadyClaimed.Has(err))
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
			Peer:          peer,
			ChainBytes:    [][]byte{ident2.CA.Raw},
			MinDifficulty: difficulty2,
		})
		if assert.Error(t, err) {
			assert.True(t, ErrInvalidClaim.Has(err))
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
			Peer:          peer,
			ChainBytes:    [][]byte{ident2.CA.Raw},
			MinDifficulty: difficulty2 + 1,
		})
		if assert.Error(t, err) {
			assert.True(t, ErrInvalidClaim.Has(err))
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

func TestAuthorizationDB_Emails(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	authDB := newTestAuthDB(t, ctx)
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

func newTestAuthDB(t *testing.T, ctx *testcontext.Context) *DB {
	dbURL := "bolt://" + ctx.File("authorizations.db")
	db, err := NewDB(dbURL, false)
	require.NoError(t, err)
	return db
}
