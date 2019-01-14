package psclient

import (
	"context"
	"fmt"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/provider"
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
func NewLiteClient(ctx context.Context) (LiteClient, error) {
	clientIdent, err := provider.NewFullIdentity(ctx, 12, 4)
	if err != nil {
		fmt.Printf("ERROR %+v\n", err)
		return nil, err
	}

	tc := transport.NewClient(clientIdent)
	n := &pb.Node{
		Address: &pb.NodeAddress{
			Address:   ":7777",
			Transport: 0,
		},
	}

	conn, err := tc.DialNode(ctx, n)
	if err != nil {
		return nil, err
	}

	return &PieceStoreLite{
		client: pb.NewPieceStoreRoutesClient(conn),
	}, nil
}
