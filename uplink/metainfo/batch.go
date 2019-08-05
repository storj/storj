// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
)

var (
	ErrInvalidType = errs.New("invalid response type")
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

// AddSetBucketAttribution TODO
func (batch *Batch) AddSetBucketAttribution(params SetBucketAttributionParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketSetAttribution{
			BucketSetAttribution: params.toRequest(),
		},
	})
}

// AddBeginObject TODO
func (batch *Batch) AddBeginObject(params BeginObjectParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectBegin{
			ObjectBegin: params.toRequest(),
		},
	})
}

// AddCommitObject TODO
func (batch *Batch) AddCommitObject(params CommitObjectParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectCommit{
			ObjectCommit: params.toRequest(),
		},
	})
}

// AddGetObject TODO
func (batch *Batch) AddGetObject(params GetObjectParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectGet{
			ObjectGet: params.toRequest(),
		},
	})
}

// AddListObjects TODO
func (batch *Batch) AddListObjects(params ListObjectsParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectList{
			ObjectList: params.toRequest(),
		},
	})
}

// AddBeginDeleteObject TODO
func (batch *Batch) AddBeginDeleteObject(params BeginDeleteObjectParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectBeginDelete{
			ObjectBeginDelete: params.toRequest(),
		},
	})
}

// AddFinishDeleteObject TODO
func (batch *Batch) AddFinishDeleteObject(params FinishDeleteObjectParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectFinishDelete{
			ObjectFinishDelete: params.toRequest(),
		},
	})
}

// AddBeginSegment TODO
func (batch *Batch) AddBeginSegment(params BeginSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentBegin{
			SegmentBegin: params.toRequest(),
		},
	})
}

// AddCommitSegment TODO
func (batch *Batch) AddCommitSegment(params CommitSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentCommit{
			SegmentCommit: params.toRequest(),
		},
	})
}

// AddListSegments TODO
func (batch *Batch) AddListSegments(params ListSegmentsParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentList{
			SegmentList: params.toRequest(),
		},
	})
}

// AddMakeInlineSegment TODO
func (batch *Batch) AddMakeInlineSegment(params MakeInlineSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentMakeInline{
			SegmentMakeInline: params.toRequest(),
		},
	})
}

// AddDownloadSegment TODO
func (batch *Batch) AddDownloadSegment(params DownloadSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentDownload{
			SegmentDownload: params.toRequest(),
		},
	})
}

// AddBeginDeleteSegment TODO
func (batch *Batch) AddBeginDeleteSegment(params BeginDeleteSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentBeginDelete{
			SegmentBeginDelete: params.toRequest(),
		},
	})
}

// AddFinishDeleteSegment TODO
func (batch *Batch) AddFinishDeleteSegment(params FinishDeleteSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentFinishDelete{
			SegmentFinishDelete: params.toRequest(),
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
		return CreateBucketResponse{}, ErrInvalidType
	}
	return newCreateBucketResponse(item.BucketCreate), nil
}

// GetBucket TODO
func (resp *Response) GetBucket() (GetBucketResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketGet)
	if !ok {
		return GetBucketResponse{}, ErrInvalidType
	}
	return newGetBucketResponse(item.BucketGet), nil
}

// ListBuckets TODO
func (resp *Response) ListBuckets() (ListBucketsResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketList)
	if !ok {
		return ListBucketsResponse{}, ErrInvalidType
	}
	return newListBucketsResponse(item.BucketList), nil
}

// ListSegment TODO
func (resp *Response) ListSegment() (ListSegmentsResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_SegmentList)
	if !ok {
		return ListSegmentsResponse{}, ErrInvalidType
	}
	return newListSegmentsResponse(item.SegmentList), nil
}

// GetObject TODO
func (resp *Response) GetObject() (GetObjectResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_ObjectGet)
	if !ok {
		return GetObjectResponse{}, ErrInvalidType
	}
	return newGetObjectResponse(item.ObjectGet), nil
}

// DownloadSegment TODO
func (resp *Response) DownloadSegment() (DownloadSegmentResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_SegmentDownload)
	if !ok {
		return DownloadSegmentResponse{}, ErrInvalidType
	}
	return newDownloadSegmentResponse(item.SegmentDownload), nil
}
