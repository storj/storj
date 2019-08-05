// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
)

// Batch handle requests sent in batch
func (endpoint *Endpoint) Batch(ctx context.Context, req *pb.BatchRequest) (resp *pb.BatchResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	resp = &pb.BatchResponse{}

	resp.Responses = make([]*pb.BatchResponseItem, 0, len(req.Requests))

	// TODO find a way to pass some parameters between request -> response > request
	// TODO maybe use reflection to shrink code
	for _, request := range req.Requests {
		switch singleRequest := request.Request.(type) {
		// BUCKET
		case *pb.BatchRequestItem_BucketCreate:
			response, err := endpoint.CreateBucket(ctx, singleRequest.BucketCreate)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_BucketCreate{
					BucketCreate: response,
				},
			})
		case *pb.BatchRequestItem_BucketGet:
			response, err := endpoint.GetBucket(ctx, singleRequest.BucketGet)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_BucketGet{
					BucketGet: response,
				},
			})
		case *pb.BatchRequestItem_BucketDelete:
			response, err := endpoint.DeleteBucket(ctx, singleRequest.BucketDelete)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_BucketDelete{
					BucketDelete: response,
				},
			})
		case *pb.BatchRequestItem_BucketList:
			response, err := endpoint.ListBuckets(ctx, singleRequest.BucketList)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_BucketList{
					BucketList: response,
				},
			})
		case *pb.BatchRequestItem_BucketSetAttribution:
			response, err := endpoint.SetBucketAttribution(ctx, singleRequest.BucketSetAttribution)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_BucketSetAttribution{
					BucketSetAttribution: response,
				},
			})
		//OBJECT
		case *pb.BatchRequestItem_ObjectBegin:
			response, err := endpoint.BeginObject(ctx, singleRequest.ObjectBegin)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_ObjectBegin{
					ObjectBegin: response,
				},
			})
		case *pb.BatchRequestItem_ObjectCommit:
			response, err := endpoint.CommitObject(ctx, singleRequest.ObjectCommit)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_ObjectCommit{
					ObjectCommit: response,
				},
			})
		case *pb.BatchRequestItem_ObjectGet:
			response, err := endpoint.GetObject(ctx, singleRequest.ObjectGet)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_ObjectGet{
					ObjectGet: response,
				},
			})
		case *pb.BatchRequestItem_ObjectList:
			response, err := endpoint.ListObjects(ctx, singleRequest.ObjectList)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_ObjectList{
					ObjectList: response,
				},
			})
		case *pb.BatchRequestItem_ObjectBeginDelete:
			response, err := endpoint.BeginDeleteObject(ctx, singleRequest.ObjectBeginDelete)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_ObjectBeginDelete{
					ObjectBeginDelete: response,
				},
			})
		case *pb.BatchRequestItem_ObjectFinishDelete:
			response, err := endpoint.FinishDeleteObject(ctx, singleRequest.ObjectFinishDelete)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_ObjectFinishDelete{
					ObjectFinishDelete: response,
				},
			})
		// SEGMENT
		case *pb.BatchRequestItem_SegmentBegin:
			response, err := endpoint.BeginSegment(ctx, singleRequest.SegmentBegin)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_SegmentBegin{
					SegmentBegin: response,
				},
			})
		case *pb.BatchRequestItem_SegmentCommit:
			response, err := endpoint.CommitSegment(ctx, singleRequest.SegmentCommit)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_SegmentCommit{
					SegmentCommit: response,
				},
			})
		case *pb.BatchRequestItem_SegmentList:
			response, err := endpoint.ListSegments(ctx, singleRequest.SegmentList)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_SegmentList{
					SegmentList: response,
				},
			})
		case *pb.BatchRequestItem_SegmentMakeInline:
			response, err := endpoint.MakeInlineSegment(ctx, singleRequest.SegmentMakeInline)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_SegmentMakeInline{
					SegmentMakeInline: response,
				},
			})
		case *pb.BatchRequestItem_SegmentDownload:
			response, err := endpoint.DownloadSegment(ctx, singleRequest.SegmentDownload)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_SegmentDownload{
					SegmentDownload: response,
				},
			})
		case *pb.BatchRequestItem_SegmentBeginDelete:
			response, err := endpoint.BeginDeleteSegment(ctx, singleRequest.SegmentBeginDelete)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_SegmentBeginDelete{
					SegmentBeginDelete: response,
				},
			})
		case *pb.BatchRequestItem_SegmentFinishDelete:
			response, err := endpoint.FinishDeleteSegment(ctx, singleRequest.SegmentFinishDelete)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_SegmentFinishDelete{
					SegmentFinishDelete: response,
				},
			})
		default:
			return nil, errs.New("unsupported request type")
		}
	}

	return resp, nil
}
