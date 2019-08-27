// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package audit

import (
	"context"
	"math/rand"
	"time"

	"go.uber.org/zap"

	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/storj"
	"storj.io/storj/satellite/metainfo"
)

// ReservoirService is a temp name for the service struct during the audit 2.0 refactor.
// Once V3-2363 and V3-2364 are implemented, ReservoirService will replace the existing Service struct.
type ReservoirService struct {
	log *zap.Logger

	reservoirSlots int
	Reservoirs     map[storj.NodeID]*Reservoir
	rand           *rand.Rand

	MetainfoLoop *metainfo.Loop
	Loop         sync2.Cycle
}

// NewReservoirService instantiates ReservoirService
func NewReservoirService(log *zap.Logger, metaLoop *metainfo.Loop, config Config) *ReservoirService {
	return &ReservoirService{
		log: log,

		reservoirSlots: config.Slots,
		rand:           rand.New(rand.NewSource(time.Now().Unix())),

		MetainfoLoop: metaLoop,
		Loop:         *sync2.NewCycle(config.Interval),
	}
}

// Run runs auditing service 2.0
func (service *ReservoirService) Run(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	service.log.Info("audit 2.0 is starting up")

	return service.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)
		pathCollector := NewPathCollector(service.reservoirSlots, service.rand)
		err = service.MetainfoLoop.Join(ctx, pathCollector)
		if err != nil {
			service.log.Error("error joining metainfoloop", zap.Error(err))
			return nil
		}
		service.Reservoirs = pathCollector.Reservoirs
		return nil
	})
}

// Close halts the reservoir service loop
func (service *ReservoirService) Close() error {
	service.Loop.Close()
	return nil
}
