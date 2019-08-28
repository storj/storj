// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/sync2"
)

var (
	rootCmd = &cobra.Command{
		Use:   "auto-updater",
		Short: "auto-updater for storage node",
	}
	runCmd = &cobra.Command{
		Use:   "run",
		Short: "Run the auto updater for storage node",
		RunE:  cmdRun,
	}

	interval   string
	versionURL string
	binaryPath string
)

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringVar(&interval, "interval", "05s", "interval for checking the new version")
	runCmd.Flags().StringVar(&versionURL, "version-url", "https://version.storj.io/", "version server URL")
	runCmd.Flags().StringVar(&binaryPath, "binary-path", "", "the storage node installation location")
}

func cmdRun(cmd *cobra.Command, args []string) (err error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-c

		signal.Stop(c)
		cancel()
	}()

	loopInterval, err := time.ParseDuration(interval)
	if err != nil {
		return fmt.Errorf("unable to parse interval parameter: %v", err)
	}

	loop := sync2.NewCycle(loopInterval)
	err = loop.Run(ctx, func(ctx context.Context) (err error) {
		fmt.Println("check new version")
		return nil
	})
	if err != context.Canceled {
		return err
	}
	return nil
}

func main() {
	rootCmd.Execute()
}
