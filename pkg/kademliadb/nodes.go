// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package kademliadb

import (
	"context"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/storage"
)

type Nodes struct {
	db storage.KeyValueStore
}

func (n *Nodes) Put(ctx context.Context, node *pb.Node) error {
	defer mon.Task()(&ctx)(&err)
	v, err := proto.Marshal(node)
	if err != nil {
		return kadDBError.Wrap(err)
	}

	err = n.db.Put(ctx, node.Id.Bytes(), v)
	if err != nil {
		return kadDBError.New("could not node to NodesDB: %s", err)
	}
	return nil
}

func (n *Nodes) Get(ctx context.Context, key []byte) (value []byte, err error) {}

func (n *Nodes) GetAll(ctx context.Context, keys [][]byte) (values [][]byte, err error) {}

func (n *Nodes) Delete(ctx context.Context, key []byte) error {}

func (n *Nodes) List(ctx context.Context, start []byte, limit int) (keys [][]byte, err error) {}

func (n *Nodes) Close() error {}