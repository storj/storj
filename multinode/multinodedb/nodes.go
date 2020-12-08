// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinodedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/multinode/console"
	"storj.io/storj/multinode/multinodedb/dbx"
)

// NodesDBError indicates about internal NodesDB error.
var NodesDBError = errs.Class("NodesDB error")

// ensures that nodes implements console.Nodes.
var _ console.Nodes = (*nodes)(nil)

// nodes exposes needed by MND NodesDB functionality.
// dbx implementation of console.Nodes.
//
// architecture: Database
type nodes struct {
	methods dbx.Methods
}

// GetAll returns all connected nodes.
func (n *nodes) GetAll(ctx context.Context) (nodes []console.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxNodes, err := n.methods.All_Node(ctx)
	if err != nil {
		return []console.Node{}, NodesDBError.Wrap(err)
	}
	if len(dbxNodes) == 0 {
		return []console.Node{}, console.ErrNoNode.New("no nodes")
	}

	for _, dbxNode := range dbxNodes {
		node, err := fromDBXNode(ctx, dbxNode)
		if err != nil {
			return []console.Node{}, NodesDBError.Wrap(err)
		}

		nodes = append(nodes, node)
	}

	return nodes, NodesDBError.Wrap(err)
}

// GetByID return node from NodesDB by its id.
func (n *nodes) GetByID(ctx context.Context, id storj.NodeID) (_ console.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxNode, err := n.methods.Get_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return console.Node{}, console.ErrNoNode.Wrap(err)
		}
		return console.Node{}, NodesDBError.Wrap(err)
	}

	node, err := fromDBXNode(ctx, dbxNode)

	return node, NodesDBError.Wrap(err)
}

// Add creates new node in NodesDB.
func (n *nodes) Add(ctx context.Context, id storj.NodeID, apiSecret []byte, publicAddress string) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = n.methods.Create_Node(
		ctx,
		dbx.Node_Id(id.Bytes()),
		dbx.Node_Name(""),
		dbx.Node_PublicAddress(publicAddress),
		dbx.Node_ApiSecret(apiSecret),
	)

	return NodesDBError.Wrap(err)
}

// Remove removed node from NodesDB.
func (n *nodes) Remove(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = n.methods.Delete_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))

	return NodesDBError.Wrap(err)
}

// fromDBXNode converts dbx.Node to console.Node.
func fromDBXNode(ctx context.Context, node *dbx.Node) (_ console.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	id, err := storj.NodeIDFromBytes(node.Id)
	if err != nil {
		return console.Node{}, err
	}

	result := console.Node{
		ID:            id,
		APISecret:     node.ApiSecret,
		Name:          node.Name,
		PublicAddress: node.PublicAddress,
	}

	return result, nil
}
