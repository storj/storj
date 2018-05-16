// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"context"
	"io"
	"time"

	"github.com/minio/cli"

	"github.com/minio/minio/cmd"
	"github.com/minio/minio/pkg/auth"
	"github.com/minio/minio/pkg/hash"
)

func init() {
	cmd.RegisterGatewayCommand(cli.Command{
		Name:            "storj",
		Usage:           "Storj",
		Action:          storjGatewayMain,
		HideHelpCommand: true,
	})
}

func storjGatewayMain(ctx *cli.Context) {
	cmd.StartGateway(ctx, &Storj{})
}

// Storj is the implementation of a minio cmd.Gateway
type Storj struct{}

// Name implements cmd.Gateway
func (s *Storj) Name() string {
	return "storj"
}

// NewGatewayLayer implements cmd.Gateway
func (s *Storj) NewGatewayLayer(creds auth.Credentials) (
	cmd.ObjectLayer, error) {
	return &storjObjects{}, nil
}

// Production implements cmd.Gateway
func (s *Storj) Production() bool {
	return false
}

type storjObjects struct {
	cmd.GatewayUnsupported
}

func (s *storjObjects) DeleteBucket(ctx context.Context, bucket string) error {
	panic("TODO")
}

func (s *storjObjects) DeleteObject(ctx context.Context, bucket,
	object string) error {
	panic("TODO")
}

func (s *storjObjects) GetBucketInfo(ctx context.Context, bucket string) (
	bucketInfo cmd.BucketInfo, err error) {
	panic("TODO")
}

func (s *storjObjects) GetObject(ctx context.Context, bucket, object string,
	startOffset int64, length int64, writer io.Writer, etag string) (err error) {

	panic("TODO")
}

func (s *storjObjects) GetObjectInfo(ctx context.Context, bucket,
	object string) (objInfo cmd.ObjectInfo, err error) {
	panic("TODO")
}

func (s *storjObjects) ListBuckets(ctx context.Context) (
	buckets []cmd.BucketInfo, err error) {
	return []cmd.BucketInfo{{
		Name:    "test-bucket",
		Created: time.Now(),
	}}, nil
}

func (s *storjObjects) ListObjects(ctx context.Context, bucket, prefix, marker,
	delimiter string, maxKeys int) (result cmd.ListObjectsInfo, err error) {
	return cmd.ListObjectsInfo{
		IsTruncated: false,
		Objects: []cmd.ObjectInfo{{
			Bucket:      "test-bucket",
			Name:        "test-file",
			ModTime:     time.Now(),
			Size:        0,
			IsDir:       false,
			ContentType: "application/octet-stream",
		}},
	}, nil
}

func (s *storjObjects) MakeBucketWithLocation(ctx context.Context,
	bucket string, location string) error {
	panic("TODO")
}

func (s *storjObjects) PutObject(ctx context.Context, bucket, object string,
	data *hash.Reader, metadata map[string]string) (objInfo cmd.ObjectInfo,
	err error) {
	panic("TODO")
}

func (s *storjObjects) Shutdown(context.Context) error {
	panic("TODO")
}

func (s *storjObjects) StorageInfo(context.Context) cmd.StorageInfo {
	panic("TODO")
}
