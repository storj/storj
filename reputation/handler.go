package reputation

import (
	"golang.org/x/net/context"
)

// Server is a struct
type Server struct{}

// UpdateReputation in handler
func (s *Server) UpdateReputation(ctx context.Context, in *NodeUpdate) (*BridgeReply, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}

	status := insertNodeUpdate(db, in)

	return &BridgeReply{
		BridgeName: "Storj",
		NodeName:   in.NodeName,
		Status:     status,
	}, nil
}

// QueryAggregatedNodeInfo in handler
func (s *Server) QueryAggregatedNodeInfo(ctx context.Context, in *NodeQuery) (*NodeReputation, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}
	node := byNodeName(db, in.NodeName)

	return &node, nil
}
