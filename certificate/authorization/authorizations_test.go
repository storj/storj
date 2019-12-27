// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package authorization

import (
	"bytes"
	"encoding/gob"
	"net"
	"testing"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/peertls/tlsopts"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/certificate/certificateclient"
)

var (
	t1 = Token{
		UserID: "user@mail.test",
		Data:   [tokenDataLength]byte{1, 2, 3},
	}
	t2 = Token{
		UserID: "user2@mail.test",
		Data:   [tokenDataLength]byte{4, 5, 6},
	}
)

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
	expectedAuths := Group{
		{Token: t1},
		{Token: t2},
	}

	authsBytes, err := expectedAuths.Marshal()
	require.NoError(t, err)
	require.NotEmpty(t, authsBytes)

	var actualAuths Group
	decoder := gob.NewDecoder(bytes.NewBuffer(authsBytes))
	err = decoder.Decode(&actualAuths)
	assert.NoError(t, err)
	assert.NotNil(t, actualAuths)
	assert.Equal(t, expectedAuths, actualAuths)
}

func TestAuthorizations_Unmarshal(t *testing.T) {
	expectedAuths := Group{
		{Token: t1},
		{Token: t2},
	}

	authsBytes, err := expectedAuths.Marshal()
	require.NoError(t, err)
	require.NotEmpty(t, authsBytes)

	var actualAuths Group
	err = actualAuths.Unmarshal(authsBytes)
	assert.NoError(t, err)
	assert.NotNil(t, actualAuths)
	assert.Equal(t, expectedAuths, actualAuths)
}

func TestAuthorizations_Group(t *testing.T) {
	auths := make(Group, 10)
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

	claimed, open := auths.GroupByClaimed()
	for _, a := range claimed {
		assert.NotNil(t, a.Claim)
	}
	for _, a := range open {
		assert.Nil(t, a.Claim)
	}
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

func TestNewClient(t *testing.T) {
	t.Skip("needs proper grpc listener to work")

	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	ident, err := testidentity.PregeneratedIdentity(0, storj.LatestIDVersion())
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

	tlsOptions, err := tlsopts.NewOptions(ident, tlsopts.Config{}, nil)
	require.NoError(t, err)

	dialer := rpc.NewDefaultDialer(tlsOptions)

	t.Run("Basic", func(t *testing.T) {
		client, err := certificateclient.New(ctx, dialer, listener.Addr().String())
		assert.NoError(t, err)
		assert.NotNil(t, client)

		defer ctx.Check(client.Close)
	})

	t.Run("ClientFrom", func(t *testing.T) {
		conn, err := dialer.DialAddressInsecure(ctx, listener.Addr().String())
		require.NoError(t, err)
		require.NotNil(t, conn)

		defer ctx.Check(conn.Close)

		client := certificateclient.NewClientFrom(pb.NewDRPCCertificatesClient(conn.Raw()))
		assert.NoError(t, err)
		assert.NotNil(t, client)

		defer ctx.Check(client.Close)
	})
}
