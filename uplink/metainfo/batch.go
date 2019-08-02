// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"storj.io/storj/pkg/pb"
)

// Batch TODO
type Batch struct {
	client   pb.MetainfoClient
	requests []*pb.BatchRequestItem
}

// AddCreateBucket TODO
func (batch *Batch) AddCreateBucket(params CreateBucketParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketCreate{
			BucketCreate: params.toRequest(),
		},
	})
}

// AddGetBucket TODO
func (batch *Batch) AddGetBucket(params GetBucketParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketGet{
			BucketGet: params.toRequest(),
		},
	})
}

// AddDeleteBucket TODO
func (batch *Batch) AddDeleteBucket(params DeleteBucketParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketDelete{
			BucketDelete: params.toRequest(),
		},
	})
}

// AddListBuckets TODO
func (batch *Batch) AddListBuckets(params ListBucketsParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketList{
			BucketList: params.toRequest(),
		},
	})
}

// Send TODO
func (batch *Batch) Send(ctx context.Context) error {
	_, err := batch.client.Batch(ctx, &pb.BatchRequest{
		Requests: batch.requests,
	})
	if err != nil {
		return err
	}
	return nil
}
