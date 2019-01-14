// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	"go.uber.org/zap"

	"github.com/golang/protobuf/ptypes"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type Endpoint struct {
	service *Service
}

// CreateBucket creates a new bucket
func (endpoint *Endpoint) CreateBucket(ctx context.Context, request *pb.CreateBucketMetainfoRequest) (*pb.CreateBucketMetainfoResponse, error) {
	var err error
	info := storj.Bucket{
		Name:       request.GetBucket(),
		PathCipher: storj.Cipher(request.GetPathCipher()),
	}

	info, err = endpoint.service.CreateBucket(ctx, &info)
	if err != nil {
		return nil, err
	}

	return &pb.CreateBucketMetainfoResponse{
		Info: convertInfo(info),
	}, nil
}

// GetBucket returns the info for a bucket
func (endpoint *Endpoint) GetBucket(ctx context.Context, request *pb.GetBucketMetainfoRequest) (*pb.GetBucketMetainfoResponse, error) {
	info, err := endpoint.service.GetBucket(ctx, request.GetBucket())
	if err != nil {
		return nil, err
	}

	return &pb.GetBucketMetainfoResponse{
		Info: convertInfo(info),
	}, nil
}

// ListBuckets lists the existing buckets
func (endpoint *Endpoint) ListBuckets(ctx context.Context, request *pb.ListBucketsMetainfoRequest) (*pb.ListBucketsMetainfoResponse, error) {
	list, err := endpoint.service.ListBuckets(ctx, storj.BucketListOptions{
		Cursor:    request.GetCursor(),
		Direction: storj.ListDirection(request.GetDirection()),
		Limit:     int(request.GetLimit()),
	})
	if err != nil {
		return nil, err
	}

	items := make([]*pb.BucketInfo, len(list.Items))
	for i := 0; i < len(list.Items); i++ {
		items[i] = convertInfo(list.Items[i])
	}

	return &pb.ListBucketsMetainfoResponse{
		Items: items,
		More:  list.More,
	}, nil
}

// DeleteBucket deletes a bucket
func (endpoint *Endpoint) DeleteBucket(ctx context.Context, request *pb.DeleteBucketMetainfoRequest) (*pb.DeleteBucketMetainfoResponse, error) {
	err := endpoint.service.DeleteBucket(ctx, request.GetBucket())
	if err != nil {
		return nil, err
	}

	return &pb.DeleteBucketMetainfoResponse{}, nil
}

func convertInfo(info storj.Bucket) *pb.BucketInfo {
	return &pb.BucketInfo{
		Name:       info.Name,
		Created:    convertTime(info.Created),
		PathCipher: int32(info.PathCipher),
	}
}

func convertTime(time time.Time) *timestamp.Timestamp {
	timestamp, err := ptypes.TimestampProto(time)
	if err != nil {
		zap.S().Warnf("Failed converting time %v: %v", time, err)
	}
	return timestamp
}
