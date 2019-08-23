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
// Once V3-2363 and V3-2364 are implemented, Service2 will replace the existing Service struct.
type ReservoirService struct {
	log *zap.Logger

	nodesToSelect  int
	PathsToAudit   []storj.Path
	reservoirSlots int
	Reservoirs     map[storj.NodeID]*Reservoir
	rand           *rand.Rand

	MetainfoLoop *metainfo.Loop
	Loop         sync2.Cycle
}

// NewReservoirService instantiates Service2
func NewReservoirService(log *zap.Logger, metaLoop *metainfo.Loop, config Config) (*ReservoirService, error) {
	return &ReservoirService{
		log: log,

		nodesToSelect:  config.NodesToSelect,
		reservoirSlots: config.Slots,
		rand:           rand.New(rand.NewSource(time.Now().Unix())),

		MetainfoLoop: metaLoop,
		Loop:         *sync2.NewCycle(config.Interval),
	}, nil
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

// Select randomly selects segments to audit
func (service *ReservoirService) Select(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)

	return service.Loop.Run(ctx, func(ctx context.Context) (err error) {
		defer mon.Task()(&ctx)(&err)

		service.PathsToAudit = []storj.Path{}
		if service.Reservoirs != nil {
			// todo: is it okay that pathsToAudit could end up being less than nodesToSelect?
			for i := 0; i < service.nodesToSelect; i++ {
				randomReservoir, err := service.GetRandomReservoir()
				if err != nil {
					return err
				}
				if randomReservoir == nil {
					continue
				}
				randomPath := GetRandomPath(randomReservoir)
				service.PathsToAudit = append(service.PathsToAudit, randomPath)
			}
		}
		return nil
	})
}

// GetRandomReservoir returns a random reservoir
func (service *ReservoirService) GetRandomReservoir() (reservoir *Reservoir, err error) {
	var src cryptoSource
	rnd := rand.New(src)

	if len(service.Reservoirs) == 0 {
		return nil, Error.New("no reservoirs available")
	}

	randomIndex := rnd.Intn(len(service.Reservoirs))
	for nodeID := range service.Reservoirs {
		if randomIndex == 0 {
			return service.Reservoirs[nodeID], nil
		}
		randomIndex--
	}
	return nil, nil
}

// GetRandomPath returns a random path
func GetRandomPath(reservoir *Reservoir) (path storj.Path) {
	var src cryptoSource
	rnd := rand.New(src)

	randomIndex := rnd.Intn(len(reservoir.Paths))
	return reservoir.Paths[randomIndex]
}
