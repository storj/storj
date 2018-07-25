// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"context"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/node"
	"storj.io/storj/protos/overlay"
)

const (
	testNetSize      = 20
	idDifficulty     = 1
	idHashLen        = 16
	idGenConcurrency = 2
)

func newNodeID(t *testing.T) dht.NodeID {
	id, err := node.NewID(idDifficulty, idHashLen, idGenConcurrency)
	assert.NoError(t, err)

	return id
}

func bootstrapTestNetwork(t *testing.T, ip, port string) ([]dht.DHT, overlay.Node) {
	bnid := newNodeID(t)
	dhts := []dht.DHT{}

	p, err := strconv.Atoi(port)
	pm := strconv.Itoa(p)
	assert.NoError(t, err)
	intro, err := GetIntroNode(bnid.String(), ip, pm)
	assert.NoError(t, err)

	boot, err := NewKademlia(bnid, []overlay.Node{*intro}, ip, pm)

	assert.NoError(t, err)
	rt, err := boot.GetRoutingTable(context.Background())
	bootNode := rt.Local()

	err = boot.ListenAndServe()
	assert.NoError(t, err)
	p++

	err = boot.Bootstrap(context.Background())
	assert.NoError(t, err)
	for i := 0; i < testNetSize; i++ {
		gg := strconv.Itoa(p)

		id := newNodeID(t)
		dht, err := NewKademlia(id, []overlay.Node{bootNode}, ip, gg)
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

func newTestKademlia(t *testing.T, ip, port string, d dht.DHT, b overlay.Node) *Kademlia {
	id := newNodeID(t)
	n := []overlay.Node{b}

	kad, err := NewKademlia(id, n, ip, port)
	assert.NoError(t, err)
	return kad
}

func TestBootstrap(t *testing.T) {
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "3000")

	defer func(d []dht.DHT) {
		for _, v := range d {
			v.Disconnect()
		}
	}(dhts)

	cases := []struct {
		k *Kademlia
	}{
		{
			k: newTestKademlia(t, "127.0.0.1", "2999", dhts[rand.Intn(testNetSize)], bootNode),
		},
	}

	for _, v := range cases {
		defer v.k.Disconnect()
		err := v.k.ListenAndServe()
		assert.NoError(t, err)
		err = v.k.Bootstrap(context.Background())
		assert.NoError(t, err)
		ctx := context.Background()

		rt, err := dhts[0].GetRoutingTable(context.Background())
		assert.NoError(t, err)

		localID := rt.Local().Id
		n, err := node.ParsePeerIdentity(localID)
		assert.NoError(t, err)

		foundNode, err := v.k.FindNode(ctx, n)
		assert.NoError(t, err)
		assert.NotEmpty(t, foundNode)
		assert.Equal(t, localID, foundNode.Id)
		v.k.dht.Disconnect()
	}

}

func TestGetNodes(t *testing.T) {
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "6001")
	defer func(d []dht.DHT) {
		for _, v := range d {
			err := v.Disconnect()
			assert.NoError(t, err)
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
		defer v.k.Disconnect()
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
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "5001")
	defer func(d []dht.DHT) {
		for _, v := range d {
			err := v.Disconnect()
			assert.NoError(t, err)
		}
	}(dhts)

	cases := []struct {
		k           *Kademlia
		start       string
		input       dht.NodeID
		expectedErr error
	}{
		{
			k:           newTestKademlia(t, "127.0.0.1", "6000", dhts[rand.Intn(testNetSize)], bootNode),
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		defer v.k.Disconnect()
		ctx := context.Background()
		go v.k.ListenAndServe()
		time.Sleep(time.Second)
		err := v.k.Bootstrap(ctx)
		assert.NoError(t, err)

		rt, err := dhts[rand.Intn(testNetSize)].GetRoutingTable(context.Background())
		assert.NoError(t, err)

		id, err := node.ParsePeerIdentity(rt.Local().Id)
		assert.NoError(t, err)

		node, err := v.k.FindNode(ctx, id)
		assert.Equal(t, v.expectedErr, err)
		assert.NotZero(t, node)
		assert.Equal(t, node.Id, id.String())
	}

}

func TestPing(t *testing.T) {
	dhts, bootNode := bootstrapTestNetwork(t, "127.0.0.1", "4001")
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
		defer v.k.Disconnect()
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

// func TestNewClient_LoadTLS(t *testing.T) {
// 	var err error
//
// 	tmpPath, err := ioutil.TempDir("", "TestNewClient")
// 	assert.NoError(t, err)
// 	defer os.RemoveAll(tmpPath)
//
// 	basePath := filepath.Join(tmpPath, "TestNewClient_LoadTLS")
// 	_, err = peertls.NewTLSHelper(nil)
//
// 	assert.NoError(t, err)
//
// 	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", 0))
// 	assert.NoError(t, err)
// 	// NB: do NOT create a cert, it should be loaded from disk
// 	srv, tlsH := newMockTLSServer(t, basePath, false)
//
// 	go srv.Serve(lis)
// 	defer srv.Stop()
//
// 	address := lis.Addr().String()
// 	c, err := NewClient(&address, tlsH.DialOption())
// 	assert.NoError(t, err)
//
// 	r, err := c.Lookup(context.Background(), &proto.LookupRequest{})
// 	assert.NoError(t, err)
// 	assert.NotNil(t, r)
// }
