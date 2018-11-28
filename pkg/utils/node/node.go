package nodeutil

import (
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// CopyNode returns a deep copy of a node
// It would be better to use `proto.Clone` but it is curently incompatible
// with gogo's customtype extension.
// (see https://github.com/gogo/protobuf/issues/147)
func CopyNode(src *pb.Node) (dst *pb.Node) {
	node := pb.Node{Id: storj.NodeID{}}
	copy(node.Id[:], src.Id[:])
	if src.Address != nil {
		node.Address = &pb.NodeAddress{
			Transport: src.Address.Transport,
			Address: src.Address.Address,
		}
	}
	if src.Metadata != nil {
		node.Metadata = &pb.NodeMetadata{
			Email: src.Metadata.Email,
			Wallet: src.Metadata.Wallet,
		}
	}
	if src.Restrictions != nil {
		node.Restrictions = &pb.NodeRestrictions{
			FreeBandwidth: src.Restrictions.FreeBandwidth,
			FreeDisk: src.Restrictions.FreeDisk,
		}
	}

	node.AuditSuccess = src.AuditSuccess
	node.IsUp = src.IsUp
	node.LatencyList = src.LatencyList
	node.Type = src.Type
	node.UpdateAuditSuccess = src.UpdateAuditSuccess
	node.UpdateLatency = src.UpdateLatency
	node.UpdateUptime = src.UpdateUptime

	return &node
}
