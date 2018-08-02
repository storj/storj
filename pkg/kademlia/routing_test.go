// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	bkad "github.com/coyle/kademlia"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/pkg/dht"
	proto "storj.io/storj/protos/overlay"
)

type TestFunc func(t *testing.T, kad *Kademlia, routeTable RouteTable)

func createID(t *testing.T) NodeID {
	bid, err := newID()
	assert.NoError(t, err)

	id := NodeID(bid)
	return id
}

func createKademliaWithRT(t *testing.T, ip, port string) *Kademlia {
	id := createID(t)
	bdht, err := bkad.NewDHT(&bkad.MemoryStore{}, &bkad.Options{
		ID:   id.Bytes(),
		IP:   ip,
		Port: port,
	})
	assert.NoError(t, err)

	routeTable := RouteTable{
		ht:  bdht.HT,
		dht: bdht,
	}

	return &Kademlia{
		routingTable: routeTable,
		dht:          bdht,
	}
}

func test(t *testing.T, testFunc TestFunc) {
	kad := createKademliaWithRT(t, "127.0.0.1", "15777")
	routeTable := NewRouteTable(*kad)

	testFunc(t, kad, routeTable)
}

func testWithBootstrap(t *testing.T, testFunc TestFunc) {
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "6001")
	defer func(d []dht.DHT) {
		for _, v := range d {
			err := v.Disconnect()
			assert.NoError(t, err)
		}
	}(dhts)

	kad := newTestKademlia(t, "127.0.0.1", "15777", bootNode)
	routeTable, err := kad.GetRoutingTable(context.Background())
	assert.NoError(t, err)

	defer kad.Disconnect()
	err = kad.ListenAndServe()
	assert.NoError(t, err)
	err = kad.Bootstrap(context.Background())
	assert.NoError(t, err)

	time.Sleep(time.Second)
	testFunc(t, kad, routeTable.(RouteTable))
}

func TestNewRouteTable(t *testing.T) {
	test(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
		assert.Equal(t, kad.dht.HT, routeTable.ht)
		assert.Equal(t, kad.dht, routeTable.dht)
	})
}

func TestK(t *testing.T) {
	test(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
		result := routeTable.K()
		assert.Equal(t, kad.dht.NumNodes(), result)
	})
}

func TestCacheSize(t *testing.T) {
	test(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
		expected := 0
		result := routeTable.CacheSize()
		assert.Equal(t, expected, result)
	})
}

func TestGetBucket(t *testing.T) {
	test(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
		id := createID(t)

		cases := []struct {
			testName, id string
			ok           bool
		}{
			{
				"IdEmptyString",
				"",
				false,
			},
			{
				"NotValidID",
				"asd",
				false,
			},
			{
				"ValidId",
				hex.EncodeToString(id.Bytes()),
				true,
			},
		}

		for _, c := range cases {
			t.Run(c.testName, func(t *testing.T) {
				_, ok := routeTable.GetBucket(c.id)
				assert.Equal(t, c.ok, ok)
			})
		}
	})
}

func TestGetBuckets(t *testing.T) {
	test(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
		buckets, err := routeTable.GetBuckets()
		assert.NoError(t, err)

		nodes := kad.dht.HT.GetBuckets()
		t.Log(nodes)
		t.Log(buckets)
		assert.Equal(t, len(nodes), len(buckets))
	})
}

func TestFindNear(t *testing.T) {
	type TestLimit func(limit, expected int)

	testNoBootstrap := func(limit, expected int) {
		test(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
			id := createID(t)

			nodes, err := routeTable.FindNear(&id, limit)
			assert.NoError(t, err)
			assert.Equal(t, expected, len(nodes))
		})
	}

	testBootstrap := func(limit, expected int) {
		testWithBootstrap(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
			id := createID(t)

			nodes, err := routeTable.FindNear(&id, limit)
			assert.NoError(t, err)
			assert.Equal(t, expected, len(nodes))
		})
	}

	cases := []struct {
		testName        string
		limit, expected int
		testFunc        TestLimit
	}{
		{
			"Limit 3, no bootstrap",
			3, 0,
			testNoBootstrap,
		},
		{
			"Limit 3, bootstrap",
			3, 3,
			testBootstrap,
		},
		{
			"Limit 7, bootstrap",
			7, 7,
			testBootstrap,
		},
		{
			"Limit 10, no bootstrap",
			10, 10,
			testBootstrap,
		},
	}

	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			c.testFunc(c.limit, c.expected)
		})
	}
}

func TestConnectionSuccess(t *testing.T) {
	test(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
		id := createID(t)

		routeTable.ConnectionSuccess(id.String(), proto.NodeAddress{})
	})
}

func TestConnectionFailed(t *testing.T) {
	test(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
		id := createID(t)

		routeTable.ConnectionFailed(id.String(), proto.NodeAddress{})
	})
}

func TestSetBucketTimestamp(t *testing.T) {
	test(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
		now := time.Now()
		cases := []struct {
			testName, id string
			isError      bool
		}{
			{
				"ValidId",
				"10",
				false,
			},
			{
				"NotValidId",
				"",
				true,
			},
		}

		for _, c := range cases {
			t.Run(c.testName, func(t *testing.T) {
				err := routeTable.SetBucketTimestamp(c.id, now)

				if c.isError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestGetBucketTimestamp(t *testing.T) {
	test(t, func(t *testing.T, kad *Kademlia, routeTable RouteTable) {
		id := createID(t)
		_, err := routeTable.GetBucketTimestamp(id.String(), &KBucket{})
		assert.NoError(t, err)
	})
}

func TestGetNodeRoutingTable(t *testing.T) {
	id := createID(t)
	_, err := GetNodeRoutingTable(context.Background(), id)
	assert.NoError(t, err)
}
