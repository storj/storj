// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package discovery

import (
	"context"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/storj"
)

// GracefulDisconnect is called when a node alerts the network they're
// going offline for a short period of time with intent to come back
func (d *Discovery) GracefulDisconnect(id storj.NodeID) {
}

// ConnFailure implements the Transport Observer interface `ConnFailure` function
func (d *Discovery) ConnFailure(ctx context.Context, node *pb.Node, err error) {
}

// ConnSuccess implements the Transport Observer interface `ConnSuccess` function
func (d *Discovery) ConnSuccess(ctx context.Context, node *pb.Node) {
}
