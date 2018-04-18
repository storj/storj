package redis

import (
	"time"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/protos/overlay"
)

const defaultNodeExpiration = 61 * time.Minute

// OverlayClient is used to store overlay data in Redis
type OverlayClient struct {
	DB Client
}

// NewOverlayClient returns a pointer to a new OverlayClient instance with an initalized connection to Redis.
func NewOverlayClient(address, password string, db int) (*OverlayClient, error) {
	rc, err := NewRedisClient(address, password, db)
	if err != nil {
		return nil, err
	}

	o := &OverlayClient{
		DB: rc,
	}

	// ping here to verify we are able to connect to the redis instacne with the initialized client.
	if err := o.DB.Ping(); err != nil {
		return nil, err
	}

	return o, nil
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
