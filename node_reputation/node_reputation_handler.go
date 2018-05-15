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

// NodeReputation in handler
func (s *Server) NodeReputation(ctx context.Context, in *NodeQuery) (*NodeReputationRecord, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}
	node, err := byNodeName(db, in.NodeName)
	if err != nil {
		return nil, err
	}

	return &node, nil
}

// FilterNodeReputation in handler
func (s *Server) FilterNodeReputation(ctx context.Context, in *NodeFilter) (*NodeReputationRecords, error) {
	db, err := SetServerDB("./Server.db")
	if err != nil {
		return nil, err
	}
	nodes, err := selectNodeWhere(db, in.ColumnName, in.Operand, in.ColumnValue)
	if err != nil {
		return nil, err
	}

	return &nodes, nil
}
