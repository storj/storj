// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc"
	"storj.io/private/process"
	"storj.io/storj/storagenode/internalpb"
)

type plannedDowntimeClient struct {
	conn *rpc.Conn
}

func dialPlannedDowntimeClient(ctx context.Context, address string) (*plannedDowntimeClient, error) {
	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &plannedDowntimeClient{conn: conn}, nil
}

func (client *plannedDowntimeClient) add(ctx context.Context, start time.Time, durationHours int32) (*internalpb.AddResponse, error) {
	return internalpb.NewDRPCNodePlannedDowntimeClient(client.conn).Add(ctx, &internalpb.AddRequest{
		Start:         start,
		DurationHours: durationHours,
	})
}

/*
func (client *plannedDowntimeClient) getScheduled(ctx context.Context) (*internalpb.GetScheduledResponse, error) {
	return internalpb.NewDRPCNodePlannedDowntimeClient(client.conn).GetScheduled(ctx, &internalpb.GetScheduledRequest{})
}

func (client *plannedDowntimeClient) getCompleted(ctx context.Context) (*internalpb.GetCompletedResponse, error) {
	return internalpb.NewDRPCNodePlannedDowntimeClient(client.conn).GetCompleted(ctx, &internalpb.GetCompletedRequest{})
}

func (client *plannedDowntimeClient) delete(ctx context.Context, id []byte) (*internalpb.DeleteResponse, error) {
	return internalpb.NewDRPCNodePlannedDowntimeClient(client.conn).Delete(ctx, &internalpb.DeleteRequest{
		Id: id,
	})
}
*/

func (client *plannedDowntimeClient) close() error {
	return client.conn.Close()
}

func cmdAddPlannedDowntime(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.L().Fatal("Failed to load identity.", zap.Error(err))
	} else {
		zap.L().Info("Identity loaded.", zap.Stringer("Node ID", ident.ID))
	}

	// TODO prompt for time
	start := time.Now().Add(5 * time.Hour)
	// TODO prompt for number of hours
	durationHours := int32(24)

	client, err := dialPlannedDowntimeClient(ctx, diagCfg.Server.PrivateAddress)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if err := client.close(); err != nil {
			zap.L().Debug("Closing planned downtime client failed.", zap.Error(err))
		}
	}()

	_, err = client.add(ctx, start, durationHours)
	if err != nil {
		fmt.Println("Can't add planned downtime.")
		return errs.Wrap(err)
	}

	fmt.Println("Successfully added planned downtime.")

	return nil
}
