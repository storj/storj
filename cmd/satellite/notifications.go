// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/rpc"
	"storj.io/storj/pkg/storj"
)

var (
	notificationsCmd = &cobra.Command{
		Use:   "notifications",
		Short: "sends notifications to storagenodes",
	}
	notifyCmd = &cobra.Command{
		Use:   "notify [storage node ID] [scope] [level] [message]",
		Short: "send notification to storagenode",
		Args:  cobra.MinimumNArgs(4),
		RunE:  cmdNotify,
	}
	broadcastCmd = &cobra.Command{
		Use:   "broadcast [scope] [level] [message]",
		Short: "broadcast notification to all reliable storagenodes",
		Args:  cobra.MinimumNArgs(3),
		RunE:  cmdBroadcast,
	}
)

// cmdNotify sends notification to particular storagenode.
func cmdNotify(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	nodeID, err := storj.NodeIDFromString(args[0])
	if err != nil {
		return err
	}

	scope, err := validateScope(args[1])
	if err != nil {
		return err
	}

	level, err := validateLevel(args[2])
	if err != nil {
		return err
	}

	client, closefn, err := newClient(ctx, runCfg.Server.PrivateAddress)
	if err != nil {
		return err
	}

	defer func() { err = errs.Combine(err, closefn()) }()

	req := &pb.NotifyRequest{
		NodeId: nodeID,
		Notification: &pb.Notification{
			Scope:     scope,
			Level:     level,
			Tags:      nil,
			Message:   args[3],
			Timestamp: time.Now(),
		},
	}

	if _, err = client.Notify(ctx, req); err != nil {
		return err
	}

	_, _ = fmt.Printf("Succesfully sent notification to %s\n", nodeID)

	return nil
}

// cmdBroadcast broadcast notification to all reliable storagenodes.
func cmdBroadcast(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	scope, err := validateScope(args[0])
	if err != nil {
		return err
	}

	level, err := validateLevel(args[1])
	if err != nil {
		return err
	}

	client, closefn, err := newClient(ctx, runCfg.Server.PrivateAddress)
	if err != nil {
		return err
	}

	defer func() { err = errs.Combine(err, closefn()) }()

	notification := &pb.Notification{
		Scope:     scope,
		Level:     level,
		Tags:      nil,
		Message:   args[2],
		Timestamp: time.Now(),
	}

	resp, err := client.Broadcast(ctx, notification)
	if err != nil {
		return err
	}

	_, _ = fmt.Printf("Succesfully broadcasted notification to %d nodes\n", resp.SuccessCount)
	_, _ = fmt.Printf("Offline nodes %d\n", len(resp.Offline))
	for _, offline := range resp.Offline {
		_, _ = fmt.Printf("- %s\n", offline)
	}
	_, _ = fmt.Printf("Failed send to %d nodes\n", len(resp.Failed))
	for _, failed := range resp.Failed {
		_, _ = fmt.Printf("- %s\n", failed)
	}

	return nil
}

// newClient dial address and return new notifications client for address.
func newClient(ctx context.Context, address string) (rpc.NotificationsClient, func() error, error) {
	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
	if err != nil {
		return nil, nil, err
	}

	return conn.NotificationsClient(), conn.Close, nil
}

// validateScope parses string scope and returns notification scope or error.
func validateScope(scope string) (pb.Notification_Scope, error) {
	switch scope {
	case "custom":
		return pb.Notification_CUSTOM, nil
	case "audit":
		return pb.Notification_AUDIT, nil
	case "uptime":
		return pb.Notification_UPTIME, nil
	case "repair":
		return pb.Notification_REPAIR, nil
	case "disqualification":
		return pb.Notification_DISQUALIFICATION, nil
	case "graceful-exit":
		return pb.Notification_GRACEFUL_EXIT, nil
	case "vetting":
		return pb.Notification_VETTING, nil
	default:
		return 0, errs.New("invalid notification scope")
	}
}

// validateLevel parses string level and returns notification level or error.
func validateLevel(level string) (pb.Notification_Level, error) {
	switch level {
	case "info":
		return pb.Notification_INFO, nil
	case "warn":
		return pb.Notification_WARN, nil
	case "error":
		return pb.Notification_ERROR, nil
	default:
		return 0, errs.New("invalid notification level")
	}
}
