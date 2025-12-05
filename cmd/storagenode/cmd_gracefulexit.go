// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/cfgstruct"
	"storj.io/common/memory"
	"storj.io/common/process"
	"storj.io/common/rpc"
	"storj.io/common/storj"
	"storj.io/storj/private/date"
	"storj.io/storj/private/prompt"
	"storj.io/storj/storagenode"
	"storj.io/storj/storagenode/internalpb"
)

type gracefulExitCfg struct {
	storagenode.Config
}

func newGracefulExitInitCmd(f *Factory) *cobra.Command {
	var cfg gracefulExitCfg
	cmd := &cobra.Command{
		Use:   "exit-satellite",
		Short: "Initiate graceful exit",
		Long: "Initiate gracefule exit.\n" +
			"The command shows the list of the available satellites that can be exited " +
			"and ask for choosing one.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdGracefulExitInit(cmd, &cfg)
		},
		Annotations: map[string]string{"type": "helper"},
	}

	process.Bind(cmd, &cfg, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir))

	return cmd
}

func newGracefulExitStatusCmd(f *Factory) *cobra.Command {
	var cfg gracefulExitCfg
	cmd := &cobra.Command{
		Use:   "exit-status",
		Short: "Display graceful exit status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdGracefulExitStatus(cmd, &cfg)
		},
		Annotations: map[string]string{"type": "helper"},
	}

	process.Bind(cmd, &cfg, f.Defaults, cfgstruct.ConfDir(f.ConfDir), cfgstruct.IdentityDir(f.IdentityDir))

	return cmd
}

type gracefulExitClient struct {
	conn *rpc.Conn
}

type unavailableSatellite struct {
	id         storj.NodeID
	monthsLeft int
}

func dialGracefulExitClient(ctx context.Context, address string) (*gracefulExitClient, error) {
	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	return &gracefulExitClient{conn: conn}, nil
}

func (client *gracefulExitClient) getNonExitingSatellites(ctx context.Context) (*internalpb.GetNonExitingSatellitesResponse, error) {
	return internalpb.NewDRPCNodeGracefulExitClient(client.conn).GetNonExitingSatellites(ctx, &internalpb.GetNonExitingSatellitesRequest{})
}

func (client *gracefulExitClient) initGracefulExit(ctx context.Context, req *internalpb.InitiateGracefulExitRequest) (*internalpb.ExitProgress, error) {
	return internalpb.NewDRPCNodeGracefulExitClient(client.conn).InitiateGracefulExit(ctx, req)
}

func (client *gracefulExitClient) getExitProgress(ctx context.Context) (*internalpb.GetExitProgressResponse, error) {
	return internalpb.NewDRPCNodeGracefulExitClient(client.conn).GetExitProgress(ctx, &internalpb.GetExitProgressRequest{})
}

func (client *gracefulExitClient) gracefulExitFeasibility(ctx context.Context, id storj.NodeID) (*internalpb.GracefulExitFeasibilityResponse, error) {
	return internalpb.NewDRPCNodeGracefulExitClient(client.conn).GracefulExitFeasibility(ctx, &internalpb.GracefulExitFeasibilityRequest{NodeId: id})
}

func (client *gracefulExitClient) close() error {
	return client.conn.Close()
}

