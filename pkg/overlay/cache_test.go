// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/protos/overlay"
	"storj.io/storj/storage/common"
)

var (
	getCases = []struct {
		testID              string
		expectedTimesCalled int
		key                 string
		expectedResponse    *overlay.NodeAddress
		expectedError       error
		data                map[string][]byte
	}{
		{
			testID:              "valid Get",
			expectedTimesCalled: 1,
			key:                 "foo",
			expectedResponse:    &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"},
			expectedError:       nil,
			data: map[string][]byte{"foo": func() []byte {
				na := &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}
				d, err := proto.Marshal(na)
				if err != nil {
					panic(err)
				}
				return d
			}()},
		},
		{
			testID:              "error Get from redis",
			expectedTimesCalled: 1,
			key:                 "error",
			expectedResponse:    nil,
			expectedError:       storage.ErrForced,
			data: map[string][]byte{"error": func() []byte {
				na := &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}
				d, err := proto.Marshal(na)
				if err != nil {
					panic(err)
				}
				return d
			}()},
		},
		{
			testID:              "get missing key",
			expectedTimesCalled: 1,
			key:                 "bar",
			expectedResponse:    nil,
			expectedError:       storage.ErrMissingKey,
			data: map[string][]byte{"foo": func() []byte {
				na := &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}
				d, err := proto.Marshal(na)
				if err != nil {
					panic(err)
				}
				return d
			}()},
		},
	}

	putCases = []struct {
		testID              string
		expectedTimesCalled int
		key                 string
		value               overlay.NodeAddress
		expectedError       error
		data                map[string][]byte
	}{
		{
			testID:              "valid Put",
			expectedTimesCalled: 1,
			key:                 "foo",
			value:               overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"},
			expectedError:       nil,
			data:                map[string][]byte{},
		},
	}
)

// func TestRedisGet(t *testing.T) {
// 	for _, c := range getCases {
// 		t.Run(c.testID, func(t *testing.T) {
//
// 			oc := Cache{DB: c.client}
//
// 			assert.Equal(t, 0, c.client.GetCalled)
//
// 			resp, err := oc.Get(context.Background(), c.key)
// 			assert.Equal(t, c.expectedError, err)
// 			assert.Equal(t, c.expectedResponse, resp)
// 			assert.Equal(t, c.expectedTimesCalled, c.client.GetCalled)
// 		})
// 	}
// }
//
// func TestRedisPut(t *testing.T) {
//
// 	for _, c := range putCases {
// 		t.Run(c.testID, func(t *testing.T) {
//
// 			oc := Cache{DB: c.client}
//
// 			assert.Equal(t, 0, c.client.PutCalled)
//
// 			err := oc.Put(c.key, c.value)
// 			assert.Equal(t, c.expectedError, err)
// 			assert.Equal(t, c.expectedTimesCalled, c.client.PutCalled)
//
// 			v := c.client.Data[c.key]
// 			na := &overlay.NodeAddress{}
//
// 			assert.NoError(t, proto.Unmarshal(v, na))
// 			assert.Equal(t, na, &c.value)
// 		})
// 	}
// }
//
// func TestBoltGet(t *testing.T) {
// 	for _, c := range getCases {
// 		t.Run(c.testID, func(t *testing.T) {
//
// 			oc := Cache{DB: c.client}
//
// 			assert.Equal(t, 0, c.client.GetCalled)
//
// 			resp, err := oc.Get(context.Background(), c.key)
// 			assert.Equal(t, c.expectedError, err)
// 			assert.Equal(t, c.expectedResponse, resp)
// 			assert.Equal(t, c.expectedTimesCalled, c.client.GetCalled)
// 		})
// 	}
// }
//
// func TestBoltPut(t *testing.T) {
// 	for _, c := range putCases {
// 		t.Run(c.testID, func(t *testing.T) {
//
// 			db := storage.NewMockStorageClient(c.data)
// 			oc := Cache{DB: db}
//
// 			assert.Equal(t, 0, db.PutCalled)
//
// 			err := oc.Put(c.key, c.value)
// 			assert.Equal(t, c.expectedError, err)
// 			assert.Equal(t, c.expectedTimesCalled, c.client.PutCalled)
//
// 			v := c.client.Data[c.key]
// 			na := &overlay.NodeAddress{}
//
// 			assert.NoError(t, proto.Unmarshal(v, na))
// 			assert.Equal(t, na, &c.value)
// 		})
// 	}
// }

func TestMockGet(t *testing.T) {
	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {

			db := storage.NewMockStorageClient(c.data)
			oc := Cache{DB: db}

			assert.Equal(t, 0, db.GetCalled)

			resp, err := oc.Get(context.Background(), c.key)
			assert.Equal(t, c.expectedError, err)
			assert.Equal(t, c.expectedResponse, resp)
			assert.Equal(t, c.expectedTimesCalled, db.GetCalled)
		})
	}
}

func TestMockPut(t *testing.T) {
	for _, c := range putCases {
		t.Run(c.testID, func(t *testing.T) {

			db := storage.NewMockStorageClient(c.data)
			oc := Cache{DB: db}

			assert.Equal(t, 0, db.PutCalled)

			err := oc.Put(c.key, c.value)
			assert.Equal(t, c.expectedError, err)
			assert.Equal(t, c.expectedTimesCalled, db.PutCalled)

			v := db.Data[c.key]
			na := &overlay.NodeAddress{}

			assert.NoError(t, proto.Unmarshal(v, na))
			assert.Equal(t, na, &c.value)
		})
	}
}
