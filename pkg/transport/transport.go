// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package transport

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gortc/stun"
	"google.golang.org/grpc"

	proto "storj.io/storj/protos/overlay"
)

// Transport interface structure
type Transport struct {
}

// NewClient returns a newly instantiated Transport Client
func NewClient() *Transport {
	return &Transport{}
}

// DialNode using the authenticated mode
func (o *Transport) DialNode(ctx context.Context, node *proto.Node) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

	if node.Address == nil {
		return nil, Error.New("no address")
	}
	/* TODO@ASK security feature under development */
	return o.DialUnauthenticated(ctx, *node.Address)
}

// DialUnauthenticated using unauthenticated mode
func (o *Transport) DialUnauthenticated(ctx context.Context, addr proto.NodeAddress) (conn *grpc.ClientConn, err error) {
	defer mon.Task()(&ctx)(&err)

	if addr.Address == "" {
		return nil, Error.New("no address")
	}

	return grpc.Dial(addr.Address, grpc.WithInsecure())
}

// Traversal finds a clients publicly expose address and port
func (o *Transport) Traversal(ctx context.Context) {
	// stun.l.google.com:19302
	c, err := stun.Dial("udp", "0.0.0.0:57508")
	if err != nil {
		log.Fatal("dial:", err)
	}

	deadline := time.Now().Add(time.Second * 5)

	if err := c.Do(stun.MustBuild(stun.TransactionID, stun.BindingRequest), deadline, func(res stun.Event) {
		if res.Error != nil {
			log.Fatalln(err)
		}
		var xorAddr stun.XORMappedAddress
		if err := xorAddr.GetFrom(res.Message); err != nil {
			log.Fatalln(err)
		}
		fmt.Println(xorAddr)
	}); err != nil {
		log.Fatal("do:", err)
	}
	if err := c.Close(); err != nil {
		log.Fatalln(err)
	}

}
