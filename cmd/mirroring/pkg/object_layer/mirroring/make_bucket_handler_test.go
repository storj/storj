package mirroring

import (
	"context"
	"errors"
	"github.com/stretchr/testify/assert"
		"testing"

	test "storj.io/mirroring/utils/testing_utils"
)

func TestMakeBucketHandler(t *testing.T) {

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
			testName: "MakeBucketHandler: both success",

			testFunc: func() {
				isAlterCalled := false

				h := NewMakeBucketHandler(&m, context.Background(), "bucket_name", "us")

				alter.MakeBucketWithLocationFunc = func (ctx context.Context, bucket string, location string) (err error) {
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
			testName: "MakeBucketHandler: Prime error, alter should not be triggered",

			testFunc: func() {
				isAlterCalled := false

				prime.MakeBucketWithLocationFunc = func (ctx context.Context, bucket string, location string) (err error) {
					return errors.New("Prime failed")
				}

				alter.MakeBucketWithLocationFunc = func (ctx context.Context, bucket string, location string) (err error) {
					isAlterCalled = true
					return nil
				}

				h := NewMakeBucketHandler(&m, context.Background(), "bucket_name", "us")

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
					return errors.New("Alter failed")
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