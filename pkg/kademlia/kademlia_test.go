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

func bootstrapTestNetwork(t *testing.T, ip, port string) []dht.DHT {
	dhts := []dht.DHT{}

	p, err := strconv.Atoi(port)
	assert.NoError(t, err)

	for i := 0; i < testNetSize; i++ {
		assert.NoError(t, err)
		dht, err := NewKademlia([]overlay.Node{GetIntroNode("127.0.0.1", strconv.Itoa(p-1))}, ip, strconv.Itoa(p))
		assert.NoError(t, err)

		p++
		dhts = append(dhts, dht)
		err = dht.ListenAndServe()
		assert.NoError(t, err)
		err = dht.Bootstrap(context.Background())
		assert.NoError(t, err)
	}

	return dhts
}

func newTestKademlia(t *testing.T, ip, port string, d dht.DHT) *Kademlia {
	rt, err := d.GetRoutingTable(context.Background())
	assert.NoError(t, err)

	n := []overlay.Node{rt.Local()}

	kad, err := NewKademlia(n, ip, port)
	assert.NoError(t, err)
	return kad
}

func TestBootstrap(t *testing.T) {
	dhts := bootstrapTestNetwork(t, "127.0.0.1", "3001")
	defer func(d []dht.DHT) {
		for _, v := range d {
			v.Disconnect()
		}
	}(dhts)

	cases := []struct {
		k *Kademlia
	}{
		{
			k: newTestKademlia(t, "127.0.0.1", "3000", dhts[rand.Intn(testNetSize)]),
		},
	}

	for _, v := range cases {
		ctx := context.Background()
		go v.k.ListenAndServe()
		time.Sleep(time.Second)
		err := v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		rt, err := dhts[0].GetRoutingTable(context.Background())
		assert.NoError(t, err)

		localID := rt.Local().Id
		n := NodeID(localID)
		node, err := v.k.FindNode(ctx, &n)
		assert.NoError(t, err)
		assert.NotEmpty(t, node)
		assert.Equal(t, localID, node.Id)
		v.k.dht.Disconnect()
	}

}

func TestGetNodes(t *testing.T) {
	dhts := bootstrapTestNetwork(t, "127.0.0.1", "6001")
	defer func(d []dht.DHT) {
		for _, v := range d {
			v.Disconnect()
		}
	}(dhts)

	cases := []struct {
		k           *Kademlia
		start       string
		limit       int
		expectedErr error
	}{
		{
			k:           newTestKademlia(t, "127.0.0.1", "6000", dhts[rand.Intn(testNetSize)]),
			limit:       10,
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		ctx := context.Background()
		err := v.k.ListenAndServe()
		assert.Equal(t, v.expectedErr, err)
		time.Sleep(time.Second)
		err = v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		rt, err := v.k.GetRoutingTable(context.Background())

		assert.NoError(t, err)
		start := rt.Local().Id

		nodes, err := v.k.GetNodes(ctx, start, v.limit)
		assert.Equal(t, v.expectedErr, err)
		assert.Len(t, nodes, v.limit)
		v.k.dht.Disconnect()
	}

}

func TestFindNode(t *testing.T) {
	dhts := bootstrapTestNetwork(t, "127.0.0.1", "6001")
	defer func(d []dht.DHT) {
		for _, v := range d {
			v.Disconnect()
		}
	}(dhts)

	cases := []struct {
		k           *Kademlia
		start       string
		input       NodeID
		expectedErr error
	}{
		{
			k:           newTestKademlia(t, "127.0.0.1", "6000", dhts[rand.Intn(testNetSize)]),
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		ctx := context.Background()
		go v.k.ListenAndServe()
		time.Sleep(time.Second)
		err := v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		rt, err := dhts[rand.Intn(testNetSize)].GetRoutingTable(context.Background())
		assert.NoError(t, err)

		id := NodeID(rt.Local().Id)
		node, err := v.k.FindNode(ctx, &id)
		assert.Equal(t, v.expectedErr, err)
		assert.NotZero(t, node)
		assert.Equal(t, node.Id, id.String())
		v.k.dht.Disconnect()
	}

}

func TestPing(t *testing.T) {
	dhts := bootstrapTestNetwork(t, "127.0.0.1", "6001")
	defer func(d []dht.DHT) {
		for _, v := range d {
			v.Disconnect()
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
			k: newTestKademlia(t, "127.0.0.1", "6000", dhts[rand.Intn(testNetSize)]),
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
		ctx := context.Background()
		go v.k.ListenAndServe()
		time.Sleep(time.Second)
		err := v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		node, err := v.k.Ping(ctx, v.input)
		assert.Equal(t, v.expectedErr, err)
		assert.NotEmpty(t, node)
		assert.Equal(t, v.input, node)
		v.k.dht.Disconnect()
	}

}
