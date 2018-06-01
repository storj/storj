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

	bkad "github.com/coyle/kademlia"
	"github.com/stretchr/testify/assert"
	"storj.io/storj/protos/overlay"
)

const (
	testNetSize = 20
)

func bootstrapTestNetwork(ip, port string) []*bkad.DHT {
	dhts := []*bkad.DHT{}
	p, err := strconv.Atoi(port)
	if err != nil {
		panic(err)
	}

	for i := 0; i < testNetSize; i++ {
		id := newID()
		dht, _ := bkad.NewDHT(&bkad.MemoryStore{}, &bkad.Options{
			ID:   id,
			IP:   ip,
			Port: strconv.Itoa(p),
			BootstrapNodes: []*bkad.NetworkNode{
				bkad.NewNetworkNode("127.0.0.1", strconv.Itoa(p-1)),
			},
		})
		p++
		dhts = append(dhts, dht)
		if err := dht.CreateSocket(); err != nil {
			panic(err)
		}
	}

	for _, dht := range dhts {
		go dht.Listen()
		go func(dht *bkad.DHT) {
			if err := dht.Bootstrap(); err != nil {
				panic(err)
			}
		}(dht)

		time.Sleep(200 * time.Millisecond)
	}

	return dhts
}

func newTestKademlia(ip, port string, d *bkad.DHT) Kademlia {
	n := []overlay.Node{
		overlay.Node{
			Id: string(d.HT.Self.ID),
			Address: &overlay.NodeAddress{
				Address: fmt.Sprintf("127.0.0.1:%d", d.HT.Self.Port),
			},
		},
	}

	return NewKademlia(n, ip, port, false)
}

func TestBootstrap(t *testing.T) {
	dhts := bootstrapTestNetwork("127.0.0.1", "3001")
	defer func(d []*bkad.DHT) {
		for _, v := range d {
			v.Disconnect()
		}
	}(dhts)

	cases := []struct {
		k Kademlia
	}{
		{
			k: newTestKademlia("127.0.0.1", "3000", dhts[rand.Intn(testNetSize)]),
		},
	}

	for _, v := range cases {
		ctx := context.Background()
		err := v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		node, err := v.k.FindNode(ctx, NodeID(dhts[0].HT.Self.ID))
		assert.NoError(t, err)
		assert.NotEmpty(t, node)
		assert.Equal(t, string(dhts[0].HT.Self.ID), node.Id)
		v.k.dht.Disconnect()
	}

}

func TestGetNodes(t *testing.T) {
	dhts := bootstrapTestNetwork("127.0.0.1", "6001")
	defer func(d []*bkad.DHT) {
		for _, v := range d {
			v.Disconnect()
		}
	}(dhts)

	cases := []struct {
		k           Kademlia
		start       string
		limit       int
		expectedErr error
	}{
		{
			k:           newTestKademlia("127.0.0.1", "6000", dhts[rand.Intn(testNetSize)]),
			start:       string(dhts[0].HT.Self.ID),
			limit:       10,
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		ctx := context.Background()
		err := v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		nodes, err := v.k.GetNodes(ctx, v.start, v.limit)
		assert.Equal(t, v.expectedErr, err)
		assert.Len(t, nodes, v.limit)
		v.k.dht.Disconnect()
	}

}

func TestFindNode(t *testing.T) {
	dhts := bootstrapTestNetwork("127.0.0.1", "6001")
	defer func(d []*bkad.DHT) {
		for _, v := range d {
			v.Disconnect()
		}
	}(dhts)

	cases := []struct {
		k           Kademlia
		start       string
		input       NodeID
		expectedErr error
	}{
		{
			k:           newTestKademlia("127.0.0.1", "6000", dhts[rand.Intn(testNetSize)]),
			start:       string(dhts[0].HT.Self.ID),
			input:       NodeID(dhts[rand.Intn(testNetSize)].HT.Self.ID),
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		ctx := context.Background()
		err := v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		node, err := v.k.FindNode(ctx, v.input)
		assert.Equal(t, v.expectedErr, err)
		assert.NotZero(t, node)
		assert.Equal(t, node.Id, string(v.input))
		v.k.dht.Disconnect()
	}

}

func TestPing(t *testing.T) {
	dhts := bootstrapTestNetwork("127.0.0.1", "6001")
	defer func(d []*bkad.DHT) {
		for _, v := range d {
			v.Disconnect()
		}
	}(dhts)

	r := dhts[rand.Intn(testNetSize)]
	cases := []struct {
		k           Kademlia
		input       overlay.Node
		expectedErr error
	}{
		{
			k: newTestKademlia("127.0.0.1", "6000", dhts[rand.Intn(testNetSize)]),
			input: overlay.Node{
				Id: string(r.HT.Self.ID),
				Address: &overlay.NodeAddress{
					Transport: defaultTransport,
					Address:   fmt.Sprintf("%s:%d", r.HT.Self.IP.String(), r.HT.Self.Port),
				},
			},
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		ctx := context.Background()
		err := v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		node, err := v.k.Ping(ctx, v.input)
		assert.Equal(t, v.expectedErr, err)
		assert.NotEmpty(t, node)
		assert.Equal(t, v.input, node)
		v.k.dht.Disconnect()
	}

}
