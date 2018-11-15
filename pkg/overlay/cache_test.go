// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"
	"math/rand"
	"net"
	"strconv"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/node"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
	"storj.io/storj/storage"
	"storj.io/storj/storage/boltdb"
	"storj.io/storj/storage/redis"
	"storj.io/storj/storage/redis/redisserver"
	"storj.io/storj/storage/teststore"
)

const (
	testNetSize = 30
)

func pbNode(address string) pb.Node {
	return pb.Node{Address: &pb.NodeAddress{Transport: pb.NodeTransport_TCP_TLS_GRPC, Address: address}}
}

func nodeData(address string) storage.Value {
	node := pbNode(address)
	data, err := proto.Marshal(&node)
	if err != nil {
		panic(err)
	}
	return storage.Value(data)
}

func testOverlay(t *testing.T, ctx context.Context, store storage.KeyValueStore) {
	overlay := Cache{DB: store}

	t.Run("Put", func(t *testing.T) {
		err := overlay.Put("valid1", pbNode("127.0.0.1:9001"))
		if err != nil {
			t.Fatal(err)
		}
		err = overlay.Put("valid2", pbNode("127.0.0.1:9002"))
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("Get", func(t *testing.T) {
		valid2, err := overlay.Get(ctx, "valid2")
		if assert.NoError(t, err) {
			assert.Equal(t, valid2.Address.Address, "127.0.0.1:9002")
		}

		invalid2, err := overlay.Get(ctx, "invalid2")
		assert.Error(t, err)
		assert.Nil(t, invalid2)

		if storeClient, ok := store.(*teststore.Client); ok {
			storeClient.ForceError++
			_, err := overlay.Get(ctx, "valid1")
			assert.Error(t, err)
		}
	})

	t.Run("GetAll", func(t *testing.T) {
		nodes, err := overlay.GetAll(ctx, []string{"valid2", "valid1", "valid2"})
		if assert.NoError(t, err) {
			assert.Equal(t, nodes[0].Address.Address, "127.0.0.1:9002")
			assert.Equal(t, nodes[1].Address.Address, "127.0.0.1:9001")
			assert.Equal(t, nodes[2].Address.Address, "127.0.0.1:9002")
		}

		nodes, err = overlay.GetAll(ctx, []string{"valid1", "invalid"})
		if assert.NoError(t, err) {
			assert.Equal(t, nodes[0].Address.Address, "127.0.0.1:9001")
			assert.Nil(t, nodes[1])
		}

		nodes, err = overlay.GetAll(ctx, []string{"", ""})
		if assert.NoError(t, err) {
			assert.Nil(t, nodes[0])
			assert.Nil(t, nodes[1])
		}

		_, err = overlay.GetAll(ctx, []string{})
		assert.True(t, OverlayError.Has(err))

		if storeClient, ok := store.(*teststore.Client); ok {
			storeClient.ForceError++
			_, err := overlay.GetAll(ctx, []string{"valid1", "valid2"})
			assert.Error(t, err)
		}
	})
}

func TestRedis(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	redisAddr, cleanup, err := redisserver.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	store, err := redis.NewClient(redisAddr, "", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(store.Close)

	testOverlay(t, ctx, store)
}

func TestBolt(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	client, err := boltdb.New(ctx.File("overlay.db"), "overlay")
	if err != nil {
		t.Fatal(err)
	}
	defer ctx.Check(client.Close)

	testOverlay(t, ctx, client)
}

func TestStore(t *testing.T) {
	ctx := testcontext.New(t)
	defer ctx.Cleanup()

	testOverlay(t, ctx, teststore.New())
}

func TestRefresh(t *testing.T) {
	t.Skip()

	ctx := context.Background()
	dhts, bootstrap := bootstrapTestNetwork(t, "127.0.0.1", "0")
	dht := newTestKademlia(t, "127.0.0.1", "0", dhts[rand.Intn(testNetSize)], bootstrap)

	_cache := &Cache{
		DB:  teststore.New(),
		DHT: dht,
	}

	err := _cache.Refresh(ctx)
	assert.NoError(t, err)
}

func newTestKademlia(t *testing.T, ip, port string, d dht.DHT, b pb.Node) *kademlia.Kademlia {
	ctx := context.Background()
	fid, err := node.NewFullIdentity(ctx, 12, 4)
	assert.NoError(t, err)
	n := []pb.Node{b}
	kad, err := kademlia.NewKademlia(fid.ID, n, net.JoinHostPort(ip, port), fid, "db", 5)
	assert.NoError(t, err)

	return kad
}

func bootstrapTestNetwork(t *testing.T, ip, port string) ([]dht.DHT, pb.Node) {
	ctx := context.Background()
	bid, err := node.NewFullIdentity(ctx, 12, 4)
	assert.NoError(t, err)

	dhts := []dht.DHT{}

	p, err := strconv.Atoi(port)
	pm := strconv.Itoa(p)
	assert.NoError(t, err)
	intro, err := kademlia.GetIntroNode(net.JoinHostPort(ip, pm))
	assert.NoError(t, err)

	ca, err := provider.NewTestCA(ctx)
	assert.NoError(t, err)
	identity, err := ca.NewIdentity()
	assert.NoError(t, err)

	boot, err := kademlia.NewKademlia(bid.ID, []pb.Node{*intro}, net.JoinHostPort(ip, pm), identity, "db", 5)

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

		fid, err := node.NewFullIdentity(ctx, 12, 4)
		assert.NoError(t, err)

		dht, err := kademlia.NewKademlia(fid.ID, []pb.Node{bootNode}, net.JoinHostPort(ip, gg), fid, "db", 5)
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
