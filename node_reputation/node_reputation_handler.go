// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package nodereputation

import (
	"golang.org/x/net/context"
)

// Server is a struct
type Server struct{}

// UpdateReputation in handler
func (s *Server) UpdateReputation(ctx context.Context, in *NodeUpdate) (*UpdateReply, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}

	status := insertNodeUpdate(db, in)

	return &UpdateReply{
		BridgeName: "Storj",
		NodeName:   in.NodeName,
		Status:     status,
	}, nil
}

// QueryAggregatedNodeInfo in handler
func (s *Server) QueryAggregatedNodeInfo(ctx context.Context, in *NodeQuery) (*NodeReputationRecord, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}
	node := byNodeName(db, in.NodeName)

	return &node, nil
}
