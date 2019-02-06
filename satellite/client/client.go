// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/kademlia"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/pkg/transport"
)

var (
	// ClientError wraps errors returned from client package
	ClientError = errs.Class("satellite client error")
)

// AgreementSender maintains variables required for reading bandwidth agreements from a DB and sending them to a Payers
type SatelliteClient struct {
	log       *zap.Logger
	transport transport.Client
	kad       *kademlia.Kademlia
}

// New creates an Satellite Client
func New(log *zap.Logger, identity *identity.FullIdentity, kad *kademlia.Kademlia) *SatelliteClient {
	return &SatelliteClient{log: log, transport: transport.NewClient(identity), kad: kad}
}

//SendAgreementsToSatellite uploads agreements to the satellite
func (sc *SatelliteClient) Stat(ctx context.Context, satID storj.NodeID, path storj.Path) (*pb.FileHealthResponse, error) {
	// todo: cache kad responses if this interval is very small
	// Get satellite ip from kademlia
	satellite, err := sc.kad.FindNode(ctx, satID)
	if err != nil {
		return nil, err
	}
	// Create client from satellite ip
	conn, err := sc.transport.DialNode(ctx, &satellite)
	if err != nil {
		return nil, err
	}

	client := pb.NewSatelliteClient(conn)
	defer func() {
		err := conn.Close()
		if err != nil {
			sc.log.Warn("Satellite Client failed to close connection", zap.Error(err))
		}
	}()

	req := &pb.FileHealthRequest{
		Path:     path,
		UplinkId: sc.transport.Identity().ID,
	}

	return client.Health(ctx, req)
}
