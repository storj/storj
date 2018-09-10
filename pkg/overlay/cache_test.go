// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"go.uber.org/zap/zaptest"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/redis/redisserver"
	"storj.io/storj/storage/storelogger"
	"storj.io/storj/storage/teststore"
)

var (
	ctx = context.Background()
)

type dbClient int
type responses map[dbClient]*overlay.Node
type errors map[dbClient]*errs.Class

const (
	mock dbClient = iota
	bolt
	_redis
	testNetSize = 30
)

func newTestKademlia(t *testing.T, ip, port string, d dht.DHT, b overlay.Node) *kademlia.Kademlia {
	i, err := kademlia.NewID()
	assert.NoError(t, err)
	id := *i
	n := []overlay.Node{b}
	kad, err := kademlia.NewKademlia(&id, n, ip, port)
	assert.NoError(t, err)

	return kad
}

func bootstrapTestNetwork(t *testing.T, ip, port string) ([]dht.DHT, overlay.Node) {
	bid, err := kademlia.NewID()
	assert.NoError(t, err)

	bnid := *bid
	dhts := []dht.DHT{}

	p, err := strconv.Atoi(port)
	pm := strconv.Itoa(p)
	assert.NoError(t, err)
	intro, err := kademlia.GetIntroNode(bnid.String(), ip, pm)
	assert.NoError(t, err)

	boot, err := kademlia.NewKademlia(&bnid, []overlay.Node{*intro}, ip, pm)

	assert.NoError(t, err)
	rt, err := boot.GetRoutingTable(context.Background())
	assert.NoError(t, err)
	bootNode := rt.Local()

	err = boot.ListenAndServe()
	assert.NoError(t, err)
	p++

	err = boot.Bootstrap(context.Background())
	assert.NoError(t, err)
	for i := 0; i < testNetSize; i++ {
		gg := strconv.Itoa(p)

		nid, err := kademlia.NewID()
		assert.NoError(t, err)
		id := *nid

		dht, err := kademlia.NewKademlia(&id, []overlay.Node{bootNode}, ip, gg)
		assert.NoError(t, err)

		p++
		dhts = append(dhts, dht)
		err = dht.ListenAndServe()
		assert.NoError(t, err)
		err = dht.Bootstrap(context.Background())
		assert.NoError(t, err)
	}

	return dhts, bootNode
}

var (
	getCases = []struct {
		testID              string
		expectedTimesCalled int
		key                 string
		expectedResponses   responses
		expectedErrors      errors
		data                []storage.ListItem
	}{
		{
			testID:              "valid Get",
			expectedTimesCalled: 1,
			key:                 "foo",
			expectedResponses: func() responses {
				na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
				return responses{
					mock:   na,
					bolt:   na,
					_redis: na,
				}
			}(),
			expectedErrors: errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			data: []storage.ListItem{{
				Key: storage.Key("foo"),
				Value: func() storage.Value {
					na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
					d, err := proto.Marshal(na)
					if err != nil {
						panic(err)
					}
					return d
				}(),
			}},
		}, {
			testID:              "forced get error",
			expectedTimesCalled: 1,
			key:                 "error",
			expectedResponses: func() responses {
				na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
				return responses{
					mock:   nil,
					bolt:   na,
					_redis: na,
				}
			}(),
			expectedErrors: errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			data: []storage.ListItem{{
				Key: storage.Key("error"),
				Value: func() storage.Value {
					na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
					d, err := proto.Marshal(na)
					if err != nil {
						panic(err)
					}
					return d
				}(),
			}},
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
			expectedErrors: errors{
				mock:   &storage.ErrKeyNotFound,
				bolt:   &storage.ErrKeyNotFound,
				_redis: &storage.ErrKeyNotFound,
			},
			data: []storage.ListItem{{
				Key: storage.Key("foo"),
				Value: func() storage.Value {
					na := &overlay.Node{Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}}
					d, err := proto.Marshal(na)
					if err != nil {
						panic(err)
					}
					return d
				}(),
			}},
		},
	}

	putCases = []struct {
		testID              string
		expectedTimesCalled int
		key                 string
		value               overlay.Node
		expectedErrors      errors
		data                []storage.ListItem
	}{
		{
			testID:              "valid Put",
			expectedTimesCalled: 1,
			key:                 "foo",
			value:               overlay.Node{Id: "foo", Address: &overlay.NodeAddress{Transport: overlay.NodeTransport_TCP, Address: "127.0.0.1:9999"}},
			expectedErrors: errors{
				mock:   nil,
				bolt:   nil,
				_redis: nil,
			},
			data: []storage.ListItem{},
		},
	}

	refreshCases = []struct {
		testID              string
		expectedTimesCalled int
		expectedErr         error
		data                []storage.ListItem
	}{
		{
			testID:              "valid update",
			expectedTimesCalled: 1,
			expectedErr:         nil,
			data:                []storage.ListItem{},
		},
	}
)

func redisTestClient(t *testing.T, addr string, items []storage.ListItem) storage.KeyValueStore {
	client, err := redis.NewClient(addr, "", 1)
	if err != nil {
		t.Fatal(err)
	}

	if err := storage.PutAll(client, items...); err != nil {
		t.Fatal(err)
	}

	return client
}

