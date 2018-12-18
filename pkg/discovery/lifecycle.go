package discovery

import (
	"context"

	"go.uber.org/zap"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// ConnectionSuccess is called when a node is connected to
func (d *Discovery) ConnectionSuccess(address string, id storj.NodeID) {
	err := d.cache.Put(context.Background(), id, pb.Node{
		Address: &pb.NodeAddress{
			Address: address,
		},
		Id: id,
	})
	if err != nil {
		zap.S().Error("error adding node to cache", zap.Error(err))
	}
}

// ConnectionFailure gets called when a node fails to be connected to
func (d *Discovery) ConnectionFailure(id storj.NodeID) {
	err := d.cache.Put(context.Background(), id, pb.Node{})
	if err != nil {
		zap.S().Error("error removing node from cache")
	}
}

// GracefulDisconnect is called when a node alerts the network they're
// going offline for a short period of time with intent to come back
func (d *Discovery) GracefulDisconnect(id storj.NodeID) {

}

// ConnFailure implements the Transport failure function
func (d *Discovery) ConnFailure(node *pb.Node, err error) {
	return
}

// ConnSuccess implements the Transport success function
func (d *Discovery) ConnSuccess(node *pb.Node) {
	return
}
