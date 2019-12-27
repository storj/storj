// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/segmentio/go-prompt"
	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/pkg/process"
)

type gracefulExitClient struct {
	conn *rpc.Conn
}

func dialGracefulExitClient(ctx context.Context, address string) (*gracefulExitClient, error) {
	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &gracefulExitClient{conn: conn}, nil
}

func (client *gracefulExitClient) getNonExitingSatellites(ctx context.Context) (*pb.GetNonExitingSatellitesResponse, error) {
	return pb.NewDRPCNodeGracefulExitClient(client.conn.Raw()).GetNonExitingSatellites(ctx, &pb.GetNonExitingSatellitesRequest{})
}

func (client *gracefulExitClient) initGracefulExit(ctx context.Context, req *pb.InitiateGracefulExitRequest) (*pb.ExitProgress, error) {
	return pb.NewDRPCNodeGracefulExitClient(client.conn.Raw()).InitiateGracefulExit(ctx, req)
}

func (client *gracefulExitClient) getExitProgress(ctx context.Context) (*pb.GetExitProgressResponse, error) {
	return pb.NewDRPCNodeGracefulExitClient(client.conn.Raw()).GetExitProgress(ctx, &pb.GetExitProgressRequest{})
}

func (client *gracefulExitClient) close() error {
	return client.conn.Close()
}

func cmdGracefulExitInit(cmd *cobra.Command, args []string) error {
	ctx, _ := process.Ctx(cmd)

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", ident.ID)
	}

	// display warning message
	if !prompt.Confirm("Please be aware that by starting a graceful exit from a satellite, you will no longer be allowed to participate in repairs or uploads from that satellite. This action can not be undone. Are you sure you want to continue? y/n\n") {
		return nil
	}

	client, err := dialGracefulExitClient(ctx, diagCfg.Server.PrivateAddress)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if err := client.close(); err != nil {
			zap.S().Debug("closing graceful exit client failed", err)
		}
	}()

	// get list of satellites
	satelliteList, err := client.getNonExitingSatellites(ctx)
	if err != nil {
		fmt.Println("Can't find any non-exiting satellites.")
		return errs.Wrap(err)
	}

	if len(satelliteList.GetSatellites()) < 1 {
		fmt.Println("Can't find any non-exiting satellites.")
		return nil
	}

	// display satellite options
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "Domain Name\tNode ID\tSpace Used\t")

	for _, satellite := range satelliteList.GetSatellites() {
		fmt.Fprintf(w, "%s\t%s\t%s\t\n", satellite.GetDomainName(), satellite.NodeId.String(), memory.Size(satellite.GetSpaceUsed()).Base10String())
	}
	fmt.Fprintln(w, "Please enter a space delimited list of satellite domain names you would like to gracefully exit. Press enter to continue:")

	var selectedSatellite []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		// parse selected satellite from user input
		inputs := strings.Split(input, " ")
		selectedSatellite = append(selectedSatellite, inputs...)
		break
	}
	if err != scanner.Err() || err != nil {
		return errs.Wrap(err)
	}

	// validate user input
	satelliteIDs := make([]storj.NodeID, 0, len(satelliteList.GetSatellites()))
	for _, selected := range selectedSatellite {
		for _, satellite := range satelliteList.GetSatellites() {
			if satellite.GetDomainName() == selected {
				satelliteIDs = append(satelliteIDs, satellite.NodeId)
			}
		}
	}

	if len(satelliteIDs) < 1 {
		fmt.Println("Invalid input. Please use valid satellite domian names.")
		return errs.New("Invalid satellite domain names")
	}

	// save satellites for graceful exit into the db
	progresses := make([]*pb.ExitProgress, 0, len(satelliteIDs))
	var errgroup errs.Group
	for _, id := range satelliteIDs {
		req := &pb.InitiateGracefulExitRequest{
			NodeId: id,
		}
		resp, err := client.initGracefulExit(ctx, req)
		if err != nil {
			zap.S().Debug("initializing graceful exit failed", zap.Stringer("Satellite ID", id), zap.Error(err))
			errgroup.Add(err)
			continue
		}
		progresses = append(progresses, resp)
	}

	if len(progresses) < 1 {
		fmt.Println("Failed to initialize graceful exit. Please try again later.")
		return errgroup.Err()
	}

	displayExitProgress(w, progresses)

	err = w.Flush()
	if err != nil {
		return errs.Wrap(err)
	}

	return nil
}

func cmdGracefulExitStatus(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", ident.ID)
	}

	client, err := dialGracefulExitClient(ctx, diagCfg.Server.PrivateAddress)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if err := client.close(); err != nil {
			zap.S().Debug("closing graceful exit client failed", err)
		}
	}()

	// call get status to get status for all satellites' that are in exiting
	progresses, err := client.getExitProgress(ctx)
	if err != nil {
		return errs.Wrap(err)
	}

	if len(progresses.GetProgress()) < 1 {
		fmt.Println("No graceful exit in progress.")
		return nil
	}

	// display exit progress
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer func() {

		err = w.Flush()
		if err != nil {
			err = errs.Wrap(err)
		}

	}()

	displayExitProgress(w, progresses.GetProgress())
	return nil
}

func displayExitProgress(w io.Writer, progresses []*pb.ExitProgress) {
	fmt.Fprintln(w, "\nDomain Name\tNode ID\tPercent Complete\tSuccessful\tCompletion Receipt")

	for _, progress := range progresses {
		isSuccessful := "N"
		receipt := "N/A"
		if progress.Successful {
			isSuccessful = "Y"
		}
		if progress.GetCompletionReceipt() != nil && len(progress.GetCompletionReceipt()) > 0 {
			receipt = fmt.Sprintf("%x", progress.GetCompletionReceipt())
		}

		fmt.Fprintf(w, "%s\t%s\t%.2f%%\t%s\t%s\t\n", progress.GetDomainName(), progress.NodeId.String(), progress.GetPercentComplete(), isSuccessful, receipt)
	}
}
