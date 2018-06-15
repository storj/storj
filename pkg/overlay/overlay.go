// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"context"

	"go.uber.org/zap"
	"gopkg.in/spacemonkeygo/monkit.v2"
	"storj.io/storj/pkg/kademlia"
	proto "storj.io/storj/protos/overlay" // naming proto to avoid confusion with this package
)

// Overlay implements our overlay RPC service
type Overlay struct {
	kad     *kademlia.Kademlia
	cache   *Cache
	logger  *zap.Logger
	metrics *monkit.Registry
}

var dummy = map[string]string{
	"bb508cf5d944de7d9735": "35.232.47.152:4242",
	"f14ee204da247d128471": "35.226.188.0:4242",
	"88ab6c3448e58d4e3ebc": "35.202.182.77:4242",
	"28e6214cf108310114ef": "35.232.88.171:4242",
	"f6e73b480cbd9b9f0eb0": "35.225.50.137:4242",
	"d3992b73cd7c8f28aef2": "130.211.168.182:4242",
	"22202445d7dbd97442f8": "104.197.5.134:4242",
	"b0f2ab9fb9b7b8d9f315": "35.192.126.41:4242",
	"9aa60c03f4b258b1484c": "35.193.196.52:4242",
	"f7fca359f0a8923cd32e": "130.211.203.52:4242",
	"443ceeb614984d1532aa": "35.224.122.76:4242",
	"855ee21c54ebf6de9f0a": "104.198.233.25:4242",
	"55f355c702a44b1fcc98": "35.226.21.152:4242",
	"0b6c79b04726f9d773ea": "130.211.221.239:4242",
	"4c59f27c21c38188f725": "35.202.144.175:4242",
	"b1074ce711fc512e86eb": "130.211.190.250:4242",
	"7bfbf8501d98f762d2ab": "35.194.56.141:4242",
	"94777159ed18ba146ea3": "35.192.108.107:4242",
	"93f407c459a51f5e0b77": "35.202.91.191:4242",
	"fa3327e79aeaaba02497": "35.192.7.33:4242",
}

// Lookup finds the address of a node in our overlay network
func (o *Overlay) Lookup(ctx context.Context, req *proto.LookupRequest) (*proto.LookupResponse, error) {
	na, err := o.cache.Get(ctx, req.NodeID)
	if err != nil {
		o.logger.Error("Error looking up node", zap.Error(err), zap.String("nodeID", req.NodeID))
		return nil, err
	}

	return &proto.LookupResponse{
		NodeAddress: &proto.NodeAddress{
			Transport: proto.NodeTransport_TCP, Address: dummy[req.NodeID],
		},
	}, nil
}

// FindStorageNodes searches the overlay network for nodes that meet the provided requirements
func (o *Overlay) FindStorageNodes(ctx context.Context, req *proto.FindStorageNodesRequest) (*proto.FindStorageNodesResponse, error) {
	// NB:  call FilterNodeReputation from node_reputation package to find nodes for storage

	// TODO(coyle): need to determine if we will pull the startID and Limit from the request or just use hardcoded data
	// for now just using 40 for demos and empty string which will default the Id to the kademlia node doing the lookup
	// nodes, err := o.kad.GetNodes(ctx, "", 40)
	// if err != nil {
	// 	o.logger.Error("Error getting nodes", zap.Error(err))
	// 	return nil, err
	// }

	return &proto.FindStorageNodesResponse{
		Node: dummyData(),
	}, nil
}

func dummyData() []*proto.Node {

	r := []*proto.Node{}

	for i, v := range dummy {
		r = append(r, &proto.Node{Id: i, Address: &proto.NodeAddress{Transport: proto.NodeTransport_TCP, Address: v}})
	}

	return r
}
