// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/identity"
	"storj.io/common/pb"
	"storj.io/common/signing"
	"storj.io/storj/shared/nodetag"
)

// GetTags returns with all the node tags including pre-defined and self-signed tags.
func GetTags(ctx context.Context, config Config, identity *identity.FullIdentity) (*pb.SignedNodeTagSets, error) {
	tags := pb.SignedNodeTagSets(config.Tags)
	if len(config.SelfSignedTags) > 0 {
		signer := signing.SignerFromFullIdentity(identity)
		tagSet := &pb.NodeTagSet{
			NodeId:   identity.ID.Bytes(),
			SignedAt: time.Now().Unix(),
		}
		for _, tag := range config.SelfSignedTags {
			key, value, ok := strings.Cut(tag, "=")
			if !ok {
				return nil, errs.New("Self signed tags should be in the format of key=value")
			}
			tagSet.Tags = append(tagSet.Tags, &pb.Tag{
				Name:  key,
				Value: []byte(value),
			})
		}
		signed, err := nodetag.Sign(ctx, tagSet, signer)
		if err != nil {
			return nil, err
		}
		tags.Tags = append(tags.Tags, signed)
	}
	return &tags, nil
}
