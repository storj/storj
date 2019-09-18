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
			if singleRequest.BucketCreate.Header == nil {
				singleRequest.BucketCreate.Header = req.Header
			}
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
			if singleRequest.BucketGet.Header == nil {
				singleRequest.BucketGet.Header = req.Header
			}
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
			if singleRequest.BucketDelete.Header == nil {
				singleRequest.BucketDelete.Header = req.Header
			}
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
			if singleRequest.BucketList.Header == nil {
				singleRequest.BucketList.Header = req.Header
			}
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
			if singleRequest.BucketSetAttribution.Header == nil {
				singleRequest.BucketSetAttribution.Header = req.Header
			}
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
			if singleRequest.ObjectBegin.Header == nil {
				singleRequest.ObjectBegin.Header = req.Header
			}
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
			if singleRequest.ObjectCommit.Header == nil {
				singleRequest.ObjectCommit.Header = req.Header
			}
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
			if singleRequest.ObjectGet.Header == nil {
				singleRequest.ObjectGet.Header = req.Header
			}
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
			if singleRequest.ObjectList.Header == nil {
				singleRequest.ObjectList.Header = req.Header
			}
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
			if singleRequest.ObjectBeginDelete.Header == nil {
				singleRequest.ObjectBeginDelete.Header = req.Header
			}
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
			if singleRequest.ObjectFinishDelete.Header == nil {
				singleRequest.ObjectFinishDelete.Header = req.Header
			}
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
			if singleRequest.SegmentBegin.Header == nil {
				singleRequest.SegmentBegin.Header = req.Header
			}
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
			if singleRequest.SegmentCommit.Header == nil {
				singleRequest.SegmentCommit.Header = req.Header
			}
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
			if singleRequest.SegmentList.Header == nil {
				singleRequest.SegmentList.Header = req.Header
			}
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
			if singleRequest.SegmentMakeInline.Header == nil {
				singleRequest.SegmentMakeInline.Header = req.Header
			}
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
			if singleRequest.SegmentDownload.Header == nil {
				singleRequest.SegmentDownload.Header = req.Header
			}
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
			if singleRequest.SegmentBeginDelete.Header == nil {
				singleRequest.SegmentBeginDelete.Header = req.Header
			}
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
			if singleRequest.SegmentFinishDelete.Header == nil {
				singleRequest.SegmentFinishDelete.Header = req.Header
			}
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
