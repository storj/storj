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

// IsNodeAvailable is a method for checking if node is up
func (s *Service) IsNodeAvailable(ctx context.Context, nodeID pb.NodeID) (bool, error) {
	_, err := s.kademlia.FetchPeerIdentity(ctx, nodeID)

	isNodeAvailable := err == nil

	return isNodeAvailable, err
}
