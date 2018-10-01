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

func TestDeleteObjectHandler(t *testing.T) {

	prime := test.NewProxyObjectLayer()
	alter := test.NewProxyObjectLayer()

	m := MirroringObjectLayer{
		Prime:  prime,
		Alter:  alter,
		Logger: &test.MockLogger{},
	}

	cases := []struct {
		testName, address string
		testFunc          func()
	}{
		{
			testName: "DeleteObjectHandler: both success",

			testFunc: func() {
				isAlterCalled := false

				h := NewDeleteObjectHandler(&m, context.Background(), "bucket_name", "object_name")

				alter.DeleteObjectFunc = func(ctx context.Context, bucket string, object string) (err error) {
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
			testName: "DeleteObjectHandler: Prime error, alter should not be triggered",

			testFunc: func() {
				isAlterCalled := false

				prime.DeleteObjectFunc = func(ctx context.Context, bucket string, object string) (err error) {
					return errors.New("prime failed")
				}

				alter.DeleteObjectFunc = func(ctx context.Context, bucket string, object string) (err error) {
					isAlterCalled = true
					return nil
				}

				h := NewDeleteObjectHandler(&m, context.Background(), "bucket_name", "object_name")

				h.Process()

				assert.Error(t, h.primeErr)
				assert.Nil(t, h.alterErr)
				//assert.Nil(t, h.m.Logger.Err)
				assert.Equal(t, false, isAlterCalled)
			},
		},
		{
			testName: "DeleteObjectHandler: Prime success, alter success",

			testFunc: func() {
				isAlterCalled := false

				prime.DeleteObjectFunc = func(ctx context.Context, bucket string, object string) (err error) {
					return nil
				}

				alter.DeleteObjectFunc = func(ctx context.Context, bucket string, object string) (err error) {
					isAlterCalled = true
					return errors.New("alter failed")
				}

				h := NewDeleteObjectHandler(&m, context.Background(), "bucket_name", "object_name")

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
