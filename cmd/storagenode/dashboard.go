// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	"github.com/golang/protobuf/ptypes"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"storj.io/storj/internal/memory"
	"storj.io/storj/internal/version"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/process"
	"storj.io/storj/pkg/transport"
)

const contactWindow = time.Minute * 10

type dashboardClient struct {
	client pb.PieceStoreInspectorClient
}

func (dash *dashboardClient) dashboard(ctx context.Context) (*pb.DashboardResponse, error) {
	return dash.client.Dashboard(ctx, &pb.DashboardRequest{})
}

func newDashboardClient(ctx context.Context, address string) (*dashboardClient, error) {
	conn, err := transport.DialAddressInsecure(ctx, address)
	if err != nil {
		return &dashboardClient{}, err
	}

	return &dashboardClient{
		client: pb.NewPieceStoreInspectorClient(conn),
	}, nil
}

func cmdDashboard(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", ident.ID)
	}

	client, err := newDashboardClient(ctx, dashboardCfg.Address)
	if err != nil {
		return err
	}

	for {
		data, err := client.dashboard(ctx)
		if err != nil {
			return err
		}

		if err := printDashboard(data); err != nil {
			return err
		}

		// Refresh the dashboard every 3 seconds
		time.Sleep(3 * time.Second)
	}
}

func printDashboard(data *pb.DashboardResponse) error {
	clearScreen()
	color.NoColor = !useColor

	heading := color.New(color.FgGreen, color.Bold)
	_, _ = heading.Printf("\nStorage Node Dashboard ( Node Version: %s)\n", version.Build.Version.String())
	_, _ = heading.Printf("\n======================\n\n")

	w := tabwriter.NewWriter(color.Output, 0, 0, 1, ' ', 0)
	fmt.Fprintf(w, "ID\t%s\n", color.YellowString(data.NodeId.String()))

	lastContacted, err := ptypes.Timestamp(data.LastPinged)
	if err != nil {
		lastContacted = time.Time{}
	}
	lastQueried, err := ptypes.Timestamp(data.LastQueried)
	if err == nil {
		if lastQueried.After(lastContacted) {
			lastContacted = lastQueried
		}
	}
	switch {
	case lastContacted.IsZero():
		fmt.Fprintf(w, "Last Contact\t%s\n", color.RedString("NEVER"))
	case time.Since(lastContacted) >= contactWindow:
		fmt.Fprintf(w, "Last Contact\t%s\n", color.RedString(fmt.Sprintf("%s ago",
			time.Since(lastContacted).Truncate(time.Second))))
	default:
		fmt.Fprintf(w, "Last Contact\t%s\n", color.GreenString(fmt.Sprintf("%s ago",
			time.Since(lastContacted).Truncate(time.Second))))
	}

	uptime, err := ptypes.Duration(data.GetUptime())
	if err != nil {
		fmt.Fprintf(w, "Uptime\t%s\n", color.RedString(uptime.Truncate(time.Second).String()))
	} else {
		fmt.Fprintf(w, "Uptime\t%s\n", color.YellowString(uptime.Truncate(time.Second).String()))
	}

	if err = w.Flush(); err != nil {
		return err
	}

	stats := data.GetStats()
	if stats != nil {
		availableBandwidth := color.WhiteString(memory.Size(stats.GetAvailableBandwidth()).Base10String())
		usedBandwidth := color.WhiteString(memory.Size(stats.GetUsedBandwidth()).Base10String())
		availableSpace := color.WhiteString(memory.Size(stats.GetAvailableSpace()).Base10String())
		usedSpace := color.WhiteString(memory.Size(stats.GetUsedSpace()).Base10String())
		usedEgress := color.WhiteString(memory.Size(stats.GetUsedEgress()).Base10String())
		usedIngress := color.WhiteString(memory.Size(stats.GetUsedIngress()).Base10String())

		w = tabwriter.NewWriter(color.Output, 0, 0, 5, ' ', tabwriter.AlignRight)
		fmt.Fprintf(w, "\n\t%s\t%s\t%s\t%s\t\n", color.GreenString("Available"), color.GreenString("Used"), color.GreenString("Egress"), color.GreenString("Ingress"))
		fmt.Fprintf(w, "Bandwidth\t%s\t%s\t%s\t%s\t\n", availableBandwidth, usedBandwidth, usedEgress, usedIngress)
		fmt.Fprintf(w, "Disk\t%s\t%s\t\n", availableSpace, usedSpace)
		if err = w.Flush(); err != nil {
			return err
		}

	} else {
		color.Yellow("Loading...\n")
	}

	w = tabwriter.NewWriter(color.Output, 0, 0, 1, ' ', 0)
	// TODO: Get addresses from server data
	fmt.Fprintf(w, "\nBootstrap\t%s\n", color.WhiteString(data.GetBootstrapAddress()))
	fmt.Fprintf(w, "Internal\t%s\n", color.WhiteString(dashboardCfg.Address))
	fmt.Fprintf(w, "External\t%s\n", color.WhiteString(data.GetExternalAddress()))
	fmt.Fprintf(w, "\nNeighborhood Size %+v\n", whiteInt(data.GetNodeConnections()))
	if err = w.Flush(); err != nil {
		return err
	}

	return nil
}

func whiteInt(value int64) string {
	return color.WhiteString(fmt.Sprintf("%+v", value))
}

// clearScreen clears the screen so it can be redrawn
func clearScreen() {
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout = os.Stdout
		_ = cmd.Run()
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		_ = cmd.Run()
	default:
		fmt.Print(strings.Repeat("\n", 100))
	}
}
