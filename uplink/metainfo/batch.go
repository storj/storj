// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"github.com/zeebo/errs"

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
func (batch *Batch) Send(ctx context.Context) (responses []Response, err error) {
	response, err := batch.client.Batch(ctx, &pb.BatchRequest{
		Requests: batch.requests,
	})
	if err != nil {
		return []Response{}, err
	}

	responses = make([]Response, len(response.Responses))
	for i, response := range response.Responses {
		responses[i] = Response{
			pbResponse: response.Response,
		}
	}

	return responses, nil
}

// Response TODO
type Response struct {
	pbResponse interface{}
}

// CreateBucket TODO
func (resp *Response) CreateBucket() (CreateBucketResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketCreate)
	if !ok {
		return CreateBucketResponse{}, errs.New("invalid response type")
	}
	return newCreateBucketResponse(item.BucketCreate), nil
}

// GetBucket TODO
func (resp *Response) GetBucket() (GetBucketResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketGet)
	if !ok {
		return GetBucketResponse{}, errs.New("invalid response type")
	}
	return newGetBucketResponse(item.BucketGet), nil
}

// ListBuckets TODO
func (resp *Response) ListBuckets() (ListBucketsResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketList)
	if !ok {
		return ListBucketsResponse{}, errs.New("invalid response type")
	}
	return newListBucketsResponse(item.BucketList), nil
}