func cmdGracefulExitInit(cmd *cobra.Command, cfg *gracefulExitCfg) error {
	ctx, _ := process.Ctx(cmd)

	ident, err := cfg.Identity.Load()
	if err != nil {
		zap.L().Fatal("Failed to load identity.", zap.Error(err))
	} else {
		zap.L().Info("Identity loaded.", zap.Stringer("Node ID", ident.ID))
	}

	// display warning message
	confirmed, err := prompt.Confirm("By starting a graceful exit from a satellite, you will no longer receive new uploads from that satellite.\nThis action can not be undone.\nAre you sure you want to continue? [y/n]\n")
	if err != nil {
		return err
	}
	if !confirmed {
		return nil
	}

	client, err := dialGracefulExitClient(ctx, cfg.Server.PrivateAddress)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if err := client.close(); err != nil {
			zap.L().Debug("Closing graceful exit client failed.", zap.Error(err))
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

	_, _ = fmt.Fprintln(w, "Domain Name\tNode ID\tSpace Used\t")

	for _, satellite := range satelliteList.GetSatellites() {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t\n", satellite.GetDomainName(), satellite.NodeId.String(), memory.Size(satellite.GetSpaceUsed()).Base10String())
	}
	_, _ = fmt.Fprintln(w, "Please enter a space delimited list of satellite domain names you would like to gracefully exit. Press enter to continue:")

	var selectedSatellite []string
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := scanner.Text()
		// parse selected satellite from user input
		inputs := strings.Split(input, " ")
		selectedSatellite = append(selectedSatellite, inputs...)
		break
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return errs.Wrap(scanErr)
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

	return gracefulExitInit(ctx, satelliteIDs, w, client)
}

func cmdGracefulExitStatus(cmd *cobra.Command, cfg *gracefulExitCfg) (err error) {
	ctx, _ := process.Ctx(cmd)

	ident, err := cfg.Identity.Load()
	if err != nil {
		zap.L().Fatal("Failed to load identity.", zap.Error(err))
	} else {
		zap.L().Info("Identity loaded.", zap.Stringer("Node ID", ident.ID))
	}

	client, err := dialGracefulExitClient(ctx, cfg.Server.PrivateAddress)
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if err := client.close(); err != nil {
			zap.L().Debug("Closing graceful exit client failed.", zap.Error(err))
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

func displayExitProgress(w io.Writer, progresses []*internalpb.ExitProgress) {
	_, _ = fmt.Fprintln(w, "\nDomain Name\tNode ID\tPercent Complete\tSuccessful\tCompletion Receipt")

	for _, progress := range progresses {
		isSuccessful := "N"
		receipt := "N/A"
		if progress.Successful {
			isSuccessful = "Y"
		}
		if progress.GetCompletionReceipt() != nil && len(progress.GetCompletionReceipt()) > 0 {
			receipt = hex.EncodeToString(progress.GetCompletionReceipt())
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%.2f%%\t%s\t%s\t\n", progress.GetDomainName(), progress.NodeId.String(), progress.GetPercentComplete(), isSuccessful, receipt)
	}
}

func gracefulExitInit(ctx context.Context, satelliteIDs []storj.NodeID, w *tabwriter.Writer, client *gracefulExitClient) (err error) {
	if len(satelliteIDs) < 1 {
		fmt.Println("Invalid input. Please use valid satellite domian names.")
		return errs.New("Invalid satellite domain names")
	}

	var satellites []unavailableSatellite
	for i := 0; i < len(satelliteIDs); i++ {
		response, err := client.gracefulExitFeasibility(ctx, satelliteIDs[i])
		if err != nil {
			return err
		}
		if !response.IsAllowed {
			left := int(response.MonthsRequired) - date.MonthsCountSince(response.JoinedAt)
			satellites = append(satellites, unavailableSatellite{id: satelliteIDs[i], monthsLeft: left})
		}
	}

	if satellites != nil {
		fmt.Println("You are not allowed to initiate graceful exit on satellite for next amount of months:")
		for _, satellite := range satellites {
			_, _ = fmt.Fprintf(w, "%s\t%d\n", satellite.id.String(), satellite.monthsLeft)
		}
		return errs.New("You are not allowed to graceful exit on some of provided satellites")
	}

	// save satellites for graceful exit into the db
	progresses := make([]*internalpb.ExitProgress, 0, len(satelliteIDs))
	var errgroup errs.Group
	for _, id := range satelliteIDs {
		req := &internalpb.InitiateGracefulExitRequest{
			NodeId: id,
		}
		resp, err := client.initGracefulExit(ctx, req)
		if err != nil {
			zap.L().Debug("Initializing graceful exit failed.", zap.Stringer("Satellite ID", id), zap.Error(err))
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
