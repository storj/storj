// Copyright (C) 2025 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"github.com/zeebo/errs"

	"storj.io/common/pb"
	"storj.io/storj/satellite/buckets"
	internalpb "storj.io/storj/satellite/internalpb"
)

func encodeBucketTags(tags []buckets.Tag) ([]byte, error) {
	if len(tags) == 0 {
		return nil, nil
	}

	pbTags := &internalpb.BucketTags{
		Tags: make([]*pb.BucketTag, 0, len(tags)),
	}
	for _, tag := range tags {
		pbTags.Tags = append(pbTags.Tags, &pb.BucketTag{
			Key:   []byte(tag.Key),
			Value: []byte(tag.Value),
		})
	}

	encodedTags, err := pb.Marshal(pbTags)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return encodedTags, nil
}

func decodeBucketTags(tagsBytes []byte) ([]buckets.Tag, error) {
	if len(tagsBytes) == 0 {
		return nil, nil
	}

	pbTags := &internalpb.BucketTags{}
	if err := pb.Unmarshal(tagsBytes, pbTags); err != nil {
		return nil, errs.Wrap(err)
	}

	tags := make([]buckets.Tag, 0, len(pbTags.Tags))
	for _, pbTag := range pbTags.Tags {
		tags = append(tags, buckets.Tag{
			Key:   string(pbTag.Key),
			Value: string(pbTag.Value),
		})
	}

	return tags, nil
}
