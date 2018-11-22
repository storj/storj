// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package distributor

import (
	"context"

	"go.uber.org/zap"

	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/pointerdb"
	"storj.io/storj/pkg/provider"
)

type Config struct {
}

func (c Config) Run(ctx context.Context, server *provider.Provider) error {
	overlay := overlay.LoadServerFromContext(ctx)
	pdb := pointerdb.LoadFromContext(ctx)

	pb.RegisterDistributorServer(server.GRPC(), NewDistributorServer(overlay, pdb, zap.L(), server.Identity()))

	return server.Run(ctx)

}
