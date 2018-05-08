// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package netstate

import (
	"context"

	"go.uber.org/zap"

	proto "github.com/storj/protos/netstate"
	"github.com/storj/storage/boltdb"
)

// NetState implements the network state RPC service
type NetState struct {
	DB     *boltdb.Client
	logger *zap.Logger
}

// Put formats and hands off a file path to be saved to boltdb
func (n *NetState) Put(ctx context.Context, filepath *proto.FilePath) (*proto.PutResponse, error) {
	n.logger.Debug("entering NetState.Put(...)")
	return &proto.PutResponse{}, nil
}

func (n *NetState) Get(ctx context.Context, filepath *proto.FilePath) (*proto.GetResponse, error) {
	// TODO: call the bolt client
	return &proto.GetResponse{}, nil
}

func (n *NetState) List(ctx context.Context, req *proto.ListRequest) (*proto.ListResponse, error) {
	// TODO: call the bolt client
	return &proto.ListResponse{}, nil
}

func (n *NetState) Delete(ctx context.Context, filepath *proto.FilePath) (*proto.DeleteResponse, error) {
	// TODO: call the bolt client
	return &proto.DeleteResponse{}, nil
}
