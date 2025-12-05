// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package multinodedb

import (
	"context"
	"database/sql"
	"errors"

	"github.com/zeebo/errs"

	"storj.io/common/storj"
	"storj.io/storj/multinode/multinodedb/dbx"
	"storj.io/storj/multinode/nodes"
	"storj.io/storj/private/multinodeauth"
)

// ErrNodesDB indicates about internal NodesDB error.
var ErrNodesDB = errs.Class("NodesDB")

// ensures that nodesdb implements console.Nodes.
var _ nodes.DB = (*nodesdb)(nil)

// nodesdb exposes needed by MND NodesDB functionality.
// dbx implementation of console.Nodes.
//
// architecture: Database
type nodesdb struct {
	methods dbx.Methods
}

// List returns all connected nodes.
func (n *nodesdb) List(ctx context.Context) (allNodes []nodes.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxNodes, err := n.methods.All_Node(ctx)
	if err != nil {
		return []nodes.Node{}, ErrNodesDB.Wrap(err)
	}
	if len(dbxNodes) == 0 {
		return []nodes.Node{}, nodes.ErrNoNode.New("no nodes")
	}

	for _, dbxNode := range dbxNodes {
		node, err := fromDBXNode(ctx, dbxNode)
		if err != nil {
			return []nodes.Node{}, ErrNodesDB.Wrap(err)
		}

		allNodes = append(allNodes, node)
	}

	return allNodes, ErrNodesDB.Wrap(err)
}

// ListPaged returns paginated nodes list.
func (n *nodesdb) ListPaged(ctx context.Context, cursor nodes.Cursor) (page nodes.Page, err error) {
	defer mon.Task()(&ctx)(&err)
	page = nodes.Page{
		CurrentPage: cursor.Page,
		Limit:       cursor.Limit,
		Offset:      (cursor.Page - 1) * cursor.Limit,
	}
	totalCount, err := n.methods.Count_Node(ctx)
	if err != nil {
		return nodes.Page{}, ErrNodesDB.Wrap(err)
	}
	page.TotalCount = totalCount
	page.PageCount = page.TotalCount / cursor.Limit
	if page.TotalCount%cursor.Limit != 0 {
		page.PageCount++
	}
	dbxNodes, err := n.methods.Limited_Node(ctx, int(page.Limit), page.Offset)
	if err != nil {
		return nodes.Page{}, ErrNodesDB.Wrap(err)
	}
	for _, dbxNode := range dbxNodes {
		node, err := fromDBXNode(ctx, dbxNode)
		if err != nil {
			return nodes.Page{}, ErrNodesDB.Wrap(err)
		}
		page.Nodes = append(page.Nodes, node)
	}
	return page, nil
}

// Get return node from NodesDB by its id.
func (n *nodesdb) Get(ctx context.Context, id storj.NodeID) (_ nodes.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	dbxNode, err := n.methods.Get_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nodes.Node{}, nodes.ErrNoNode.Wrap(err)
		}
		return nodes.Node{}, ErrNodesDB.Wrap(err)
	}

	node, err := fromDBXNode(ctx, dbxNode)

	return node, ErrNodesDB.Wrap(err)
}

// Add creates new node in NodesDB.
func (n *nodesdb) Add(ctx context.Context, node nodes.Node) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = n.methods.Create_Node(
		ctx,
		dbx.Node_Id(node.ID.Bytes()),
		dbx.Node_Name(node.Name),
		dbx.Node_PublicAddress(node.PublicAddress),
		dbx.Node_ApiSecret(node.APISecret[:]),
	)

	return ErrNodesDB.Wrap(err)
}

// Remove removed node from NodesDB.
func (n *nodesdb) Remove(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx)(&err)

	_, err = n.methods.Delete_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()))

	return ErrNodesDB.Wrap(err)
}

// UpdateName will update name of the specified node in database.
func (n *nodesdb) UpdateName(ctx context.Context, id storj.NodeID, name string) (err error) {
	defer mon.Task()(&ctx)(&err)

	err = n.methods.UpdateNoReturn_Node_By_Id(ctx, dbx.Node_Id(id.Bytes()), dbx.Node_Update_Fields{
		Name: dbx.Node_Name(name),
	})

	return ErrNodesDB.Wrap(err)
}

// fromDBXNode converts dbx.Node to console.Node.
func fromDBXNode(ctx context.Context, node *dbx.Node) (_ nodes.Node, err error) {
	defer mon.Task()(&ctx)(&err)

	id, err := storj.NodeIDFromBytes(node.Id)
	if err != nil {
		return nodes.Node{}, err
	}

	secret, err := multinodeauth.SecretFromBytes(node.ApiSecret)
	if err != nil {
		return nodes.Node{}, err
	}

	result := nodes.Node{
		ID:            id,
		APISecret:     secret,
		Name:          node.Name,
		PublicAddress: node.PublicAddress,
	}

	return result, nil
}
