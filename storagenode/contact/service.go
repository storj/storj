// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package contact

import (
	"context"
	"sync"
	"time"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/common/sync2"
	"storj.io/storj/storagenode/trust"
)

var (
	mon = monkit.Package()

	// Error is the default error class for contact package.
	Error = errs.Class("contact")

	errPingSatellite = errs.Class("ping satellite error")
)

const initialBackOff = time.Second

// Config contains configurable values for contact service.
type Config struct {
	ExternalAddress string `user:"true" help:"the public address of the node, useful for nodes behind NAT" default:""`

	// Chore config values
	Interval time.Duration `help:"how frequently the node contact chore should run" releaseDefault:"1h" devDefault:"30s"`
}

// NodeInfo contains information necessary for introducing storagenode to satellite.
type NodeInfo struct {
	ID       storj.NodeID
	Address  string
	Version  pb.NodeVersion
	Capacity pb.NodeCapacity
	Operator pb.NodeOperator
}

// Service is the contact service between storage nodes and satellites.
type Service struct {
	log    *zap.Logger
	dialer rpc.Dialer

	mu   sync.Mutex
	self NodeInfo

	trust *trust.Pool

	initialized sync2.Fence
}

// NewService creates a new contact service.
func NewService(log *zap.Logger, dialer rpc.Dialer, self NodeInfo, trust *trust.Pool) *Service {
	return &Service{
		log:    log,
		dialer: dialer,
		trust:  trust,
		self:   self,
	}
}

// PingSatellites attempts to ping all satellites in trusted list until backoff reaches maxInterval.
func (service *Service) PingSatellites(ctx context.Context, maxInterval time.Duration) (err error) {
	defer mon.Task()(&ctx)(&err)
	satellites := service.trust.GetSatellites(ctx)
	var group errgroup.Group
	for _, satellite := range satellites {
		satellite := satellite
		group.Go(func() error {
			return service.pingSatellite(ctx, satellite, maxInterval)
		})
	}
	return group.Wait()
}

func (service *Service) pingSatellite(ctx context.Context, satellite storj.NodeID, maxInterval time.Duration) error {
	interval := initialBackOff
	attempts := 0
	for {

		mon.Meter("satellite_contact_request").Mark(1) //mon:locked

		err := service.pingSatelliteOnce(ctx, satellite)
		attempts++
		if err == nil {
			return nil
		}
		service.log.Error("ping satellite failed ", zap.Stringer("Satellite ID", satellite), zap.Int("attempts", attempts), zap.Error(err))

		// Sleeps until interval times out, then continue. Returns if context is cancelled.
		if !sync2.Sleep(ctx, interval) {
			service.log.Info("context cancelled", zap.Stringer("Satellite ID", satellite))
			return nil
		}
		interval *= 2
		if interval >= maxInterval {
			service.log.Info("retries timed out for this cycle", zap.Stringer("Satellite ID", satellite))
			return nil
		}
	}

}

func (service *Service) pingSatelliteOnce(ctx context.Context, id storj.NodeID) (err error) {
	defer mon.Task()(&ctx, id)(&err)

	nodeurl, err := service.trust.GetNodeURL(ctx, id)
	if err != nil {
		return errPingSatellite.Wrap(err)
	}

	conn, err := service.dialer.DialNodeURL(ctx, nodeurl)
	if err != nil {
		return errPingSatellite.Wrap(err)
	}
	defer func() { err = errs.Combine(err, conn.Close()) }()

	self := service.Local()
	_, err = pb.NewDRPCNodeClient(conn).CheckIn(ctx, &pb.CheckInRequest{
		Address:  self.Address,
		Version:  &self.Version,
		Capacity: &self.Capacity,
		Operator: &self.Operator,
	})
	if err != nil {
		return errPingSatellite.Wrap(err)
	}
	return nil
}

// Local returns the storagenode info.
func (service *Service) Local() NodeInfo {
	service.mu.Lock()
	defer service.mu.Unlock()
	return service.self
}

// UpdateSelf updates the local node with the capacity.
func (service *Service) UpdateSelf(capacity *pb.NodeCapacity) {
	service.mu.Lock()
	defer service.mu.Unlock()
	if capacity != nil {
		service.self.Capacity = *capacity
	}
	service.initialized.Release()
}
