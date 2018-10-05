// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package mirroring

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
		"testing"

	minio "github.com/minio/minio/cmd"
	test "storj.io/mirroring/utils/testing_utils"
)

func TestCopyObjectHandler(t *testing.T) {

	prime := test.NewProxyObjectLayer()
	alter := test.NewProxyObjectLayer()

	m := MirroringObjectLayer{
		Prime: prime,
		Alter: alter,
		Logger: nil,//&utils.LoggerV{},
	}

	cases := []struct {
		testName, address string
		testFunc          func()
	}{
		{
			testName: "CopyObjectHandler: both success",

			testFunc: func() {
				isAlterCalled := false

				h := NewCopyObjectHandler(&m, context.Background(), "src_bucket", "src_obj", "dst_bucket",
										  "dst_obj", minio.ObjectInfo{}, minio.ObjectOptions{}, minio.ObjectOptions{})

				alter.CopyObjectFunc = func(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo, srcOpts, dstOpts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
					isAlterCalled = true
					return minio.ObjectInfo{},nil
				}

				h.Process()

				assert.Nil(t, h.primeErr)
				assert.Nil(t, h.alterErr)
				//assert.Nil(t, h.m.Logger.Err)
				assert.Equal(t, true, isAlterCalled)
			},
		},
		{
			testName: "CopyObjectHandler: Prime error, alter should not be triggered",

			testFunc: func() {
				isAlterCalled := false

				h := NewCopyObjectHandler(&m, context.Background(), "src_bucket", "src_obj", "dst_bucket",
					"dst_obj", minio.ObjectInfo{}, minio.ObjectOptions{}, minio.ObjectOptions{})

				prime.CopyObjectFunc = func(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo, srcOpts, dstOpts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
					return srcInfo, errors.New("prime failed")
				}

				alter.CopyObjectFunc = func(ctx context.Context, srcBucket, srcObject, destBucket, destObject string, srcInfo minio.ObjectInfo, srcOpts, dstOpts minio.ObjectOptions) (objInfo minio.ObjectInfo, err error) {
					isAlterCalled = true
					return srcInfo,nil
				}

				h.Process()

				assert.Error(t, h.primeErr)
				assert.Nil(t, h.alterErr)
				//assert.Nil(t, h.m.Logger.Err)
				assert.Equal(t, false, isAlterCalled)
			},
		},
		{
			testName: "MakeBucketHandler: Prime success, alter success",

			testFunc: func() {
				isAlterCalled := false

				prime.MakeBucketWithLocationFunc = func (ctx context.Context, bucket string, location string) (err error) {
					return nil
				}

				alter.MakeBucketWithLocationFunc = func (ctx context.Context, bucket string, location string) (err error) {
					isAlterCalled = true
					return errors.New("alter failed")
				}

				h := NewMakeBucketHandler(&m, context.Background(), "bucket_name", "us")

				h.Process()

				assert.Nil(t, h.primeErr)
				assert.Error(t, h.alterErr)
				//assert.Error(t, h.m.Logger.Err)
				assert.Equal(t, true, isAlterCalled)
				assert.Error(t, h.alterErr)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.testFunc()
		})
	}
}
