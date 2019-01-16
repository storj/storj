// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package repairer

import (
	"context"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/pkg/datarepair/queue"
	"storj.io/storj/pkg/overlay"
	"storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
	ecclient "storj.io/storj/pkg/storage/ec"
	"storj.io/storj/pkg/storage/segments"
)

// Config contains configurable values for repairer
type Config struct {
	MaxRepair     int           `help:"maximum segments that can be repaired concurrently" default:"100"`
	Interval      time.Duration `help:"how frequently checker should audit segments" default:"3600s"`
	OverlayAddr   string        `help:"Address to contact overlay server through"`
	PointerDBAddr string        `help:"Address to contact pointerdb server through"`
	MaxBufferMem  memory.Size   `help:"maximum buffer memory (in bytes) to be allocated for read buffers" default:"4M"`
	APIKey        string        `help:"repairer-specific pointerdb access credential"`
}

// Run runs the repair service with configured values
func (c Config) Run(ctx context.Context, server *provider.Provider) (err error) {
	q, ok := ctx.Value("masterdb").(interface {
		RepairQueue() queue.RepairQueue
	})
	if !ok {
		return Error.New("unable to get master db instance")
	}

	repairer, err := c.getSegmentRepairer(ctx, server.Identity())
	if err != nil {
		return Error.Wrap(err)
	}

	service := newService(q.RepairQueue(), repairer, c.Interval, c.MaxRepair)

	ctx, cancel := context.WithCancel(ctx)

	// TODO(coyle): we need to figure out how to propagate the error up to cancel the service
	go func() {
		if err := service.Run(ctx); err != nil {
			defer cancel()
			zap.L().Debug("Repair service is shutting down", zap.Error(err))
		}
	}()

	return server.Run(ctx)
}

// getSegmentRepairer creates a new segment repairer from storeConfig values
func (c Config) getSegmentRepairer(ctx context.Context, identity *provider.FullIdentity) (ss SegmentRepairer, err error) {
	defer mon.Task()(&ctx)(&err)

	var oc overlay.Client
	oc, err = overlay.NewClient(identity, c.OverlayAddr)
	if err != nil {
		return nil, err
	}

	pdb, err := pdbclient.NewClient(identity, c.PointerDBAddr, c.APIKey)
	if err != nil {
		return nil, err
	}

	ec := ecclient.NewClient(identity, c.MaxBufferMem.Int())

	return segments.NewSegmentRepairer(oc, ec, pdb), nil
}
