// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/pb"
	"storj.io/common/storj"
)

// Batch handle requests sent in batch.
func (endpoint *Endpoint) Batch(ctx context.Context, req *pb.BatchRequest) (resp *pb.BatchResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	resp = &pb.BatchResponse{}

	resp.Responses = make([]*pb.BatchResponseItem, 0, len(req.Requests))

	var lastStreamID storj.StreamID
	var lastSegmentID storj.SegmentID
	var prevSegmentReq *pb.BatchRequestItem
	for i, request := range req.Requests {
		switch singleRequest := request.Request.(type) {
		// BUCKET
		case *pb.BatchRequestItem_BucketCreate:
			singleRequest.BucketCreate.Header = req.Header
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
			singleRequest.BucketGet.Header = req.Header
			response, err := endpoint.GetBucket(ctx, singleRequest.BucketGet)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_BucketGet{
					BucketGet: response,
				},
			})
		case *pb.BatchRequestItem_BucketGetLocation:
			singleRequest.BucketGetLocation.Header = req.Header
			response, err := endpoint.GetBucketLocation(ctx, singleRequest.BucketGetLocation)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_BucketGetLocation{
					BucketGetLocation: response,
				},
			})
		case *pb.BatchRequestItem_BucketGetVersioning:
			singleRequest.BucketGetVersioning.Header = req.Header
			response, err := endpoint.GetBucketVersioning(ctx, singleRequest.BucketGetVersioning)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_BucketGetVersioning{
					BucketGetVersioning: response,
				},
			})
		case *pb.BatchRequestItem_BucketSetVersioning:
			singleRequest.BucketSetVersioning.Header = req.Header
			response, err := endpoint.SetBucketVersioning(ctx, singleRequest.BucketSetVersioning)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_BucketSetVersioning{
					BucketSetVersioning: response,
				},
			})
		case *pb.BatchRequestItem_BucketDelete:
			singleRequest.BucketDelete.Header = req.Header
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
			singleRequest.BucketList.Header = req.Header
			response, err := endpoint.ListBuckets(ctx, singleRequest.BucketList)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_BucketList{
					BucketList: response,
				},
			})

		// OBJECT
		case *pb.BatchRequestItem_ObjectBegin:
			singleRequest.ObjectBegin.Header = req.Header
			response, err := endpoint.BeginObject(ctx, singleRequest.ObjectBegin)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_ObjectBegin{
					ObjectBegin: response,
				},
			})
			if response != nil {
				lastStreamID = response.StreamId
			}
		case *pb.BatchRequestItem_ObjectCommit:
			singleRequest.ObjectCommit.Header = req.Header

			if singleRequest.ObjectCommit.StreamId.IsZero() && !lastStreamID.IsZero() {
				singleRequest.ObjectCommit.StreamId = lastStreamID
			}

			var response *pb.ObjectCommitResponse
			var err error
			switch {
			case prevSegmentReq.GetSegmentMakeInline() != nil:
				segmentResp, segmentErr := endpoint.MakeInlineSegment(ctx, prevSegmentReq.GetSegmentMakeInline())
				prevSegmentReq = nil
				if segmentErr != nil {
					return resp, segmentErr
				}

				resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
					Response: &pb.BatchResponseItem_SegmentMakeInline{
						SegmentMakeInline: segmentResp,
					},
				})
				response, err = endpoint.CommitObject(ctx, singleRequest.ObjectCommit)
			case prevSegmentReq.GetSegmentCommit() != nil:
				segmentResp, segmentErr := endpoint.CommitSegment(ctx, prevSegmentReq.GetSegmentCommit())
				prevSegmentReq = nil
				if segmentErr != nil {
					return resp, segmentErr
				}

				resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
					Response: &pb.BatchResponseItem_SegmentCommit{
						SegmentCommit: segmentResp,
					},
				})
				response, err = endpoint.CommitObject(ctx, singleRequest.ObjectCommit)
			default:
				response, err = endpoint.CommitObject(ctx, singleRequest.ObjectCommit)
			}
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_ObjectCommit{
					ObjectCommit: response,
				},
			})
		case *pb.BatchRequestItem_ObjectGet:
			singleRequest.ObjectGet.Header = req.Header
			response, err := endpoint.GetObject(ctx, singleRequest.ObjectGet)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_ObjectGet{
					ObjectGet: response,
				},
			})
			if response != nil && response.Object != nil {
				lastStreamID = response.Object.StreamId
			}
		case *pb.BatchRequestItem_ObjectList:
			singleRequest.ObjectList.Header = req.Header
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
			singleRequest.ObjectBeginDelete.Header = req.Header
			response, err := endpoint.BeginDeleteObject(ctx, singleRequest.ObjectBeginDelete)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_ObjectBeginDelete{
					ObjectBeginDelete: response,
				},
			})
			if response != nil {
				lastStreamID = response.StreamId
			}
		case *pb.BatchRequestItem_ObjectFinishDelete:
			singleRequest.ObjectFinishDelete.Header = req.Header

			if singleRequest.ObjectFinishDelete.StreamId.IsZero() && !lastStreamID.IsZero() {
				singleRequest.ObjectFinishDelete.StreamId = lastStreamID
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
			singleRequest.SegmentBegin.Header = req.Header

			justCreatedObject := false
			if singleRequest.SegmentBegin.StreamId.IsZero() && !lastStreamID.IsZero() {
				singleRequest.SegmentBegin.StreamId = lastStreamID
				justCreatedObject = true
			}

			response, err := endpoint.beginSegment(ctx, singleRequest.SegmentBegin, justCreatedObject)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_SegmentBegin{
					SegmentBegin: response,
				},
			})
			if response != nil {
				lastSegmentID = response.SegmentId
			}
		case *pb.BatchRequestItem_SegmentCommit:
			singleRequest.SegmentCommit.Header = req.Header

			if singleRequest.SegmentCommit.SegmentId.IsZero() && !lastSegmentID.IsZero() {
				singleRequest.SegmentCommit.SegmentId = lastSegmentID
			}

			segmentID, err := endpoint.unmarshalSatSegmentID(ctx, singleRequest.SegmentCommit.SegmentId)
			if err != nil {
				endpoint.log.Error("unable to unmarshal segment id", zap.Error(err))
			} else if endpoint.shouldCombine(segmentID.Index, i, req.Requests) {
				prevSegmentReq = request
				continue
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
			singleRequest.SegmentList.Header = req.Header

			if singleRequest.SegmentList.StreamId.IsZero() && !lastStreamID.IsZero() {
				singleRequest.SegmentList.StreamId = lastStreamID
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
			singleRequest.SegmentMakeInline.Header = req.Header

			if singleRequest.SegmentMakeInline.StreamId.IsZero() && !lastStreamID.IsZero() {
				singleRequest.SegmentMakeInline.StreamId = lastStreamID
			}

			if endpoint.shouldCombine(singleRequest.SegmentMakeInline.Position.Index, i, req.Requests) {
				prevSegmentReq = request
				continue
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
			singleRequest.SegmentDownload.Header = req.Header

			if singleRequest.SegmentDownload.StreamId.IsZero() && !lastStreamID.IsZero() {
				singleRequest.SegmentDownload.StreamId = lastStreamID
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
			if response != nil {
				lastSegmentID = response.SegmentId
			}
		case *pb.BatchRequestItem_SegmentBeginDelete:
			singleRequest.SegmentBeginDelete.Header = req.Header

			if singleRequest.SegmentBeginDelete.StreamId.IsZero() && !lastStreamID.IsZero() {
				singleRequest.SegmentBeginDelete.StreamId = lastStreamID
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
			if response != nil {
				lastSegmentID = response.SegmentId
			}
		case *pb.BatchRequestItem_SegmentFinishDelete:
			singleRequest.SegmentFinishDelete.Header = req.Header

			if singleRequest.SegmentFinishDelete.SegmentId.IsZero() && !lastSegmentID.IsZero() {
				singleRequest.SegmentFinishDelete.SegmentId = lastSegmentID
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

			// Revoke API key.
		case *pb.BatchRequestItem_RevokeApiKey:
			singleRequest.RevokeApiKey.Header = req.Header
			response, err := endpoint.RevokeAPIKey(ctx, singleRequest.RevokeApiKey)
			if err != nil {
				return resp, err
			}
			resp.Responses = append(resp.Responses, &pb.BatchResponseItem{
				Response: &pb.BatchResponseItem_RevokeApiKey{
					RevokeApiKey: response,
				},
			})
		default:
			return nil, errs.New("unsupported request type")
		}
	}

	return resp, nil
}

// shouldCombine returns true if we are able to combine current request with next one. Main case is
// combining CommitSegment/MakeInlineSegment with ObjectCommmit.
//
// This method has a workaround for a bug in uplink where ObjectCommit was batched with
// segment N-2 instead of N-1 if list segment was inline segment. We are checking that
// current request segment index is last one before 'l' segment and we are not combining otherwise.
func (endpoint *Endpoint) shouldCombine(segmentIndex int32, reqIndex int, requests []*pb.BatchRequestItem) bool {
	if reqIndex < len(requests)-1 && requests[reqIndex+1].GetObjectCommit() != nil {
		objCommitReq := requests[reqIndex+1].GetObjectCommit()

		streamMeta := pb.StreamMeta{}
		err := pb.Unmarshal(objCommitReq.EncryptedMetadata, &streamMeta)
		if err != nil {
			endpoint.log.Error("unable to unmarshal stream meta", zap.Error(err))
			return false
		}

		return int64(segmentIndex) != streamMeta.NumberOfSegments-2
	}
	return false
}
