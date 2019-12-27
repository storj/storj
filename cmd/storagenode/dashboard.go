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

	"storj.io/common/memory"
	"storj.io/common/pb"
	"storj.io/common/rpc"
	"storj.io/storj/pkg/process"
	"storj.io/storj/private/version"
)

const contactWindow = time.Hour * 2

type dashboardClient struct {
	conn *rpc.Conn
}

func dialDashboardClient(ctx context.Context, address string) (*dashboardClient, error) {
	conn, err := rpc.NewDefaultDialer(nil).DialAddressUnencrypted(ctx, address)
	if err != nil {
		return &dashboardClient{}, err
	}
	return &dashboardClient{conn: conn}, nil
}

func (dash *dashboardClient) dashboard(ctx context.Context) (*pb.DashboardResponse, error) {
	return pb.NewDRPCPieceStoreInspectorClient(dash.conn.Raw()).Dashboard(ctx, &pb.DashboardRequest{})
}

func (dash *dashboardClient) close() error {
	return dash.conn.Close()
}

func cmdDashboard(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := process.Ctx(cmd)

	ident, err := runCfg.Identity.Load()
	if err != nil {
		zap.S().Fatal(err)
	} else {
		zap.S().Info("Node ID: ", ident.ID)
	}

	client, err := dialDashboardClient(ctx, dashboardCfg.Address)
	if err != nil {
		return err
	}
	defer func() {
		if err := client.close(); err != nil {
			zap.S().Debug("closing dashboard client failed", err)
		}
	}()

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
	var warnFlag bool
	color.NoColor = !useColor

	heading := color.New(color.FgGreen, color.Bold)
	_, _ = heading.Printf("\nStorage Node Dashboard ( Node Version: %s )\n", version.Build.Version.String())
	_, _ = heading.Printf("\n======================\n\n")

	w := tabwriter.NewWriter(color.Output, 0, 0, 1, ' ', 0)
	fmt.Fprintf(w, "ID\t%s\n", color.YellowString(data.NodeId.String()))

	if data.LastPinged.IsZero() || time.Since(data.LastPinged) >= contactWindow {
		fmt.Fprintf(w, "Last Contact\t%s\n", color.RedString("OFFLINE"))
	} else {
		fmt.Fprintf(w, "Last Contact\t%s\n", color.GreenString("ONLINE"))
	}

	// TODO: use stdtime in protobuf
	uptime, err := ptypes.Duration(data.GetUptime())
	if err == nil {
		fmt.Fprintf(w, "Uptime\t%s\n", color.YellowString(uptime.Truncate(time.Second).String()))
	}

	if err = w.Flush(); err != nil {
		return err
	}

	stats := data.GetStats()
	if stats != nil {
		availBW := memory.Size(stats.GetAvailableBandwidth())
		usedBandwidth := color.WhiteString(memory.Size(stats.GetUsedBandwidth()).Base10String())
		if availBW < 0 {
			warnFlag = true
			availBW = 0
		}
		availableBandwidth := color.WhiteString((availBW).Base10String())
		availableSpace := color.WhiteString(memory.Size(stats.GetAvailableSpace()).Base10String())
		usedSpace := color.WhiteString(memory.Size(stats.GetUsedSpace()).Base10String())
		usedEgress := color.WhiteString(memory.Size(stats.GetUsedEgress()).Base10String())
		usedIngress := color.WhiteString(memory.Size(stats.GetUsedIngress()).Base10String())

		w = tabwriter.NewWriter(color.Output, 0, 0, 5, ' ', tabwriter.AlignRight)
		fmt.Fprintf(w, "\n\t%s\t%s\t%s\t%s\t\n", color.GreenString("Available"), color.GreenString("Used"), color.GreenString("Egress"), color.GreenString("Ingress"))
		fmt.Fprintf(w, "Bandwidth\t%s\t%s\t%s\t%s\t (since %s 1)\n", availableBandwidth, usedBandwidth, usedEgress, usedIngress, time.Now().Format("Jan"))
		fmt.Fprintf(w, "Disk\t%s\t%s\t\n", availableSpace, usedSpace)
		if err = w.Flush(); err != nil {
			return err
		}

	} else {
		color.Yellow("Loading...\n")
	}

	w = tabwriter.NewWriter(color.Output, 0, 0, 1, ' ', 0)
	// TODO: Get addresses from server data
	fmt.Fprintf(w, "Internal\t%s\n", color.WhiteString(dashboardCfg.Address))
	fmt.Fprintf(w, "External\t%s\n", color.WhiteString(data.GetExternalAddress()))
	// Disabling the Link to the Dashboard as its not working yet
	// fmt.Fprintf(w, "Dashboard\t%s\n", color.WhiteString(data.GetDashboardAddress()))
	if err = w.Flush(); err != nil {
		return err
	}

	if warnFlag {
		fmt.Fprintf(w, "\nWARNING!!!!! %s\n", color.WhiteString("Increase your bandwidth"))
	}

	return nil
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
