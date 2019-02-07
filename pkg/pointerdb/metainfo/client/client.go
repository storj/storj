// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package metainfoclient

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/transport"
)

var (
	// ClientError wraps errors returned from client package
	ClientError = errs.Class("metainfo client error")
)

// SatelliteClient maintains variables required for talking to basic satellite endpoints
type SatelliteClient struct {
	log       *zap.Logger
	transport transport.Client
	satellite *pb.Node
}

// New creates an Satellite Client
func New(log *zap.Logger, transport transport.Client, satellite *pb.Node) *SatelliteClient {
	return &SatelliteClient{log: log, transport: transport, satellite: satellite}
}

// Stat will return the health of a specific path
func (sc *SatelliteClient) Stat(ctx context.Context, path []byte, bucket []byte) (*pb.ObjectHealthResponse, error) {
	// Create client from satellite ip
	conn, err := sc.transport.DialNode(ctx, sc.satellite)
	if err != nil {
		return nil, ClientError.Wrap(err)
	}

	client := pb.NewMetainfoClient(conn)
	defer func() {
		err := conn.Close()
		if err != nil {
			sc.log.Warn("Satellite Client failed to close connection", zap.Error(err))
		}
	}()

	req := &pb.ObjectHealthRequest{
		EncryptedPath:     path,
		Bucket: 			bucket,
		UplinkId: sc.transport.Identity().ID,
	}

	return client.Health(ctx, req)
}
