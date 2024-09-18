// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package nodetag

import (
	"bytes"
	"context"

	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/common/signing"
)

var (
	// SignatureErr means that the signature wash wrong.
	SignatureErr = errs.Class("invalid signature")

	// SerializationErr is returned when the tags are signed, but the payload couldn't be unmarshalled.
	SerializationErr = errs.Class("invalid tag serialization")

	// WrongSignee is returned when the tags are signed, but the signee field has a different NodeID.
	WrongSignee = errs.Class("node id mismatch")
)

// Sign create a signed tag set from a raw one.
func Sign(ctx context.Context, tagSet *pb.NodeTagSet, signer signing.Signer) (*pb.SignedNodeTagSet, error) {
	signed := &pb.SignedNodeTagSet{}
	raw, err := pb.Marshal(tagSet)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	signature, err := signer.HashAndSign(ctx, raw)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	signed.Signature = signature
	signed.SignerNodeId = signer.ID().Bytes()
	signed.SerializedTag = raw
	return signed, nil
}

// Verify checks the signature of a signed tag set.
func Verify(ctx context.Context, tags *pb.SignedNodeTagSet, signee signing.Signee) (*pb.NodeTagSet, error) {
	if !bytes.Equal(tags.SignerNodeId, signee.ID().Bytes()) {
		return nil, WrongSignee.New("wrong signee to verify")
	}
	err := signee.HashAndVerifySignature(ctx, tags.SerializedTag, tags.Signature)
	if err != nil {
		return nil, SignatureErr.Wrap(err)
	}
	tagset := &pb.NodeTagSet{}
	err = pb.Unmarshal(tags.SerializedTag, tagset)
	if err != nil {
		return nil, SerializationErr.Wrap(err)
	}
	return tagset, nil
}
