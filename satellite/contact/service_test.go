// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"testing"

	"github.com/stretchr/testify/require"

	"storj.io/common/identity/testidentity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/common/storj"
	"storj.io/common/testcontext"
	"storj.io/storj/shared/nodetag"
)

func TestVerifyTags(t *testing.T) {
	ctx := testcontext.New(t)
	snIdentity := testidentity.MustPregeneratedIdentity(0, storj.LatestIDVersion())
	signerIdentity := testidentity.MustPregeneratedIdentity(1, storj.LatestIDVersion())
	signer := signing.SignerFromFullIdentity(signerIdentity)
	authority := nodetag.Authority{
		signing.SignerFromFullIdentity(signerIdentity),
	}
	t.Run("ok tags", func(t *testing.T) {
		tags, err := nodetag.Sign(ctx, &pb.NodeTagSet{
			NodeId: snIdentity.ID.Bytes(),
			Tags: []*pb.Tag{
				{
					Name:  "foo",
					Value: []byte("bar"),
				},
			},
		}, signer)
		require.NoError(t, err)

		verifiedTags, signerID, err := verifyTags(ctx, authority, snIdentity.ID, tags)
		require.NoError(t, err)

		require.Equal(t, signerIdentity.ID, signerID)
		require.Len(t, verifiedTags.Tags, 1)
		require.Equal(t, "foo", verifiedTags.Tags[0].Name)
		require.Equal(t, []byte("bar"), verifiedTags.Tags[0].Value)
	})

	t.Run("wrong signer ID", func(t *testing.T) {
		tags, err := nodetag.Sign(ctx, &pb.NodeTagSet{
			NodeId: snIdentity.ID.Bytes(),
			Tags: []*pb.Tag{
				{
					Name:  "foo",
					Value: []byte("bar"),
				},
			},
		}, signer)
		require.NoError(t, err)
		tags.SignerNodeId = []byte{1, 2, 3, 4}

		_, _, err = verifyTags(ctx, authority, snIdentity.ID, tags)

		require.Error(t, err)
		require.ErrorContains(t, err, "01020304")
		require.ErrorContains(t, err, "failed to parse signerNodeID")

	})

	t.Run("wrong signature", func(t *testing.T) {
		tags, err := nodetag.Sign(ctx, &pb.NodeTagSet{
			NodeId: snIdentity.ID.Bytes(),
			Tags: []*pb.Tag{
				{
					Name:  "foo",
					Value: []byte("bar"),
				},
			},
		}, signer)
		require.NoError(t, err)
		tags.Signature = []byte{4, 3, 2, 1}

		_, _, err = verifyTags(ctx, authority, snIdentity.ID, tags)

		require.Error(t, err)
		require.ErrorContains(t, err, "04030201")
		require.ErrorContains(t, err, "wrong/unknown signature")
	})

	t.Run("unknown signer", func(t *testing.T) {
		otherSignerIdentity := testidentity.MustPregeneratedIdentity(2, storj.LatestIDVersion())
		otherSigner := signing.SignerFromFullIdentity(otherSignerIdentity)

		tags, err := nodetag.Sign(ctx, &pb.NodeTagSet{
			NodeId: snIdentity.ID.Bytes(),
			Tags: []*pb.Tag{
				{
					Name:  "foo",
					Value: []byte("bar"),
				},
			},
		}, otherSigner)
		require.NoError(t, err)

		_, _, err = verifyTags(ctx, authority, snIdentity.ID, tags)

		require.Error(t, err)
		require.ErrorContains(t, err, "wrong/unknown signature")
	})

	t.Run("signed for different node", func(t *testing.T) {
		otherNodeID := testidentity.MustPregeneratedIdentity(3, storj.LatestIDVersion()).ID
		tags, err := nodetag.Sign(ctx, &pb.NodeTagSet{
			NodeId: otherNodeID.Bytes(),
			Tags: []*pb.Tag{
				{
					Name:  "foo",
					Value: []byte("bar"),
				},
			},
		}, signer)
		require.NoError(t, err)

		_, _, err = verifyTags(ctx, authority, snIdentity.ID, tags)

		require.Error(t, err)
		require.ErrorContains(t, err, snIdentity.ID.String())
		require.ErrorContains(t, err, "the tag is signed for a different node")
	})

	t.Run("wrong NodeID", func(t *testing.T) {
		tags, err := nodetag.Sign(ctx, &pb.NodeTagSet{
			NodeId: []byte{4, 4, 4},
			Tags: []*pb.Tag{
				{
					Name:  "foo",
					Value: []byte("bar"),
				},
			},
		}, signer)
		require.NoError(t, err)

		_, _, err = verifyTags(ctx, authority, snIdentity.ID, tags)

		require.Error(t, err)
		require.ErrorContains(t, err, "040404")
		require.ErrorContains(t, err, "failed to parse nodeID")
	})

}
