// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"time"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
	"storj.io/storj/pkg/transport"
	"storj.io/storj/satellite/orders"
)

// Config contains configurable values for repairer
type Config struct {
	MaxRepair    int           `help:"maximum segments that can be repaired concurrently" default:"100"`
	Interval     time.Duration `help:"how frequently checker should audit segments" default:"1h0m0s"`
	Timeout      time.Duration `help:"time limit for uploading repaired pieces to new storage nodes" default:"1m0s"`
	MaxBufferMem memory.Size   `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4M"`
}

// GetSegmentRepairer creates a new segment repairer from storeConfig values
func (c Config) GetSegmentRepairer(ctx context.Context, tc transport.Client, pointerdb *pointerdb.Service, orders *orders.Service, cache *overlay.Cache, identity *identity.FullIdentity) (ss SegmentRepairer, err error) {
	defer mon.Task()(&ctx)(&err)

	ec := ecclient.NewClient(tc, c.MaxBufferMem.Int())

	return segments.NewSegmentRepairer(pointerdb, orders, cache, ec, identity, c.Timeout), nil
}
