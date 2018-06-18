// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"storj.io/storj/pkg/dht"

	"github.com/stretchr/testify/assert"
	"storj.io/storj/protos/overlay"
)

const (
	testNetSize = 5
)

func bootstrapTestNetwork(t *testing.T, ip, port string) ([]dht.DHT, overlay.Node) {
	bid, err := newID()
	assert.NoError(t, err)

	bnid := NodeID(bid)
	dhts := []dht.DHT{}

	p, err := strconv.Atoi(port)
	pm := strconv.Itoa(p - 1)
	assert.NoError(t, err)
	intro := GetIntroNode(bnid.String(), ip, pm)
	fmt.Printf("KADEMLIA FMT:: %#v\n", intro.Address)
	boot, err := NewKademlia(&bnid, []overlay.Node{intro}, ip, pm)

	assert.NoError(t, err)
	rt, err := boot.GetRoutingTable(context.Background())
	bootNode := rt.Local()
	fmt.Printf("KADEMLIA BOOTNODE:: %#v\n", bootNode.Address)
	err = boot.ListenAndServe()
	assert.NoError(t, err)

	for i := 0; i < testNetSize; i++ {
		gg := strconv.Itoa(p)
		// fmt.Printf("strconv.Itoa(p)::%v\n", gg)
		// fmt.Printf("BOOTNODE :%#v\n", bootNode)

		nid, err := newID()
		assert.NoError(t, err)
		id := NodeID(nid)

		dht, err := NewKademlia(&id, []overlay.Node{bootNode}, ip, gg)
		assert.NoError(t, err)

		p++
		dhts = append(dhts, dht)
		err = dht.ListenAndServe()
		assert.NoError(t, err)
		time.Sleep(500 * time.Millisecond)
		err = dht.Bootstrap(context.Background())
		assert.NoError(t, err)

	}

	return dhts, bootNode
}

func newTestKademlia(t *testing.T, ip, port string, d dht.DHT, b overlay.Node) *Kademlia {
	// rt, err := d.GetRoutingTable(context.Background())
	// assert.NoError(t, err)
	i, err := newID()
	assert.NoError(t, err)
	id := NodeID(i)
	n := []overlay.Node{b}

	kad, err := NewKademlia(&id, n, ip, port)
	assert.NoError(t, err)
	return kad
}

func TestBootstrap(t *testing.T) {
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "3001")

	defer func(d []dht.DHT) {
		for _, v := range d {
			v.Disconnect()
		}
	}(dhts)

	cases := []struct {
		k *Kademlia
	}{
		{
			k: newTestKademlia(t, "127.0.0.1", "3001", dhts[rand.Intn(testNetSize)], bootNode),
		},
	}

	for _, v := range cases {
		ctx := context.Background()
		// time.Sleep(time.Second)

		rt, err := dhts[4].GetRoutingTable(context.Background())
		assert.NoError(t, err)
		b, err := rt.GetBuckets()
		assert.NoError(t, err)
		fmt.Printf("RoutingTable: %#v\n", b)
		for i, vv := range b {
			if len(vv.Nodes()) != 0 {
				fmt.Printf("[%d] %#v\n", i, vv.Nodes())
				for i, vvv := range vv.Nodes() {
					fmt.Printf("[%d] %#v : %v\n", i, vvv, vvv.Address.String())
					fmt.Printf("[%d] %#v : %v\n", i, vvv, vvv.Address.String())
				}
			}
		}
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
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "6001")
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
			k:           newTestKademlia(t, "127.0.0.1", "6000", dhts[rand.Intn(testNetSize)], bootNode),
			limit:       10,
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		// fmt.Printf("\n\n\nHERE\n\n\n")
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
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "6001")
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
			k:           newTestKademlia(t, "127.0.0.1", "6000", dhts[rand.Intn(testNetSize)], bootNode),
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
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "6001")
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
			k: newTestKademlia(t, "127.0.0.1", "6000", dhts[rand.Intn(testNetSize)], bootNode),
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
