// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"storj.io/storj/pkg/dht"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/protos/overlay"
)

const (
	testNetSize = 20
)

func bootstrapTestNetwork(t *testing.T, ip, port string) ([]dht.DHT, overlay.Node) {
	bid, err := newID()
	assert.NoError(t, err)

	bnid := NodeID(bid)
	dhts := []dht.DHT{}

	p, err := strconv.Atoi(port)
	pm := strconv.Itoa(p)
	assert.NoError(t, err)
	intro, err := GetIntroNode(bnid.String(), ip, pm)
	assert.NoError(t, err)

	boot, err := NewKademlia(&bnid, []overlay.Node{*intro}, ip, pm)
	assert.NoError(t, err)

	//added bootnode to dhts so it could be closed in defer as well
	dhts = append(dhts, boot)

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

		nid, err := newID()
		assert.NoError(t, err)
		id := NodeID(nid)

		dht, err := NewKademlia(&id, []overlay.Node{bootNode}, ip, gg)
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

func newTestKademlia(t *testing.T, ip, port string, b overlay.Node) *Kademlia {
	i, err := newID()
	assert.NoError(t, err)
	id := NodeID(i)
	n := []overlay.Node{b}

	kad, err := NewKademlia(&id, n, ip, port)
	assert.NoError(t, err)
	return kad
}

func TestBootstrap(t *testing.T) {
	t.Skip()
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "3000")

	defer func(d []dht.DHT) {
		for _, v := range d {
			_ = v.Disconnect()
		}
	}(dhts)

	cases := []struct {
		k *Kademlia
	}{
		{
			k: newTestKademlia(t, "127.0.0.1", "2999", bootNode),
		},
	}

	for _, v := range cases {
		defer func() { assert.NoError(t, v.k.Disconnect()) }()
		err := v.k.ListenAndServe()
		assert.NoError(t, err)
		err = v.k.Bootstrap(context.Background())
		assert.NoError(t, err)
		ctx := context.Background()

		rt, err := dhts[0].GetRoutingTable(context.Background())
		assert.NoError(t, err)

		localID := rt.Local().Id
		n := NodeID(localID)
		node, err := v.k.FindNode(ctx, &n)
		assert.NoError(t, err)
		assert.NotEmpty(t, node)
		assert.Equal(t, localID, node.Id)

		assert.NoError(t, v.k.dht.Disconnect())
	}

}

func TestGetNodes(t *testing.T) {
	t.Skip()
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "6001")
	defer func(d []dht.DHT) {
		for _, v := range d {
			assert.NoError(t, v.Disconnect())
		}
	}(dhts)

	cases := []struct {
		k            *Kademlia
		limit        int
		expectedErr  error
		restrictions []overlay.Restriction
	}{
		{
			k:           newTestKademlia(t, "127.0.0.1", "6000", bootNode),
			limit:       10,
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		defer func() { assert.NoError(t, v.k.Disconnect()) }()
		ctx := context.Background()
		err := v.k.ListenAndServe()
		assert.Equal(t, v.expectedErr, err)
		time.Sleep(time.Second)
		err = v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		rt, err := v.k.GetRoutingTable(context.Background())

		assert.NoError(t, err)
		start := rt.Local().Id

		nodes, err := v.k.GetNodes(ctx, start, v.limit, v.restrictions...)
		assert.Equal(t, v.expectedErr, err)
		assert.Len(t, nodes, v.limit)
		assert.NoError(t, v.k.dht.Disconnect())
	}

}

func TestFindNode(t *testing.T) {
	t.Skip()
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "5001")
	defer func(d []dht.DHT) {
		for _, v := range d {
			assert.NoError(t, v.Disconnect())
		}
	}(dhts)

	cases := []struct {
		k           *Kademlia
		expectedErr error
	}{
		{
			k:           newTestKademlia(t, "127.0.0.1", "6000", bootNode),
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		defer func() { assert.NoError(t, v.k.Disconnect()) }()
		ctx := context.Background()
		go func() { assert.NoError(t, v.k.ListenAndServe()) }()
		time.Sleep(time.Second)
		assert.NoError(t, v.k.Bootstrap(ctx))

		rt, err := dhts[rand.Intn(testNetSize)].GetRoutingTable(context.Background())
		assert.NoError(t, err)

		id := NodeID(rt.Local().Id)
		node, err := v.k.FindNode(ctx, &id)
		assert.Equal(t, v.expectedErr, err)
		assert.NotZero(t, node)
		assert.Equal(t, node.Id, id.String())
	}

}

func TestPing(t *testing.T) {
	t.Skip()
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "4001")
	defer func(d []dht.DHT) {
		for _, v := range d {
			assert.NoError(t, v.Disconnect())
		}
	}(dhts)

	r := dhts[rand.Intn(testNetSize)]
	rt, err := r.GetRoutingTable(context.Background())
	addr := rt.Local().Address
	assert.NoError(t, err)

	cases := []struct {
		k           *Kademlia
		input       overlay.Node
		expectedErr error
	}{
		{
			k: newTestKademlia(t, "127.0.0.1", "6000", bootNode),
			input: overlay.Node{
				Id: rt.Local().Id,
				Address: &overlay.NodeAddress{
					Transport: defaultTransport,
					Address:   addr.Address,
				},
			},
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		defer func() { assert.NoError(t, v.k.Disconnect()) }()
		ctx := context.Background()
		go func() { assert.NoError(t, v.k.ListenAndServe()) }()
		time.Sleep(time.Second)
		err := v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		node, err := v.k.Ping(ctx, v.input)
		assert.Equal(t, v.expectedErr, err)
		assert.NotEmpty(t, node)
		assert.Equal(t, v.input, node)
		assert.NoError(t, v.k.dht.Disconnect())
	}

}
