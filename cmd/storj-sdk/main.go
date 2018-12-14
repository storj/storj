// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"os"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/utils"
)

// Flags contains different flags for commands
type Flags struct {
	Directory string

	SatelliteCount   int
	StorageNodeCount int
	Identities       int
}

func main() {
	cobra.EnableCommandSorting = false

	var flags Flags

	rootCmd := &cobra.Command{
		Use:   "storj-local-network",
		Short: "Storj Local Network",
	}

	rootCmd.PersistentFlags().StringVarP(&flags.Directory, "dir", "", fpath.ApplicationDir("storj", "local-network"), "base project directory")

	rootCmd.PersistentFlags().IntVarP(&flags.SatelliteCount, "satellites", "", 1, "number of satellites to start")
	rootCmd.PersistentFlags().IntVarP(&flags.StorageNodeCount, "storage-nodes", "", 10, "number of storage nodes to start")
	rootCmd.PersistentFlags().IntVarP(&flags.Identities, "identities", "", 10, "number of identities to create")

	networkCmd := &cobra.Command{
		Use:   "network",
		Short: "local network for testing",
	}

	networkCmd.AddCommand(
		&cobra.Command{
			Use:   "run",
			Short: "run peers",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return runProcesses(&flags, args, "run")
			},
		}, &cobra.Command{
			Use:   "setup",
			Short: "setup peers",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return runProcesses(&flags, args, "setup")
			},
		},
	)

	testCmd := &cobra.Command{
		Use:   "test <command>",
		Short: "run command with a in-memory network",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) (err error) {
			return runTestPlanet(&flags, args[0], args[1:])
		},
	}

	rootCmd.AddCommand(
		networkCmd,
		testCmd,
	)

	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func runProcesses(flags *Flags, args []string, command string) error {
	processes, err := NewProcesses(flags.Directory, flags.SatelliteCount, flags.StorageNodeCount)
	if err != nil {
		return err
	}

	ctx, cancel := NewCLIContext(context.Background())
	defer cancel()

	err = processes.Exec(ctx, command)
	closeErr := processes.Close()

	return utils.CombineErrors(err, closeErr)
}
