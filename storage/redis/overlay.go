// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package redis

import (
	"context"
	"errors"
	"time"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/protos/overlay"
)

const defaultNodeExpiration = 61 * time.Minute

// OverlayClient is used to store overlay data in Redis
type OverlayClient struct {
	DB  Client
	DHT kademlia.DHT
}

// NewOverlayClient returns a pointer to a new OverlayClient instance with an initalized connection to Redis.
func NewOverlayClient(address, password string, db int, DHT kademlia.DHT) (*OverlayClient, error) {
	rc, err := NewRedisClient(address, password, db)
	if err != nil {
		return nil, err
	}

	return &OverlayClient{
		DB: rc,
	}, nil
}

// Get looks up the provided nodeID from the redis cache
func (o *OverlayClient) Get(key string) (*overlay.NodeAddress, error) {
	d, err := o.DB.Get(key)
	if err != nil {
		return nil, err
	}

	na := &overlay.NodeAddress{}

	return na, proto.Unmarshal(d, na)
}

// Set adds a nodeID to the redis cache with a binary representation of proto defined NodeAddress
func (o *OverlayClient) Set(nodeID string, value overlay.NodeAddress) error {
	data, err := proto.Marshal(&value)
	if err != nil {
		return err
	}

	return o.DB.Set(nodeID, data, defaultNodeExpiration)
}

// Bootstrap walks the initialized network and populates the cache
func (o *OverlayClient) Bootstrap(ctx context.Context) error {
	return errors.New("TODO")
}

// Refresh walks the network looking for new nodes and pings existing nodes to eliminate stale addresses
func (o *OverlayClient) Refresh(ctx context.Context) error {
	return errors.New("TODO")
}
