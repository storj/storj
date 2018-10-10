// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"context"

	"storj.io/storj/pkg/pb"
)

// MockClient is a mock implementation of a Node client
type MockClient struct {
	response []*pb.Node
}

// Lookup is a mock of a node.Client Lookup
// it echoes the request as the stored response on the struct
func (mc *MockClient) Lookup(ctx context.Context, to pb.Node, find pb.Node) ([]*pb.Node, error) {
	return mc.response, nil
}

// NewMockClient initializes a mock client with the default values and returns a pointer to a MockClient
func NewMockClient(response []*pb.Node) *MockClient {
	return &MockClient{
		response: response,
	}
}
