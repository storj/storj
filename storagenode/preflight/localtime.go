// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package preflight

import (
	"context"
	"math"
	"time"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/storagenode/trust"
)

// ErrClockOutOfSyncMinor is the error class for system clock is off by more than 10m.
var ErrClockOutOfSyncMinor = errs.Class("system clock is off")

// ErrClockOutOfSyncMajor is the error class for system clock is out of sync by more than 30m.
var ErrClockOutOfSyncMajor = errs.Class("system clock is out of sync")

// LocalTime checks local system clock against all trusted satellites.
type LocalTime struct {
	log    *zap.Logger
	config Config
	trust  *trust.Pool
	dialer rpc.Dialer
}

// NewLocalTime creates a new localtime instance.
func NewLocalTime(log *zap.Logger, config Config, trust *trust.Pool, dialer rpc.Dialer) *LocalTime {
	return &LocalTime{
		log:    log,
		config: config,
		trust:  trust,
		dialer: dialer,
	}
}

// Check compares local system clock with all trusted satellites' system clock.
// it returns an error when local system clock is out of sync by more than 24h with all trusted satellites' clock.
func (localTime *LocalTime) Check(ctx context.Context) (err error) {
	defer mon.Task()(&ctx)(&err)
	if !localTime.config.LocalTimeCheck {
		localTime.log.Debug("local system clock check is not enabled")
		return nil
	}

	localTime.log.Info("start checking local system clock with trusted satellites' system clock.")

	group, ctx := errgroup.WithContext(ctx)

	// get trusted satellites
	satellites := localTime.trust.GetSatellites(ctx)
	results := make([]error, len(satellites))
	for i, satellite := range satellites {
		i := i
		satellite := satellite
		group.Go(func() error {
			// get a current timestamp
			currentLocalTime := time.Now().UTC()
			satelliteTime, err := localTime.getSatelliteTime(ctx, satellite)
			if err != nil {
				localTime.log.Error("unable to get satellite system time", zap.Stringer("Satellite ID", satellite), zap.Error(err))
				results[i] = ErrClockOutOfSyncMajor.Wrap(err)
				return nil
			}

			err = localTime.checkSatelliteTime(ctx, satelliteTime.GetTimestamp(), currentLocalTime)
			if err != nil {
				localTime.log.Error("system clock is out of sync with satellite", zap.Stringer("Satellite ID", satellite), zap.Error(err))
				if ErrClockOutOfSyncMinor.Has(err) {
					return nil
				}
				results[i] = err
			}

			return nil
		})
	}

	_ = group.Wait()

	errsCounter := 0
	for _, result := range results {
		if ErrClockOutOfSyncMajor.Has(result) {
			errsCounter++
		}
	}
	if errsCounter == len(satellites) {
		return ErrClockOutOfSyncMajor.New("system clock is out of sync with all trusted satellites")
	}

	localTime.log.Info("local system clock is in sync with trusted satellites' system clock.")
	return nil
}

func (localTime *LocalTime) getSatelliteTime(ctx context.Context, satelliteID storj.NodeID) (_ *pb.GetTimeResponse, err error) {
	defer mon.Task()(&ctx)(&err)

	nodeurl, err := localTime.trust.GetNodeURL(ctx, satelliteID)
	if err != nil {
		return nil, err
	}
	conn, err := localTime.dialer.DialNodeURL(ctx, nodeurl)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errs.Combine(err, conn.Close())
	}()

	resp, err := pb.NewDRPCNodeClient(conn).GetTime(ctx, &pb.GetTimeRequest{})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (localTime *LocalTime) checkSatelliteTime(ctx context.Context, satelliteTime time.Time, systemTime time.Time) (err error) {
	defer mon.Task()(&ctx)(&err)

	diff := math.Abs(satelliteTime.Sub(systemTime).Minutes())
	// check to see if the timestamp received from satellites are off by more than 30m
	if diff > 30 {
		return ErrClockOutOfSyncMajor.New("clock off by %f minutes", diff)
	}
	// check to see if the timestamp received from satellites are off by more than 10m
	if diff > 10 {
		return ErrClockOutOfSyncMinor.New("clock off by %f minutes", diff)
	}

	return nil
}
