// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psclient

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

// LiteClient is the lightweight client for getting stats
type LiteClient interface {
	Stats(ctx context.Context) (*pb.StatSummary, error)
	Dashboard(ctx context.Context) (pb.PieceStoreRoutes_DashboardClient, error)
}

// PieceStoreLite is the struct that holds the client
type PieceStoreLite struct {
	client pb.PieceStoreRoutesClient
}

// Dashboard returns a simple terminal dashboard displaying info
func (psl *PieceStoreLite) Dashboard(ctx context.Context) (pb.PieceStoreRoutes_DashboardClient, error) {
	return psl.client.Dashboard(ctx, &pb.DashboardReq{})
}

// Stats will retrieve stats about a piece storage node
func (psl *PieceStoreLite) Stats(ctx context.Context) (*pb.StatSummary, error) {
	return psl.client.Stats(ctx, &pb.StatsReq{})
}

// NewLiteClient returns a new LiteClient
func NewLiteClient(ctx context.Context, tc transport.Client, n *pb.Node) (LiteClient, error) {
	conn, err := tc.DialNode(ctx, n)
	if err != nil {
		return nil, err
	}

	return &PieceStoreLite{
		client: pb.NewPieceStoreRoutesClient(conn),
	}, nil
}
