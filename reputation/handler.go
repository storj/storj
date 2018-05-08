package reputation

import (
	"golang.org/x/net/context"
)

type Server struct{}

// UpdateReputation in handler
func (s *Server) UpdateReputation(ctx context.Context, in *NodeReputation) (*BridgeReply, error) {
	return &BridgeReply{
		BridgeName: "Storj",
		NodeName:   "Alice",
		Status:     1,
	}, nil
}

func (s *Server) QueryAggregatedNodeInfo(ctx context.Context, in *NodeQuery) (*NodeReputation, error) {

	return &NodeReputation{
		Source:             "Bob",
		NodeName:           "Alice",
		Timestamp:          "",
		Uptime:             1,
		AuditSuccess:       1,
		AuditFail:          0,
		Latency:            1,
		AmountOfDataStored: 1,
		FalseClaims:        0,
		ShardsModified:     0,
	}, nil
}
