// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package planneddowntime

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/storj/storagenode/internalpb"
	"storj.io/storj/storagenode/satellites"
)

// Endpoint implements private inspector for planned downtime.
type Endpoint struct {
	internalpb.DRPCNodePlannedDowntimeUnimplementedServer

	log        *zap.Logger
	satellites satellites.DB
	dialer     rpc.Dialer
}

// NewEndpoint creates a new planned downtime endpoint.
func NewEndpoint(log *zap.Logger, satellites satellites.DB, dialer rpc.Dialer) *Endpoint {
	return &Endpoint{
		log:        log,
		satellites: satellites,
		dialer:     dialer,
	}
}

// GetNonExitingSatellites returns a list of satellites that the storagenode has not begun a graceful exit for.
func (e *Endpoint) Add(ctx context.Context, req *internalpb.AddRequest) (_ *internalpb.AddResponse, err error) {
	e.log.Debug("initialize planned downtime: Add")
	// get all trusted satellites
	/*
		trustedSatellites := e.trust.GetSatellites(ctx)

		for _, trusted := range trustedSatellites {
			// get domain name
			saturl, err := e.trust.GetNodeURL(ctx, trusted)
			if err != nil {
				e.log.Error("planned downtime: get satellite address", zap.Stringer("Satellite ID", trusted), zap.Error(err))
				return &internalpb.AddResponse{}, errs.Wrap(err)
			}
			conn, err := worker.dialer.DialNodeURL(ctx, saturl)
			if err != nil {
				e.log.Error("planned downtime: connect to satellite", zap.Stringer("Satellite ID", trusted), zap.Error(err))
				return &internalpb.AddResponse{}, errs.Wrap(err)
			}
			defer func() {
				err = errs.Combine(err, conn.Close())
			}()

			client := pb.NewDRPCSatellitePlannedDowntimeClient(conn)

			c, err := client.Add(ctx)
			if err != nil {
				return &internalpb.AddResponse{}, errs.Wrap(err)
			}
			defer func() { _ = c.CloseSend() }()

		}
	*/
	fmt.Println("hello")
	fmt.Println(req.DurationHours)
	fmt.Println(req.Start)

	return &internalpb.AddResponse{}, nil
}
