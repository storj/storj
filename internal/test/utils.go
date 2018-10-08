// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
)

// NewNodeStorageValue provides a convient way to create a node as a storage.Value for testing purposes
func NewNodeStorageValue(t *testing.T, address string) storage.Value {
	na := &pb.Node{Id: "", Address: &pb.NodeAddress{Transport: pb.NodeTransport_TCP, Address: address}}
	d, err := proto.Marshal(na)
	assert.NoError(t, err)
	return d
}
