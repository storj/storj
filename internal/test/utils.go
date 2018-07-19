// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"testing"

	pb "github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay"
	"storj.io/storj/storage"
)

// NewNodeAddressValue provides a convient way to create a storage.Value for testing purposes
func NewNodeAddressValue(t *testing.T, address string) storage.Value {
	na := &proto.NodeAddress{Transport: proto.NodeTransport_TCP, Address: address}
	d, err := pb.Marshal(na)
	assert.NoError(t, err)

	return d
}

// NewNodeID returns the string representation of a dht node ID
func NewNodeID(t *testing.T) string {
	id, err := kademlia.NewID()
	assert.NoError(t, err)

	return id.String()
}
