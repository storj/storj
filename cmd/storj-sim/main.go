// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"os"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
)

// Flags contains different flags for commands
type Flags struct {
	Directory string
	Host      string

	SatelliteCount   int
	StorageNodeCount int
	Identities       int
}

var printCommands bool

func main() {
	cobra.EnableCommandSorting = false

	var flags Flags

	rootCmd := &cobra.Command{
		Use:   "storj-sim",
		Short: "Storj Network Simulator",
	}

	defaultConfigDir := fpath.ApplicationDir("storj", "local-network")
	configDir := defaultConfigDir
	if os.Getenv("STORJ_NETWORK_DIR") != "" {
		configDir = os.Getenv("STORJ_NETWORK_DIR")
	}

	rootCmd.PersistentFlags().StringVarP(&flags.Directory, "config-dir", "", configDir, "base project directory")
	rootCmd.PersistentFlags().StringVarP(&flags.Host, "host", "", "127.0.0.1", "host to use for network")

	rootCmd.PersistentFlags().IntVarP(&flags.SatelliteCount, "satellites", "", 1, "number of satellites to start")
	rootCmd.PersistentFlags().IntVarP(&flags.StorageNodeCount, "storage-nodes", "", 10, "number of storage nodes to start")
	rootCmd.PersistentFlags().IntVarP(&flags.Identities, "identities", "", 10, "number of identities to create")

	rootCmd.PersistentFlags().BoolVarP(&printCommands, "print-commands", "x", false, "print commands as they are run")

	networkCmd := &cobra.Command{
		Use:   "network",
		Short: "local network for testing",
	}

	networkCmd.AddCommand(
		&cobra.Command{
			Use:   "run",
			Short: "run network",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return networkExec(&flags, args, "run")
			},
		}, &cobra.Command{
			Use:   "setup",
			Short: "setup network",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return networkExec(&flags, args, "setup")
			},
		}, &cobra.Command{
			Use:   "test <command>",
			Short: "run command with an actual network",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return networkTest(&flags, args[0], args[1:])
			},
		}, &cobra.Command{
			Use:   "destroy",
			Short: "destroys network if it exists",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return networkDestroy(&flags, args)
			},
		},
	)

	inmemoryCmd := &cobra.Command{
		Use:   "inmemory",
		Short: "in-memory single process network",
	}

	inmemoryCmd.AddCommand(
		&cobra.Command{
			Use:   "run",
			Short: "run an in-memory network",
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return inmemoryRun(&flags)
			},
		},
		&cobra.Command{
			Use:   "test <command>",
			Short: "run command with an in-memory network",
			Args:  cobra.MinimumNArgs(1),
			RunE: func(cmd *cobra.Command, args []string) (err error) {
				return inmemoryTest(&flags, args[0], args[1:])
			},
		},
	)

	rootCmd.AddCommand(
		networkCmd,
		inmemoryCmd,
	)

	rootCmd.SilenceUsage = true
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}