func boltTestClient(t *testing.T, items []storage.ListItem) (_ storage.KeyValueStore, _ func()) {
	boltPath, err := filepath.Abs("test_bolt.db")
	assert.NoError(t, err)

	client, err := boltdb.New(boltPath, "testBoltdb")
	assert.NoError(t, err)

	cleanup := func() {
		assert.NoError(t, client.Close())
		assert.NoError(t, os.Remove(boltPath))
	}

	if err := storage.PutAll(client, items...); err != nil {
		t.Fatal(err)
	}

	return storelogger.New(zaptest.NewLogger(t), client), cleanup
}

func TestRedisGet(t *testing.T) {
	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {
			db := redisTestClient(t, redisAddr, c.data)
			oc := Cache{DB: db}

			resp, err := oc.Get(ctx, c.key)
			assertErrClass(t, c.expectedErrors[_redis], err)
			assert.Equal(t, c.expectedResponses[_redis], resp)
		})
	}
}

func assertErrClass(t *testing.T, class *errs.Class, err error) {
	t.Helper()
	if class != nil {
		assert.True(t, class.Has(err))
	} else {
		assert.NoError(t, err)
	}
}

func TestRedisPut(t *testing.T) {
	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	for _, c := range putCases {
		t.Run(c.testID, func(t *testing.T) {
			db := redisTestClient(t, redisAddr, c.data)
			oc := Cache{DB: db}

			err := oc.Put(c.key, c.value)
			assertErrClass(t, c.expectedErrors[_redis], err)

			v, err := db.Get([]byte(c.key))
			assert.NoError(t, err)

			na := &overlay.Node{}
			assert.NoError(t, proto.Unmarshal(v, na))
			assert.True(t, proto.Equal(na, &c.value))
		})
	}
}

func TestBoltGet(t *testing.T) {
	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {
			db, cleanup := boltTestClient(t, c.data)
			defer cleanup()

			oc := Cache{DB: db}

			resp, err := oc.Get(ctx, c.key)
			assertErrClass(t, c.expectedErrors[bolt], err)
			assert.Equal(t, c.expectedResponses[bolt], resp)
		})
	}
}

func TestBoltPut(t *testing.T) {
	for _, c := range putCases {
		t.Run(c.testID, func(t *testing.T) {
			db, cleanup := boltTestClient(t, c.data)
			defer cleanup()

			oc := Cache{DB: db}

			err := oc.Put(c.key, c.value)
			assertErrClass(t, c.expectedErrors[_redis], err)

			v, err := db.Get([]byte(c.key))
			assert.NoError(t, err)
			na := &overlay.Node{}

			assert.NoError(t, proto.Unmarshal(v, na))
			assert.True(t, proto.Equal(na, &c.value))
		})
	}
}

func TestMockGet(t *testing.T) {
	for _, c := range getCases {
		t.Run(c.testID, func(t *testing.T) {

			db := teststore.New()
			if err := storage.PutAll(db, c.data...); err != nil {
				t.Fatal(err)
			}
			oc := Cache{DB: db}

			if c.key == "error" {
				db.ForceError = 1
			}
			assert.Equal(t, 0, db.CallCount.Get)

			resp, err := oc.Get(ctx, c.key)
			if c.key == "error" {
				assert.Error(t, err)
			} else {
				assertErrClass(t, c.expectedErrors[mock], err)
			}
			assert.Equal(t, c.expectedResponses[mock], resp)
			assert.Equal(t, c.expectedTimesCalled, db.CallCount.Get)
		})
	}
}

func TestMockPut(t *testing.T) {
	for _, c := range putCases {
		t.Run(c.testID, func(t *testing.T) {
			db := teststore.New()
			if err := storage.PutAll(db, c.data...); err != nil {
				t.Fatal(err)
			}
			db.CallCount.Put = 0

			oc := Cache{DB: db}

			err := oc.Put(c.key, c.value)
			assertErrClass(t, c.expectedErrors[mock], err)
			assert.Equal(t, c.expectedTimesCalled, db.CallCount.Put)

			v, err := db.Get(storage.Key(c.key))
			assert.NoError(t, err)

			na := &overlay.Node{}
			assert.NoError(t, proto.Unmarshal(v, na))
			assert.True(t, proto.Equal(na, &c.value))
		})
	}
}

func TestRefresh(t *testing.T) {
	t.Skip()
	for _, c := range refreshCases {
		t.Run(c.testID, func(t *testing.T) {
			dhts, b := bootstrapTestNetwork(t, "127.0.0.1", "3000")
			ctx := context.Background()

			db := teststore.New()
			if err := storage.PutAll(db, c.data...); err != nil {
				t.Fatal(err)
			}

			dht := newTestKademlia(t, "127.0.0.1", "2999", dhts[rand.Intn(testNetSize)], b)

			_cache := &Cache{
				DB:  db,
				DHT: dht,
			}

			err := _cache.Refresh(ctx)
			assert.Equal(t, err, c.expectedErr)
		})
	}
}

func TestNewRedisOverlayCache(t *testing.T) {
	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	cases := []struct {
		testName, address string
		testFunc          func(string)
	}{
		{
			testName: "NewRedisOverlayCache valid",
			address:  redisAddr,
			testFunc: func(address string) {
				cache, err := NewRedisOverlayCache(address, "", 1, nil)

				assert.NoError(t, err)
				assert.NotNil(t, cache)
			},
		},
		{
			testName: "NewRedisOverlayCache fail",
			address:  "",
			testFunc: func(address string) {
				cache, err := NewRedisOverlayCache(address, "", 1, nil)

				assert.Error(t, err)
				assert.Nil(t, cache)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.testFunc(c.address)
		})
	}
}
