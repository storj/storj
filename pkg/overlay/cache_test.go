// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"testing"
	"path/filepath"
	"os"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/protos/overlay"
	"storj.io/storj/storage/common"
	"storj.io/storj/storage/redis"
	"github.com/zeebo/errs"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/pkg/utils"
)

type dbClient int
type responses map[dbClient]*overlay.NodeAddress
type _errors map[dbClient]error

const (
	mock   dbClient = iota
	bolt
	_redis
)

var (
	getCases = []struct {
		testID              string
		expectedTimesCalled int
		key                 string
		expectedResponses   responses
		expectedErrors      _errors
		data                map[string][]byte
	}{
		{
			testID:              "valid Get",
			expectedTimesCalled: 1,
			key:                 "foo",
			expectedResponses: func() responses {
				na := &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}
				return responses{
					mock:   na,
					bolt:   na,
					_redis: na,
				}
			}(),
			expectedErrors: _errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
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
			testID:              "forced get error",
			expectedTimesCalled: 1,
			key:                 "error",
			expectedResponses: func() responses {
				na := &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}
				return responses{
					mock:   nil,
					bolt:   na,
					_redis: na,
				}
			}(),
			expectedErrors: _errors{
				mock:   storage.ErrForced,
				bolt:   nil,
				_redis: nil,
			},
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
			expectedResponses: responses{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			// TODO(bryanchriswhite): compare actual errors
			expectedErrors: _errors{
				mock:   storage.ErrMissingKey,
				bolt:   errs.New("boltdb error"),
				_redis: errs.New("redis error"),
			},
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
		expectedErrors      _errors
		data                map[string][]byte
	}{
		{
			testID:              "valid Put",
			expectedTimesCalled: 1,
			key:                 "foo",
			value:               overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"},
			expectedErrors: _errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			data: map[string][]byte{},
		},
	}
)

func redisTestClient(data map[string][]byte) storage.DB {
	client, err := redis.NewClient("127.0.0.1:6379", "", 1)
	if err != nil {
		panic(err)
	}

	populateStorage(client, data)

	return client
}

func boltTestClient(data map[string][]byte) (_ storage.DB, _ func()) {
	boltPath, err := filepath.Abs("test_bolt.db")
	if err != nil {
		panic(err)
	}

	logger, err := utils.NewLogger("dev")
	if err != nil {
		panic(err)
	}

	client, err := boltdb.NewClient(logger, boltPath, "testBoltdb")
	if err != nil {
		panic(err)
	}

	cleanup := func() {
		if err := os.Remove(boltPath); err != nil {
			panic(err)
		}
	}

	populateStorage(client, data)

	return client, cleanup
}

func populateStorage(client storage.DB, data map[string][]byte) {
	for k, v := range data {
		if err := client.Put([]byte(k), v); err != nil {
			panic(errs.New("Error while trying to store test data"))
		}
	}
}

func TestRedisGet(t *testing.T) {
	done := storage.EnsureRedis()
	defer done()

	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {
			db := redisTestClient(c.data)
			oc := Cache{DB: db}

			resp, err := oc.Get(context.Background(), c.key)
			if expectedErr := c.expectedErrors[_redis]; expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.Equal(t, expectedErr, err)
			}
			assert.Equal(t, c.expectedResponses[_redis], resp)
		})
	}
}

func TestRedisPut(t *testing.T) {
	done := storage.EnsureRedis()
	defer done()

	for _, c := range putCases {
		t.Run(c.testID, func(t *testing.T) {
			db, cleanup := boltTestClient(c.data)
			defer cleanup()

			oc := Cache{DB: db}

			err := oc.Put(c.key, c.value)
			assert.Equal(t, c.expectedErrors[_redis], err)

			v, err := db.Get([]byte(c.key))
			assert.NoError(t, err)
			na := &overlay.NodeAddress{}

			assert.NoError(t, proto.Unmarshal(v, na))
			assert.Equal(t, na, &c.value)
		})
	}
}


func TestBoltGet(t *testing.T) {
	done := storage.EnsureRedis()
	defer done()

	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {
			db, cleanup := boltTestClient(c.data)
			defer cleanup()

			oc := Cache{DB: db}

			resp, err := oc.Get(context.Background(), c.key)
			if expectedErr := c.expectedErrors[bolt]; expectedErr != nil {
				assert.Error(t, err)
			} else {
				assert.Equal(t, expectedErr, err)
			}
			assert.Equal(t, c.expectedResponses[bolt], resp)

		})
	}
}

func TestBoltPut(t *testing.T) {
	done := storage.EnsureRedis()
	defer done()

	for _, c := range putCases {
		t.Run(c.testID, func(t *testing.T) {
			db, cleanup := boltTestClient(c.data)
			defer cleanup()

			oc := Cache{DB: db}

			err := oc.Put(c.key, c.value)
			assert.Equal(t, c.expectedErrors[_redis], err)

			v, err := db.Get([]byte(c.key))
			assert.NoError(t, err)
			na := &overlay.NodeAddress{}

			assert.NoError(t, proto.Unmarshal(v, na))
			assert.Equal(t, na, &c.value)
		})
	}
}

func TestMockGet(t *testing.T) {
	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {

			db := storage.NewMockStorageClient(c.data)
			oc := Cache{DB: db}

			assert.Equal(t, 0, db.GetCalled)

			resp, err := oc.Get(context.Background(), c.key)
			assert.Equal(t, c.expectedErrors[mock], err)
			assert.Equal(t, c.expectedResponses[mock], resp)
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
			assert.Equal(t, c.expectedErrors[mock], err)
			assert.Equal(t, c.expectedTimesCalled, db.PutCalled)

			v := db.Data[c.key]
			na := &overlay.NodeAddress{}

			assert.NoError(t, proto.Unmarshal(v, na))
			assert.Equal(t, na, &c.value)
		})
	}
}
