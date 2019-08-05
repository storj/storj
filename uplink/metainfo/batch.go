// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
)

var (
	// ErrInvalidType error for inalid response type casting
	ErrInvalidType = errs.New("invalid response type")
)

// Batch represents sending requests in batch
type Batch struct {
	client   pb.MetainfoClient
	requests []*pb.BatchRequestItem
}

// AddCreateBucket adds request to batch
func (batch *Batch) AddCreateBucket(params CreateBucketParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketCreate{
			BucketCreate: params.toRequest(),
		},
	})
}

// AddGetBucket adds request to batch
func (batch *Batch) AddGetBucket(params GetBucketParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketGet{
			BucketGet: params.toRequest(),
		},
	})
}

// AddDeleteBucket adds request to batch
func (batch *Batch) AddDeleteBucket(params DeleteBucketParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketDelete{
			BucketDelete: params.toRequest(),
		},
	})
}

// AddListBuckets adds request to batch
func (batch *Batch) AddListBuckets(params ListBucketsParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketList{
			BucketList: params.toRequest(),
		},
	})
}

// AddSetBucketAttribution adds request to batch
func (batch *Batch) AddSetBucketAttribution(params SetBucketAttributionParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_BucketSetAttribution{
			BucketSetAttribution: params.toRequest(),
		},
	})
}

// AddBeginObject adds request to batch
func (batch *Batch) AddBeginObject(params BeginObjectParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectBegin{
			ObjectBegin: params.toRequest(),
		},
	})
}

// AddCommitObject adds request to batch
func (batch *Batch) AddCommitObject(params CommitObjectParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectCommit{
			ObjectCommit: params.toRequest(),
		},
	})
}

// AddGetObject adds request to batch
func (batch *Batch) AddGetObject(params GetObjectParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectGet{
			ObjectGet: params.toRequest(),
		},
	})
}

// AddListObjects adds request to batch
func (batch *Batch) AddListObjects(params ListObjectsParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectList{
			ObjectList: params.toRequest(),
		},
	})
}

// AddBeginDeleteObject adds request to batch
func (batch *Batch) AddBeginDeleteObject(params BeginDeleteObjectParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectBeginDelete{
			ObjectBeginDelete: params.toRequest(),
		},
	})
}

// AddFinishDeleteObject adds request to batch
func (batch *Batch) AddFinishDeleteObject(params FinishDeleteObjectParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_ObjectFinishDelete{
			ObjectFinishDelete: params.toRequest(),
		},
	})
}

// AddBeginSegment adds request to batch
func (batch *Batch) AddBeginSegment(params BeginSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentBegin{
			SegmentBegin: params.toRequest(),
		},
	})
}

// AddCommitSegment adds request to batch
func (batch *Batch) AddCommitSegment(params CommitSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentCommit{
			SegmentCommit: params.toRequest(),
		},
	})
}

// AddListSegments adds request to batch
func (batch *Batch) AddListSegments(params ListSegmentsParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentList{
			SegmentList: params.toRequest(),
		},
	})
}

// AddMakeInlineSegment adds request to batch
func (batch *Batch) AddMakeInlineSegment(params MakeInlineSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentMakeInline{
			SegmentMakeInline: params.toRequest(),
		},
	})
}

// AddDownloadSegment adds request to batch
func (batch *Batch) AddDownloadSegment(params DownloadSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentDownload{
			SegmentDownload: params.toRequest(),
		},
	})
}

// AddBeginDeleteSegment adds request to batch
func (batch *Batch) AddBeginDeleteSegment(params BeginDeleteSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentBeginDelete{
			SegmentBeginDelete: params.toRequest(),
		},
	})
}

// AddFinishDeleteSegment adds request to batch
func (batch *Batch) AddFinishDeleteSegment(params FinishDeleteSegmentParams) {
	batch.requests = append(batch.requests, &pb.BatchRequestItem{
		Request: &pb.BatchRequestItem_SegmentFinishDelete{
			SegmentFinishDelete: params.toRequest(),
		},
	})
}

