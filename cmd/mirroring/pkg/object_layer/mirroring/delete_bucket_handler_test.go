// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package mirroring

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"

	test "storj.io/mirroring/utils/testing_utils"
)

func TestDeleteBucketHandler(t *testing.T) {

	prime := test.NewProxyObjectLayer()
	alter := test.NewProxyObjectLayer()

	m := MirroringObjectLayer{
		Prime: prime,
		Alter: alter,
		Logger: &test.MockLogger{},
	}

	cases := []struct {
		testName, address string
		testFunc          func()
	}{
		{
			testName: "DeleteBucketHandler: both success",

			testFunc: func() {
				isAlterCalled := false

				h := NewDeleteBucketHandler(&m, context.Background(), "bucket_name")

				alter.DeleteBucketFunc = func (ctx context.Context, bucket string) (err error) {
					isAlterCalled = true
					return nil
				}

				h.Process()

				assert.Nil(t, h.primeErr)
				assert.Nil(t, h.alterErr)
				//assert.Nil(t, h.m.Logger.Err)
				assert.Equal(t, true, isAlterCalled)
			},
		},
		{
			testName: "DeleteBucketHandler: Prime error, alter should not be triggered",

			testFunc: func() {
				isAlterCalled := false

				prime.DeleteBucketFunc = func (ctx context.Context, bucket string) (err error) {
					return errors.New("prime failed")
				}

				alter.DeleteBucketFunc = func (ctx context.Context, bucket string) (err error) {
					isAlterCalled = true
					return nil
				}

				h := NewDeleteBucketHandler(&m, context.Background(), "bucket_name")

				h.Process()

				assert.Error(t, h.primeErr)
				assert.Nil(t, h.alterErr)
				//assert.Nil(t, h.m.Logger.Err)
				assert.Equal(t, false, isAlterCalled)
			},
		},
		{
			testName: "DeleteBucketHandler: Prime success, alter success",

			testFunc: func() {
				isAlterCalled := false

				prime.DeleteBucketFunc = func (ctx context.Context, bucket string) (err error) {
					return nil
				}

				alter.DeleteBucketFunc = func (ctx context.Context, bucket string) (err error) {
					isAlterCalled = true
					return errors.New("alter failed")
				}

				h := NewDeleteBucketHandler(&m, context.Background(), "bucket_name")

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

