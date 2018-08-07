// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"

	proto "storj.io/storj/protos/overlay"
)

// MockClient is a mock implementation of a Node client
type MockClient struct {
	response []*proto.Node
}

// Lookup is a mock of a node.Client Lookup
// it echoes the request as the stored response on the struct
func (mc *MockClient) Lookup(ctx context.Context, to proto.Node, find proto.Node) ([]*proto.Node, error) {
	return mc.response, nil
}

// NewMockClient initalizes a mock client with the default values and returns a pointer to a MockClient
func NewMockClient(response []*proto.Node) *MockClient {
	return &MockClient{
		response: response,
	}
}