// Send sends batch request
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
			pbRequest:  batch.requests[i].Request,
			pbResponse: response.Response,
		}
	}

	return responses, nil
}

// Response single response from batch call
type Response struct {
	pbRequest  interface{}
	pbResponse interface{}
}

// CreateBucket returns response for CreateBucket request
func (resp *Response) CreateBucket() (CreateBucketResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketCreate)
	if !ok {
		return CreateBucketResponse{}, ErrInvalidType
	}

	createResponse, err := newCreateBucketResponse(item.BucketCreate)
	if err != nil {
		return CreateBucketResponse{}, err
	}
	return createResponse, nil
}

// GetBucket returns response for GetBucket request
func (resp *Response) GetBucket() (GetBucketResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketGet)
	if !ok {
		return GetBucketResponse{}, ErrInvalidType
	}
	getResponse, err := newGetBucketResponse(item.BucketGet)
	if err != nil {
		return GetBucketResponse{}, err
	}
	return getResponse, nil
}

// ListBuckets returns response for ListBuckets request
func (resp *Response) ListBuckets() (ListBucketsResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_BucketList)
	if !ok {
		return ListBucketsResponse{}, ErrInvalidType
	}
	return newListBucketsResponse(item.BucketList), nil
}

// BeginObject returns response for BeginObject request
func (resp *Response) BeginObject() (BeginObjectResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_ObjectBegin)
	if !ok {
		return BeginObjectResponse{}, ErrInvalidType
	}
	return newBeginObjectResponse(item.ObjectBegin), nil
}

// BeginDeleteObject returns response for BeginDeleteObject request
func (resp *Response) BeginDeleteObject() (BeginDeleteObjectResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_ObjectBeginDelete)
	if !ok {
		return BeginDeleteObjectResponse{}, ErrInvalidType
	}
	return newBeginDeleteObjectResponse(item.ObjectBeginDelete), nil
}

// GetObject returns response for GetObject request
func (resp *Response) GetObject() (GetObjectResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_ObjectGet)
	if !ok {
		return GetObjectResponse{}, ErrInvalidType
	}
	return newGetObjectResponse(item.ObjectGet), nil
}

// ListObjects returns response for ListObjects request
func (resp *Response) ListObjects() (ListObjectsResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_ObjectList)
	if !ok {
		return ListObjectsResponse{}, ErrInvalidType
	}

	requestItem, ok := resp.pbRequest.(*pb.BatchRequestItem_ObjectList)
	if !ok {
		return ListObjectsResponse{}, ErrInvalidType
	}

	return newListObjectsResponse(item.ObjectList, requestItem.ObjectList.EncryptedPrefix, requestItem.ObjectList.Recursive), nil
}

// BeginSegment returns response for BeginSegment request
func (resp *Response) BeginSegment() (BeginSegmentResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_SegmentBegin)
	if !ok {
		return BeginSegmentResponse{}, ErrInvalidType
	}

	return newBeginSegmentResponse(item.SegmentBegin), nil
}

// BeginDeleteSegment returns response for BeginDeleteSegment request
func (resp *Response) BeginDeleteSegment() (BeginDeleteSegmentResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_SegmentBeginDelete)
	if !ok {
		return BeginDeleteSegmentResponse{}, ErrInvalidType
	}

	return newBeginDeleteSegmentResponse(item.SegmentBeginDelete), nil
}

// ListSegment returns response for ListSegment request
func (resp *Response) ListSegment() (ListSegmentsResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_SegmentList)
	if !ok {
		return ListSegmentsResponse{}, ErrInvalidType
	}
	return newListSegmentsResponse(item.SegmentList), nil
}

// DownloadSegment returns response for DownloadSegment request
func (resp *Response) DownloadSegment() (DownloadSegmentResponse, error) {
	item, ok := resp.pbResponse.(*pb.BatchResponseItem_SegmentDownload)
	if !ok {
		return DownloadSegmentResponse{}, ErrInvalidType
	}
	return newDownloadSegmentResponse(item.SegmentDownload), nil
}
