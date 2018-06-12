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

func TestGet(t *testing.T) {
	cases := []struct {
		testID              string
		expectedTimesCalled int
		key                 string
		expectedResponse    *overlay.NodeAddress
		expectedError       error
		client              *storage.MockStorageClient
	}{
		{
			testID:              "valid Get",
			expectedTimesCalled: 1,
			key:                 "foo",
			expectedResponse:    &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"},
			expectedError:       nil,
			client: storage.NewMockStorageClient(map[string][]byte{"foo": func() []byte {
				na := &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}
				d, err := proto.Marshal(na)
				assert.NoError(t, err)
				return d
			}()}),
		},
		{
			testID:              "error Get from redis",
			expectedTimesCalled: 1,
			key:                 "error",
			expectedResponse:    nil,
			expectedError:       storage.ErrForced,
			client: storage.NewMockStorageClient(map[string][]byte{"error": func() []byte {
				na := &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}
				d, err := proto.Marshal(na)
				assert.NoError(t, err)
				return d
			}()}),
		},
		{
			testID:              "get missing key",
			expectedTimesCalled: 1,
			key:                 "bar",
			expectedResponse:    nil,
			expectedError:       storage.ErrMissingKey,
			client: storage.NewMockStorageClient(map[string][]byte{"foo": func() []byte {
				na := &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}
				d, err := proto.Marshal(na)
				assert.NoError(t, err)
				return d
			}()}),
		},
	}

	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {

			oc := Cache{DB: c.client}

			assert.Equal(t, 0, c.client.GetCalled)

			resp, err := oc.Get(context.Background(), c.key)
			assert.Equal(t, c.expectedError, err)
			assert.Equal(t, c.expectedResponse, resp)
			assert.Equal(t, c.expectedTimesCalled, c.client.GetCalled)
		})
	}
}

func TestPut(t *testing.T) {
	cases := []struct {
		testID              string
		expectedTimesCalled int
		key                 string
		value               overlay.NodeAddress
		expectedError       error
		client              *storage.MockStorageClient
	}{
		{
			testID:              "valid Put",
			expectedTimesCalled: 1,
			key:                 "foo",
			value:               overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"},
			expectedError:       nil,
			client:              storage.NewMockStorageClient(map[string][]byte{}),
		},
	}

	for _, c := range cases {
		t.Run(c.testID, func(t *testing.T) {

			oc := Cache{DB: c.client}

			assert.Equal(t, 0, c.client.PutCalled)

			err := oc.Put(c.key, c.value)
			assert.Equal(t, c.expectedError, err)
			assert.Equal(t, c.expectedTimesCalled, c.client.PutCalled)

			v := c.client.Data[c.key]
			na := &overlay.NodeAddress{}

			assert.NoError(t, proto.Unmarshal(v, na))
			assert.Equal(t, na, &c.value)
		})
	}
}
