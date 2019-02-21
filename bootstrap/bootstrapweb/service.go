// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bootstrapweb

import (
	"context"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
)

const (
	findNearLimit = 1
)

// Service is handling bootstrap related logic
type Service struct {
	log      *zap.Logger
	kademlia *kademlia.Kademlia
}

// NewService returns new instance of Service
func NewService(log *zap.Logger, kademlia *kademlia.Kademlia) (*Service, error) {
	if log == nil {
		return nil, errs.New("log can't be nil")
	}

	if kademlia == nil {
		return nil, errs.New("kademlia can't be nil")
	}

	return &Service{log: log, kademlia: kademlia}, nil
}

// IsNodeUp is a method for checking if node is up
func (s *Service) IsNodeUp(ctx context.Context, nodeID pb.NodeID) (bool, error) {
	nodes, err := s.kademlia.FindNear(ctx, nodeID, findNearLimit)
	if err != nil {
		return false, err
	}

	if len(nodes) == 0 {
		return false, errs.New("node not found")
	}

	isNodeFound := nodes[0].Id == nodeID

	return isNodeFound, nil
}
