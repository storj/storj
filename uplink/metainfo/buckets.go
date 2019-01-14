// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfo

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

type Client struct {
	client pb.MetainfoClient
}

func NewClient(conn *grpc.ClientConn) *Client {
	return &Client{client: pb.NewMetainfoClient(conn)}
}

func (c *Client) CreateBucket(ctx context.Context, bucket string, info *storj.Bucket) (storj.Bucket, error) {
	request := pb.CreateBucketMetainfoRequest{Bucket: bucket}
	if info != nil {
		request.PathCipher = int32(info.PathCipher)
	}

	response, err := c.client.CreateBucket(ctx, &request)
	if err != nil {
		return storj.Bucket{}, err
	}

	return convertInfo(response.GetInfo()), nil
}

func (c *Client) DeleteBucket(ctx context.Context, bucket string) error {
	request := pb.DeleteBucketMetainfoRequest{Bucket: bucket}
	_, err := c.client.DeleteBucket(ctx, &request)
	return err
}

func (c *Client) GetBucket(ctx context.Context, bucket string) (storj.Bucket, error) {
	request := pb.GetBucketMetainfoRequest{Bucket: bucket}
	response, err := c.client.GetBucket(ctx, &request)
	if err != nil {
		return storj.Bucket{}, err
	}

	return convertInfo(response.Info), nil
}

func (c *Client) ListBuckets(ctx context.Context, options storj.BucketListOptions) (storj.BucketList, error) {
	request := pb.ListBucketsMetainfoRequest{
		Cursor:    options.Cursor,
		Direction: int32(options.Direction),
		Limit:     int32(options.Limit),
	}

	response, err := c.client.ListBuckets(ctx, &request)
	if err != nil {
		return storj.BucketList{}, err
	}

	list := storj.BucketList{
		More:  response.GetMore(),
		Items: make([]storj.Bucket, len(response.GetItems())),
	}

	for i := 0; i < len(list.Items); i++ {
		list.Items[i] = convertInfo(response.GetItems()[i])
	}

	return list, nil
}

func convertInfo(info *pb.BucketInfo) storj.Bucket {
	if info == nil {
		return storj.Bucket{}
	}

	return storj.Bucket{
		Name:       info.Name,
		Created:    convertTime(info.Created),
		PathCipher: storj.Cipher(info.PathCipher),
	}
}

func convertTime(ts *timestamp.Timestamp) time.Time {
	if ts == nil {
		return time.Time{}
	}
	t, err := ptypes.Timestamp(ts)
	if err != nil {
		zap.S().Warnf("Failed converting timestamp %v: %v", ts, err)
	}
	return t
}
