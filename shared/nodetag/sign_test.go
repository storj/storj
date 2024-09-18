// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodetag

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
)

func TestSigning(t *testing.T) {
	dss := testidentity.MustPregeneratedSignedIdentity(0, storj.LatestIDVersion())
	nodeIdentity := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())

	tagSet := &pb.NodeTagSet{
		NodeId: nodeIdentity.ID.Bytes(),
		Tags: []*pb.Tag{
			{
				Name:  "foo",
				Value: []byte("bar"),
			},
		},
	}
	ctx := testcontext.New(t)

	t.Run("good signature", func(t *testing.T) {
		signed, err := Sign(ctx, tagSet, signing.SignerFromFullIdentity(dss))
		require.NoError(t, err)

		verified, err := Verify(ctx, signed, signing.SigneeFromPeerIdentity(dss.PeerIdentity()))
		require.NoError(t, err)

		require.Len(t, verified.Tags, 1)
		require.Equal(t, "foo", verified.Tags[0].Name)
		require.Equal(t, []byte("bar"), verified.Tags[0].Value)
	})

	t.Run("bad signature", func(t *testing.T) {
		signed, err := Sign(ctx, tagSet, signing.SignerFromFullIdentity(dss))
		require.NoError(t, err)

		signed.Signature = []byte{1, 2, 3}

		_, err = Verify(ctx, signed, signing.SigneeFromPeerIdentity(dss.PeerIdentity()))
		require.Error(t, err)
	})

	t.Run("signed by other key", func(t *testing.T) {
		otherDss := testidentity.MustPregeneratedSignedIdentity(2, storj.LatestIDVersion())
		signed, err := Sign(ctx, tagSet, signing.SignerFromFullIdentity(otherDss))
		require.NoError(t, err)

		_, err = Verify(ctx, signed, signing.SigneeFromPeerIdentity(dss.PeerIdentity()))
		require.Error(t, err)
	})

	t.Run("signed by wrong peer", func(t *testing.T) {
		otherDss := testidentity.MustPregeneratedSignedIdentity(1, storj.LatestIDVersion())
		signed, err := Sign(ctx, tagSet, signing.SignerFromFullIdentity(otherDss))
		require.NoError(t, err)

		signed.SignerNodeId = testidentity.MustPregeneratedSignedIdentity(4, storj.LatestIDVersion()).ID.Bytes()

		_, err = Verify(ctx, signed, signing.SigneeFromPeerIdentity(dss.PeerIdentity()))
		require.Error(t, err)
	})

}
