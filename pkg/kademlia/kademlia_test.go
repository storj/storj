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

	bkad "github.com/coyle/kademlia"
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
		id, err := newID()
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

	n := []overlay.Node{rt.Local()},
	

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

		n := NodeID(dhts[0].HT.Self.ID)
		node, err := v.k.FindNode(ctx, &n)
		assert.NoError(t, err)
		assert.NotEmpty(t, node)
		assert.Equal(t, string(dhts[0].HT.Self.ID), node.Id)
		v.k.dht.Disconnect()
	}

}

func TestGetNodes(t *testing.T) {
	dhts := bootstrapTestNetwork(t, "127.0.0.1", "6001")
	defer func(d []*bkad.DHT) {
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
			start:       string(dhts[0].HT.Self.ID),
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

		nodes, err := v.k.GetNodes(ctx, v.start, v.limit)
		assert.Equal(t, v.expectedErr, err)
		assert.Len(t, nodes, v.limit)
		v.k.dht.Disconnect()
	}

}

func TestFindNode(t *testing.T) {
	dhts := bootstrapTestNetwork(t, "127.0.0.1", "6001")
	defer func(d []*bkad.DHT) {
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
			start:       string(dhts[0].HT.Self.ID),
			input:       NodeID(dhts[rand.Intn(testNetSize)].HT.Self.ID),
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		ctx := context.Background()
		go v.k.ListenAndServe()
		time.Sleep(time.Second)
		err := v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		node, err := v.k.FindNode(ctx, &v.input)
		assert.Equal(t, v.expectedErr, err)
		assert.NotZero(t, node)
		assert.Equal(t, node.Id, string(v.input))
		v.k.dht.Disconnect()
	}

}

func TestPing(t *testing.T) {
	dhts := bootstrapTestNetwork(t, "127.0.0.1", "6001")
	defer func(d []*bkad.DHT) {
		for _, v := range d {
			v.Disconnect()
		}
	}(dhts)

	r := dhts[rand.Intn(testNetSize)]
	cases := []struct {
		k           *Kademlia
		input       overlay.Node
		expectedErr error
	}{
		{
			k: newTestKademlia(t, "127.0.0.1", "6000", dhts[rand.Intn(testNetSize)]),
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
