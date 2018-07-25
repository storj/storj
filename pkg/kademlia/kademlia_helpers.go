// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	// "context"
	"crypto/rand"
	// "fmt"
	"log"
	// "net"
	// "strconv"

	// bkad "github.com/coyle/kademlia"
	"github.com/zeebo/errs"

	// "storj.io/storj/pkg/dht"
	proto "storj.io/storj/protos/overlay"
)



// NodeErr is the class for all errors pertaining to node operations
var NodeErr = errs.Class("node error")

//TODO: shouldn't default to TCP but not sure what to do yet
var defaultTransport = proto.NodeTransport_TCP

// KadOptions for configuring Kademlia
type KadOptions struct {
	
}

// FindNodes ...TODO
func FindNodes() {

}

// ListenAndServe connects the kademlia node to the network and listens for incoming requests
func (k *Kademlia) ListenAndServe() error {
	if err := k.dht.CreateSocket(); err != nil {
		return err
	}

	go func() {
		if err := k.dht.Listen(); err != nil {
			log.Printf("Failed to listen on the dht: %s\n", err)
		}
	}()

	return nil
}


// newID generates a new random ID.
// This purely to get things working. We shouldn't use this as the ID in the actual network
func newID() ([]byte, error) {
	result := make([]byte, 20)
	_, err := rand.Read(result)
	return result, err
}

// GetIntroNode determines the best node to bootstrap a new node onto the network
func GetIntroNode(id, ip, port string) (*proto.Node, error) {
	addr := "bootstrap.storj.io:8080"
	if ip != "" && port != "" {
		addr = ip + ":" + port
	}

	if id == "" {
		i, err := newID()
		if err != nil {
			return nil, err
		}

		id = string(i)
	}

	return &proto.Node{
		Id: id,
		Address: &proto.NodeAddress{
			Transport: defaultTransport,
			Address:   addr,
		},
	}, nil
}
