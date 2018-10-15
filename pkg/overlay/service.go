// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package overlay

import (
	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
)

// NewClient connects to grpc server at the provided address with the provided options
// returns a new instance of an overlay Client
func NewClient(serverAddr string, opts ...grpc.DialOption) (pb.OverlayClient, error) {
	conn, err := grpc.Dial(serverAddr, opts...)
	if err != nil {
		return nil, err
	}

	return pb.NewOverlayClient(conn), nil
}
